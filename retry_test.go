package retryflow_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Vealcoo/retryflow"
	"golang.org/x/time/rate"
)

func TestBasicRetrySuccess(t *testing.T) {
	ctx := context.Background()
	var output int
	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			return 42, nil
		}).Do(&output),
	)

	err := retryflow.Retry(ctx, steps)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if output != 42 {
		t.Errorf("expected output 42, got %d", output)
	}
}

func TestRetryFailureExceedsMaxRetries(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			attempts++
			return 0, errors.New("fail")
		}).Do(new(int)),
	)

	err := retryflow.Retry(ctx, steps, retryflow.WithMaxRetries(3))
	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestCheckpointResumesFromLastSuccess(t *testing.T) {
	ctx := context.Background()
	var output1, output2 int
	attempts := atomic.Int32{}

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			attempts.Add(1)
			return 1, nil
		}).Do(&output1).Checkpoint(),

		retryflow.Chain(func(ctx context.Context, input any) (int, error) {
			attempts.Add(1)
			if attempts.Load() < 3 {
				return 0, errors.New("fail")
			}
			return 2, nil
		}).Do(&output2),
	)

	err := retryflow.Retry(ctx, steps, retryflow.WithMaxRetries(5))
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if output1 != 1 || output2 != 2 {
		t.Errorf("expected outputs 1 and 2, got %d and %d", output1, output2)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 total calls (1+2), got %d", attempts.Load())
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			cancel()
			return 0, errors.New("fail")
		}).Do(new(int)),
	)

	err := retryflow.Retry(ctx, steps)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestNonRetryableError(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			attempts++
			return 0, errors.New("permanent fail")
		}).Do(new(int)),
	)

	err := retryflow.Retry(ctx, steps, retryflow.WithRetryable(func(err error) bool {
		return err.Error() != "permanent fail"
	}))
	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDifferentBackoffStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy func(int, time.Duration) time.Duration
	}{
		{"Exponential", retryflow.ExponentialBackoff},
		{"Constant", retryflow.ConstantBackoff},
		{"Fibonacci", retryflow.FibonacciBackoff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			attempts := 0

			steps := retryflow.Seq(
				retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
					attempts++
					if attempts >= 3 {
						return 42, nil
					}
					return 0, errors.New("fail")
				}).Do(new(int)),
			)

			err := retryflow.Retry(ctx, steps,
				retryflow.WithBackoffStrategy(tt.strategy),
				retryflow.WithInitialBackoff(10*time.Millisecond),
				retryflow.WithMaxRetries(5),
			)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if attempts != 3 {
				t.Errorf("expected 3 attempts, got %d", attempts)
			}
		})
	}
}

func TestJitter(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			attempts++
			return 0, errors.New("fail")
		}).Do(new(int)),
	)

	start := time.Now()
	err := retryflow.Retry(ctx, steps,
		retryflow.WithMaxRetries(3),
		retryflow.WithJitter(50*time.Millisecond),
		retryflow.WithInitialBackoff(100*time.Millisecond),
	)
	if err == nil {
		t.Error("expected error, got nil")
	}
	duration := time.Since(start)
	expectedMin := 150 * time.Millisecond
	if duration < expectedMin {
		t.Errorf("duration too short: %v (expected > %v)", duration, expectedMin)
	}
}

func TestMaxElapsedTime(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			attempts++
			return 0, errors.New("fail")
		}).Do(new(int)),
	)

	err := retryflow.Retry(ctx, steps,
		retryflow.WithMaxElapsedTime(500*time.Millisecond),
		retryflow.WithInitialBackoff(200*time.Millisecond),
		retryflow.WithMaxRetries(-1),
		retryflow.WithJitter(0),
		retryflow.WithBackoffStrategy(retryflow.ConstantBackoff),
	)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts < 2 || attempts > 4 {
		t.Errorf("unexpected attempts: %d", attempts)
	}
}

func TestHooksCalled(t *testing.T) {
	ctx := context.Background()
	attemptStarts := 0
	stepSuccesses := 0
	retries := 0

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			if attemptStarts == 1 {
				return 0, errors.New("fail")
			}
			return 42, nil
		}).Do(new(int)),
	)

	err := retryflow.Retry(ctx, steps,
		retryflow.WithOnAttemptStart(func(attempt int) { attemptStarts++ }),
		retryflow.WithOnStepSuccess(func(step int, output any) { stepSuccesses++ }),
		retryflow.WithOnRetry(func(attempt int, err error) { retries++ }),
		retryflow.WithMaxRetries(3),
	)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attemptStarts != 2 || stepSuccesses != 1 || retries != 1 {
		t.Errorf("hooks: starts=%d, successes=%d, retries=%d", attemptStarts, stepSuccesses, retries)
	}
}

