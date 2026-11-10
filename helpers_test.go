package flightbyte

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// recorded is one request the test server received.
type recorded struct {
	Method string
	Path   string // escaped, as on the wire
	Query  string
	Header http.Header
	Body   string
}

// harness is a test server that records requests and answers with reply.
type harness struct {
	t      *testing.T
	server *httptest.Server
	client *Client

	mu       sync.Mutex
	requests []recorded
	reply    func(w http.ResponseWriter, r *http.Request)
}

func newHarness(t *testing.T, opts ...Option) *harness {
	t.Helper()
	h := &harness{t: t}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.requests = append(h.requests, recorded{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Query:  r.URL.RawQuery,
			Header: r.Header.Clone(),
			Body:   string(body),
		})
		reply := h.reply
		h.mu.Unlock()
		if reply == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		reply(w, r)
	}))
	t.Cleanup(h.server.Close)

	client, err := NewClient("frk_test_key", append([]Option{WithBaseURL(h.server.URL + "/v1")}, opts...)...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	h.client = client
	return h
}

// json makes the server answer with status and body.
func (h *harness) json(status int, body string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reply = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// last returns the latest recorded request.
func (h *harness) last() recorded {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) == 0 {
		h.t.Fatal("no request was sent")
	}
	return h.requests[len(h.requests)-1]
}

func (h *harness) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.requests)
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
