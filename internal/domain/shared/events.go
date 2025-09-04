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

// Conversation-specific domain events

type ConversationCreated struct {
	ConversationID string
	UserID         string
	Intent         interface{}
	CreatedAt      time.Time
}

func (e ConversationCreated) EventName() string     { return "ConversationCreated" }
func (e ConversationCreated) OccurredAt() time.Time { return e.CreatedAt }

type MessageAdded struct {
	ConversationID string
	MessageID      string
	Role           string
	Content        string
	CreatedAt      time.Time
}

func (e MessageAdded) EventName() string     { return "MessageAdded" }
func (e MessageAdded) OccurredAt() time.Time { return e.CreatedAt }

type ConversationTitleUpdated struct {
	ConversationID string
	OldTitle       string
	NewTitle       string
	UpdatedAt      time.Time
}

func (e ConversationTitleUpdated) EventName() string     { return "ConversationTitleUpdated" }
func (e ConversationTitleUpdated) OccurredAt() time.Time { return e.UpdatedAt }

type ConversationArchived struct {
	ConversationID string
	ArchivedAt     time.Time
}

func (e ConversationArchived) EventName() string     { return "ConversationArchived" }
func (e ConversationArchived) OccurredAt() time.Time { return e.ArchivedAt }

type ConversationDeleted struct {
	ConversationID string
	DeletedAt      time.Time
}

func (e ConversationDeleted) EventName() string     { return "ConversationDeleted" }
func (e ConversationDeleted) OccurredAt() time.Time { return e.DeletedAt }