func TestPerErrorLimits(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			attempts++
			return 0, fmt.Errorf("specific error %d", attempts)
		}).Do(new(int)),
	)

	err := retryflow.Retry(ctx, steps,
		retryflow.WithPerErrorLimits(map[retryflow.ErrorClass]int{retryflow.ClassPermanent: 2}),
		retryflow.WithErrorClassifier(func(err error) retryflow.ErrorClass {
			if strings.Contains(err.Error(), "specific error") {
				return retryflow.ClassPermanent
			}
			return retryflow.ClassUnknown
		}),
		retryflow.WithMaxRetries(10),
	)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRateLimiter(t *testing.T) {
	ctx := context.Background()
	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	attempts := 0

	steps := retryflow.Seq(
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			attempts++
			return 0, errors.New("fail")
		}).Do(new(int)),
	)

	start := time.Now()
	err := retryflow.Retry(ctx, steps,
		retryflow.WithRateLimiter(limiter),
		retryflow.WithMaxRetries(3),
		retryflow.WithInitialBackoff(1*time.Millisecond),
	)
	if err == nil {
		t.Error("expected error, got nil")
	}
	duration := time.Since(start)
	if duration < 180*time.Millisecond {
		t.Errorf("rate limiter not respected: %v", duration)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestCheckpointRecovery(t *testing.T) {
	ctx := context.Background()

	var (
		userID  int
		email   string
		token   string
		profile string
		final   string
	)

	// Record the input received by each step
	type inputLog struct {
		step int
		got  string
	}
	var inputHistory []inputLog

	steps := retryflow.Seq(
		// Step 1 → succeed
		retryflow.Chain(func(ctx context.Context, _ any) (int, error) {
			inputHistory = append(inputHistory, inputLog{step: 1, got: "<nil>"})
			return 1001, nil
		}).Do(&userID),

		// Step 2 → succeed → checkpoint
		retryflow.Chain(func(ctx context.Context, prev int) (string, error) {
			got := prev
			if got != 1001 {
				return "", fmt.Errorf("unexpected input: %v", got)
			}
			inputHistory = append(inputHistory, inputLog{step: 2, got: fmt.Sprintf("%d", got)})
			return "step2@gmail.com", nil
		}).Do(&email).Checkpoint(),

		// Step 3 → succeed
		retryflow.Chain(func(ctx context.Context, prev string) (string, error) {
			got := prev
			if got != "step2@gmail.com" {
				return "", fmt.Errorf("unexpected input: %v", got)
			}
			inputHistory = append(inputHistory, inputLog{step: 3, got: got})
			return "jwt-very-secret", nil
		}).Do(&token),

		// Step 4 → Fail intentionally first 3 times, return poison, succeed on 4th attempt
		retryflow.Chain(func(ctx context.Context, prev any) (string, error) {
			got := "<nil>"
			if prev != nil {
				got = fmt.Sprintf("%v", prev)
			}
			inputHistory = append(inputHistory, inputLog{step: 4, got: got})

			// Count how many times this step was called
			callCount := 0
			for _, log := range inputHistory {
				if log.step == 4 {
					callCount++
				}
			}

			if callCount <= 3 {
				return "POISON_PROFILE_DATA", fmt.Errorf("transient network error #%d", callCount)
			}

			// On final success: must receive correct token
			if got != "jwt-very-secret" {
				return "", fmt.Errorf("CRITICAL BUG: step3 final attempt received wrong input: %q (want %q)", got, "jwt-very-secret")
			}
			return "marc.huang", nil
		}).Do(&profile),

		// Step 5 → Final validation
		retryflow.Chain(func(ctx context.Context, prev any) (string, error) {
			got := "<nil>"
			if prev != nil {
				got = prev.(string)
			}
			inputHistory = append(inputHistory, inputLog{step: 4, got: got})

			if got != "marc.huang" {
				return "", fmt.Errorf("step4 received wrong input: %q", got)
			}
			return "login success", nil
		}).Do(&final),
	)

	err := retryflow.Retry(ctx, steps, retryflow.WithMaxRetries(3))
	if err != nil {
		t.Fatalf("should eventually succeed, got: %v", err)
	}

	// Final result validation
	if userID != 1001 || token != "jwt-very-secret" || email != "step2@gmail.com" || profile != "marc.huang" || final != "login success" {
		t.Errorf("final state mismatch: userID=%d, token=%s, email=%s, profile=%s, final=%s", userID, token, email, profile, final)
	}

	// Key: check if Step3 always receives the correct email as input on each call
	step3Inputs := []string{}
	for _, log := range inputHistory {
		if log.step == 3 {
			step3Inputs = append(step3Inputs, log.got)
		}
		t.Logf("Step %d received input: %s", log.step, log.got)
	}

	// Should be called 4 times, and each input must be "step2@gmail.com"
	if len(step3Inputs) != 4 {
		t.Fatalf("Step3 expected 4 calls, got %d", len(step3Inputs))
	}
}
