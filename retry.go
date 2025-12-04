package retryflow

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"time"
)

// Retry executes the sequence of steps with retry logic.
func Retry(ctx context.Context, steps Steps, opts ...Option) error {
	if len(steps) == 0 {
		return nil
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	// Validate options
	if o.initialBackoff <= 0 {
		return errors.New("initialBackoff must be positive")
	}
	if o.maxBackoff < o.initialBackoff {
		return errors.New("maxBackoff must be >= initialBackoff")
	}
	if o.jitter < 0 {
		return errors.New("jitter must be non-negative")
	}
	if o.maxRetries < 0 && o.maxElapsedTime == 0 {
		return errors.New("infinite retry without maxElapsedTime is dangerous")
	}

	// Initialize checkpoint and attempt counter
	var checkpoint int
	var currentAttempt int

	currentBackoff := o.initialBackoff
	start := time.Now()
	checkpoint = 0                                                    // Reset checkpoint at start
	currentAttempt = 0                                                // Reset attempt counter at start
	perErrorCounts := make(map[ErrorClass]int, len(o.perErrorLimits)) // Reset error counts at start

	var prevOutput any
	var lastCheckpointOutput any = nil
	for {
		currentAttempt += 1
		prevOutput = lastCheckpointOutput

		// Apply rate limiter if present
		if o.rateLimiter != nil {
			if err := o.rateLimiter.Wait(ctx); err != nil {
				return err
			}
		}

		if o.onAttemptStart != nil {
			o.onAttemptStart(currentAttempt)
		}

		var err error
		failed := false
		startIdx := checkpoint // 0-based

		for i := startIdx; i < len(steps); i++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			step := steps[i]

			var output any
			output, err = step.run(ctx, prevOutput)
			if err != nil {
				failed = true
				err = &AttemptError{Attempt: currentAttempt, Step: i + 1, Err: err}
				if step.onFail != nil {
					step.onFail()
				}
				break
			}

			// Store output if outputPtr is provided
			if step.outputPtr != nil {
				ptrVal := reflect.ValueOf(step.outputPtr)
				if ptrVal.Kind() != reflect.Ptr || ptrVal.IsNil() {
					return errors.New("outputPtr must be a non-nil pointer")
				}
				outType := ptrVal.Elem().Type()
				if output != nil && !reflect.TypeOf(output).AssignableTo(outType) {
					return fmt.Errorf("output type mismatch: expected %s, got %T", outType, output)
				}
				ptrVal.Elem().Set(reflect.ValueOf(output))
			}
			// if step success, rewrite the previous output even the new output is nil
			prevOutput = output

			if o.onStepSuccess != nil {
				o.onStepSuccess(i+1, output)
			}

			if step.checkpoint {
				checkpoint = i + 1
				currentAttempt = 0
				lastCheckpointOutput = output
				currentBackoff = o.initialBackoff
				if o.resetErrorLimitOnCheckpoint {
					perErrorCounts = make(map[ErrorClass]int, len(o.perErrorLimits))
				}
			}
		}

		if !failed {
			return nil
		}

		// Check if retryable
		unwrappedErr := fullUnwrap(err)
		if !o.retryable(unwrappedErr) {
			return err
		}

		// Check per-error limits
		key := o.errorClassifier(unwrappedErr)
		perErrorCounts[key]++
		if limit, ok := o.perErrorLimits[key]; ok && perErrorCounts[key] > limit {
			return err
		}

		if o.onRetry != nil {
			o.onRetry(currentAttempt, err)
		}

		if o.maxRetries >= 0 && currentAttempt >= o.maxRetries {
			return err
		}
		if o.maxElapsedTime > 0 && time.Since(start) >= o.maxElapsedTime {
			return err
		}

		next := o.backoffStrategy(currentAttempt, currentBackoff)
		next = min(next, o.maxBackoff)

		sleep := next
		if o.jitter > 0 {
			j := time.Duration(rand.Int63n(int64(o.jitter*2))) - o.jitter
			sleep += j
			if sleep < 10*time.Millisecond {
				sleep = 10 * time.Millisecond
			}
		}

		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return ctx.Err()
		}

		currentBackoff = next
	}
}
