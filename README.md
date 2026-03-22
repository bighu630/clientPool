# Client Pool

Go 泛型客户端池，自带负载均衡、熔断、中间件（监控/限流/重试/超时）。支持代码生成。

## 安装

```bash
go get github.com/bighu630/clientPool
```

## 快速开始

```go
pool := clientpool.NewClientPool[string](
    3,                      // 连续失败 3 次后熔断
    5*time.Second,          // 熔断冷却时间
    clientpool.RoundRobin,  // 负载均衡策略: RoundRobin / WeightedRandom / Random
)

// 添加客户端（名称 + 权重）
pool.AddClient("client-1", 1)
pool.AddClient("client-2", 2)

// 注册中间件（按需）
pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())
pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[string](10, 20, 2*time.Second))

// 使用
err := pool.Do(ctx, func(ctx context.Context, client string) error {
    // 业务逻辑
    return nil
})
```

## 中间件

| 中间件 | 说明 |
|--------|------|
| `RecoverMiddleware` | panic 恢复（默认已注册） |
| `PrometheusMiddleware` | 请求计数、耗时、错误数 |
| `NewRateLimiterMiddleware(qps, burst, timeout)` | 令牌桶限流 |
| `RetryMiddleware` | 重试 |
| `TimeoutMiddleware` | 超时控制 |

自定义中间件：实现 `Middleware[T]` 接口，或用 `WrapMiddleware()` 包装函数。

## 代码生成

自动为接口/结构体生成池包装代码，每个方法自动走 `pool.Do()`。

### 编译

```bash
go build -o codeGen ./cmd/codegen
```

### 参数说明

| 参数 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `-package` | 是 | — | 源接口或结构体的完整包导入路径 |
| `-client` | 是 | — | 客户端类型，支持指针类型（如 `*rpc.Client`）和值类型（如 `codegen.It`） |
| `-type` | 否 | 从 `-client` 自动推断 | 源接口或结构体名称。例如 `-client='*rpc.Client'` 会推断为 `Client` |
| `-wrapper` | 否 | `{type}Pool` | 生成的包装器结构体名称。例如类型为 `Client` 时默认生成 `ClientPool` |
| `-pool` | 否 | `pool` | 生成结构体中客户端池的字段名 |
| `-output` | 否 | `./generated/{type}_pool/client.go` | 输出文件路径 |
| `-prometheus` | 否 | `true` | 是否在生成的代码中包含 Prometheus 监控（方法级别标签） |

### 示例

```bash
# 最简用法：只需指定包路径和客户端类型，其余自动推断
./codeGen -package=github.com/bighu630/clientPool/codegen -client='codegen.It'

# 完整用法：手动指定所有参数
./codeGen \
  -package=github.com/gagliardetto/solana-go/rpc \
  -client='*rpc.Client' \
  -wrapper=RPCPool \
  -output=./generated/rpc_pool/client.go \
  -prometheus=false
```

## License

MIT
