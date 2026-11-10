// Command lookup looks a flight up and prints its status.
//
//	FLIGHTBYTE_API_KEY=frk_live_... go run ./examples/lookup TK 1848 2026-09-08
//
// FLIGHTBYTE_BASE_URL points it at another API root, for example a local one.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	flightbyte "github.com/FlightByte-dev/flightbyte-go"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) != 4 {
		log.Fatal("usage: lookup <carrier> <flight number> <YYYY-MM-DD>")
	}

	var opts []flightbyte.Option
	if base := os.Getenv("FLIGHTBYTE_BASE_URL"); base != "" {
		opts = append(opts, flightbyte.WithBaseURL(base))
	}
	client, err := flightbyte.NewClient(os.Getenv("FLIGHTBYTE_API_KEY"), opts...)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	flight, err := client.Flights.Get(ctx, flightbyte.FlightParams{
		Carrier:      os.Args[1],
		FlightNumber: os.Args[2],
		Date:         os.Args[3],
	})
	var apiErr *flightbyte.APIError
	switch {
	case errors.Is(err, flightbyte.ErrNotFound):
		log.Fatal("No such flight on that date.")
	case errors.As(err, &apiErr) && len(apiErr.Legs) > 0:
		fmt.Println("This flight number operates several legs that day:")
		for _, leg := range apiErr.Legs {
			fmt.Printf("  %s → %s at %s\n", leg.Origin, leg.Destination, leg.ScheduledDepartureLocal)
		}
		os.Exit(1)
	case err != nil:
		log.Fatal(err)
	}

	fmt.Printf("%s  %s → %s\n", flight.FlightKey, flight.Departure.Airport.IATA, flight.Arrival.Airport.IATA)
	fmt.Printf("Status     %s (%s)\n", flight.Status.Label, flight.Status.Phase)
	if t := flight.Departure.Times.BestGate(); t != nil {
		fmt.Printf("Departure  %s %s, gate %s\n", t.Local, t.Timezone, orDash(flight.Departure.Gate))
	}
	if t := flight.Arrival.Times.BestGate(); t != nil {
		fmt.Printf("Arrival    %s %s, baggage %s\n", t.Local, t.Timezone, orDash(flight.Arrival.Baggage))
	}
	if d := flight.Departure.DelayMinutes; d != nil && *d != 0 {
		fmt.Printf("Delay      %d min\n", *d)
	}
	if rl := client.LastRateLimit(); rl != nil && rl.Minute != nil {
		fmt.Printf("Quota      %d of %d left this minute\n", rl.Minute.Remaining, rl.Minute.Limit)
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
