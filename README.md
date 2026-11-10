<p align="center">
  <a href="https://flightbyte.dev">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
      <img src="assets/logo-light.svg" alt="FlightByte" width="340">
    </picture>
  </a>
</p>

<h3 align="center">FlightByte for Go</h3>

<p align="center">
  The official Go client for the FlightByte flight status API.<br>
  Look flights up, watch them with signed webhooks and verify every delivery,<br>
  from a web service, a worker or a CLI.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/FlightByte-dev/flightbyte-go"><img src="https://img.shields.io/badge/go.dev-reference-EF6A1A?style=flat-square&logo=go&logoColor=white" alt="Go reference"></a>
  <img src="https://img.shields.io/badge/Go-1.23%2B-121212?style=flat-square&logo=go&logoColor=white" alt="Go 1.23+">
  <img src="https://img.shields.io/badge/dependencies-none-EF6A1A?style=flat-square" alt="No dependencies">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-121212?style=flat-square" alt="MIT license"></a>
</p>

<p align="center">
  <a href="https://flightbyte.dev/docs"><b>Docs</b></a> ·
  <a href="https://app.flightbyte.dev/signup?utm_source=github&utm_medium=readme&utm_campaign=go-sdk"><b>Get a free key</b></a> ·
  <a href="https://flightbyte.dev/docs/reference"><b>API reference</b></a> ·
  <a href="https://flightbyte.dev/docs/webhooks"><b>Webhook docs</b></a> ·
  <a href="https://t.me/flightbyte"><b>Support</b></a>
</p>

---

## Quick start

