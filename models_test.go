package flightbyte

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// decodeStrict decodes data into v and fails on any field v has no place
// for, so a field the API sends but a model lacks breaks the test.
func decodeStrict(t *testing.T, data []byte, v any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("decoding into %T: %v", v, err)
	}
}

// missingKeys lists the paths of non-null keys in want that got lost in got.
func missingKeys(prefix string, want, got any) []string {
	var missing []string
	switch w := want.(type) {
	case map[string]any:
		g, _ := got.(map[string]any)
		for key, value := range w {
			if value == nil {
				continue
			}
			child, ok := g[key]
			if !ok {
				missing = append(missing, prefix+key)
				continue
			}
			missing = append(missing, missingKeys(prefix+key+".", value, child)...)
		}
	case []any:
		g, _ := got.([]any)
		for i, value := range w {
			if i >= len(g) {
				missing = append(missing, prefix+"["+strconv.Itoa(i)+"]")
				continue
			}
			missing = append(missing, missingKeys(prefix, value, g[i])...)
		}
	}
	return missing
}

// roundTrip checks that every key in the fixture survives encoding, and that
// decoding the encoded value gives back the same struct.
func roundTrip[T any](t *testing.T, data []byte) T {
	t.Helper()
	var first T
	decodeStrict(t, data, &first)

	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	_ = json.Unmarshal(data, &want)
	_ = json.Unmarshal(encoded, &got)
	if missing := missingKeys("", want, got); len(missing) > 0 {
		t.Errorf("%T drops %s when encoded", first, strings.Join(missing, ", "))
	}

	var second T
	decodeStrict(t, encoded, &second)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("%T changes across a round trip:\n%+v\n%+v", first, first, second)
	}
	return first
}

func TestFlightFixture(t *testing.T) {
	f := roundTrip[Flight](t, fixture(t, "flight.json"))

	if f.FlightKey != "TK1848:2026-09-08" || f.Carrier.ICAO != "THY" {
		t.Errorf("identity: %q %q", f.FlightKey, f.Carrier.ICAO)
	}
	if want := time.Date(2026, 9, 8, 13, 20, 0, 0, time.UTC); !f.FetchedAt.Equal(want) {
		t.Errorf("FetchedAt = %v", f.FetchedAt)
	}
	if f.Departure.Airport.Country.Emoji != "🇹🇷" || f.Departure.Airport.City != "İstanbul" {
		t.Errorf("departure airport: %+v", f.Departure.Airport)
	}
	if f.Departure.DelayMinutes == nil || *f.Departure.DelayMinutes != 15 {
		t.Errorf("departure delay: %v", f.Departure.DelayMinutes)
	}
	if f.Arrival.DelayMinutes == nil || *f.Arrival.DelayMinutes != -5 {
		t.Errorf("arrival delay: %v", f.Arrival.DelayMinutes)
	}
	if f.Arrival.Gate != "" || f.Arrival.Baggage != "16" {
		t.Errorf("arrival gate %q baggage %q", f.Arrival.Gate, f.Arrival.Baggage)
	}
	if got := f.Departure.Times.BestGate(); got == nil || got.Local != "2026-09-08T12:25:00.000" {
		t.Errorf("departure BestGate = %+v, want the estimated time", got)
	}
	if got := f.Arrival.Times.BestGate(); got == nil || got.Timezone != "CEST" {
		t.Errorf("arrival BestGate = %+v, want the scheduled time", got)
	}
	if f.Diverted != nil {
		t.Errorf("Diverted = %+v, want nil", f.Diverted)
	}
	if f.Status.Phase != PhaseScheduled || !f.Status.Delayed || f.Status.ArrivalGateDelayMinutes != nil {
		t.Errorf("status: %+v", f.Status)
	}
	if f.Duration.ScheduledMinutes == nil || *f.Duration.ScheduledMinutes != 225 || f.Duration.ActualMinutes != nil {
		t.Errorf("duration: %+v", f.Duration)
	}
	if len(f.Codeshares) != 1 || f.Codeshares[0].FlightNumber != "3160" {
		t.Errorf("codeshares: %+v", f.Codeshares)
	}
	if len(f.Timeline) != 1 || len(f.Timeline[0].Events) != 1 {
		t.Errorf("timeline: %+v", f.Timeline)
	}
	if len(f.Alternates) != 1 || !f.Alternates[0].Selected || f.Alternates[0].DepartureIATA != "IST" {
		t.Errorf("alternates: %+v", f.Alternates)
	}
	if !f.Cached || f.Equipment.TailNumber != "TC-LTP" {
		t.Errorf("cached %v, tail %q", f.Cached, f.Equipment.TailNumber)
	}
}

func TestSubscriptionFixture(t *testing.T) {
	s := roundTrip[Subscription](t, fixture(t, "subscription.json"))

	if s.Status != StatusActive || !s.Status.Known() {
		t.Errorf("Status = %q", s.Status)
	}
	if s.Headers["x-team"] != "ops" || s.Metadata["booking"] != "42" {
		t.Errorf("headers %v metadata %v", s.Headers, s.Metadata)
	}
	want := []FlightChangeType{ChangeDeparted, ChangeDepartureGate, "some.future.type"}
	if !reflect.DeepEqual(s.Events, want) {
		t.Errorf("Events = %v, want %v", s.Events, want)
	}
	if s.Events[2].Known() {
		t.Error("an unknown change type reports Known")
	}
	if s.LastPolledAt == nil || s.LastChangeAt != nil {
		t.Errorf("LastPolledAt %v LastChangeAt %v", s.LastPolledAt, s.LastChangeAt)
	}
	if s.DelayThresholdMinutes != 10 || s.DeliveredCount != 3 || s.FailedCount != 1 || !s.FlightResolved {
		t.Errorf("counters: %+v", s)
	}
}

