# retryflow

**Go 最有新意的重試庫。**

[![Go Reference](https://pkg.go.dev/badge/github.com/Vealcoo/retryflow.svg)](https://pkg.go.dev/github.com/Vealcoo/retryflow)
[![Tests](https://github.com/Vealcoo/retryflow/actions/workflows/test.yml/badge.svg)](https://github.com/Vealcoo/retryflow/actions/workflows/test.yml)
[![Coverage](https://codecov.io/gh/Vealcoo/retryflow/branch/main/graph/badge.svg)](https://codecov.io/gh/Vealcoo/retryflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/Vealcoo/retryflow)](https://goreportcard.com/report/github.com/Vealcoo/retryflow)
[![GitHub stars](https://img.shields.io/github/stars/Vealcoo/retryflow.svg?style=social&label=Star)](https://github.com/Vealcoo/retryflow/stargazers)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/Vealcoo/retryflow)](https://github.com/Vealcoo/retryflow/releases)

`retryflow` 是專為「多步驟、偶爾失敗、需要部分恢復」的場景而生的現代 Go 重試框架，**終於把 checkpoint 做對了**。

不再重跑已經成功的步驟。  
不再手動對付 `any` 轉型。  
再也不會出現「為什麼卡住不重試了？」的靈異事件。

```bash
go get github.com/Vealcoo/retryflow
```

## 為什麼你需要 retryflow？

| 場景                         | 傳統重試方式           | retryflow                                  |
|------------------------------|------------------------|--------------------------------------------|
| 登入 → 取 token → 呼叫 API   | 全程重來               | 只從最後成功 checkpoint 開始重試            |
| 步驟之間有依賴關係           | 自己手動串             | 自動泛型傳遞輸入輸出，型別安全              |
| 429、網路閃斷                 | 無腦重試               | 可針對不同錯誤類型設定獨立重試上限          |
| 想知道是第幾次 attempt、哪一步失敗 | 沒有資訊           | 錯誤自動帶上 `Attempt` 與 `Step` 資訊       |

## 核心功能

- 鏈式寫法 `Chain().Do(&var).Checkpoint()`
- **真正的 checkpoint 恢復**
- 完整泛型輸入/輸出鏈接
- 按錯誤類型設定重試上限（`WithPerErrorLimits`）
- 內建 Exponential、Constant、Fibonacci backoff
- Jitter、Rate limiter、Context 完全支援
- 零依賴 · 極致效能 · 生產級驗證

## 30 秒上手

```go
var token   string
var profile string

err := retryflow.Retry(ctx, retryflow.Seq(
    retryflow.Chain(func(_ any) (string, error) {
        return login() // 可能失敗
    }).Do(&token).Checkpoint(),

    retryflow.Chain(func(token string) (string, error) {
        return fetchProfile(token) // 可能 429、timeout
    }).Do(&profile),
),
    retryflow.WithMaxRetries(10),
    retryflow.WithInitialBackoff(200*time.Millisecond),
    retryflow.WithJitter(100*time.Millisecond),
)
```

`fetchProfile` 失敗？**只重試這一步**，登入永遠不會重跑！

## 進階：按錯誤類型限制重試次數

```go
retryflow.WithErrorClassifier(func(err error) retryflow.ErrorClass {
    if errors.Is(err, context.DeadlineExceeded) {
        return "timeout"
    }
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
    "auth":       1,  // 認證失敗一次就別試了
    "other":      5,
}),
```

## 測試

```bash
go test -v -race -cover
# > 83.2% coverage
```

## 貢獻

非常歡迎 PR！請遵守以下原則：

- 新功能必須附上測試
- 保持 **零外部依賴**
- 遵循 Go 官方慣例

## License

MIT License © 2025 retryflow

---
