package example_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Vealcoo/retryflow"
)

// Custom error types
type TransientError struct{ Msg string }

func (e TransientError) Error() string { return "transient: " + e.Msg }

type RateLimitError struct{}

func (RateLimitError) Error() string { return "429 too many requests" }

type PermanentError struct{ Reason string }

func (e PermanentError) Error() string { return "permanent: " + e.Reason }

var ErrAuth = errors.New("auth failed")

// Counters simulating unstable services
var (
	authAttempts    int32
	apiAttempts     int32
	processAttempts int32
)

func TestComplexFlow(t *testing.T) {
	// Reset all counters
	atomic.StoreInt32(&authAttempts, 0)
	atomic.StoreInt32(&apiAttempts, 0)
	atomic.StoreInt32(&processAttempts, 0)

	var token string
	var userID int
	var profile map[string]any
	var finalResult string

	// For capturing logs
	var retryLog []string
	var stepLog []string

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err := retryflow.Retry(ctx,
		retryflow.Seq(
			// Step 1: Obtain Token (fails 3 times then succeeds) → Checkpoint
			retryflow.Chain(func(ctx context.Context, _ any) (string, error) {
				attempt := atomic.AddInt32(&authAttempts, 1)
				if attempt <= 3 {
					return "", fmt.Errorf("auth failed #%d: %w", attempt, ErrAuth)
				}
				return "jwt-super-secret-2025", nil
			}).
				Do(&token).
				Checkpoint().
				OnFail(func() { t.Log("Token step failed → will retry from start") }),

			// Step 2: Call external API (complex failure mode) → Checkpoint
			retryflow.Chain(func(ctx context.Context, token string) (int, error) {
				t.Logf("Calling API with token: %s", token)
				attempt := atomic.AddInt32(&apiAttempts, 1)

				switch attempt {
				case 1, 2:
					return 0, RateLimitError{} // 429 for first two attempts
				case 3:
					return 0, TransientError{Msg: "connection reset by peer"}
				case 4:
					return 987654, nil // success at last
				default:
					return 0, fmt.Errorf("unexpected api attempt: %d", attempt)
				}
			}).
				Do(&userID).
				Checkpoint().
				OnFail(func() { t.Log("API step failed → retry from checkpoint") }),

			// Step 3: Retrieve user profile (might fail once)
			retryflow.Chain(func(ctx context.Context, userID int) (map[string]any, error) {
				if atomic.AddInt32(&processAttempts, 1) == 1 {
					return nil, TransientError{Msg: "db timeout"}
				}
				return map[string]any{
					"name":  "Mario",
					"level": 99,
					"admin": true,
				}, nil
			}).
				Do(&profile),

			// Step 4: Final result assembling (always succeeds)
			retryflow.Chain(func(ctx context.Context, profile map[string]any) (string, error) {
				return fmt.Sprintf("User %s (id=%d) logged in with token %s",
					profile["name"], userID, token), nil
			}).
				Do(&finalResult),
		),

		// Error classifier & per-class retry limits (key feature!)
		retryflow.WithErrorClassifier(func(err error) retryflow.ErrorClass {
			switch {
			case errors.Is(err, ErrAuth):
				return "auth"
			case errors.As(err, new(RateLimitError)):
				return "ratelimit"
			case errors.As(err, new(TransientError)):
				return "transient"
			case errors.As(err, new(PermanentError)):
				return "permanent"
			default:
				return "unknown"
			}
		}),

		retryflow.WithPerErrorLimits(map[retryflow.ErrorClass]int{
			"auth":      10, // allow many retries
			"ratelimit": 3,  // 3 retries for 429 (we'll get 2)
			"transient": 5,
			"permanent": 0, // never retry permanent error
			"unknown":   2,
		}),

		retryflow.WithInitialBackoff(100*time.Millisecond),
		retryflow.WithJitter(80*time.Millisecond),
		retryflow.WithMaxElapsedTime(10*time.Second),

		retryflow.WithOnRetry(func(attempt int, err error) {
			retryLog = append(retryLog, fmt.Sprintf("#%d → %v", attempt, err))
		}),

		retryflow.WithOnStepSuccess(func(step int, output any) {
			stepLog = append(stepLog, fmt.Sprintf("Step %d → %v", step, output))
		}),

		retryflow.WithResetErrorLimitOnCheckpoint(true), // Reset counter at checkpoint
	)

	if err != nil {
		t.Fatalf("Retry was expected to succeed, but failed: %v", err)
	}

	// =============== Strict result assertions ===============

	if token != "jwt-super-secret-2025" {
		t.Errorf("Token incorrect: %s", token)
	}
	if userID != 987654 {
		t.Errorf("UserID incorrect: %d", userID)
	}
	if profile["name"] != "Mario" {
		t.Errorf("Profile incorrect: %v", profile)
	}
	if !strings.Contains(finalResult, "Mario") {
		t.Errorf("Final result incorrect: %s", finalResult)
	}

	// Expected retry behaviors
	expectedMinRetries := 6 // auth fails 3 times + api 3 times + profile fails once = min 6 retries
	if len(retryLog) < expectedMinRetries {
		t.Errorf("Expected at least %d retries, got only %d:\n%v", expectedMinRetries, len(retryLog), retryLog)
	}

	if len(stepLog) != 4 {
		t.Logf("Step log: %v", stepLog)
	}

	t.Log("Complex flow full feature test passed!")
	t.Logf("Final result: %s", finalResult)
	t.Logf("Total retries: %d", len(retryLog))
	for i, log := range retryLog {
		t.Logf("Retry %d: %s", i+1, log)
	}
}
