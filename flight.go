package flightbyte

import "time"

// Flight is everything known about one flight leg at one moment.
//
// Strings the API may send as null are empty here; numbers and times it may
// send as null are pointers, so zero and unknown stay apart.
type Flight struct {
	// FlightKey is a stable key, for example "TK1848:2026-09-08" or
	// "TK1:2026-09-08@IST-JFK".
	FlightKey string `json:"flightKey"`
	// CarrierCode is the carrier code from the request, for example "TK".
	CarrierCode string `json:"carrierCode"`
	// FlightNumber is the flight number without leading zeros.
	FlightNumber string `json:"flightNumber"`
	// Date is the local departure date at the origin, YYYY-MM-DD.
	Date string `json:"date"`
	// ProviderFlightID is the upstream flight id.
	ProviderFlightID string `json:"providerFlightId"`
	// FetchedAt is when the API fetched this data from upstream.
	FetchedAt time.Time `json:"fetchedAt"`
	// Carrier is the marketing carrier.
	Carrier Carrier `json:"carrier"`
	// OperatedBy is the operating carrier's name, when different.
	OperatedBy string `json:"operatedBy"`
	// Codeshares lists the codeshare flight numbers.
	Codeshares []Codeshare `json:"codeshares"`
	// Departure is the departure side.
	Departure Endpoint `json:"departure"`
	// Arrival is the arrival side.
	Arrival Endpoint `json:"arrival"`
	// Diverted is where the flight actually went, when diverted.
	Diverted *Endpoint `json:"diverted"`
	// Status is the flight's status.
	Status FlightStatus `json:"status"`
	// Equipment is the aircraft.
	Equipment Equipment `json:"equipment"`
	// Duration holds the block times.
	Duration FlightDuration `json:"duration"`
	// Timeline is the upstream history, newest first.
	Timeline []TimelineEntry `json:"timeline"`
	// Alternates lists every leg flown under this flight number on this date.
	Alternates []FlightLeg `json:"alternates"`
	// Fingerprint is a hash over the change-relevant fields.
	Fingerprint string `json:"fingerprint"`
	// Cached reports whether a lookup was answered from the API's cache.
	// Always false on flights inside webhook events.
	Cached bool `json:"cached"`
}

// Carrier is the marketing carrier.
type Carrier struct {
	// IATA is the IATA code, for example "TK".
	IATA string `json:"iata"`
	// ICAO is the ICAO code, for example "THY".
	ICAO string `json:"icao"`
	// Name is the airline name.
	Name string `json:"name"`
}

// Codeshare is a codeshare flight number sold on the same flight.
type Codeshare struct {
	// Carrier is the carrier code.
	Carrier string `json:"carrier"`
	// Name is the carrier name.
	Name string `json:"name"`
	// FlightNumber is the flight number.
	FlightNumber string `json:"flightNumber"`
}

// Endpoint is the departure or arrival side of a flight.
type Endpoint struct {
	// Airport is the airport.
	Airport Airport `json:"airport"`
	// Terminal is the terminal.
	Terminal string `json:"terminal"`
	// Gate is the gate.
	Gate string `json:"gate"`
	// Baggage is the baggage carousel, on arrival.
	Baggage string `json:"baggage"`
	// Times holds the gate and runway times.
	Times EndpointTimes `json:"times"`
	// BestTimeUTC is the best known time: actual, else estimated, else
	// scheduled.
	BestTimeUTC *time.Time `json:"bestTimeUtc"`
	// DelayMinutes is the gate delay in minutes; negative when early.
	DelayMinutes *int `json:"delayMinutes"`
}

// Airport is an airport.
type Airport struct {
	// IATA is the IATA code, for example "IST".
	IATA string `json:"iata"`
	// ICAO is the ICAO code, for example "LTFM".
	ICAO string `json:"icao"`
	// Code is the upstream's own airport code; it may differ from IATA.
	Code string `json:"code"`
	// Name is the airport name.
	Name string `json:"name"`
	// City is the city.
	City string `json:"city"`
	// State is the state or region.
	State string `json:"state"`
	// Country is the country.
	Country CountryRef `json:"country"`
	// Timezone is the IANA time zone, for example "Europe/Istanbul".
	Timezone string `json:"timezone"`
}

// EndpointTimes holds the gate and runway times of a departure or an arrival.
type EndpointTimes struct {
	// ScheduledGate is the scheduled gate time.
	ScheduledGate *FlightTime `json:"scheduledGate"`
	// EstimatedGate is the estimated gate time.
	EstimatedGate *FlightTime `json:"estimatedGate"`
	// ActualGate is the actual gate time.
	ActualGate *FlightTime `json:"actualGate"`
	// ScheduledRunway is the scheduled runway time.
	ScheduledRunway *FlightTime `json:"scheduledRunway"`
	// EstimatedRunway is the estimated runway time.
	EstimatedRunway *FlightTime `json:"estimatedRunway"`
	// ActualRunway is the actual runway time.
	ActualRunway *FlightTime `json:"actualRunway"`
}

