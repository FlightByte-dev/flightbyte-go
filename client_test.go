package flightbyte

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewClientValidates(t *testing.T) {
	cases := []struct {
		name string
		key  string
		opts []Option
	}{
		{"empty key", "", nil},
		{"blank key", "   ", nil},
		{"relative base URL", "k", []Option{WithBaseURL("/v1")}},
		{"ftp base URL", "k", []Option{WithBaseURL("ftp://api.flightbyte.dev/v1")}},
		{"base URL with a query", "k", []Option{WithBaseURL("https://api.flightbyte.dev/v1?x=1")}},
		{"nil HTTP client", "k", []Option{WithHTTPClient(nil)}},
		{"negative timeout", "k", []Option{WithTimeout(-time.Second)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.key, tc.opts...); err == nil {
				t.Error("NewClient succeeded")
			}
		})
	}
}

func TestNewClientDefaults(t *testing.T) {
	c, err := NewClient("  frk_test  ", WithBaseURL("http://localhost:3000/v1/"))
	if err != nil {
		t.Fatal(err)
	}
	if c.apiKey != "frk_test" {
		t.Errorf("the key is not trimmed: %q", c.apiKey)
	}
	if c.BaseURL() != "http://localhost:3000/v1" {
		t.Errorf("BaseURL = %q", c.BaseURL())
	}
	if c.timeout != DefaultTimeout || c.LastRateLimit() != nil {
		t.Errorf("timeout %v, rate limit %v", c.timeout, c.LastRateLimit())
	}
	d, _ := NewClient("k")
	if d.BaseURL() != DefaultBaseURL {
		t.Errorf("default BaseURL = %q", d.BaseURL())
	}
}

func TestRequestHeaders(t *testing.T) {
	h := newHarness(t, WithUserAgent("my-app/1.2"))
	h.json(200, `{"data":[]}`)
	if _, err := h.client.Countries.List(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	r := h.last()
	if got := r.Header.Get("X-API-Key"); got != "frk_test_key" {
		t.Errorf("X-API-Key = %q", got)
	}
	if got := r.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	if got := r.Header.Get("User-Agent"); got != "my-app/1.2 flightbyte-go/"+Version {
		t.Errorf("User-Agent = %q", got)
	}
	if got := r.Header.Get("Content-Type"); got != "" {
		t.Errorf("a GET sends Content-Type %q", got)
	}
	if r.Path != "/v1/countries" {
		t.Errorf("Path = %q", r.Path)
	}
}

func TestPathSegmentsAreEscaped(t *testing.T) {
	h := newHarness(t)
	h.json(200, `{}`)
	_, _ = h.client.Subscriptions.Get(context.Background(), "sub/../../admin?x=1")
	if got := h.last().Path; got != "/v1/subscriptions/sub%2F..%2F..%2Fadmin%3Fx=1" {
		t.Errorf("Path = %q", got)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.reply = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit-Minute", "60")
		w.Header().Set("X-RateLimit-Remaining-Minute", "59")
		w.Header().Set("X-RateLimit-Reset-Minute", "1788858660")
		w.Header().Set("X-RateLimit-Limit-Month", "120")
		w.Header().Set("X-RateLimit-Remaining-Month", "bad")
		_, _ = io.WriteString(w, `{"data":[]}`)
	}
	h.mu.Unlock()

	if _, err := h.client.Countries.List(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	rl := h.client.LastRateLimit()
	if rl == nil || rl.Minute == nil {
		t.Fatalf("LastRateLimit = %+v", rl)
	}
	if rl.Minute.Limit != 60 || rl.Minute.Remaining != 59 || rl.Minute.String() != "59/60" {
		t.Errorf("Minute = %+v", rl.Minute)
	}
	if !rl.Minute.ResetAt.Equal(time.Unix(1788858660, 0)) {
		t.Errorf("ResetAt = %v", rl.Minute.ResetAt)
	}
	if rl.Day != nil || rl.Month != nil {
		t.Errorf("Day %v Month %v, want nil for missing or broken headers", rl.Day, rl.Month)
	}

	// A response without the headers keeps the last known state.
	h.json(200, `{"data":[]}`)
	_, _ = h.client.Countries.List(context.Background(), "")
	if h.client.LastRateLimit() == nil {
		t.Error("a response without quota headers cleared LastRateLimit")
	}

	// The returned value is a copy.
	rl.Minute = nil
	if h.client.LastRateLimit().Minute == nil {
		t.Error("LastRateLimit returns shared state")
	}
}

func TestConcurrentRequests(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.reply = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit-Minute", "60")
		w.Header().Set("X-RateLimit-Remaining-Minute", "1")
		_, _ = io.WriteString(w, `{"data":[]}`)
	}
	h.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.client.Countries.List(context.Background(), ""); err != nil {
				t.Error(err)
			}
			_ = h.client.LastRateLimit()
		}()
	}
	wg.Wait()
	if h.count() != 20 {
		t.Errorf("%d requests, want 20", h.count())
	}
}

