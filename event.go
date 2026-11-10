package flightbyte

import "time"

// WebhookEvent is a webhook event, as posted to your receiver and as listed
// by [SubscriptionsService.Events].
type WebhookEvent struct {
	// ID is the event id, "evt_<uuid>". It is the same across delivery
	// retries: deduplicate on it.
	ID string `json:"id"`
	// Type is the event type.
	Type WebhookEventType `json:"type"`
	// CreatedAt is when the event was created.
	CreatedAt time.Time `json:"createdAt"`
	// SubscriptionID is the subscription it belongs to.
	SubscriptionID string `json:"subscriptionId"`
	// FlightKey is the flight's key.
	FlightKey string `json:"flightKey"`
	// Flight is the flight after the change. Nil on some lifecycle events.
	Flight *Flight `json:"flight,omitempty"`
	// Changes says what changed. Empty except on flight.updated.
	Changes []FlightChange `json:"changes"`
	// PreviousFetchedAt is when the previous snapshot was fetched, on
	// flight.updated.
	PreviousFetchedAt *time.Time `json:"previousFetchedAt,omitempty"`
	// Metadata is the subscription's metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// FlightChange is one change inside a flight.updated event.
type FlightChange struct {
	// Type says what changed.
	Type FlightChangeType `json:"type"`
	// Path is the dotted path into the flight, for example "departure.gate".
	Path string `json:"path"`
	// From is the old value: a string, a float64 or nil. Times are
	// RFC 3339 UTC strings.
	From any `json:"from"`
	// To is the new value: a string, a float64 or nil. Times are RFC 3339
	// UTC strings.
	To any `json:"to"`
	// Message is a sentence describing the change.
	Message string `json:"message"`
	// DeltaMinutes is the signed minute delta, for time changes.
	DeltaMinutes *int `json:"deltaMinutes"`
}
