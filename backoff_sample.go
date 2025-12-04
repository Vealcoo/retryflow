package retryflow

import "time"

// Backoff strategies
func ExponentialBackoff(attempt int, prev time.Duration) time.Duration {
	if prev == 0 {
		return 500 * time.Millisecond
	}
	return prev * 2
}

func ConstantBackoff(attempt int, prev time.Duration) time.Duration {
	if prev == 0 {
		return 500 * time.Millisecond
	}
	return prev // Constant uses initial, but since prev is initial after first, it stays constant
}

func FibonacciBackoff(attempt int, _ time.Duration) time.Duration {
	if attempt <= 1 {
		return 500 * time.Millisecond
	}
	a, b := int64(1), int64(1)
	for i := 3; i <= attempt; i++ {
		a, b = b, a+b
	}
	return time.Duration(b) * 500 * time.Millisecond
}
