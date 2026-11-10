package flightbyte

import (
	"context"
	"net/url"
)

// FlightsService looks flights up. Reach it as client.Flights.
type FlightsService struct{ c *Client }

// FlightParams identifies a flight.
type FlightParams struct {
	// Carrier is the IATA or ICAO carrier code, for example "TK". Required.
	Carrier string
	// FlightNumber is the flight number, for example "1848". Required.
	FlightNumber string
	// Date is the local departure date at the origin, YYYY-MM-DD; see
	// [FormatDate]. Required.
	Date string
	// Origin selects a leg by its origin IATA code, when the flight number
	// operates several legs that day.
	Origin string
	// Destination narrows the leg further by its destination IATA code.
	Destination string
	// FlightID selects a leg by its upstream id, from [FlightLeg]. Ignored by
	// Timeline.
	FlightID string
	// Refresh skips the API's cache and asks upstream. Ignored by Timeline.
	Refresh bool
}

func (p FlightParams) path(extra ...string) ([]string, error) {
	if err := required("Carrier", p.Carrier, "FlightNumber", p.FlightNumber, "Date", p.Date); err != nil {
		return nil, err
	}
	return append([]string{"flights", p.Carrier, p.FlightNumber, p.Date}, extra...), nil
}

// Get looks a flight up. The API normalizes the carrier to upper case and
// strips leading zeros from the flight number.
//
// A flight number can operate several legs on one date. The flight lists them
// all in [Flight.Alternates]; set Origin (and optionally Destination) or a
// leg's FlightID to choose one. Without them, an ambiguous flight returns an
// [*APIError] matching [ErrBadRequest] whose Legs lists the choices.
//
// It returns an error matching [ErrNotFound] when the flight did not operate
// that day.
func (s *FlightsService) Get(ctx context.Context, params FlightParams) (*Flight, error) {
	path, err := params.path()
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	setIf(q, "origin", params.Origin)
	setIf(q, "destination", params.Destination)
	setIf(q, "flightId", params.FlightID)
	if params.Refresh {
		q.Set("refresh", "true")
	}
	var flight Flight
	if err := s.c.do(ctx, "GET", path, q, nil, nil, &flight); err != nil {
		return nil, err
	}
	return &flight, nil
}

// Timeline returns the flight's change history as recorded by the API,
// newest first.
func (s *FlightsService) Timeline(ctx context.Context, params FlightParams) (*FlightTimeline, error) {
	path, err := params.path("timeline")
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	setIf(q, "origin", params.Origin)
	setIf(q, "destination", params.Destination)
	var timeline FlightTimeline
	if err := s.c.do(ctx, "GET", path, q, nil, nil, &timeline); err != nil {
		return nil, err
	}
	return &timeline, nil
}