func TestTimeout(t *testing.T) {
	h := newHarness(t, WithTimeout(50*time.Millisecond))
	// Registered after the harness, so it runs before the server closes:
	// Close waits for the blocked handler to return.
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h.mu.Lock()
	h.reply = func(http.ResponseWriter, *http.Request) { <-release }
	h.mu.Unlock()

	_, err := h.client.Countries.List(context.Background(), "")
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("err = %v, want a *ConnectionError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to match context.DeadlineExceeded", err)
	}
	if connErr.Method != "GET" || !strings.HasSuffix(connErr.URL, "/v1/countries") {
		t.Errorf("ConnectionError = %+v", connErr)
	}
}

func TestCancelledContext(t *testing.T) {
	h := newHarness(t)
	h.json(200, `{"data":[]}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.client.Countries.List(ctx, "")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRefusedConnection(t *testing.T) {
	c, _ := NewClient("k", WithBaseURL("http://127.0.0.1:1/v1"))
	_, err := c.Countries.List(context.Background(), "")
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Errorf("err = %v, want a *ConnectionError", err)
	}
}

func TestNonJSONSuccess(t *testing.T) {
	h := newHarness(t)
	h.json(200, `<html>maintenance</html>`)
	_, err := h.client.Countries.List(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "decoding the response") {
		t.Errorf("err = %v", err)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Error("a 200 became an *APIError")
	}
}

func TestEmptySuccessBody(t *testing.T) {
	h := newHarness(t)
	h.json(201, ``)
	if _, err := h.client.Subscriptions.Pause(context.Background(), "sub_1"); err != nil {
		t.Errorf("an empty 201: %v", err)
	}
}

func TestRequiredParameters(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	checks := map[string]error{
		"flight without carrier": func() error {
			_, err := h.client.Flights.Get(ctx, FlightParams{FlightNumber: "1", Date: "2026-09-08"})
			return err
		}(),
		"timeline without date": func() error {
			_, err := h.client.Flights.Timeline(ctx, FlightParams{Carrier: "TK", FlightNumber: "1"})
			return err
		}(),
		"create without webhook URL": func() error {
			_, err := h.client.Subscriptions.Create(ctx, CreateSubscriptionParams{Carrier: "TK", FlightNumber: "1", Date: "2026-09-08"})
			return err
		}(),
		"get without id":    func() error { _, err := h.client.Subscriptions.Get(ctx, ""); return err }(),
		"delete without id": h.client.Subscriptions.Delete(ctx, " "),
		"country without code": func() error {
			_, err := h.client.Countries.Get(ctx, "", "")
			return err
		}(),
	}
	for name, err := range checks {
		if err == nil || !strings.Contains(err.Error(), "is required") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if h.count() != 0 {
		t.Errorf("%d requests sent for invalid input", h.count())
	}
}
