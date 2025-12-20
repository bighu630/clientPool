# Client Pool

一个用 Go 语言实现的高性能、功能丰富的对象池。

自动负载均衡+熔断，把监控，限流，日志追踪...交给中间件，代码只需要关注对象的功能实现

## 目录

- [特性](#特性)
- [快速开始](#快速开始)
  - [安装依赖](#安装依赖)
  - [基本使用](#基本使用)
- [详细功能](#详细功能)
  - [负载均衡](#负载均衡)
  - [熔断器](#熔断器)
  - [中间件系统](#中间件系统)
  - [自定义中间件](#自定义中间件)
- [代码生成工具 (CodeGen)](#代码生成工具-codegen)
  - [安装](#安装)
  - [基本使用](#基本使用-1)
  - [命令行参数](#命令行参数)
  - [使用示例](#使用示例)
  - [生成的代码特性](#生成的代码特性)
  - [特性说明](#特性说明)
  - [完整示例](#完整示例)
- [完整示例项目](#完整示例项目)
- [运行测试](#运行测试)
- [最佳实践](#最佳实践)

## 特性

- 🔄 **多种负载均衡算法**：轮询、加权随机、随机
- 🚨 **熔断器机制**：自动检测和恢复故障客户端
- 🛡️ **限流保护**：基于令牌桶算法的流量控制
- 📊 **Prometheus 监控**：内置指标收集（支持按方法统计）
- 💥 **Panic 恢复**：自动捕获和处理 panic
- 🧵 **线程安全**：支持高并发访问
- 🔧 **泛型支持**：支持任意类型的对象
- 📈 **中间件架构**：灵活可扩展的中间件系统
- 🤖 **代码生成工具**：自动生成客户端包装代码

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

## 完整示例项目

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

1. **客户端类型选择**：推荐使用自定义类型（如 `*HTTPClient`），Prometheus 和 Trace 中间件支持任意类型。建议通过 `context.WithValue(ctx, middleware.PrometheusClientKey{}, label)` 注入监控 label，详见 `example/main.go`。

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

## 代码生成工具 (CodeGen)

为了简化客户端包装代码的编写，本项目提供了自动代码生成工具，可以自动分析接口或结构体，生成完整的连接池包装代码。

### 安装

```bash
# 编译代码生成工具
go build -o codeGen ./cmd/codegen
```

### 基本使用

只需要两个必需参数：

```bash
./codeGen \
  -package=github.com/your/project/client \
  -client='*client.RPCClient'
```

工具会自动：
- 从 `-client` 参数提取类型名（如 `RPCClient`）
- 生成包装器名称（如 `RPCClientPool`）
- 生成包名（如 `rpcclient_pool`）
- 创建输出文件（如 `./generated/rpcclient_pool/client.go`）

### 命令行参数

```bash
codeGen [选项]

必需参数:
  -package string
        源接口或结构体的包路径 (如: github.com/your/project/rpc)
  -client string
        客户端类型 (如: *rpc.Client 或 codegen.It)

可选参数:
  -type string
        源接口或结构体名称 (默认从 -client 自动推断)
  -wrapper string
        生成的包装器名称 (默认: 类型名+Pool，如 ItPool)
  -output string
        输出文件路径 (默认: ./generated/{类型名}_pool/client.go)
  -pool string
        客户端池字段名 (默认: pool)
  -prometheus
        是否包含 Prometheus 监控 (默认: true)
```

### 使用示例

#### 示例 1: 接口类型（最简化）

```bash
./codegen -package=github.com/bighu630/clientPool/codegen -client='codegen.It'
```

生成：
- 包名: `it_pool`
- 结构体: `ItPool`
- 文件: `./generated/it_pool/client.go`

#### 示例 2: 结构体指针类型

```bash
./codegen -package=github.com/bighu630/clientPool/codegen -client='*codegen.St'
```

生成：
- 包名: `st_pool`
- 结构体: `StPool`
- 文件: `./generated/st_pool/client.go`

#### 示例 3: 自定义包装器名称

```bash
./codegen \
  -package=github.com/your/rpc \
  -client='*rpc.Client' \
  -wrapper=MultiRPC \
  -output=./pkg/rpc_pool/client.go
```

### 生成的代码特性

工具会自动生成包含以下功能的完整代码：

```go
// 1. 结构体定义
type ItPool struct {
    pool *clientPool.ClientPool[codegen.It]
}

// 2. 构造函数
func NewItPool(maxFails int, cooldown time.Duration, balancer clientPool.BalancerType) *ItPool

// 3. 客户端管理方法
func (p *ItPool) AddClient(client codegen.It, name string, weight int)
func (p *ItPool) RegisterMiddleware(mw middleware.Middleware[codegen.It])

// 4. 自动包装所有公开方法
func (m *ItPool) InterfaceTest1(a int, b string) error {
    ctx := context.WithValue(context.Background(), middleware.PrometheusMethodKey{}, "interface_test1")
    ret0 = m.pool.Do(ctx, func(ctx context.Context, client codegen.It) error {
        ret0 = client.InterfaceTest1(a, b)
        return ret0
    })
    return
}
```

### 特性说明

生成的代码包含：

✅ **自动类型推断** - 从 `-client` 参数自动提取类型信息  
✅ **连接池管理** - 完整的客户端池初始化和管理  
✅ **Prometheus 监控** - 每个方法自动添加方法级监控（可选）  
✅ **Context 传递** - 自动检测和传递 context 参数  
✅ **方法包装** - 自动分析并包装所有公开方法  
✅ **导入管理** - 自动处理所有必需的导入  

### 完整示例

```bash
# 1. 生成代码
./codegen -package=github.com/bighu630/clientPool/codegen -client='codegen.It'

# 2. 使用生成的代码
package main

import (
    "context"
    "time"
    
    clientpool "github.com/bighu630/clientPool"
    "your/project/generated/it_pool"
)

func main() {
    // 创建连接池
    pool := it_pool.NewItPool(
        3,                        // 最大失败次数
        5*time.Second,           // 熔断恢复时间
        clientpool.RoundRobin,   // 负载均衡策略
    )
    
    // 添加客户端
    client1 := &YourItImplementation{}
    pool.AddClient(client1, "client-1", 1)
    
    // 直接调用方法
    err := pool.InterfaceTest1(1, "test")
    if err != nil {
        panic(err)
    }
}
```

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
