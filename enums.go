package flightbyte

// The enum types below are strings. A value the API adds after this package
// was released decodes as is, so nothing is lost; Known reports whether this
// package has a constant for it.

// FlightPhase is the coarse lifecycle bucket of a flight.
type FlightPhase string

// Flight phases.
const (
	PhaseScheduled  FlightPhase = "scheduled"
	PhaseBoarding   FlightPhase = "boarding"
	PhaseDeparted   FlightPhase = "departed"
	PhaseEnRoute    FlightPhase = "en_route"
	PhaseLanded     FlightPhase = "landed"
	PhaseArrived    FlightPhase = "arrived"
	PhaseCancelled  FlightPhase = "cancelled"
	PhaseDiverted   FlightPhase = "diverted"
	PhaseRedirected FlightPhase = "redirected"
	PhaseUnknown    FlightPhase = "unknown"
)

// IsTerminal reports whether the flight is finished: arrived, cancelled,
// diverted or redirected.
func (p FlightPhase) IsTerminal() bool {
	switch p {
	case PhaseArrived, PhaseCancelled, PhaseDiverted, PhaseRedirected:
		return true
	}
	return false
}

// Known reports whether p is one of the constants above.
func (p FlightPhase) Known() bool {
	switch p {
	case PhaseScheduled, PhaseBoarding, PhaseDeparted, PhaseEnRoute, PhaseLanded,
		PhaseArrived, PhaseCancelled, PhaseDiverted, PhaseRedirected, PhaseUnknown:
		return true
	}
	return false
}

// SubscriptionStatus is the lifecycle of a subscription.
type SubscriptionStatus string

// Subscription statuses.
const (
	// StatusActive: polling the flight and delivering webhooks.
	StatusActive SubscriptionStatus = "active"
	// StatusPending: accepted, but the flight is not published upstream yet.
	StatusPending SubscriptionStatus = "pending"
	// StatusPaused: paused by you; nothing is polled or delivered.
	StatusPaused SubscriptionStatus = "paused"
	// StatusCompleted: the flight finished and the grace window passed.
	StatusCompleted SubscriptionStatus = "completed"
	// StatusSuspended: disabled by the API, for example after repeated
	// delivery failures.
	StatusSuspended SubscriptionStatus = "suspended"
)

// Known reports whether s is one of the constants above.
func (s SubscriptionStatus) Known() bool {
	switch s {
	case StatusActive, StatusPending, StatusPaused, StatusCompleted, StatusSuspended:
		return true
	}
	return false
}

// FlightChangeType is the kind of change a flight.updated webhook reports,
// and what a subscription's Events filter is made of.
type FlightChangeType string

// Flight change types.
const (
	ChangeStatus             FlightChangeType = "status.changed"
	ChangeDeparted           FlightChangeType = "flight.departed"
	ChangeArrived            FlightChangeType = "flight.arrived"
	ChangeCancelled          FlightChangeType = "flight.cancelled"
	ChangeDiverted           FlightChangeType = "flight.diverted"
	ChangeReinstated         FlightChangeType = "flight.reinstated"
	ChangeDepartureGate      FlightChangeType = "departure.gate.changed"
	ChangeDepartureTerminal  FlightChangeType = "departure.terminal.changed"
	ChangeArrivalGate        FlightChangeType = "arrival.gate.changed"
	ChangeArrivalTerminal    FlightChangeType = "arrival.terminal.changed"
	ChangeBaggage            FlightChangeType = "arrival.baggage.changed"
	ChangeDepartureScheduled FlightChangeType = "departure.scheduled.changed"
	ChangeDepartureEstimated FlightChangeType = "departure.estimated.changed"
	ChangeDepartureActual    FlightChangeType = "departure.actual.changed"
	ChangeArrivalScheduled   FlightChangeType = "arrival.scheduled.changed"
	ChangeArrivalEstimated   FlightChangeType = "arrival.estimated.changed"
	ChangeArrivalActual      FlightChangeType = "arrival.actual.changed"
	ChangeDelayIncreased     FlightChangeType = "delay.increased"
	ChangeDelayDecreased     FlightChangeType = "delay.decreased"
	ChangeAircraft           FlightChangeType = "aircraft.changed"
	ChangeTailNumber         FlightChangeType = "aircraft.tail.changed"
	ChangeRoute              FlightChangeType = "route.changed"
	ChangeCodeshares         FlightChangeType = "codeshares.changed"
)

// FlightChangeTypes lists every change type this package knows, in the
// API's order.
var FlightChangeTypes = []FlightChangeType{
	ChangeStatus, ChangeDeparted, ChangeArrived, ChangeCancelled, ChangeDiverted,
	ChangeReinstated, ChangeDepartureGate, ChangeDepartureTerminal, ChangeArrivalGate,
	ChangeArrivalTerminal, ChangeBaggage, ChangeDepartureScheduled,
	ChangeDepartureEstimated, ChangeDepartureActual, ChangeArrivalScheduled,
	ChangeArrivalEstimated, ChangeArrivalActual, ChangeDelayIncreased,
	ChangeDelayDecreased, ChangeAircraft, ChangeTailNumber, ChangeRoute,
	ChangeCodeshares,
}

// Known reports whether t is one of the constants above.
func (t FlightChangeType) Known() bool {
	for _, known := range FlightChangeTypes {
		if t == known {
			return true
		}
	}
	return false
}

// WebhookEventType is the type of a webhook event.
type WebhookEventType string

// Webhook event types.
const (
	// EventSubscriptionCreated: the subscription was created.
	EventSubscriptionCreated WebhookEventType = "subscription.created"
	// EventSubscriptionActivated: a pending subscription found its flight.
	EventSubscriptionActivated WebhookEventType = "subscription.activated"
	// EventSubscriptionTest: a test event sent with SubscriptionsService.SendTest.
	EventSubscriptionTest WebhookEventType = "subscription.test"
	// EventSubscriptionCompleted: the flight finished and the subscription
	// completed.
	EventSubscriptionCompleted WebhookEventType = "subscription.completed"
	// EventSubscriptionSuspended: the subscription was suspended.
	EventSubscriptionSuspended WebhookEventType = "subscription.suspended"
	// EventFlightUpdated: something about the flight changed; see the event's
	// Changes.
	EventFlightUpdated WebhookEventType = "flight.updated"
)

// Known reports whether t is one of the constants above.
func (t WebhookEventType) Known() bool {
	switch t {
	case EventSubscriptionCreated, EventSubscriptionActivated, EventSubscriptionTest,
		EventSubscriptionCompleted, EventSubscriptionSuspended, EventFlightUpdated:
		return true
	}
	return false
}

// DeliveryStatus is the outcome of one webhook delivery attempt.
type DeliveryStatus string

// Delivery statuses.
const (
	// DeliveryDelivered: the receiver answered 2xx.
	DeliveryDelivered DeliveryStatus = "delivered"
	// DeliveryFailed: the attempt failed.
	DeliveryFailed DeliveryStatus = "failed"
)

// Known reports whether s is one of the constants above.
func (s DeliveryStatus) Known() bool {
	return s == DeliveryDelivered || s == DeliveryFailed
}
