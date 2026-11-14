package flightbyte

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Sentinels an [*APIError] matches with [errors.Is], by HTTP status.
var (
	// ErrBadRequest matches a 400: the request was invalid. When a flight
	// number operates several legs that day, [APIError.Legs] lists them.
	ErrBadRequest = errors.New("flightbyte: bad request")
	// ErrUnauthorized matches a 401: the API key is missing, invalid or
	// revoked.
	ErrUnauthorized = errors.New("flightbyte: unauthorized")
	// ErrPaymentRequired matches a 402: no watched flights can be created.
	// [APIError.Code] is "watched_flight_limit" (the plan's cap),
	// "overage_cap_reached" (the account's overage cap) or
	// "credits_exhausted" (prepaid credits used up).
	ErrPaymentRequired = errors.New("flightbyte: payment required")
	// ErrForbidden matches a 403: the account is suspended
	// ("account_suspended") or a token lacks the scope ("insufficient_scope").
	ErrForbidden = errors.New("flightbyte: forbidden")
	// ErrNotFound matches a 404: the flight did not operate, or the
	// subscription does not exist or belongs to another key.
	ErrNotFound = errors.New("flightbyte: not found")
	// ErrRateLimited matches a 429: a minute, day or month quota is used up.
	// [APIError.RetryAfter] says how long to wait.
	ErrRateLimited = errors.New("flightbyte: rate limited")
	// ErrUpstream matches a 502: the upstream flight data provider failed.
	// Safe to retry after a short wait.
	ErrUpstream = errors.New("flightbyte: upstream unavailable")
	// ErrServer matches any 5xx, 502 included.
	ErrServer = errors.New("flightbyte: server error")
)

// APIError is returned when the API answers with a non-2xx status. Its
// fields mirror the API's error body.
//
// Branch on it with [errors.Is] and the sentinels, or read its fields with
// [errors.As]:
//
//	var apiErr *flightbyte.APIError
//	if errors.As(err, &apiErr) && errors.Is(err, flightbyte.ErrRateLimited) {
//		time.Sleep(apiErr.RetryAfter)
//	}
type APIError struct {
	// StatusCode is the HTTP status.
	StatusCode int
	// Messages holds every message the API sent. Validation errors send one
	// per field.
	Messages []string
	// Code is a machine-readable reason, when the API sends one, for example
	// "watched_flight_limit" or "account_suspended".
	Code string
	// RequestID identifies the request; quote it when contacting support.
	RequestID string

	// Legs lists the legs to choose from, on a 400 for a flight number that
	// operates several legs that day. Pass one's Origin or FlightID and try
	// again.
	Legs []AmbiguousLeg
	// Limit is the number of watched flights the period allows, on a 402.
	// Not with "credits_exhausted".
	Limit *int
	// Used is the number of watched flights created this period, on a 402.
	// Not with "credits_exhausted".
	Used *int
	// ResetsAt is when the period, and so the count, resets, on a 402.
	// Not with "credits_exhausted".
	ResetsAt *time.Time
	// CapCents is the account's overage cap in US cents, on a 402 with
	// "overage_cap_reached".
	CapCents *int
	// Balance is the prepaid credits left, on a 402 with "credits_exhausted".
	Balance *int
	// ExpiresAt is when the prepaid credits expire or expired, 12 months
	// after the last top-up, on a 402 with "credits_exhausted". Nil when the
	// API sends none.
	ExpiresAt *time.Time
	// Window is the quota window that ran out, on a 429: "minute", "day" or
	// "month".
	Window string
	// RetryAfter is how long to wait before retrying, on a 429.
	RetryAfter time.Duration

	// Body is the decoded error body; empty when it was not a JSON object.
	Body map[string]any
	// RawBody is the body exactly as received.
	RawBody []byte
	// Header holds the response headers.
	Header http.Header
}

