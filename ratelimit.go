package flightbyte

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RateLimit is the quota state sent with a response, read from the
// X-RateLimit-{Limit,Remaining,Reset}-{Minute,Day,Month} headers. A window
// the key has no limit on is nil.
type RateLimit struct {
	// Minute is the per-minute window.
	Minute *RateLimitWindow
	// Day is the per-day window.
	Day *RateLimitWindow
	// Month is the per-month window.
	Month *RateLimitWindow
}

// RateLimitWindow is one quota window.
type RateLimitWindow struct {
	// Limit is the number of requests the window allows.
	Limit int
	// Remaining is the number of requests left in the window.
	Remaining int
	// ResetAt is when the window resets; zero when not reported.
	ResetAt time.Time
}

// String formats the window as "remaining/limit".
func (w RateLimitWindow) String() string {
	return fmt.Sprintf("%d/%d", w.Remaining, w.Limit)
}

// parseRateLimit returns nil when the headers report no window.
func parseRateLimit(h http.Header) *RateLimit {
	rl := &RateLimit{
		Minute: parseWindow(h, "Minute"),
		Day:    parseWindow(h, "Day"),
		Month:  parseWindow(h, "Month"),
	}
	if rl.Minute == nil && rl.Day == nil && rl.Month == nil {
		return nil
	}
	return rl
}

func parseWindow(h http.Header, name string) *RateLimitWindow {
	limit, err := strconv.Atoi(h.Get("X-RateLimit-Limit-" + name))
	if err != nil {
		return nil
	}
	remaining, err := strconv.Atoi(h.Get("X-RateLimit-Remaining-" + name))
	if err != nil {
		return nil
	}
	w := &RateLimitWindow{Limit: limit, Remaining: remaining}
	if reset, err := strconv.ParseInt(h.Get("X-RateLimit-Reset-"+name), 10, 64); err == nil {
		w.ResetAt = time.Unix(reset, 0).UTC()
	}
	return w
}
