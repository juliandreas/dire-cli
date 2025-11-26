package events

import (
	"time"

	"github.com/juliandreas/dire/internal/types"
)

// Type represents the type of event.
type Type string

// Event represents a structured event update.
type Event struct {
	Type     Type
	Message  string
	Source   types.Locale
	Target   types.Locale
	Duration time.Duration
	Attempt  int
}

// Callback is a function called to report events during translation.
type Callback func(event *Event)

const (
	TranslationPairStarted   Type = "translation_pair_started"
	TranslationPairCompleted Type = "translation_pair_completed"
	TranslationPairCancelled Type = "translation_pair_cancelled"
	Warning                  Type = "warning"
	Error                    Type = "error"
)

// newEvent centralizes common initialization.
func newEvent(eventType Type, message string, source, target types.Locale, duration time.Duration) *Event {
	return &Event{
		Type:     eventType,
		Message:  message,
		Source:   source,
		Target:   target,
		Duration: duration,
		Attempt:  0,
	}
}

func NewTranslationPairStarted(source, target types.Locale) *Event {
	return newEvent(TranslationPairStarted, "", source, target, 0)
}

func NewTranslationPairCompleted(source, target types.Locale, duration time.Duration) *Event {
	return newEvent(TranslationPairCompleted, "", source, target, duration)
}

func NewTranslationPairCancelled(source, target types.Locale) *Event {
	return newEvent(TranslationPairCancelled, "", source, target, 0)
}

func NewWarning(message string) *Event {
	return newEvent(Warning, message, "", "", 0)
}

func NewError(message string) *Event {
	return newEvent(Error, message, "", "", 0)
}
