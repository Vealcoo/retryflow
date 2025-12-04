package retryflow

import (
	"errors"
	"fmt"
)

// AttemptError wraps an error with attempt and step information.
type AttemptError struct {
	Attempt int
	Step    int
	Err     error
}

func (e *AttemptError) Error() string {
	return fmt.Sprintf("attempt %d, step %d: %v", e.Attempt, e.Step, e.Err)
}

func (e *AttemptError) Unwrap() error {
	return e.Err
}

func fullUnwrap(err error) error {
	for {
		u := errors.Unwrap(err)
		if u == nil {
			return err
		}
		err = u
	}
}
