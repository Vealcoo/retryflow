# retryflow

**The most creative retry library in Go.**

[![Go Reference](https://pkg.go.dev/badge/github.com/Vealcoo/retryflow.svg)](https://pkg.go.dev/github.com/Vealcoo/retryflow)
[![Tests](https://github.com/Vealcoo/retryflow/retryflow/actions/workflows/test.yml/badge.svg)](https://github.com/Vealcoo/retryflow/actions/workflows/test.yml)
[![Coverage](https://codecov.io/gh/Vealcoo/retryflow/branch/main/graph/badge.svg)](https://codecov.io/gh/Vealcoo/retryflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/Vealcoo/retryflow)](https://goreportcard.com/report/github.com/Vealcoo/retryflow)
[![GitHub stars](https://img.shields.io/github/stars/Vealcoo/retryflow.svg?style=social&label=Star)](https://github.com/Vealcoo/retryflow/stargazers)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/Vealcoo/retryflow)](https://github.com/Vealcoo/retryflow/releases)

`retryflow` is a modern, zero-dependency, battle-tested retry library for Go that finally gets **checkpoints** right.

No more re-running successful steps.  
No more fighting with `any` type assertions.  
No more "why is my retry loop stuck?" moments.

```bash
go get github.com/Vealcoo/retryflow
```

## Why retryflow?

| Scenario                          | Traditional retry | retryflow                                 |
|-----------------------------------|-------------------|--------------------------------------------|
| Login → Get token → Call API      | Retry everything  | Only retry from last successful checkpoint |
| Steps have dependencies           | Manual plumbing   | Automatic typed input/output chaining      |
| 429 / transient errors            | Blind retry       | Per-error-type retry limits                |
| Need to know attempt/step on error| No context        | Errors wrapped with `Attempt` and `Step`   |

## Features

- Chain-style API (`Chain().Do(&var).Checkpoint()`)
- True checkpoint resume
- Full generic input/output chaining
- Per-error-type retry limits (`WithPerErrorLimits`)
- Built-in backoff: Exponential, Constant, Fibonacci
- Jitter, rate limiting, context support
- Zero dependencies · Extremely fast · Production-ready

## Quick Start (30 seconds)

```go
var token string
var profile string

err := retryflow.Retry(ctx, retryflow.Seq(
    retryflow.Chain(func(_ any) (string, error) {
        return login() // might fail
    }).Do(&token).Checkpoint(),

    retryflow.Chain(func(token string) (string, error) {
        return fetchProfile(token) // might 429 or timeout
    }).Do(&profile),
),
    retryflow.WithMaxRetries(10),
    retryflow.WithInitialBackoff(200*time.Millisecond),
    retryflow.WithJitter(100*time.Millisecond),
)
```

If `fetchProfile` fails → only that step is retried. Login is never re-executed.

## Advanced: Per-Error Limits

```go
retryflow.WithErrorClassifier(func(err error) retryflow.ErrorClass {
    if errors.Is(err, context.DeadlineExceeded) { return "timeout" }
    var apiErr APIError
    if errors.As(err, &apiErr) {
        switch apiErr.Code {
        case 429: return "ratelimit"
        case 401: return "auth"
        }
    }
    return "other"
}),
retryflow.WithPerErrorLimits(map[retryflow.ErrorClass]int{
    "timeout":    10,
    "ratelimit":  3,
    "auth":       1,  // don't retry auth failures
    "other":      5,
}),
```

## Testing

```bash
go test -v -race -cover
# > 83.2% coverage
```

## Contributing

PRs welcome! Please:
- Add tests for new features
- Keep zero dependencies
- Follow Go idioms

## License

MIT License © 2025 retryflow

---