func TestWebhookEventFixtures(t *testing.T) {
	test := roundTrip[WebhookEvent](t, fixture(t, "webhook_test.json"))
	if test.Type != EventSubscriptionTest || test.Changes == nil || len(test.Changes) != 0 || test.Flight != nil {
		t.Errorf("test event: %+v", test)
	}

	updated := roundTrip[WebhookEvent](t, fixture(t, "webhook_flight_updated.json"))
	if updated.Type != EventFlightUpdated || updated.Metadata["booking"] != "42" {
		t.Errorf("updated event: %+v", updated)
	}
	if len(updated.Changes) != 1 {
		t.Fatalf("Changes = %+v", updated.Changes)
	}
	c := updated.Changes[0]
	if c.Type != ChangeDepartureGate || c.From != "A11" || c.To != "A22" || c.DeltaMinutes != nil {
		t.Errorf("change: %+v", c)
	}
	if !strings.Contains(c.Message, "İstanbul") {
		t.Errorf("Message = %q", c.Message)
	}
}

func TestCreatedSubscriptionDecodesEmbeddedFields(t *testing.T) {
	var sub map[string]any
	_ = json.Unmarshal(fixture(t, "subscription.json"), &sub)
	delete(sub, "secretHint")
	sub["secret"] = "whsec_AbCdEfGhIjKlMnOpQrStUvwxyz"
	sub["reused"] = true
	sub["flight"] = nil
	data, _ := json.Marshal(sub)

	var created CreatedSubscription
	decodeStrict(t, data, &created)
	if created.ID != "sub_7f3a9c21d04e4b6f8a1c" || !created.Reused || created.Flight != nil {
		t.Errorf("created: %+v", created)
	}
	if created.Secret != "whsec_AbCdEfGhIjKlMnOpQrStUvwxyz" {
		t.Errorf("Secret = %q", created.Secret)
	}
}

func TestCountryEmbedsRef(t *testing.T) {
	data := []byte(`{"code":"TR","alpha2":"TR","name":"Türkiye","emoji":"🇹🇷","hasFlag":true,
		"flagSvg":"https://api.flightbyte.dev/v1/flags/tr.svg",
		"flagSvgSquare":"https://api.flightbyte.dev/v1/flags/tr.svg?shape=1x1"}`)
	c := roundTrip[Country](t, data)
	if c.Code != "TR" || c.Name != "Türkiye" || !c.HasFlag || c.FlagSVGSquare == "" {
		t.Errorf("country: %+v", c)
	}
}

func TestEnums(t *testing.T) {
	for _, p := range []FlightPhase{PhaseArrived, PhaseCancelled, PhaseDiverted, PhaseRedirected} {
		if !p.IsTerminal() {
			t.Errorf("%s is terminal", p)
		}
	}
	for _, p := range []FlightPhase{PhaseScheduled, PhaseBoarding, PhaseDeparted, PhaseEnRoute, PhaseLanded, PhaseUnknown, "later"} {
		if p.IsTerminal() {
			t.Errorf("%s is not terminal", p)
		}
	}
	if FlightPhase("later").Known() || !PhaseEnRoute.Known() {
		t.Error("FlightPhase.Known")
	}
	if len(FlightChangeTypes) != 23 {
		t.Errorf("%d change types, the API has 23", len(FlightChangeTypes))
	}
	seen := map[FlightChangeType]bool{}
	for _, ct := range FlightChangeTypes {
		if seen[ct] {
			t.Errorf("%s listed twice", ct)
		}
		seen[ct] = true
	}
	if !EventFlightUpdated.Known() || WebhookEventType("x").Known() {
		t.Error("WebhookEventType.Known")
	}
	if !DeliveryFailed.Known() || DeliveryStatus("x").Known() {
		t.Error("DeliveryStatus.Known")
	}
	if !StatusSuspended.Known() || SubscriptionStatus("x").Known() {
		t.Error("SubscriptionStatus.Known")
	}
}

func TestHelpers(t *testing.T) {
	if got := MaskSecret("whsec_AbCdEfGhIjKlMnOpQrStUvwxyz"); got != "whsec_AbCdE...wxyz" {
		t.Errorf("MaskSecret = %q", got)
	}
	if got := MaskSecret("short"); got != "short" {
		t.Errorf("MaskSecret(short) = %q", got)
	}
	page := SubscriptionPage{Total: 3, Offset: 1, Items: make([]Subscription, 1)}
	if !page.HasMore() || page.NextOffset() != 2 {
		t.Errorf("page 2 of 3: HasMore %v NextOffset %d", page.HasMore(), page.NextOffset())
	}
	page.Offset = 2
	if page.HasMore() {
		t.Error("the last page reports HasMore")
	}
	if (EndpointTimes{}).BestGate() != nil {
		t.Error("BestGate of nothing")
	}
	loc := time.FixedZone("UTC+3", 3*3600)
	if got := FormatDate(time.Date(2026, 9, 8, 1, 0, 0, 0, loc)); got != "2026-09-08" {
		t.Errorf("FormatDate = %q", got)
	}
	if *Ptr(0) != 0 {
		t.Error("Ptr")
	}
}
