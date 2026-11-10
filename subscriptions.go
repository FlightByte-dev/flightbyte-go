package flightbyte

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// SubscriptionsService watches flights and manages their webhooks. Reach it
// as client.Subscriptions.
//
// Every subscription is a billable watched flight.
type SubscriptionsService struct{ c *Client }

// CreateSubscriptionParams describes a flight to watch.
type CreateSubscriptionParams struct {
	// Carrier is the carrier code, for example "TK". Required.
	Carrier string `json:"carrier"`
	// FlightNumber is the flight number, for example "1848". Required.
	FlightNumber string `json:"flightNumber"`
	// Date is the local departure date at the origin, YYYY-MM-DD. Required.
	Date string `json:"date"`
	// WebhookURL is where webhooks are posted. Required.
	WebhookURL string `json:"webhookUrl"`
	// Origin selects a leg, when the flight number operates several that day.
	Origin string `json:"origin,omitempty"`
	// Destination narrows the leg further.
	Destination string `json:"destination,omitempty"`
	// FlightID selects a leg by its upstream id.
	FlightID string `json:"flightId,omitempty"`
	// Secret sets the webhook signing secret. By default the API generates
	// one, returned once in [CreatedSubscription.Secret]. Setting the same
	// secret on every subscription lets one [WebhookHandler] verify them all.
	Secret string `json:"secret,omitempty"`
	// Events filters the change types. By default every type is sent.
	Events []FlightChangeType `json:"events,omitempty"`
	// DelayThresholdMinutes (1 to 240) ignores smaller delay swings.
	DelayThresholdMinutes *int `json:"delayThresholdMinutes,omitempty"`
	// Headers are added to every delivery.
	Headers map[string]string `json:"headers,omitempty"`
	// Metadata is echoed back in every webhook.
	Metadata map[string]string `json:"metadata,omitempty"`
	// IdempotencyKey makes retries safe: repeating the request with the same
	// key within 24 hours returns the original subscription with Reused set,
	// instead of creating and billing a second one.
	IdempotencyKey string `json:"-"`
}

// Create starts watching a flight. The API posts a signed webhook to
// WebhookURL each time something about it changes.
//
// It returns an error matching [ErrPaymentRequired] when the period's watched
// flights are used up, and one matching [ErrBadRequest], with
// [APIError.Legs] set, when the flight number operates several legs that day
// and neither Origin nor FlightID chooses one.
func (s *SubscriptionsService) Create(ctx context.Context, params CreateSubscriptionParams) (*CreatedSubscription, error) {
	if err := required("Carrier", params.Carrier, "FlightNumber", params.FlightNumber,
		"Date", params.Date, "WebhookURL", params.WebhookURL); err != nil {
		return nil, err
	}
	var header http.Header
	if params.IdempotencyKey != "" {
		header = http.Header{"Idempotency-Key": {params.IdempotencyKey}}
	}
	var created CreatedSubscription
	if err := s.c.do(ctx, "POST", []string{"subscriptions"}, nil, params, header, &created); err != nil {
		return nil, err
	}
	if created.SecretHint == "" && created.Secret != "" {
		created.SecretHint = MaskSecret(created.Secret)
	}
	return &created, nil
}

// ListSubscriptionsParams filters [SubscriptionsService.List]. Zero values
// are not sent.
type ListSubscriptionsParams struct {
	// Status keeps subscriptions in this status.
	Status SubscriptionStatus
	// Carrier keeps subscriptions for this carrier.
	Carrier string
	// FlightNumber keeps subscriptions for this flight number.
	FlightNumber string
	// Date keeps subscriptions for this date, YYYY-MM-DD.
	Date string
	// Limit is the page size; the API clamps it to 1 to 200 and defaults
	// to 25.
	Limit int
	// Offset is the number of subscriptions to skip.
	Offset int
}

func (p ListSubscriptionsParams) query() url.Values {
	q := url.Values{}
	setIf(q, "status", string(p.Status))
	setIf(q, "carrier", p.Carrier)
	setIf(q, "flightNumber", p.FlightNumber)
	setIf(q, "date", p.Date)
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		q.Set("offset", strconv.Itoa(p.Offset))
	}
	return q
}

