package main

import (
	clientpool "client_pool"
	"client_pool/middleware"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPClient 封装HTTP客户端
type HTTPClient struct {
	Name   string
	Client *http.Client
	URL    string
}

// Get 发起GET请求
func (h *HTTPClient) Get(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", h.URL, nil)
	if err != nil {
		return err
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	fmt.Printf("✅ Client %s: Successfully requested %s, status: %d\n", h.Name, h.URL, resp.StatusCode)
	return nil
}

func main() {
	fmt.Println("🚀 Starting Client Pool Demo")

	// 启动Prometheus指标服务器
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		fmt.Println("📊 Prometheus metrics server started on http://localhost:8080/metrics")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端池 - 最大失败3次，冷却时间5秒，默认使用轮询
	pool := clientpool.NewClientPool[string](3, 5*time.Second, clientpool.RoundRobin)

	// 注册所有中间件
	fmt.Println("🔧 Registering middlewares...")
	pool.RegisterMiddleware(middleware.TraceMiddleware[string]())                              // 链路追踪
	pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())                         // Prometheus监控
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[string](5, 10, 2*time.Second)) // 限流：每秒5个请求，桶大小10

	// 创建多个HTTP客户端
	clients := []*HTTPClient{
		{Name: "bilibili-1", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "bilibili-2", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "bilibili-3", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
	}

	// 添加客户端到池中，设置不同权重
	fmt.Println("➕ Adding clients to pool...")
	for i, client := range clients {
		pool.AddClient(client.Name, i+1) // 权重分别为1,2,3
		fmt.Printf("   Added %s with weight %d\n", client.Name, i+1)
	}

	// 创建HTTP客户端映射
	clientMap := make(map[string]*HTTPClient)
	for _, client := range clients {
		clientMap[client.Name] = client
	}

	// 业务逻辑函数
	businessLogic := func(ctx context.Context, clientName string) error {
		if client, ok := clientMap[clientName]; ok {
			return client.Get(ctx)
		}
		return fmt.Errorf("client %s not found", clientName)
	}

	// 演示1: 轮询负载均衡
	fmt.Println("\n🔄 Demo 1: Round Robin Load Balancing")
	for i := 0; i < 6; i++ {
		ctx := context.Background()
		fmt.Printf("Request %d: ", i+1)
		err := pool.DoRoundRobinClient(ctx, businessLogic)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 演示2: 加权随机负载均衡
	fmt.Println("\n🎲 Demo 2: Weighted Random Load Balancing")
	for i := 0; i < 6; i++ {
		ctx := context.Background()
		fmt.Printf("Request %d: ", i+1)
		err := pool.DoWeightedRandomClient(ctx, businessLogic)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 演示3: 使用默认策略
	fmt.Println("\n⚖️ Demo 3: Using Default Balancer (Round Robin)")
	for i := 0; i < 6; i++ {
		ctx := context.Background()
		fmt.Printf("Request %d: ", i+1)
		err := pool.Do(ctx, businessLogic)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 演示4: 熔断器测试
	fmt.Println("\n🚨 Demo 4: Circuit Breaker Test")

	// 创建一个新的池用于熔断测试
	circuitPool := clientpool.NewClientPool[string](2, 3*time.Second, clientpool.RoundRobin)
	circuitPool.RegisterMiddleware(middleware.TraceMiddleware[string]())
	circuitPool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())

	// 添加正常客户端和会失败的客户端
	circuitPool.AddClient("normal", 1)
	circuitPool.AddClient("failing", 1)

	failingClientMap := map[string]*HTTPClient{
		"normal":  {Name: "normal", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		"failing": {Name: "failing", Client: &http.Client{Timeout: 1 * time.Millisecond}, URL: "https://httpstat.us/500"}, // 会超时失败
	}

	circuitBusinessLogic := func(ctx context.Context, clientName string) error {
		if client, ok := failingClientMap[clientName]; ok {
			return client.Get(ctx)
		}
		return fmt.Errorf("client %s not found", clientName)
	}

	// 触发失败，使failing客户端被熔断
	for i := 0; i < 8; i++ {
		ctx := context.Background()
		fmt.Printf("Circuit test request %d: ", i+1)
		err := circuitPool.Do(ctx, circuitBusinessLogic)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 演示5: Panic恢复
	fmt.Println("\n💥 Demo 5: Panic Recovery")
	panicPool := clientpool.NewClientPool[string](3, 5*time.Second, clientpool.RoundRobin)
	panicPool.RegisterMiddleware(middleware.TraceMiddleware[string]())
	panicPool.AddClient("panic-client", 1)

	panicLogic := func(ctx context.Context, clientName string) error {
		if clientName == "panic-client" {
			panic("intentional panic for testing")
		}
		return nil
	}

	ctx := context.Background()
	err := panicPool.Do(ctx, panicLogic)
	if err != nil {
		fmt.Printf("✅ Successfully recovered from panic: %v\n", err)
	}

	fmt.Println("\n🎉 Demo completed!")
	fmt.Println("📊 Check Prometheus metrics at: http://localhost:8080/metrics")
	fmt.Println("🔍 Look for metrics like:")
	fmt.Println("   - middleware_requests_total")
	fmt.Println("   - middleware_request_duration_seconds")
	fmt.Println("   - middleware_request_errors_total")

	// 保持程序运行一段时间以便查看metrics
	fmt.Println("\n⏱️ Keeping server alive for 30 seconds to check metrics...")
	time.Sleep(30 * time.Second)
	fmt.Println("👋 Goodbye!")
}