// AmbiguousLeg is one of the legs listed by [APIError.Legs].
type AmbiguousLeg struct {
	// Origin is the origin IATA code.
	Origin string `json:"origin"`
	// Destination is the destination IATA code.
	Destination string `json:"destination"`
	// Carrier is the operating carrier.
	Carrier string `json:"carrier"`
	// ScheduledDepartureLocal is the scheduled local departure, "HH:mm".
	ScheduledDepartureLocal string `json:"scheduledDepartureLocal"`
	// FlightID is the upstream flight id.
	FlightID string `json:"flightId"`
}

// Error describes the status, the API's messages and the request id.
func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "flightbyte: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	if len(e.Messages) > 0 {
		b.WriteString(": ")
		b.WriteString(strings.Join(e.Messages, "; "))
	}
	if e.RequestID != "" {
		fmt.Fprintf(&b, " (request %s)", e.RequestID)
	}
	return b.String()
}

// Is reports whether target is the sentinel for e's status.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrBadRequest:
		return e.StatusCode == http.StatusBadRequest
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrPaymentRequired:
		return e.StatusCode == http.StatusPaymentRequired
	case ErrForbidden:
		return e.StatusCode == http.StatusForbidden
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrRateLimited:
		return e.StatusCode == http.StatusTooManyRequests
	case ErrUpstream:
		return e.StatusCode == http.StatusBadGateway
	case ErrServer:
		return e.StatusCode >= 500 && e.StatusCode <= 599
	}
	return false
}

// ConnectionError is returned when a request got no HTTP response: no
// network, a DNS failure, a refused connection, a timeout or a cancelled
// context. It unwraps to the cause, so errors.Is(err,
// context.DeadlineExceeded) works.
type ConnectionError struct {
	// Method is the HTTP method.
	Method string
	// URL is the request URL.
	URL string
	// Err is the cause.
	Err error
}

// Error describes the failed request and its cause.
func (e *ConnectionError) Error() string {
	return fmt.Sprintf("flightbyte: %s %s: %v", e.Method, e.URL, e.Err)
}

// Unwrap returns the cause.
func (e *ConnectionError) Unwrap() error { return e.Err }

func newAPIError(status int, raw []byte, header http.Header) *APIError {
	e := &APIError{StatusCode: status, RawBody: raw, Header: header, Body: map[string]any{}}

	var body map[string]any
	if json.Unmarshal(raw, &body) == nil && body != nil {
		e.Body = body
	}

	switch message := e.Body["message"].(type) {
	case string:
		e.Messages = []string{message}
	case []any:
		for _, m := range message {
			e.Messages = append(e.Messages, fmt.Sprint(m))
		}
	}
	e.Code = stringField(e.Body, "code")
	e.RequestID = stringField(e.Body, "requestId")
	if e.RequestID == "" {
		e.RequestID = header.Get("X-Request-Id")
	}

	if legs, ok := e.Body["legs"].([]any); ok {
		for _, item := range legs {
			if m, ok := item.(map[string]any); ok {
				e.Legs = append(e.Legs, AmbiguousLeg{
					Origin:                  stringField(m, "origin"),
					Destination:             stringField(m, "destination"),
					Carrier:                 stringField(m, "carrier"),
					ScheduledDepartureLocal: stringField(m, "scheduledDepartureLocal"),
					FlightID:                stringField(m, "flightId"),
				})
			}
		}
	}
	e.Limit = intField(e.Body, "limit")
	e.Used = intField(e.Body, "used")
	e.CapCents = intField(e.Body, "capCents")
	e.Balance = intField(e.Body, "balance")
	e.ResetsAt = timeField(e.Body, "resetsAt")
	e.ExpiresAt = timeField(e.Body, "expiresAt")
	e.Window = stringField(e.Body, "window")
	if seconds := intField(e.Body, "retryAfterSeconds"); seconds != nil {
		e.RetryAfter = time.Duration(*seconds) * time.Second
	} else if seconds, err := strconv.Atoi(header.Get("Retry-After")); err == nil {
		e.RetryAfter = time.Duration(seconds) * time.Second
	}
	return e
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func timeField(m map[string]any, key string) *time.Time {
	if s := stringField(m, key); s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return &t
		}
	}
	return nil
}

func intField(m map[string]any, key string) *int {
	if f, ok := m[key].(float64); ok {
		n := int(f)
		return &n
	}
	return nil
}
