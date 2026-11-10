// Package flightbyte is the official Go client for the FlightByte flight
// status API (https://flightbyte.dev).
//
// Look flights up, watch them with signed webhooks, and verify every delivery:
//
//	client, err := flightbyte.NewClient(os.Getenv("FLIGHTBYTE_API_KEY"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	flight, err := client.Flights.Get(ctx, flightbyte.FlightParams{
//		Carrier:      "TK",
//		FlightNumber: "1848",
//		Date:         "2026-09-08", // local departure date at the origin
//	})
//
// Every method takes a [context.Context] and returns an error. Errors from the
// API are [*APIError] values that match sentinels such as [ErrNotFound] and
// [ErrRateLimited] with [errors.Is]; network failures are [*ConnectionError].
//
// Webhooks are verified with [VerifyWebhook] or [ConstructWebhookEvent], or
// received with [WebhookHandler], which does both.
//
// A [Client] is safe for concurrent use by multiple goroutines.
package flightbyte
