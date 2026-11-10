package flightbyte_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	flightbyte "github.com/FlightByte-dev/flightbyte-go"
)

func ExampleNewClient() {
	client, err := flightbyte.NewClient(os.Getenv("FLIGHTBYTE_API_KEY"),
		flightbyte.WithTimeout(10*time.Second),
		flightbyte.WithUserAgent("my-app/1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleFlightsService_Get() {
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
	fmt.Println(flight.Status.Label, flight.Departure.Gate)
}

func ExampleSubscriptionsService_Create() {
	client, err := flightbyte.NewClient(os.Getenv("FLIGHTBYTE_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	created, err := client.Subscriptions.Create(context.Background(), flightbyte.CreateSubscriptionParams{
		Carrier:               "TK",
		FlightNumber:          "1848",
		Date:                  "2026-09-08",
		WebhookURL:            "https://example.com/hooks/flightbyte",
		Events:                []flightbyte.FlightChangeType{flightbyte.ChangeDepartureGate, flightbyte.ChangeDelayIncreased},
		DelayThresholdMinutes: flightbyte.Ptr(15),
		Metadata:              map[string]string{"booking": "42"},
		IdempotencyKey:        "booking-42", // safe to retry
	})
	if err != nil {
		log.Fatal(err)
	}
	// The full secret comes back only here: store it for your receiver.
	fmt.Println(created.ID, created.SecretHint)
}

func ExampleSubscriptionsService_All() {
	client, err := flightbyte.NewClient(os.Getenv("FLIGHTBYTE_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	params := flightbyte.ListSubscriptionsParams{Status: flightbyte.StatusActive}
	for sub, err := range client.Subscriptions.All(context.Background(), params) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(sub.FlightKey, sub.Status)
	}
}

func ExampleAPIError() {
	client, err := flightbyte.NewClient(os.Getenv("FLIGHTBYTE_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.Flights.Get(context.Background(), flightbyte.FlightParams{
		Carrier: "TK", FlightNumber: "1", Date: "2026-09-08",
	})

	var apiErr *flightbyte.APIError
	switch {
	case errors.Is(err, flightbyte.ErrNotFound):
		fmt.Println("the flight did not operate that day")
	case errors.Is(err, flightbyte.ErrBadRequest) && errors.As(err, &apiErr) && len(apiErr.Legs) > 0:
		for _, leg := range apiErr.Legs {
			fmt.Println("choose a leg:", leg.Origin, "→", leg.Destination)
		}
	case errors.Is(err, flightbyte.ErrRateLimited) && errors.As(err, &apiErr):
		fmt.Println("retry in", apiErr.RetryAfter)
	case err != nil:
		log.Fatal(err)
	}
}

func ExampleVerifyWebhook() {
	secret := "whsec_example"
	payload := []byte(`{"id":"evt_1","type":"subscription.test"}`)
	signature := flightbyte.SignWebhook(payload, secret, time.Now())

	err := flightbyte.VerifyWebhook(payload, signature, secret)
	fmt.Println("valid:", err == nil)

	err = flightbyte.VerifyWebhook(payload, signature, "the wrong secret")
	fmt.Println("rejected:", errors.Is(err, flightbyte.ErrInvalidSignature))
	// Output:
	// valid: true
	// rejected: true
}

func ExampleWebhookHandler() {
	secret := os.Getenv("FLIGHTBYTE_WEBHOOK_SECRET")
	http.Handle("POST /hooks/flightbyte", flightbyte.WebhookHandler(secret,
		func(ctx context.Context, event *flightbyte.WebhookEvent) error {
			// Retries carry the same event.ID: deduplicate on it.
			for _, change := range event.Changes {
				fmt.Println(event.FlightKey, change.Message)
			}
			return nil
		}))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