1. Get a free API key at [app.flightbyte.dev/signup](https://app.flightbyte.dev/signup?utm_source=github&utm_medium=readme&utm_campaign=go-sdk).
2. Add the module:

   ```bash
   go get github.com/FlightByte-dev/flightbyte-go
   ```

3. Look a flight up:

   ```go
   package main

   import (
   	"context"
   	"fmt"
   	"log"
   	"os"

   	flightbyte "github.com/FlightByte-dev/flightbyte-go"
   )

   func main() {
   	client, err := flightbyte.NewClient(os.Getenv("FLIGHTBYTE_API_KEY"))
   	if err != nil {
   		log.Fatal(err)
   	}

   	flight, err := client.Flights.Get(context.Background(), flightbyte.FlightParams{
   		Carrier:      "TK",
   		FlightNumber: "1848",
   		Date:         "2026-09-08", // local departure date at the origin
   	})
   	if err != nil {
   		log.Fatal(err)
   	}

   	fmt.Println(flight.Status.Label)                     // Scheduled
   	fmt.Println(flight.Departure.Gate)                   // A11
   	fmt.Println(flight.Departure.Times.BestGate().Local) // 2026-09-08T12:25:00.000
   }
   ```

`flightbyte.FormatDate(time.Time)` turns a date into the `YYYY-MM-DD` the API expects. To try it from a terminal, run the bundled example:

```bash
export FLIGHTBYTE_API_KEY=frk_live_...
go run github.com/FlightByte-dev/flightbyte-go/examples/lookup@latest TK 1 2026-11-11
```

```text
TK1:2026-11-11@SAW-AMS  SAW → AMS
Status     Scheduled (scheduled)
Departure  2026-11-11T10:40:00.000 +03, gate -
Arrival    2026-11-11T12:35:00.000 CET, baggage -
Quota      23 of 30 left this minute
```

TK1 flies two legs that day, and without an `Origin` the API picked one; `flight.Alternates` lists both.

## What you can do

| You want to | You call |
| --- | --- |
| *Show a passenger their gate and delay* | `client.Flights.Get(ctx, ...)` returns the live snapshot: phase, gate, terminal, baggage belt, scheduled, estimated and actual times, delay, aircraft and codeshares. |
| *Look up TK1 tomorrow, which flies two legs* | Set `Origin: "IST"`. Without it the flight lists every leg in `Alternates`, and a subscription refuses to guess: the `*APIError` carries the `Legs` to choose from. |
| *Be told when anything changes* | `client.Subscriptions.Create(ctx, ...)` starts the watch and returns the signing secret, shown once. |
| *Find out why webhooks stopped arriving* | `client.Subscriptions.Get(ctx, id)` and `Deliveries(ctx, id, 0)` show the status and the HTTP code and error of every recent attempt. |
| *Trust a webhook on your server* | `flightbyte.WebhookHandler(secret, fn)` verifies the signature against the raw body, decodes the event and calls you. |

## API

**Flights**: read-only.

| Method | What it does |
| --- | --- |
| `Flights.Get` | Live status of one flight leg, and every leg the number flies that day.<br><sub>`GET /v1/flights/{carrier}/{flightNumber}/{date}`</sub> |
| `Flights.Timeline` | The airline's own change history for a leg, newest first.<br><sub>`GET /v1/flights/{carrier}/{flightNumber}/{date}/timeline`</sub> |

**Subscriptions**

| Method | What it does | Access |
| --- | --- | --- |
| `Subscriptions.Create` | Watch a flight and get a signed webhook for every change. Counts as a watched flight on your plan; set `IdempotencyKey` so a retry never counts twice.<br><sub>`POST /v1/subscriptions`</sub> | writes |
| `Subscriptions.List` | One page of your subscriptions, filtered by status, carrier, number or date.<br><sub>`GET /v1/subscriptions`</sub> | read-only |
| `Subscriptions.All` | Every matching subscription as an `iter.Seq2`, fetched page by page as you range over it.<br><sub>`GET /v1/subscriptions`</sub> | read-only |
| `Subscriptions.Get` | One subscription with its status and delivery counters.<br><sub>`GET /v1/subscriptions/{id}`</sub> | read-only |
| `Subscriptions.Update` | Change the webhook URL, event filter, delay threshold, headers or metadata.<br><sub>`PATCH /v1/subscriptions/{id}`</sub> | writes |
| `Subscriptions.Pause` | Stop polling and deliveries without losing anything.<br><sub>`POST /v1/subscriptions/{id}/pause`</sub> | writes |
| `Subscriptions.Resume` | Resume a paused or auto-suspended subscription.<br><sub>`POST /v1/subscriptions/{id}/resume`</sub> | writes |
| `Subscriptions.Delete` | Delete a subscription for good.<br><sub>`DELETE /v1/subscriptions/{id}`</sub> | **destructive** |
| `Subscriptions.SendTest` | Send a signed `subscription.test` webhook to the subscription's URL.<br><sub>`POST /v1/subscriptions/{id}/test`</sub> | writes |
| `Subscriptions.Deliveries` | Recent delivery attempts with HTTP status and errors.<br><sub>`GET /v1/subscriptions/{id}/deliveries`</sub> | read-only |
| `Subscriptions.Events` | Recent events with the exact payloads that were sent.<br><sub>`GET /v1/subscriptions/{id}/events`</sub> | read-only |
| `Subscriptions.Secret` | Reveal the signing secret again.<br><sub>`GET /v1/subscriptions/{id}/secret`</sub> | read-only |
| `Subscriptions.RotateSecret` | Generate a new signing secret. Previous signatures stop validating.<br><sub>`POST /v1/subscriptions/{id}/secret/rotate`</sub> | writes |

**Countries**: read-only.

| Method | What it does |
| --- | --- |
| `Countries.List` | Every country with its name in any language, emoji and flag URLs.<br><sub>`GET /v1/countries`</sub> |
| `Countries.Get` | One country by its ISO 3166-1 alpha-2 code.<br><sub>`GET /v1/countries/{code}`</sub> |
| `Countries.FlagURL` | The URL of a country's SVG flag, 4:3 or square. Needs no key, so it can go straight into an `<img>`.<br><sub>`GET /v1/flags/{code}.svg`</sub> |

Every method takes a `context.Context` first. Struct fields carry the API's own names in Go form (`Carrier`, `FlightNumber`, `Date`, `Origin`, `WebhookURL`, `IdempotencyKey`, …), and the models carry every field of the API's JSON. The full snapshot is documented under [Flights](https://flightbyte.dev/docs/flights).

## Watch a flight

```go
created, err := client.Subscriptions.Create(ctx, flightbyte.CreateSubscriptionParams{
	Carrier:      "TK",
	FlightNumber: "1848",
	Date:         "2026-09-08",
	WebhookURL:   "https://api.example.com/hooks/flights",
	Events: []flightbyte.FlightChangeType{
		flightbyte.ChangeDepartureGate,
		flightbyte.ChangeDelayIncreased,
		flightbyte.ChangeDeparted,
	},
	Metadata:       map[string]string{"bookingId": "BK-1042"},
	IdempotencyKey: "BK-1042", // safe to retry
})
if err != nil {
	return err
}

// Shown in full only now: store it where your webhook receiver can read it.
signingSecret := created.Secret
```

FlightByte only delivers to public HTTPS addresses. To receive on your machine, expose the port with a tunnel (`cloudflared tunnel --url http://localhost:8080` or `ngrok http 8080`) and use that URL. `Subscriptions.SendTest(ctx, id)` sends a signed `subscription.test` whenever you want to see the path work.

## Receive webhooks

A receiver has to do four things, in this order: verify the signature against the raw body, deduplicate on the event id, answer `2xx` before doing the work, and branch on the event type. `WebhookHandler` does the first and gives you an ordinary `http.Handler`:

```go
mux := http.NewServeMux()
mux.Handle("POST /hooks/flights", flightbyte.WebhookHandler(signingSecret,
	func(ctx context.Context, event *flightbyte.WebhookEvent) error {
		// A retry carries the same id: answer it, but don't handle it twice.
		if !markHandled(event.ID) {
			return nil
		}
		// Answer first: FlightByte treats a delivery as failed after 10 seconds.
		jobs <- event
		return nil
	}))
```

It answers `401` for a bad signature, `400` for a body that is not an event, `500` when your function returns an error (so FlightByte retries) and `200` otherwise. Then, off the request path:

```go
for event := range jobs {
	switch event.Type {
	case flightbyte.EventFlightUpdated:
		for _, change := range event.Changes {
			log.Printf("%s %s: %s", event.FlightKey, change.Type, change.Message)
		}
	case flightbyte.EventSubscriptionCompleted:
		log.Printf("%s finished", event.FlightKey)
	}
}
```

With another router, read the body yourself and call `flightbyte.ConstructWebhookEvent(payload, r.Header.Get(flightbyte.SignatureHeader), secret)`. A runnable receiver is in [examples/webhook-server](examples/webhook-server).

<details>
<summary><b>How the signature works</b></summary>

<br>

```text
x-flightbyte-signature: t=1789305971,v1=5f1c9a...e3b2
v1 = HMAC-SHA256(secret, "<t>." + <raw body>)
```

`VerifyWebhook` computes the HMAC over the body you received, not over JSON you decoded and encoded again, compares it in constant time with `hmac.Equal`, accepts any of several `v1` values, and rejects a `t` more than five minutes from now. `SignWebhook` produces the same header, for testing your receiver. The test suite checks both against signatures made by the API's own signer, and fuzzes the header parser.

The reasons behind each step are in [Verifying webhook signatures](https://flightbyte.dev/guides/verifying-webhook-signatures), and reference receivers in Node.js, Python and Go are in [flightbyte-webhook-examples](https://github.com/FlightByte-dev/flightbyte-webhook-examples).

</details>

## Errors

A failed request returns an `*APIError` that matches a sentinel with `errors.Is`, so you branch on the status without comparing numbers, and read the details with `errors.As`.

| Sentinel | Status | When, and what to do |
| --- | --- | --- |
| `ErrBadRequest` | `400` | Invalid input; `Messages` has one line per field. For a number that flies twice, `Legs` lists them: set one's `Origin`. |
| `ErrUnauthorized` | `401` | Missing, invalid, revoked or expired API key. |
| `ErrPaymentRequired` | `402` | No watched flights left this period. `Code` says why (`watched_flight_limit` or `overage_cap_reached`); `ResetsAt` says when the count resets. |
| `ErrForbidden` | `403` | The account is suspended, or an OAuth token lacks the scope (`Code`). |
| `ErrNotFound` | `404` | The flight did not operate that day, or no such subscription. |
| `ErrRateLimited` | `429` | A minute, day or month quota is used up. Wait `RetryAfter`. |
| `ErrUpstream` | `502` | The flight data provider failed. Retry shortly. |
| `ErrServer` | `5xx` | Any server error, `502` included. `RequestID` is what support asks for. |
| `*ConnectionError` | none | No response at all: network error, timeout or a cancelled context. It unwraps to the cause. |

```go
_, err := client.Subscriptions.Create(ctx, params)

var apiErr *flightbyte.APIError
switch {
case errors.Is(err, flightbyte.ErrBadRequest) && errors.As(err, &apiErr) && len(apiErr.Legs) > 0:
	for _, leg := range apiErr.Legs {
		fmt.Println("Pick a leg:", leg.Origin, "→", leg.Destination)
	}
case errors.Is(err, flightbyte.ErrPaymentRequired) && errors.As(err, &apiErr):
	fmt.Println("No watched flights left until", apiErr.ResetsAt)
case errors.Is(err, flightbyte.ErrRateLimited) && errors.As(err, &apiErr):
	time.Sleep(apiErr.RetryAfter)
case err != nil:
	return err
}
```

Every status is explained in the [errors docs](https://flightbyte.dev/docs/errors).

## Built for Go services

- **No dependencies.** Only the standard library: `net/http`, `encoding/json` and `crypto/hmac`. Nothing to audit but this module.
- **One client, many goroutines.** A `*Client` is safe for concurrent use; create it once and share it. Every call honours its context's deadline and cancellation.
- **New API values don't break old binaries.** Phases, statuses, change types and event types are string types, so a value added after this release arrives as is. `Known()` tells you whether this version has a constant for it.
- **Zero stays apart from unknown.** Delays, durations and times the API may leave out are pointers, so a delay of `0` and no delay reading are different things.
- **Errors say what to do next.** A number that flies twice comes back with its legs, a `429` with how long to wait, a `402` with the reason and when the allowance resets.
- **Quotas are visible.** `client.LastRateLimit()` returns the minute, day and month windows from the latest response's `X-RateLimit-*` headers.

## Configuration

```go
client, err := flightbyte.NewClient(apiKey,
	flightbyte.WithBaseURL("http://localhost:3000/v1"),
	flightbyte.WithHTTPClient(myHTTPClient),
	flightbyte.WithTimeout(10*time.Second),
	flightbyte.WithUserAgent("my-app/1.2"),
)
```

| Option | Default | Meaning |
| --- | --- | --- |
| `apiKey` | required | Your API key, sent as `X-API-Key`. Keep it on your server. |
| `WithBaseURL` | `https://api.flightbyte.dev/v1` | API base URL, e.g. `http://localhost:3000/v1` for a local API. |
| `WithHTTPClient` | a new `http.Client` | Bring your own for retries, proxies, logging or tests. It is copied, not changed, and a redirect to another host or from https to http is never followed, so your key stays with the API. |
| `WithTimeout` | 30 seconds | Bounds each request, response body included. `0` leaves it to the context. |
| `WithUserAgent` | none | Prepended to `flightbyte-go/<version>` in the `User-Agent` header. |

## Development

```bash
go test -race ./...                # everything, against a local test server
go vet ./...
gofmt -l .                         # prints nothing when formatted
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

The tests never touch the real API and need no key. They cover every method's request mapping, the error mapping for each status, the models against fixtures shaped like the API's JSON (decoded strictly, so a field the models lack fails the build), and webhook verification against signatures made with Node's `createHmac` exactly as the API signs them. CI runs the same commands on Go 1.23 and the latest release.

How to add a field, an enum value or an endpoint is in [CONTRIBUTING.md](CONTRIBUTING.md). Report security issues by email, as [SECURITY.md](SECURITY.md) explains, never in a public issue.

<details>
<summary><b>Project layout</b></summary>

```text
client.go                  NewClient and its options
request.go                 requests, headers, timeouts, error mapping
flights.go                 Flights.Get and Timeline
subscriptions.go           Subscriptions, including the All iterator
countries.go               Countries and flag URLs
flight.go subscription.go  the models
event.go country.go
enums.go                   phases, statuses, change and event types
errors.go                  APIError, the sentinels and ConnectionError
webhook.go                 VerifyWebhook, ConstructWebhookEvent, SignWebhook, WebhookHandler
ratelimit.go               X-RateLimit-* headers
examples/lookup            looks a flight up from the terminal
examples/webhook-server    a receiver on net/http
testdata/                  API-shaped JSON and signed webhook vectors
tool/webhook_vectors.mjs   regenerates the signed vectors with Node, as the API signs
tool/release_notes.sh      a version's CHANGELOG.md section, for the GitHub release
```

</details>

<details>
<summary><b>Releasing</b></summary>

1. Set `Version` in `version.go` to the new version.
2. Give the version its own `## <version>` section in `CHANGELOG.md`.
3. Push a tag such as `v0.1.1`.

A Go module is released by its tag; there is nothing to upload. The release workflow checks that the tag matches `Version` and that the changelog has the section, runs the tests, and creates the GitHub release from the changelog section. Run it by hand from the Actions tab to rehearse without releasing.

</details>

## License

[MIT](LICENSE)

<br>

<p align="center">
  <a href="https://flightbyte.dev"><img src="assets/icon.svg" alt="FlightByte" width="36"></a>
  <br>
  <sub><b>FlightByte</b> · Flight intelligence, byte by byte · <a href="https://flightbyte.dev">flightbyte.dev</a></sub>
</p>
