// Command webhook-server receives FlightByte webhooks and logs each change.
//
//	FLIGHTBYTE_WEBHOOK_SECRET=whsec_... go run ./examples/webhook-server
//
// It listens on :8080 (or $PORT) at POST /hooks/flightbyte. Point a
// subscription's webhookUrl there, through a tunnel when running locally.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	flightbyte "github.com/FlightByte-dev/flightbyte-go"
)

// seen remembers event ids, because retries carry the same id. A real
// receiver keeps them in its database.
var seen sync.Map

func main() {
	secret := os.Getenv("FLIGHTBYTE_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set FLIGHTBYTE_WEBHOOK_SECRET to the subscription's signing secret")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.Handle("/hooks/flightbyte", flightbyte.WebhookHandler(secret, handle))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on :%s/hooks/flightbyte", port)
	log.Fatal(server.ListenAndServe())
}

func handle(_ context.Context, event *flightbyte.WebhookEvent) error {
	if _, dup := seen.LoadOrStore(event.ID, true); dup {
		return nil
	}
	switch event.Type {
	case flightbyte.EventFlightUpdated:
		for _, change := range event.Changes {
			log.Printf("%s %s: %s", event.FlightKey, change.Type, change.Message)
		}
	default:
		log.Printf("%s %s", event.FlightKey, event.Type)
	}
	return nil
}
