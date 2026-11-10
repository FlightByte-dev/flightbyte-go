package flightbyte

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the production API root.
const DefaultBaseURL = "https://api.flightbyte.dev/v1"

// DefaultTimeout bounds each request, response body included, unless
// [WithTimeout] says otherwise.
const DefaultTimeout = 30 * time.Second

// Client talks to the FlightByte API. Create one with [NewClient] and share
// it: it is safe for concurrent use by multiple goroutines.
//
// Keep API keys on a server you control. A key shipped inside a mobile or web
// app can be extracted by anyone who has the app.
type Client struct {
	// Flights looks flights up.
	Flights *FlightsService
	// Subscriptions watches flights and manages their webhooks.
	Subscriptions *SubscriptionsService
	// Countries lists countries and their flags.
	Countries *CountriesService

	apiKey     string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	userAgent  string

	mu            sync.Mutex
	lastRateLimit *RateLimit
}

// Option configures a [Client].
type Option func(*Client) error

// WithBaseURL points the client at another API root, for example
// "http://localhost:3000/v1".
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		u, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("flightbyte: invalid base URL: %w", err)
		}
		if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("flightbyte: invalid base URL %q: want an http or https URL", baseURL)
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("flightbyte: invalid base URL %q: no query or fragment allowed", baseURL)
		}
		c.baseURL = strings.TrimRight(u.String(), "/")
		return nil
	}
}

// WithHTTPClient sends requests through httpClient, for example one with a
// custom transport for retries, proxies or logging.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		if httpClient == nil {
			return errors.New("flightbyte: nil HTTP client")
		}
		c.httpClient = httpClient
		return nil
	}
}

// WithTimeout bounds each request, response body included. Zero disables the
// limit and leaves it to the context. The default is [DefaultTimeout].
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if timeout < 0 {
			return errors.New("flightbyte: negative timeout")
		}
		c.timeout = timeout
		return nil
	}
}

// WithUserAgent puts userAgent in front of this package's own User-Agent, so
// requests read as, for example, "my-app/1.2 flightbyte-go/0.1.0".
func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		c.userAgent = strings.TrimSpace(userAgent) + " " + c.userAgent
		return nil
	}
}

// NewClient returns a client that authenticates with apiKey.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("flightbyte: the API key is empty")
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{},
		timeout:    DefaultTimeout,
		userAgent:  "flightbyte-go/" + Version,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	c.Flights = &FlightsService{c: c}
	c.Subscriptions = &SubscriptionsService{c: c}
	c.Countries = &CountriesService{c: c}
	return c, nil
}

// BaseURL returns the API root this client talks to.
func (c *Client) BaseURL() string { return c.baseURL }

// LastRateLimit returns the quota state from the latest response that
// reported one, or nil before any did. Keys without limits get none.
func (c *Client) LastRateLimit() *RateLimit {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastRateLimit == nil {
		return nil
	}
	copied := *c.lastRateLimit
	return &copied
}

func (c *Client) setRateLimit(rl *RateLimit) {
	c.mu.Lock()
	c.lastRateLimit = rl
	c.mu.Unlock()
}

// FormatDate formats t as the YYYY-MM-DD the API expects for flight dates.
//
// Only the calendar fields are used, in t's own location, so pass the local
// departure date at the origin airport.
func FormatDate(t time.Time) string { return t.Format(time.DateOnly) }

// Ptr returns a pointer to v. It fills optional fields such as
// [CreateSubscriptionParams.DelayThresholdMinutes]:
//
//	DelayThresholdMinutes: flightbyte.Ptr(15)
func Ptr[T any](v T) *T { return &v }
