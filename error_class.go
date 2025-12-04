package retryflow

import (
	"fmt"
	"strings"
)

// ErrorClass represents the type of an error for categorization.
type ErrorClass string

const (
	// ClassTransient indicates a transient error that is typically retryable.
	ClassTransient ErrorClass = "transient"
	// ClassPermanent indicates a permanent error that should not be retried.
	ClassPermanent ErrorClass = "permanent"
	// ClassRateLimit indicates an error caused by rate limiting.
	ClassRateLimit ErrorClass = "ratelimit"
	// ClassTimeout indicates an error caused by timeout.
	ClassTimeout ErrorClass = "timeout"
	// ClassAuth indicates an error caused by authentication failure.
	ClassAuth ErrorClass = "auth"
	// ClassUnknown indicates an error of unknown type.
	ClassUnknown ErrorClass = "unknown"
)

// Classifier interface can be implemented by custom errors to provide their class.
type Classifier interface {
	Class() ErrorClass
}

// NewErrorClass returns an ErrorClass based on the error's type name (lowercased).
// This is a helper for error categorization.
func NewErrorClass(err error) ErrorClass {
	if c, ok := err.(Classifier); ok {
		return c.Class()
	}
	return ErrorClass(strings.ToLower(fmt.Sprintf("%T", err)))
}

// errorClassLimit defines a map of ErrorClass to retry limits.
type errorClassLimit map[ErrorClass]int

// NewErrorClassLimit creates a new errorClassLimit map.
func NewErrorClassLimit() errorClassLimit {
	return make(errorClassLimit)
}

// AddLimit sets the retry limit for a given ErrorClass and returns the map for chaining.
func (e errorClassLimit) AddLimit(class ErrorClass, limit int) errorClassLimit {
	e[class] = limit
	return e
}
