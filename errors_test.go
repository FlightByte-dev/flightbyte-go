package flightbyte

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

var sentinels = []error{
	ErrBadRequest, ErrUnauthorized, ErrPaymentRequired, ErrForbidden,
	ErrNotFound, ErrRateLimited, ErrUpstream, ErrServer,
}

func TestStatusSentinels(t *testing.T) {
	cases := []struct {
		status int
		want   []error
	}{
		{400, []error{ErrBadRequest}},
		{401, []error{ErrUnauthorized}},
		{402, []error{ErrPaymentRequired}},
		{403, []error{ErrForbidden}},
		{404, []error{ErrNotFound}},
		{409, nil},
		{422, nil},
		{429, []error{ErrRateLimited}},
		{500, []error{ErrServer}},
		{502, []error{ErrUpstream, ErrServer}},
		{503, []error{ErrServer}},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			h := newHarness(t)
			h.json(tc.status, `{"statusCode":0,"message":"x"}`)
			_, err := h.client.Subscriptions.Get(context.Background(), "sub_1")

			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status {
				t.Fatalf("err = %v, want an *APIError with status %d", err, tc.status)
			}
			for _, sentinel := range sentinels {
				want := false
				for _, w := range tc.want {
					want = want || w == sentinel
				}
				if got := errors.Is(err, sentinel); got != want {
					t.Errorf("errors.Is(err, %v) = %v, want %v", sentinel, got, want)
				}
			}
		})
	}
}

func TestErrorBody(t *testing.T) {
	h := newHarness(t)
	h.json(400, `{"statusCode":400,"error":"Bad Request","message":["carrier must be 2 or 3 characters","date must be YYYY-MM-DD"],
		"path":"/v1/flights/X/1/bad","timestamp":"2026-09-08T09:10:00.000Z","requestId":"req_123"}`)
	_, err := h.client.Flights.Get(context.Background(), FlightParams{Carrier: "X", FlightNumber: "1", Date: "bad"})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal(err)
	}
	if len(apiErr.Messages) != 2 || apiErr.RequestID != "req_123" || apiErr.Body["error"] != "Bad Request" {
		t.Errorf("apiErr = %+v", apiErr)
	}
	want := "flightbyte: 400 Bad Request: carrier must be 2 or 3 characters; date must be YYYY-MM-DD (request req_123)"
	if err.Error() != want {
		t.Errorf("Error() = %q\nwant      %q", err.Error(), want)
	}
}

func TestAmbiguousFlight(t *testing.T) {
	h := newHarness(t)
	h.json(400, `{"statusCode":400,"message":"TK1 operates 2 legs on 2026-09-08; pass origin to choose one.",
		"legs":[{"origin":"IST","destination":"JFK","carrier":"TK","scheduledDepartureLocal":"07:00","flightId":"1"},
		        {"origin":"JFK","destination":"IST","scheduledDepartureLocal":"21:00"}, "junk"]}`)
	_, err := h.client.Flights.Get(context.Background(), FlightParams{Carrier: "TK", FlightNumber: "1", Date: "2026-09-08"})

	var apiErr *APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, ErrBadRequest) {
		t.Fatal(err)
	}
	want := []AmbiguousLeg{
		{Origin: "IST", Destination: "JFK", Carrier: "TK", ScheduledDepartureLocal: "07:00", FlightID: "1"},
		{Origin: "JFK", Destination: "IST", ScheduledDepartureLocal: "21:00"},
	}
	if len(apiErr.Legs) != 2 || apiErr.Legs[0] != want[0] || apiErr.Legs[1] != want[1] {
		t.Errorf("Legs = %+v", apiErr.Legs)
	}
}

func TestPaymentRequired(t *testing.T) {
	h := newHarness(t)
	h.json(402, `{"statusCode":402,"message":"No watched flights left.","code":"overage_cap_reached",
		"limit":10,"used":10,"resetsAt":"2026-10-01T00:00:00.000Z","capCents":500}`)
	_, err := h.client.Subscriptions.Create(context.Background(), CreateSubscriptionParams{
		Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08", WebhookURL: "https://example.com/h",
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, ErrPaymentRequired) {
		t.Fatal(err)
	}
	if apiErr.Code != "overage_cap_reached" || *apiErr.Limit != 10 || *apiErr.Used != 10 || *apiErr.CapCents != 500 {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if apiErr.ResetsAt == nil || !apiErr.ResetsAt.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ResetsAt = %v", apiErr.ResetsAt)
	}
}

func TestPaymentRequiredCreditsExhausted(t *testing.T) {
	h := newHarness(t)
	h.json(402, `{"statusCode":402,"message":"No credits left.","code":"credits_exhausted",
		"balance":0,"expiresAt":"2027-01-01T00:00:00.000Z"}`)
	_, err := h.client.Subscriptions.Create(context.Background(), CreateSubscriptionParams{
		Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08", WebhookURL: "https://example.com/h",
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, ErrPaymentRequired) {
		t.Fatal(err)
	}
	if apiErr.Code != "credits_exhausted" || apiErr.Balance == nil || *apiErr.Balance != 0 {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if apiErr.ExpiresAt == nil || !apiErr.ExpiresAt.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ExpiresAt = %v", apiErr.ExpiresAt)
	}
	if apiErr.Limit != nil || apiErr.Used != nil || apiErr.ResetsAt != nil {
		t.Errorf("Limit, Used, ResetsAt = %v, %v, %v; want nil", apiErr.Limit, apiErr.Used, apiErr.ResetsAt)
	}
}

func TestRateLimited(t *testing.T) {
	h := newHarness(t)
	h.json(429, `{"statusCode":429,"message":"Too many requests.","window":"minute","retryAfterSeconds":12}`)
	_, err := h.client.Countries.List(context.Background(), "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Window != "minute" || apiErr.RetryAfter != 12*time.Second {
		t.Errorf("err = %v %+v", err, apiErr)
	}

	// Without the body field, the Retry-After header counts.
	h.mu.Lock()
	h.reply = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		_, _ = io.WriteString(w, `{"statusCode":429,"message":"Too many requests."}`)
	}
	h.mu.Unlock()
	_, err = h.client.Countries.List(context.Background(), "")
	if !errors.As(err, &apiErr) || apiErr.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v", apiErr.RetryAfter)
	}
}

func TestNonJSONErrorBody(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.reply = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req_from_header")
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `<html>Service Unavailable</html>`)
	}
	h.mu.Unlock()
	_, err := h.client.Countries.List(context.Background(), "")

	var apiErr *APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, ErrServer) {
		t.Fatal(err)
	}
	if len(apiErr.Messages) != 0 || len(apiErr.Body) != 0 || string(apiErr.RawBody) != `<html>Service Unavailable</html>` {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if apiErr.RequestID != "req_from_header" {
		t.Errorf("RequestID = %q", apiErr.RequestID)
	}
	if err.Error() != "flightbyte: 503 Service Unavailable (request req_from_header)" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestSuspendedAccount(t *testing.T) {
	h := newHarness(t)
	h.json(403, `{"statusCode":403,"message":"This account is suspended.","code":"account_suspended"}`)
	_, err := h.client.Countries.List(context.Background(), "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, ErrForbidden) || apiErr.Code != "account_suspended" {
		t.Errorf("err = %v", err)
	}
}
