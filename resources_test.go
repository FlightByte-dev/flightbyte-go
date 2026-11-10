package flightbyte

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestFlightsGet(t *testing.T) {
	h := newHarness(t)
	h.json(200, string(fixture(t, "flight.json")))

	flight, err := h.client.Flights.Get(context.Background(), FlightParams{
		Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08",
		Origin: "IST", Destination: "CDG", FlightID: "1407084032", Refresh: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := h.last()
	if r.Method != "GET" || r.Path != "/v1/flights/TK/1848/2026-09-08" {
		t.Errorf("%s %s", r.Method, r.Path)
	}
	if r.Query != "destination=CDG&flightId=1407084032&origin=IST&refresh=true" {
		t.Errorf("Query = %q", r.Query)
	}
	if flight.FlightKey != "TK1848:2026-09-08" {
		t.Errorf("FlightKey = %q", flight.FlightKey)
	}

	_, _ = h.client.Flights.Get(context.Background(), FlightParams{Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08"})
	if q := h.last().Query; q != "" {
		t.Errorf("empty params send %q", q)
	}
}

func TestFlightsTimeline(t *testing.T) {
	h := newHarness(t)
	h.json(200, `{"flightKey":"TK1848:2026-09-08","count":1,"data":[
		{"occurredAt":"2026-09-08T08:38:16.000Z","title":"Time Adjustment","source":"Airline","events":["x"]}]}`)

	tl, err := h.client.Flights.Timeline(context.Background(), FlightParams{
		Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08", Origin: "IST", FlightID: "ignored", Refresh: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := h.last()
	if r.Path != "/v1/flights/TK/1848/2026-09-08/timeline" || r.Query != "origin=IST" {
		t.Errorf("%s?%s", r.Path, r.Query)
	}
	if tl.FlightKey != "TK1848:2026-09-08" || tl.Count != 1 || len(tl.Entries) != 1 || tl.Entries[0].Title != "Time Adjustment" {
		t.Errorf("timeline: %+v", tl)
	}
}

func TestSubscriptionsCreate(t *testing.T) {
	h := newHarness(t)
	var sub map[string]any
	_ = json.Unmarshal(fixture(t, "subscription.json"), &sub)
	delete(sub, "secretHint")
	sub["secret"] = "whsec_AbCdEfGhIjKlMnOpQrStUvwxyz"
	sub["reused"] = false
	var flight any
	_ = json.Unmarshal(fixture(t, "flight.json"), &flight)
	sub["flight"] = flight
	body, _ := json.Marshal(sub)
	h.json(201, string(body))

	created, err := h.client.Subscriptions.Create(context.Background(), CreateSubscriptionParams{
		Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08",
		WebhookURL:            "https://example.com/hooks/flights",
		Origin:                "IST",
		Events:                []FlightChangeType{ChangeDeparted, ChangeDepartureGate},
		DelayThresholdMinutes: Ptr(10),
		Headers:               map[string]string{"x-team": "ops"},
		Metadata:              map[string]string{"booking": "42"},
		IdempotencyKey:        "booking-42",
	})
	if err != nil {
		t.Fatal(err)
	}

	r := h.last()
	if r.Method != "POST" || r.Path != "/v1/subscriptions" {
		t.Errorf("%s %s", r.Method, r.Path)
	}
	if got := r.Header.Get("Idempotency-Key"); got != "booking-42" {
		t.Errorf("Idempotency-Key = %q", got)
	}
	if got := r.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(r.Body), &sent); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"carrier": "TK", "flightNumber": "1848", "date": "2026-09-08",
		"webhookUrl": "https://example.com/hooks/flights", "origin": "IST",
		"events":                []any{"flight.departed", "departure.gate.changed"},
		"delayThresholdMinutes": float64(10),
		"headers":               map[string]any{"x-team": "ops"},
		"metadata":              map[string]any{"booking": "42"},
	}
	if !reflect.DeepEqual(sent, want) {
		t.Errorf("body = %v\nwant   %v", sent, want)
	}

	if created.ID != "sub_7f3a9c21d04e4b6f8a1c" || created.Reused {
		t.Errorf("created: %+v", created.Subscription)
	}
	if created.SecretHint != "whsec_AbCdE...wxyz" {
		t.Errorf("SecretHint = %q, want it derived from the secret", created.SecretHint)
	}
	if created.Flight == nil || created.Flight.FlightKey != "TK1848:2026-09-08" {
		t.Errorf("Flight = %+v", created.Flight)
	}
}

func TestSubscriptionsCreateMinimal(t *testing.T) {
	h := newHarness(t)
	h.json(201, `{"id":"sub_1","secret":"s"}`)
	_, err := h.client.Subscriptions.Create(context.Background(), CreateSubscriptionParams{
		Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08", WebhookURL: "https://example.com/h",
		DelayThresholdMinutes: Ptr(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	r := h.last()
	want := `{"carrier":"TK","flightNumber":"1848","date":"2026-09-08","webhookUrl":"https://example.com/h","delayThresholdMinutes":0}`
	if r.Body != want {
		t.Errorf("body = %s\nwant   %s", r.Body, want)
	}
	if _, ok := r.Header["Idempotency-Key"]; ok {
		t.Error("Idempotency-Key sent without a key")
	}
}

func TestSubscriptionsList(t *testing.T) {
	h := newHarness(t)
	h.json(200, `{"total":1,"limit":5,"offset":10,"data":[`+string(fixture(t, "subscription.json"))+`]}`)

	page, err := h.client.Subscriptions.List(context.Background(), ListSubscriptionsParams{
		Status: StatusActive, Carrier: "TK", FlightNumber: "1848", Date: "2026-09-08", Limit: 5, Offset: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := h.last()
	if r.Path != "/v1/subscriptions" ||
		r.Query != "carrier=TK&date=2026-09-08&flightNumber=1848&limit=5&offset=10&status=active" {
		t.Errorf("%s?%s", r.Path, r.Query)
	}
	if page.Total != 1 || page.Limit != 5 || page.Offset != 10 || len(page.Items) != 1 {
		t.Errorf("page: %+v", page)
	}
}

func TestSubscriptionsAll(t *testing.T) {
	h := newHarness(t)
	const total = 5
	h.mu.Lock()
	h.reply = func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		var items []string
		for i := offset; i < total && i < offset+limit; i++ {
			items = append(items, fmt.Sprintf(`{"id":"sub_%d"}`, i))
		}
		fmt.Fprintf(w, `{"total":%d,"limit":%d,"offset":%d,"data":[%s]}`,
			total, limit, offset, joinComma(items))
	}
	h.mu.Unlock()

	var ids []string
	for sub, err := range h.client.Subscriptions.All(context.Background(), ListSubscriptionsParams{Limit: 2, Status: StatusActive}) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, sub.ID)
	}
	if want := []string{"sub_0", "sub_1", "sub_2", "sub_3", "sub_4"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ids = %v", ids)
	}
	if h.count() != 3 {
		t.Errorf("%d pages fetched, want 3", h.count())
	}
	if q := h.last().Query; q != "limit=2&offset=4&status=active" {
		t.Errorf("last page query = %q", q)
	}

	// Breaking out of the loop stops fetching.
	before := h.count()
	for range h.client.Subscriptions.All(context.Background(), ListSubscriptionsParams{Limit: 2}) {
		break
	}
	if h.count() != before+1 {
		t.Errorf("break fetched %d pages, want 1", h.count()-before)
	}

	// The default page size is 200.
	for range h.client.Subscriptions.All(context.Background(), ListSubscriptionsParams{}) {
	}
	if q := h.last().Query; q != "limit=200" {
		t.Errorf("default page query = %q", q)
	}
}

func TestSubscriptionsAllStopsOnError(t *testing.T) {
	h := newHarness(t)
	h.json(500, `{"statusCode":500,"message":"boom"}`)
	n := 0
	for sub, err := range h.client.Subscriptions.All(context.Background(), ListSubscriptionsParams{}) {
		n++
		if sub != nil || !errors.Is(err, ErrServer) {
			t.Errorf("sub %v err %v", sub, err)
		}
	}
	if n != 1 {
		t.Errorf("%d values after an error, want 1", n)
	}
}

func TestSubscriptionActions(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		reply  string
		call   func(c *Client) (any, error)
		method string
		path   string
		query  string
		body   string
		check  func(t *testing.T, got any)
	}{
		{
			name: "get", reply: `{"id":"sub_1"}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.Get(ctx, "sub_1") },
			method: "GET", path: "/v1/subscriptions/sub_1",
		},
		{
			name: "update", reply: `{"id":"sub_1","delayThresholdMinutes":30}`,
			call: func(c *Client) (any, error) {
				return c.Subscriptions.Update(ctx, "sub_1", UpdateSubscriptionParams{
					WebhookURL: "https://example.com/new", DelayThresholdMinutes: Ptr(30),
					Events: []FlightChangeType{ChangeCancelled},
				})
			},
			method: "PATCH", path: "/v1/subscriptions/sub_1",
			body: `{"webhookUrl":"https://example.com/new","events":["flight.cancelled"],"delayThresholdMinutes":30}`,
		},
		{
			name: "update nothing", reply: `{"id":"sub_1"}`,
			call: func(c *Client) (any, error) {
				return c.Subscriptions.Update(ctx, "sub_1", UpdateSubscriptionParams{})
			},
			method: "PATCH", path: "/v1/subscriptions/sub_1", body: `{}`,
		},
		{
			name: "pause", reply: `{"id":"sub_1","status":"paused"}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.Pause(ctx, "sub_1") },
			method: "POST", path: "/v1/subscriptions/sub_1/pause",
			check: func(t *testing.T, got any) {
				if s := got.(*Subscription); s.Status != StatusPaused {
					t.Errorf("Status = %q", s.Status)
				}
			},
		},
		{
			name: "resume", reply: `{"id":"sub_1","status":"active"}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.Resume(ctx, "sub_1") },
			method: "POST", path: "/v1/subscriptions/sub_1/resume",
		},
		{
			name: "send test", reply: `{"eventId":"evt_1"}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.SendTest(ctx, "sub_1") },
			method: "POST", path: "/v1/subscriptions/sub_1/test",
			check: func(t *testing.T, got any) {
				if got != "evt_1" {
					t.Errorf("event id = %v", got)
				}
			},
		},
		{
			name: "deliveries",
			reply: `{"data":[{"eventId":"evt_1","attempt":2,"status":"failed","httpStatus":500,
				"error":"HTTP 500","durationMs":120,"at":"2026-09-08T09:10:00.000Z","changeTypes":["departure.gate.changed"]}]}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.Deliveries(ctx, "sub_1", 10) },
			method: "GET", path: "/v1/subscriptions/sub_1/deliveries", query: "limit=10",
			check: func(t *testing.T, got any) {
				d := got.([]Delivery)
				if len(d) != 1 || d[0].Status != DeliveryFailed || d[0].HTTPStatus == nil || *d[0].HTTPStatus != 500 ||
					d[0].ChangeTypes[0] != ChangeDepartureGate {
					t.Errorf("deliveries: %+v", d)
				}
			},
		},
		{
			name: "events", reply: `{"data":[` + string(fixture(t, "webhook_flight_updated.json")) + `]}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.Events(ctx, "sub_1", 0) },
			method: "GET", path: "/v1/subscriptions/sub_1/events",
			check: func(t *testing.T, got any) {
				if e := got.([]WebhookEvent); len(e) != 1 || e[0].Type != EventFlightUpdated {
					t.Errorf("events: %+v", e)
				}
			},
		},
		{
			name: "secret", reply: `{"secret":"whsec_x"}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.Secret(ctx, "sub_1") },
			method: "GET", path: "/v1/subscriptions/sub_1/secret",
			check: func(t *testing.T, got any) {
				if got != "whsec_x" {
					t.Errorf("secret = %v", got)
				}
			},
		},
		{
			name: "rotate secret", reply: `{"secret":"whsec_y"}`,
			call:   func(c *Client) (any, error) { return c.Subscriptions.RotateSecret(ctx, "sub_1") },
			method: "POST", path: "/v1/subscriptions/sub_1/secret/rotate",
			check: func(t *testing.T, got any) {
				if got != "whsec_y" {
					t.Errorf("secret = %v", got)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.json(201, tc.reply)
			got, err := tc.call(h.client)
			if err != nil {
				t.Fatal(err)
			}
			r := h.last()
			if r.Method != tc.method || r.Path != tc.path || r.Query != tc.query || r.Body != tc.body {
				t.Errorf("sent %s %s?%s %s\nwant %s %s?%s %s", r.Method, r.Path, r.Query, r.Body, tc.method, tc.path, tc.query, tc.body)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestSubscriptionsDelete(t *testing.T) {
	h := newHarness(t) // answers 204 with no body
	if err := h.client.Subscriptions.Delete(context.Background(), "sub_1"); err != nil {
		t.Fatal(err)
	}
	if r := h.last(); r.Method != "DELETE" || r.Path != "/v1/subscriptions/sub_1" {
		t.Errorf("%s %s", r.Method, r.Path)
	}
}

func TestCountries(t *testing.T) {
	h := newHarness(t)
	country := `{"code":"TR","alpha2":"TR","name":"Türkiye","emoji":"🇹🇷","hasFlag":true,
		"flagSvg":"https://api.flightbyte.dev/v1/flags/tr.svg","flagSvgSquare":"https://api.flightbyte.dev/v1/flags/tr.svg?shape=1x1"}`

	h.json(200, `{"data":[`+country+`]}`)
	list, err := h.client.Countries.List(context.Background(), "tr")
	if err != nil {
		t.Fatal(err)
	}
	if r := h.last(); r.Path != "/v1/countries" || r.Query != "lang=tr" {
		t.Errorf("%s?%s", r.Path, r.Query)
	}
	if len(list) != 1 || list[0].Name != "Türkiye" || !list[0].HasFlag {
		t.Errorf("list: %+v", list)
	}

	h.json(200, country)
	one, err := h.client.Countries.Get(context.Background(), "TR", "")
	if err != nil {
		t.Fatal(err)
	}
	if r := h.last(); r.Path != "/v1/countries/TR" || r.Query != "" {
		t.Errorf("%s?%s", r.Path, r.Query)
	}
	if one.Alpha2 != "TR" {
		t.Errorf("country: %+v", one)
	}

	base := h.server.URL + "/v1"
	if got := h.client.Countries.FlagURL("TR", false); got != base+"/flags/tr.svg" {
		t.Errorf("FlagURL = %q", got)
	}
	if got := h.client.Countries.FlagURL("TR", true); got != base+"/flags/tr.svg?shape=1x1" {
		t.Errorf("FlagURL square = %q", got)
	}
}

func joinComma(items []string) string { return strings.Join(items, ",") }