// BestGate returns the best known gate time: actual, else estimated, else
// scheduled. It returns nil when none is known.
func (t EndpointTimes) BestGate() *FlightTime {
	switch {
	case t.ActualGate != nil:
		return t.ActualGate
	case t.EstimatedGate != nil:
		return t.EstimatedGate
	}
	return t.ScheduledGate
}

// FlightTime is a point in time, in UTC and in the airport's local time.
type FlightTime struct {
	// UTC is the instant.
	UTC time.Time `json:"utc"`
	// Local is the airport-local wall time without an offset, for example
	// "2026-09-08T12:10:00.000". It is a string because it carries no offset;
	// pair it with the airport's time zone.
	Local string `json:"local"`
	// Timezone is the time zone abbreviation, for example "EEST".
	Timezone string `json:"timezone"`
}

// FlightStatus is the status of a flight.
type FlightStatus struct {
	// Code is the upstream status code: S, A, L, C, D, R, U or NO.
	Code string `json:"code"`
	// Label is the human-readable status, for example "Arrived".
	Label string `json:"label"`
	// Description is a longer description, for example "On time".
	Description string `json:"description"`
	// Color is a suggested color, for example "green".
	Color string `json:"color"`
	// Phase is the lifecycle phase.
	Phase FlightPhase `json:"phase"`
	// Delayed reports whether the flight is delayed.
	Delayed bool `json:"delayed"`
	// Cancelled reports whether the flight is cancelled.
	Cancelled bool `json:"cancelled"`
	// Diverted reports whether the flight is diverted.
	Diverted bool `json:"diverted"`
	// DepartureGateDelayMinutes is the departure gate delay in minutes.
	DepartureGateDelayMinutes *int `json:"departureGateDelayMinutes"`
	// ArrivalGateDelayMinutes is the arrival gate delay in minutes.
	ArrivalGateDelayMinutes *int `json:"arrivalGateDelayMinutes"`
}

// Equipment is the aircraft.
type Equipment struct {
	// TailNumber is the registration, for example "TC-LTP".
	TailNumber string `json:"tailNumber"`
	// IATA is the IATA aircraft type, for example "32Q".
	IATA string `json:"iata"`
	// Name is the aircraft name, for example "Airbus A321neo".
	Name string `json:"name"`
	// Source is where the data came from, for example "Actual".
	Source string `json:"source"`
}

// FlightDuration holds block times in minutes.
type FlightDuration struct {
	// ScheduledMinutes is the scheduled duration.
	ScheduledMinutes *int `json:"scheduledMinutes"`
	// EstimatedMinutes is the estimated duration.
	EstimatedMinutes *int `json:"estimatedMinutes"`
	// ActualMinutes is the actual duration.
	ActualMinutes *int `json:"actualMinutes"`
}

// TimelineEntry is one entry of a flight's upstream history.
type TimelineEntry struct {
	// OccurredAt is when it happened.
	OccurredAt time.Time `json:"occurredAt"`
	// Title is the title, for example "Time Adjustment".
	Title string `json:"title"`
	// Source is the source, for example "Airline".
	Source string `json:"source"`
	// Events describes what changed, as sentences.
	Events []string `json:"events"`
}

// FlightLeg is a leg listed for the same flight number on the same date.
//
// Flight numbers are not unique per day, so the API lists every leg and marks
// the one the flight describes with Selected.
type FlightLeg struct {
	// ProviderFlightID is the upstream flight id; pass it as
	// [FlightParams.FlightID] to select this leg.
	ProviderFlightID string `json:"providerFlightId"`
	// Carrier is the operating carrier code.
	Carrier string `json:"carrier"`
	// DepartureIATA is the origin IATA code.
	DepartureIATA string `json:"departureIata"`
	// ArrivalIATA is the destination IATA code.
	ArrivalIATA string `json:"arrivalIata"`
	// ScheduledDepartureLocal is the scheduled local departure, "HH:mm".
	ScheduledDepartureLocal string `json:"scheduledDepartureLocal"`
	// ScheduledArrivalLocal is the scheduled local arrival, "HH:mm".
	ScheduledArrivalLocal string `json:"scheduledArrivalLocal"`
	// Selected reports whether this is the leg the flight describes.
	Selected bool `json:"selected"`
}

// FlightTimeline is the response of [FlightsService.Timeline].
type FlightTimeline struct {
	// FlightKey is the flight's key.
	FlightKey string `json:"flightKey"`
	// Count is the number of entries.
	Count int `json:"count"`
	// Entries holds the history entries, newest first.
	Entries []TimelineEntry `json:"data"`
}
