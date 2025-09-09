# Client Pool

一个用 Go 语言实现的高性能、功能丰富的客户端连接池，支持负载均衡、熔断器、限流、监控和链路追踪等企业级特性。

## 特性

- 🔄 **多种负载均衡算法**：轮询、加权随机、随机
- 🚨 **熔断器机制**：自动检测和恢复故障客户端
- 🛡️ **限流保护**：基于令牌桶算法的流量控制
- 📊 **Prometheus 监控**：内置指标收集
- 🔍 **链路追踪**：自动生成和传递 TraceID
- 💥 **Panic 恢复**：自动捕获和处理 panic
- 🧵 **线程安全**：支持高并发访问
- 🔧 **泛型支持**：支持任意类型的客户端
- 📈 **中间件架构**：灵活可扩展的中间件系统

## 快速开始

### 安装依赖

```go
go get github.com/bighu630/clientPool
```

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    
    clientpool "github.com/bighu630/clientPool"
    "github.com/bighu630/clientPool/middleware"
)

func main() {
    // 创建客户端池
    pool := clientpool.NewClientPool[string](
        3,                        // 最大失败次数
        5*time.Second,           // 熔断器冷却时间
        clientpool.RoundRobin,   // 默认负载均衡策略
    )
    
    // 注册中间件
    pool.RegisterMiddleware(middleware.TraceMiddleware[string]())
    pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())
    pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[string](
        10,              // 每秒请求数
        20,              // 令牌桶大小
        2*time.Second,   // 超时时间
    ))
    
    // 添加客户端
    pool.AddClient("client-1", 1) // 客户端名称，权重
    pool.AddClient("client-2", 2)
    pool.AddClient("client-3", 3)
    
    // 使用客户端池
    err := pool.Do(context.Background(), func(ctx context.Context, clientName string) error {
        fmt.Printf("Using client: %s\n", clientName)
        // 在这里实现你的业务逻辑
        return nil
    })
    
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

## 详细功能

### 负载均衡

支持三种负载均衡策略：

#### 轮询 (Round Robin)
```go
err := pool.DoRoundRobinClient(ctx, businessLogic)
```

#### 加权随机 (Weighted Random)
```go
err := pool.DoWeightedRandomClient(ctx, businessLogic)
```

#### 随机 (Random)
```go
err := pool.DoRandomClient(ctx, businessLogic)
```

#### 使用默认策略
```go
err := pool.Do(ctx, businessLogic)
```

### 熔断器

当客户端连续失败达到设定次数时，会被自动熔断：

```go
pool := clientpool.NewClientPool[string](
    3,                // 连续失败3次后熔断
    5*time.Second,    // 5秒后尝试恢复
    clientpool.RoundRobin,
)
```

### 中间件系统

#### 链路追踪中间件
```go
pool.RegisterMiddleware(middleware.TraceMiddleware[string]())

// 在业务逻辑中获取 TraceID
traceID := middleware.GetTraceID(ctx)
fmt.Printf("TraceID: %s\n", traceID)
```

#### Prometheus 监控中间件
```go
// 注册中间件
pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())

// 启动指标服务器
http.Handle("/metrics", promhttp.Handler())
go http.ListenAndServe(":8080", nil)
```

监控指标包括：
- `middleware_requests_total` - 总请求数
- `middleware_request_duration_seconds` - 请求耗时
- `middleware_request_errors_total` - 错误总数

#### 限流中间件
```go
rateLimiter := middleware.NewRateLimiterMiddleware[string](
    10,               // 每秒最多10个请求
    20,               // 令牌桶大小为20
    2*time.Second,    // 等待超时时间
)
pool.RegisterMiddleware(rateLimiter)
```

#### 恢复中间件
```go
// 默认已注册，自动捕获 panic
pool.RegisterMiddleware(middleware.RecoverMiddleware[string]())
```

### 自定义中间件

实现 `Middleware` 接口来创建自定义中间件：

```go
type LoggingMiddleware[T any] struct{}

func (m *LoggingMiddleware[T]) Execute(
    ctx context.Context, 
    client T, 
    next func(context.Context, T) error,
) error {
    start := time.Now()
    fmt.Printf("Request started: %v\n", client)
    
    err := next(ctx, client)
    
    duration := time.Since(start)
    fmt.Printf("Request completed: %v, duration: %v, error: %v\n", 
        client, duration, err)
    
    return err
}

// 注册中间件
pool.RegisterMiddleware(&LoggingMiddleware[string]{})
```

或者使用函数包装器：

```go
loggingMiddleware := middleware.WrapMiddleware(
    func(ctx context.Context, client string, next func(context.Context, string) error) error {
        fmt.Printf("Before: %s\n", client)
        err := next(ctx, client)
        fmt.Printf("After: %s, error: %v\n", client, err)
        return err
    },
)
pool.RegisterMiddleware(loggingMiddleware)
```

## 完整示例

查看 `example/main.go` 获取完整的使用示例，包括：

- HTTP 客户端封装
- 所有中间件的使用
- 不同负载均衡策略测试
- 熔断器演示
- Prometheus 指标收集

运行示例：

```bash
cd example
go run main.go
```

然后访问 `http://localhost:8080/metrics` 查看 Prometheus 指标。

## 运行测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试
go test -v -run TestClientPool_BasicFunctionality
go test -v -run TestClientPool_CircuitBreaker
go test -v -run TestClientPool_Concurrency

# 运行基准测试
go test -bench=.
```

## 最佳实践

1. **客户端类型选择**：对于需要使用 `PrometheusMiddleware` 和 `TraceMiddleware` 的场景，客户端类型应该约束为 `~string`

2. **中间件顺序**：按照以下顺序注册中间件以获得最佳效果：
   ```go
   pool.RegisterMiddleware(middleware.RecoverMiddleware[string]())     // 最外层
   pool.RegisterMiddleware(middleware.TraceMiddleware[string]())       
   pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())  
   pool.RegisterMiddleware(rateLimiterMiddleware)                      // 最内层
   ```

3. **熔断器参数调优**：
   - `maxFails`：建议设置为 3-5 次
   - `cooldown`：建议设置为 5-30 秒，根据下游服务恢复时间调整

4. **限流参数设置**：
   - 根据下游服务的承载能力设置 QPS 限制
   - 令牌桶大小通常设置为 QPS 的 1.5-2 倍

5. **监控告警**：基于 Prometheus 指标设置告警：
   - 错误率过高
   - 请求耗时过长
   - 熔断器频繁触发

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License