// List returns one page of your subscriptions, newest first.
func (s *SubscriptionsService) List(ctx context.Context, params ListSubscriptionsParams) (*SubscriptionPage, error) {
	var page SubscriptionPage
	if err := s.c.do(ctx, "GET", []string{"subscriptions"}, params.query(), nil, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// All returns every subscription matching params, fetched page by page as
// the loop asks for more. params.Limit sets the page size (default 200) and
// params.Offset where to start.
//
//	for sub, err := range client.Subscriptions.All(ctx, flightbyte.ListSubscriptionsParams{}) {
//		if err != nil {
//			return err
//		}
//		fmt.Println(sub.FlightKey)
//	}
//
// After an error the sequence ends.
func (s *SubscriptionsService) All(ctx context.Context, params ListSubscriptionsParams) iter.Seq2[*Subscription, error] {
	return func(yield func(*Subscription, error) bool) {
		if params.Limit <= 0 {
			params.Limit = 200
		}
		for {
			page, err := s.List(ctx, params)
			if err != nil {
				yield(nil, err)
				return
			}
			for i := range page.Items {
				if !yield(&page.Items[i], nil) {
					return
				}
			}
			if !page.HasMore() || len(page.Items) == 0 {
				return
			}
			params.Offset = page.NextOffset()
		}
	}
}

// Get returns one subscription. It returns an error matching [ErrNotFound]
// when the subscription does not exist or belongs to another key.
func (s *SubscriptionsService) Get(ctx context.Context, id string) (*Subscription, error) {
	return s.subscription(ctx, "GET", id, nil)
}

// UpdateSubscriptionParams changes a subscription. Zero values are left as
// they are.
type UpdateSubscriptionParams struct {
	// WebhookURL moves deliveries to another URL.
	WebhookURL string `json:"webhookUrl,omitempty"`
	// Events replaces the change type filter.
	Events []FlightChangeType `json:"events,omitempty"`
	// DelayThresholdMinutes replaces the delay threshold.
	DelayThresholdMinutes *int `json:"delayThresholdMinutes,omitempty"`
	// Headers replaces the delivery headers. The API cannot clear them, so an
	// empty map leaves them as they are.
	Headers map[string]string `json:"headers,omitempty"`
	// Metadata replaces the metadata. The API cannot clear it, so an empty map
	// leaves it as it is.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Update changes a subscription. Only the fields set in params change.
func (s *SubscriptionsService) Update(ctx context.Context, id string, params UpdateSubscriptionParams) (*Subscription, error) {
	return s.subscription(ctx, "PATCH", id, params)
}

// Delete stops watching the flight and deletes the subscription.
func (s *SubscriptionsService) Delete(ctx context.Context, id string) error {
	if err := required("id", id); err != nil {
		return err
	}
	return s.c.do(ctx, "DELETE", []string{"subscriptions", id}, nil, nil, nil, nil)
}

// Pause pauses polling and deliveries; [SubscriptionsService.Resume] picks
// up where it left off.
func (s *SubscriptionsService) Pause(ctx context.Context, id string) (*Subscription, error) {
	return s.subscription(ctx, "POST", id, nil, "pause")
}

// Resume resumes a paused or suspended subscription. A completed one cannot
// be resumed and returns an error matching [ErrBadRequest].
func (s *SubscriptionsService) Resume(ctx context.Context, id string) (*Subscription, error) {
	return s.subscription(ctx, "POST", id, nil, "resume")
}

// SendTest sends a subscription.test webhook and returns its event id.
func (s *SubscriptionsService) SendTest(ctx context.Context, id string) (string, error) {
	if err := required("id", id); err != nil {
		return "", err
	}
	var out struct {
		EventID string `json:"eventId"`
	}
	if err := s.c.do(ctx, "POST", []string{"subscriptions", id, "test"}, nil, nil, nil, &out); err != nil {
		return "", err
	}
	return out.EventID, nil
}

// Deliveries returns the latest delivery attempts, newest first. limit is 1
// to 100; zero means the API's default of 25.
func (s *SubscriptionsService) Deliveries(ctx context.Context, id string, limit int) ([]Delivery, error) {
	if err := required("id", id); err != nil {
		return nil, err
	}
	var out struct {
		Data []Delivery `json:"data"`
	}
	if err := s.c.do(ctx, "GET", []string{"subscriptions", id, "deliveries"}, limitQuery(limit), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Events returns the latest events, as they were posted, newest first. limit
// is 1 to 100; zero means the API's default of 25.
func (s *SubscriptionsService) Events(ctx context.Context, id string, limit int) ([]WebhookEvent, error) {
	if err := required("id", id); err != nil {
		return nil, err
	}
	var out struct {
		Data []WebhookEvent `json:"data"`
	}
	if err := s.c.do(ctx, "GET", []string{"subscriptions", id, "events"}, limitQuery(limit), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Secret returns the full webhook signing secret.
func (s *SubscriptionsService) Secret(ctx context.Context, id string) (string, error) {
	return s.secret(ctx, "GET", id, "secret")
}

// RotateSecret replaces the webhook signing secret and returns the new one.
// Deliveries are signed with the new secret from now on, so update your
// receiver right away.
func (s *SubscriptionsService) RotateSecret(ctx context.Context, id string) (string, error) {
	return s.secret(ctx, "POST", id, "secret", "rotate")
}

func (s *SubscriptionsService) subscription(ctx context.Context, method, id string, body any, extra ...string) (*Subscription, error) {
	if err := required("id", id); err != nil {
		return nil, err
	}
	var sub Subscription
	path := append([]string{"subscriptions", id}, extra...)
	if err := s.c.do(ctx, method, path, nil, body, nil, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *SubscriptionsService) secret(ctx context.Context, method, id string, extra ...string) (string, error) {
	if err := required("id", id); err != nil {
		return "", err
	}
	var out struct {
		Secret string `json:"secret"`
	}
	path := append([]string{"subscriptions", id}, extra...)
	if err := s.c.do(ctx, method, path, nil, nil, nil, &out); err != nil {
		return "", err
	}
	return out.Secret, nil
}

func limitQuery(limit int) url.Values {
	if limit <= 0 {
		return nil
	}
	return url.Values{"limit": {strconv.Itoa(limit)}}
}
