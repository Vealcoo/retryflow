package retryflow

import (
	"time"

	"golang.org/x/time/rate"
)

// Option defines a function to configure retry options.
type Option func(*options)

// options holds the configuration for the retry mechanism.
type options struct {
	initialBackoff  time.Duration
	maxBackoff      time.Duration
	jitter          time.Duration
	maxRetries      int
	maxElapsedTime  time.Duration
	onRetry         func(attempt int, err error)
	onAttemptStart  func(attempt int)
	onStepSuccess   func(step int, output any)
	backoffStrategy func(attempt int, prev time.Duration) time.Duration
	retryable       func(err error) bool
	perErrorLimits  errorClassLimit
	errorClassifier func(err error) ErrorClass
	rateLimiter     *rate.Limiter
	// default reset error limit on checkpoint
	resetErrorLimitOnCheckpoint bool
}

// defaultOptions returns the default retry configuration.
func defaultOptions() options {
	return options{
		initialBackoff:              500 * time.Millisecond,
		maxBackoff:                  30 * time.Second,
		jitter:                      200 * time.Millisecond,
		maxRetries:                  5, // Changed to finite default to avoid infinite loops
		maxElapsedTime:              5 * time.Minute,
		backoffStrategy:             ExponentialBackoff,
		retryable:                   func(err error) bool { return true },
		errorClassifier:             func(err error) ErrorClass { return NewErrorClass(err) },
		resetErrorLimitOnCheckpoint: true,
	}
}

// Option functions
func WithInitialBackoff(d time.Duration) Option { return func(o *options) { o.initialBackoff = d } }
func WithMaxBackoff(d time.Duration) Option     { return func(o *options) { o.maxBackoff = d } }
func WithJitter(d time.Duration) Option         { return func(o *options) { o.jitter = d } }
func WithMaxRetries(n int) Option               { return func(o *options) { o.maxRetries = n } }
func WithMaxElapsedTime(d time.Duration) Option { return func(o *options) { o.maxElapsedTime = d } }
func WithOnRetry(f func(attempt int, err error)) Option {
	return func(o *options) { o.onRetry = f }
}
func WithOnAttemptStart(f func(attempt int)) Option {
	return func(o *options) { o.onAttemptStart = f }
}
func WithOnStepSuccess(f func(step int, output any)) Option {
	return func(o *options) { o.onStepSuccess = f }
}
func WithBackoffStrategy(f func(attempt int, prev time.Duration) time.Duration) Option {
	return func(o *options) { o.backoffStrategy = f }
}
func WithRetryable(f func(err error) bool) Option {
	return func(o *options) { o.retryable = f }
}
func WithPerErrorLimits(limits errorClassLimit) Option {
	return func(o *options) { o.perErrorLimits = limits }
}
func WithErrorClassifier(f func(err error) ErrorClass) Option {
	return func(o *options) { o.errorClassifier = f }
}
func WithRateLimiter(limiter *rate.Limiter) Option {
	return func(o *options) { o.rateLimiter = limiter }
}
func WithResetErrorLimitOnCheckpoint(b bool) Option {
	return func(o *options) { o.resetErrorLimitOnCheckpoint = b }
}
