package flightbyte

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Headers FlightByte sends with every webhook delivery.
const (
	// SignatureHeader carries the signature, "t=<unix seconds>,v1=<hex>".
	SignatureHeader = "X-Flightbyte-Signature"
	// TimestampHeader carries the signing time, in unix seconds.
	TimestampHeader = "X-Flightbyte-Timestamp"
	// EventIDHeader carries the event id.
	EventIDHeader = "X-Flightbyte-Event-Id"
	// EventTypeHeader carries the event type.
	EventTypeHeader = "X-Flightbyte-Event-Type"
	// SubscriptionIDHeader carries the subscription id.
	SubscriptionIDHeader = "X-Flightbyte-Subscription-Id"
	// DeliveryAttemptHeader carries the delivery attempt number, starting
	// at 1.
	DeliveryAttemptHeader = "X-Flightbyte-Delivery-Attempt"
)

// DefaultWebhookTolerance is how far the signing time may be from now:
// 5 minutes, as on the API.
const DefaultWebhookTolerance = 5 * time.Minute

// MaxWebhookBodyBytes is the largest body [WebhookHandler] reads.
const MaxWebhookBodyBytes = 1 << 20

// ErrInvalidSignature matches every [*WebhookSignatureError] with
// [errors.Is].
var ErrInvalidSignature = errors.New("flightbyte: invalid webhook signature")

// WebhookSignatureError is returned when a webhook signature does not
// verify.
type WebhookSignatureError struct {
	// Reason says why verification failed.
	Reason string
}

// Error describes why verification failed.
func (e *WebhookSignatureError) Error() string {
	return "flightbyte: invalid webhook signature: " + e.Reason
}

// Is reports whether target is [ErrInvalidSignature].
func (e *WebhookSignatureError) Is(target error) bool { return target == ErrInvalidSignature }

// VerifyOption configures [VerifyWebhook] and the functions built on it.
type VerifyOption func(*verifyConfig)

type verifyConfig struct {
	tolerance time.Duration
	now       time.Time
}

// WithTolerance sets how far the signing time may be from now. Zero skips
// the check. The default is [DefaultWebhookTolerance].
func WithTolerance(tolerance time.Duration) VerifyOption {
	return func(c *verifyConfig) { c.tolerance = tolerance }
}

// WithNow checks the signing time against now instead of the current time.
// Useful in tests.
func WithNow(now time.Time) VerifyOption {
	return func(c *verifyConfig) { c.now = now }
}

// VerifyWebhook checks signature (the X-Flightbyte-Signature header) against
// the raw request body payload and the subscription's signing secret.
//
// The signature has the form "t=<unix seconds>,v1=<hex>", where the hex is
// HMAC-SHA256 of "<t>.<payload>" keyed with the secret. payload must be the
// body exactly as received, before any JSON decoding. A signing time further
// than the tolerance from now is rejected, which stops replays.
//
// It returns a [*WebhookSignatureError], matching [ErrInvalidSignature], when
// the signature does not verify.
func VerifyWebhook(payload []byte, signature, secret string, opts ...VerifyOption) error {
	cfg := verifyConfig{tolerance: DefaultWebhookTolerance}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.now.IsZero() {
		cfg.now = time.Now()
	}

	timestamp, signatures, err := parseSignature(signature)
	if err != nil {
		return err
	}
	if cfg.tolerance > 0 {
		// Compared against both edges of the window, so an extreme timestamp
		// cannot overflow a subtraction and slip through.
		now, window := cfg.now.Unix(), int64(cfg.tolerance/time.Second)
		if timestamp < now-window || timestamp > now+window {
			return &WebhookSignatureError{Reason: "the signature timestamp is outside the tolerance window"}
		}
	}

	expected := []byte(sign(payload, secret, timestamp))
	for _, candidate := range signatures {
		if hmac.Equal([]byte(candidate), expected) {
			return nil
		}
	}
	return &WebhookSignatureError{Reason: "the signature does not match the payload"}
}

// ConstructWebhookEvent verifies the signature, as [VerifyWebhook] does, and
// decodes the event.
//
// It returns a [*WebhookSignatureError] when the signature does not verify,
// and a JSON error when the verified body is not a webhook event.
func ConstructWebhookEvent(payload []byte, signature, secret string, opts ...VerifyOption) (*WebhookEvent, error) {
	if err := VerifyWebhook(payload, signature, secret, opts...); err != nil {
		return nil, err
	}
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("flightbyte: decoding the webhook event: %w", err)
	}
	return &event, nil
}

// SignWebhook signs payload the way the API does and returns the
// X-Flightbyte-Signature header value. Useful for testing your receiver.
func SignWebhook(payload []byte, secret string, at time.Time) string {
	t := at.Unix()
	return "t=" + strconv.FormatInt(t, 10) + ",v1=" + sign(payload, secret, t)
}

// WebhookHandler returns an http.Handler that receives FlightByte webhooks.
// It reads up to [MaxWebhookBodyBytes] of the body, verifies the signature
// with secret, decodes the event and calls handle.
//
// It answers 401 when the signature does not verify, 400 when the body is
// not an event, 500 when handle returns an error (so the API retries the
// delivery) and 200 otherwise. Retries carry the same event id, so make
// handle idempotent by deduplicating on [WebhookEvent.ID].
//
// The API retries a delivery that takes too long, so do slow work after
// returning, for example on a queue.
func WebhookHandler(secret string, handle func(context.Context, *WebhookEvent) error, opts ...VerifyOption) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxWebhookBodyBytes))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "could not read the body", http.StatusBadRequest)
			return
		}
		event, err := ConstructWebhookEvent(payload, r.Header.Get(SignatureHeader), secret, opts...)
		if errors.Is(err, ErrInvalidSignature) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		if err := handle(r.Context(), event); err != nil {
			http.Error(w, "handler failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
}

// parseSignature splits "t=<seconds>,v1=<hex>[,v1=<hex>...]".
func parseSignature(header string) (int64, []string, error) {
	if strings.TrimSpace(header) == "" {
		return 0, nil, &WebhookSignatureError{Reason: "the signature header is missing"}
	}
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "t":
			timestamp = strings.TrimSpace(value)
		case "v1":
			signatures = append(signatures, strings.ToLower(strings.TrimSpace(value)))
		}
	}
	t, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return 0, nil, &WebhookSignatureError{Reason: "the signature header has no valid timestamp"}
	}
	if len(signatures) == 0 {
		return 0, nil, &WebhookSignatureError{Reason: "the signature header has no v1 signature"}
	}
	return t, signatures, nil
}

func sign(payload []byte, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte{'.'})
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
