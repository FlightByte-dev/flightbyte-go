package flightbyte

import "time"

// Subscription is a watched flight: the API polls it and posts a signed
// webhook to WebhookURL whenever something about it changes.
type Subscription struct {
	// ID is the id, for example "sub_7Kd91mQx0Zb2c3d4e5f6".
	ID string `json:"id"`
	// Status is the lifecycle status.
	Status SubscriptionStatus `json:"status"`
	// FlightKey is the watched flight's key, for example "TK1848:2026-09-08".
	FlightKey string `json:"flightKey"`
	// Carrier is the carrier code.
	Carrier string `json:"carrier"`
	// FlightNumber is the flight number.
	FlightNumber string `json:"flightNumber"`
	// Date is the local departure date at the origin, YYYY-MM-DD.
	Date string `json:"date"`
	// WebhookURL is where webhooks are posted.
	WebhookURL string `json:"webhookUrl"`
	// SecretHint is the masked signing secret, for example
	// "whsec_AbCdE...wxyz".
	SecretHint string `json:"secretHint"`
	// Headers are extra headers sent with every delivery.
	Headers map[string]string `json:"headers"`
	// Events lists the change types that trigger a webhook.
	Events []FlightChangeType `json:"events"`
	// DelayThresholdMinutes: delay swings smaller than this are ignored.
	DelayThresholdMinutes int `json:"delayThresholdMinutes"`
	// Metadata is your own data, echoed back in every webhook.
	Metadata map[string]string `json:"metadata"`
	// CreatedAt is when it was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// LastPolledAt is when the flight was last polled.
	LastPolledAt *time.Time `json:"lastPolledAt"`
	// LastChangeAt is when a change was last detected.
	LastChangeAt *time.Time `json:"lastChangeAt"`
	// LastDeliveryAt is when a webhook was last delivered.
	LastDeliveryAt *time.Time `json:"lastDeliveryAt"`
	// DeliveredCount is the number of webhooks delivered.
	DeliveredCount int `json:"deliveredCount"`
	// FailedCount is the number of webhook attempts that failed.
	FailedCount int `json:"failedCount"`
	// FlightResolved is false while the flight is not found upstream yet.
	FlightResolved bool `json:"flightResolved"`
	// StatusReason says why the subscription was suspended or completed.
	StatusReason string `json:"statusReason"`
}

// CreatedSubscription is the response of [SubscriptionsService.Create].
type CreatedSubscription struct {
	Subscription
	// Secret is the full webhook signing secret. The API returns it in full
	// only here and from [SubscriptionsService.Secret]; store it where your
	// webhook receiver can read it.
	Secret string `json:"secret"`
	// Reused is true when an earlier request with the same idempotency key,
	// within 24 hours, was replayed: the original subscription is returned and
	// nothing new is created or billed.
	Reused bool `json:"reused"`
	// Flight is the flight as first seen, when it was found. Nil when Reused.
	Flight *Flight `json:"flight"`
}

// SubscriptionPage is one page of [SubscriptionsService.List].
type SubscriptionPage struct {
	// Total is the number of subscriptions matching the filters.
	Total int `json:"total"`
	// Limit is the page size used.
	Limit int `json:"limit"`
	// Offset is the number of subscriptions skipped.
	Offset int `json:"offset"`
	// Items holds the subscriptions on this page.
	Items []Subscription `json:"data"`
}

// HasMore reports whether more pages follow.
func (p *SubscriptionPage) HasMore() bool { return p.Offset+len(p.Items) < p.Total }

// NextOffset returns the offset of the next page.
func (p *SubscriptionPage) NextOffset() int { return p.Offset + len(p.Items) }

// Delivery is one webhook delivery attempt.
type Delivery struct {
	// EventID is the delivered event's id.
	EventID string `json:"eventId"`
	// Attempt is the attempt number, starting at 1.
	Attempt int `json:"attempt"`
	// Status is the outcome.
	Status DeliveryStatus `json:"status"`
	// HTTPStatus is the receiver's HTTP status, when it answered.
	HTTPStatus *int `json:"httpStatus"`
	// Error says what went wrong, when the attempt failed.
	Error string `json:"error"`
	// DurationMs is how long the attempt took, in milliseconds.
	DurationMs int `json:"durationMs"`
	// At is when the attempt was made.
	At time.Time `json:"at"`
	// ChangeTypes lists the change types the event carried.
	ChangeTypes []FlightChangeType `json:"changeTypes"`
}

// MaskSecret masks a signing secret the way the API does: the first 11
// characters, "...", and the last 4. Short secrets are returned as is.
func MaskSecret(secret string) string {
	if len(secret) <= 15 {
		return secret
	}
	return secret[:11] + "..." + secret[len(secret)-4:]
}
