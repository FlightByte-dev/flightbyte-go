package flightbyte

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// vectors are signatures made by tool/webhook_vectors.mjs with Node's
// createHmac, exactly as the API signs deliveries.
type vectors struct {
	Secret    string `json:"secret"`
	Timestamp int64  `json:"timestamp"`
	Cases     []struct {
		File      string `json:"file"`
		Signature string `json:"signature"`
	} `json:"cases"`
}

func loadVectors(t *testing.T) vectors {
	t.Helper()
	var v vectors
	if err := json.Unmarshal(fixture(t, "webhook_vectors.json"), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Cases) == 0 {
		t.Fatal("no webhook vectors")
	}
	return v
}

func TestVerifyWebhookNodeVectors(t *testing.T) {
	v := loadVectors(t)
	at := time.Unix(v.Timestamp, 0)
	for _, c := range v.Cases {
		t.Run(c.File, func(t *testing.T) {
			payload := fixture(t, c.File)
			if err := VerifyWebhook(payload, c.Signature, v.Secret, WithNow(at)); err != nil {
				t.Fatalf("a Node signature does not verify: %v", err)
			}
			if got := SignWebhook(payload, v.Secret, at); got != c.Signature {
				t.Errorf("SignWebhook = %s\nNode signs     %s", got, c.Signature)
			}
			event, err := ConstructWebhookEvent(payload, c.Signature, v.Secret, WithNow(at))
			if err != nil || event.SubscriptionID != "sub_1" {
				t.Errorf("event %+v, err %v", event, err)
			}
		})
	}
}

func TestVerifyWebhookRejects(t *testing.T) {
	v := loadVectors(t)
	payload := fixture(t, v.Cases[1].File)
	good := v.Cases[1].Signature
	at := time.Unix(v.Timestamp, 0)
	_, sig, _ := strings.Cut(good, ",v1=")

	cases := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		opts      []VerifyOption
		reason    string
	}{
		{"wrong secret", payload, good, v.Secret + "x", nil, "does not match"},
		{"changed body", bytes.Replace(payload, []byte("A22"), []byte("A23"), 1), good, v.Secret, nil, "does not match"},
		{"re-encoded body", reencode(t, payload), good, v.Secret, nil, "does not match"},
		{"missing header", payload, "", v.Secret, nil, "missing"},
		{"no timestamp", payload, "v1=" + sig, v.Secret, nil, "no valid timestamp"},
		{"bad timestamp", payload, "t=soon,v1=" + sig, v.Secret, nil, "no valid timestamp"},
		{"no signature", payload, "t=1788858600", v.Secret, nil, "no v1 signature"},
		{"other scheme only", payload, "t=1788858600,v0=" + sig, v.Secret, nil, "no v1 signature"},
		{"garbage", payload, "nonsense", v.Secret, nil, "no valid timestamp"},
		{"too old", payload, good, v.Secret, []VerifyOption{WithNow(at.Add(5*time.Minute + time.Second))}, "tolerance"},
		{"from the future", payload, good, v.Secret, []VerifyOption{WithNow(at.Add(-6 * time.Minute))}, "tolerance"},
		{"tighter tolerance", payload, good, v.Secret, []VerifyOption{WithNow(at.Add(time.Minute)), WithTolerance(30 * time.Second)}, "tolerance"},
		{"extreme past timestamp", payload, "t=-9223372036854775808,v1=" + sig, v.Secret, nil, "tolerance"},
		{"extreme future timestamp", payload, "t=9223372036854775807,v1=" + sig, v.Secret, nil, "tolerance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]VerifyOption{WithNow(at)}, tc.opts...)
			err := VerifyWebhook(tc.payload, tc.signature, tc.secret, opts...)
			var sigErr *WebhookSignatureError
			if !errors.As(err, &sigErr) || !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("err = %v, want a *WebhookSignatureError", err)
			}
			if !strings.Contains(sigErr.Reason, tc.reason) {
				t.Errorf("Reason = %q, want it to mention %q", sigErr.Reason, tc.reason)
			}
			if _, err := ConstructWebhookEvent(tc.payload, tc.signature, tc.secret, opts...); !errors.Is(err, ErrInvalidSignature) {
				t.Errorf("ConstructWebhookEvent: %v", err)
			}
		})
	}
}

