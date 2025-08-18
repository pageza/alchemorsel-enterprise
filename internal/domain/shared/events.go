package shared

import "time"

// DomainEvent represents an event that has occurred in the domain
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

// EventDispatcher dispatches domain events to handlers
type EventDispatcher interface {
	Dispatch(event DomainEvent) error
	Register(eventName string, handler EventHandler)
}

// EventHandler handles domain events
type EventHandler func(event DomainEvent) error

// AggregateRoot is the base type for aggregate roots
type AggregateRoot struct {
	events []DomainEvent
}

// AddEvent adds a domain event to be dispatched
func (a *AggregateRoot) AddEvent(event DomainEvent) {
	a.events = append(a.events, event)
}

// Events returns and clears pending domain events
func (a *AggregateRoot) Events() []DomainEvent {
	events := a.events
	a.events = []DomainEvent{}
	return events
}

// ClearEvents clears all pending events
func (a *AggregateRoot) ClearEvents() {
	a.events = []DomainEvent{}
}