func TestVerifyWebhookAccepts(t *testing.T) {
	v := loadVectors(t)
	payload := fixture(t, v.Cases[0].File)
	good := v.Cases[0].Signature
	at := time.Unix(v.Timestamp, 0)
	ts, sig, _ := strings.Cut(good, ",")

	cases := map[string][]VerifyOption{
		"at the edge of the window": {WithNow(at.Add(5 * time.Minute))},
		"tolerance off":             {WithNow(at.Add(24 * time.Hour)), WithTolerance(0)},
	}
	for name, opts := range cases {
		if err := VerifyWebhook(payload, good, v.Secret, opts...); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	headers := map[string]string{
		"two v1, the second right":     ts + ",v1=" + strings.Repeat("0", 64) + "," + sig,
		"upper-case hex":               ts + ",v1=" + strings.ToUpper(strings.TrimPrefix(sig, "v1=")),
		"spaces around parts":          " " + ts + " , " + sig + " ",
		"an unknown scheme as well":    ts + ",v0=abc," + sig,
		"a part without an equal sign": ts + ",junk," + sig,
	}
	for name, header := range headers {
		if err := VerifyWebhook(payload, header, v.Secret, WithNow(at)); err != nil {
			t.Errorf("%s (%q): %v", name, header, err)
		}
	}
}

func TestSignWebhookRoundTrip(t *testing.T) {
	payload := []byte(`{"id":"evt_9","type":"subscription.test"}`)
	header := SignWebhook(payload, "whsec_roundtrip", time.Now())
	if err := VerifyWebhook(payload, header, "whsec_roundtrip"); err != nil {
		t.Error(err)
	}
}

func TestConstructWebhookEventBadJSON(t *testing.T) {
	payload := []byte(`not json`)
	header := SignWebhook(payload, "s", time.Now())
	_, err := ConstructWebhookEvent(payload, header, "s")
	if err == nil || errors.Is(err, ErrInvalidSignature) {
		t.Errorf("err = %v, want a decoding error", err)
	}
}

func TestWebhookHandler(t *testing.T) {
	const secret = "whsec_handler"
	payload := fixture(t, "webhook_flight_updated.json")

	cases := []struct {
		name       string
		method     string
		body       []byte
		signature  string
		handleErr  error
		wantStatus int
		wantCalled bool
	}{
		{"valid", http.MethodPost, payload, SignWebhook(payload, secret, time.Now()), nil, 200, true},
		{"handler fails", http.MethodPost, payload, SignWebhook(payload, secret, time.Now()), errors.New("db down"), 500, true},
		{"bad signature", http.MethodPost, payload, SignWebhook(payload, "other", time.Now()), nil, 401, false},
		{"no signature", http.MethodPost, payload, "", nil, 401, false},
		{"stale", http.MethodPost, payload, SignWebhook(payload, secret, time.Now().Add(-time.Hour)), nil, 401, false},
		{"not an event", http.MethodPost, []byte(`[1,2]`), SignWebhook([]byte(`[1,2]`), secret, time.Now()), nil, 400, false},
		{"GET", http.MethodGet, nil, "", nil, 405, false},
		{"too large", http.MethodPost, bytes.Repeat([]byte("a"), MaxWebhookBodyBytes+1), "t=1,v1=00", nil, 413, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got *WebhookEvent
			var reported error
			handler := WebhookHandler(secret, func(ctx context.Context, e *WebhookEvent) error {
				if ctx == nil {
					t.Error("nil context")
				}
				got = e
				return tc.handleErr
			}, WithErrorHandler(func(_ *http.Request, e *WebhookEvent, err error) {
				if e == nil || e.ID != "evt_2" {
					t.Errorf("the error handler got event %+v", e)
				}
				reported = err
			}))
			req := httptest.NewRequest(tc.method, "/hooks/flightbyte", bytes.NewReader(tc.body))
			if tc.signature != "" {
				req.Header.Set("x-flightbyte-signature", tc.signature)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if (got != nil) != tc.wantCalled {
				t.Errorf("handle called = %v, want %v", got != nil, tc.wantCalled)
			}
			if got != nil && (got.ID != "evt_2" || got.Changes[0].To != "A22") {
				t.Errorf("event = %+v", got)
			}
			if !errors.Is(reported, tc.handleErr) || (reported == nil) != (tc.handleErr == nil) {
				t.Errorf("reported error = %v, want %v", reported, tc.handleErr)
			}
		})
	}
}

func TestWebhookHandlerLogsHandlerErrorsByDefault(t *testing.T) {
	const secret = "whsec_handler"
	payload := fixture(t, "webhook_flight_updated.json")
	var logged bytes.Buffer
	defer log.SetOutput(log.Writer())
	log.SetOutput(&logged)

	handler := WebhookHandler(secret, func(context.Context, *WebhookEvent) error { return errors.New("db down") })
	req := httptest.NewRequest(http.MethodPost, "/hooks/flightbyte", bytes.NewReader(payload))
	req.Header.Set(SignatureHeader, SignWebhook(payload, secret, time.Now()))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(logged.String(), "evt_2") || !strings.Contains(logged.String(), "db down") {
		t.Errorf("logged %q, want the event id and the error", logged.String())
	}
}

func TestEmptySecretIsRejected(t *testing.T) {
	payload := []byte(`{"id":"evt_forged","type":"flight.updated"}`)
	for _, secret := range []string{"", "  "} {
		forged := SignWebhook(payload, secret, time.Now())
		if err := VerifyWebhook(payload, forged, secret); !errors.Is(err, ErrEmptySecret) {
			t.Errorf("VerifyWebhook with secret %q: err = %v, want ErrEmptySecret", secret, err)
		}
		if _, err := ConstructWebhookEvent(payload, forged, secret); !errors.Is(err, ErrEmptySecret) {
			t.Errorf("ConstructWebhookEvent with secret %q: err = %v, want ErrEmptySecret", secret, err)
		}
	}
}

func TestWebhookHandlerPanicsWithoutSecretOrHandle(t *testing.T) {
	mustPanic := func(name string, build func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		build()
	}
	mustPanic("an empty secret", func() {
		WebhookHandler("", func(context.Context, *WebhookEvent) error { return nil })
	})
	mustPanic("a nil handle", func() { WebhookHandler("whsec_x", nil) })
}

func TestConstructWebhookEventNeedsAnEvent(t *testing.T) {
	const secret = "whsec_x"
	for _, body := range []string{`{}`, `{"id":"evt_1"}`, `{"type":"flight.updated"}`} {
		payload := []byte(body)
		_, err := ConstructWebhookEvent(payload, SignWebhook(payload, secret, time.Now()), secret)
		if err == nil || errors.Is(err, ErrInvalidSignature) {
			t.Errorf("%s: err = %v, want a decoding error", body, err)
		}
	}
}

func FuzzParseSignature(f *testing.F) {
	f.Add("t=1788858600,v1=ed6073a6ed461e20c46f50b3268a548ad271f14613f48e7996662fdab68d1912")
	f.Add("t=1,v1=a,v1=b")
	f.Add("")
	f.Add(",,,=,t=,v1=")
	f.Add("t=-9223372036854775808,v1=00")
	f.Fuzz(func(t *testing.T, header string) {
		ts, sigs, err := parseSignature(header)
		if err != nil {
			if !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("parseSignature(%q) returned a foreign error %v", header, err)
			}
			return
		}
		if len(sigs) == 0 {
			t.Fatalf("parseSignature(%q) accepted a header without signatures", header)
		}
		// Verification must never panic, whatever the header holds.
		_ = VerifyWebhook([]byte("body"), header, "secret", WithNow(time.Unix(ts, 0)))
	})
}

func reencode(t *testing.T, payload []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(payload, &v); err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
