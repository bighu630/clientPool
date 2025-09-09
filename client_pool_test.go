package clientpool

import (
	"client_pool/middleware"
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPClient 封装HTTP客户端，实现string接口以支持中间件
type HTTPClient struct {
	Name   string
	Client *http.Client
	URL    string
}

// String 实现string接口
func (h *HTTPClient) String() string {
	return h.Name
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

	fmt.Printf("Client %s: Successfully requested %s, status: %d\n", h.Name, h.URL, resp.StatusCode)
	return nil
}

// 启动Prometheus指标服务器
func startPrometheusServer() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Prometheus metrics server started on :8080/metrics")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
}

// TestClientPool_BasicFunctionality 测试基本功能
func TestClientPool_BasicFunctionality(t *testing.T) {
	// 启动Prometheus服务器
	startPrometheusServer()

	// 创建客户端池
	pool := NewClientPool[string](3, 5*time.Second, RoundRobin)

	// 注册所有中间件
	pool.RegisterMiddleware(middleware.TraceMiddleware[string]())
	pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[string](5, 10, 2*time.Second))

	// 创建多个HTTP客户端
	clients := []*HTTPClient{
		{Name: "client1", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "client2", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "client3", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
	}

	// 添加客户端到池中，设置不同权重
	for i, client := range clients {
		pool.AddClient(client.Name, i+1) // 权重分别为1,2,3
	}

	// 创建HTTP客户端映射
	clientMap := make(map[string]*HTTPClient)
	for _, client := range clients {
		clientMap[client.Name] = client
	}

	// 测试函数
	testFn := func(ctx context.Context, clientName string) error {
		if client, ok := clientMap[clientName]; ok {
			return client.Get(ctx)
		}
		return fmt.Errorf("client %s not found", clientName)
	}

	// 测试轮询
	t.Run("RoundRobin", func(t *testing.T) {
		fmt.Println("\n=== Testing Round Robin ===")
		for i := 0; i < 6; i++ {
			ctx := context.Background()
			err := pool.DoRoundRobinClient(ctx, testFn)
			if err != nil {
				t.Logf("Request %d failed: %v", i+1, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	})

	// 测试加权随机
	t.Run("WeightedRandom", func(t *testing.T) {
		fmt.Println("\n=== Testing Weighted Random ===")
		for i := 0; i < 6; i++ {
			ctx := context.Background()
			err := pool.DoWeightedRandomClient(ctx, testFn)
			if err != nil {
				t.Logf("Request %d failed: %v", i+1, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	})

	// 测试随机
	t.Run("Random", func(t *testing.T) {
		fmt.Println("\n=== Testing Random ===")
		for i := 0; i < 6; i++ {
			ctx := context.Background()
			err := pool.DoRandomClient(ctx, testFn)
			if err != nil {
				t.Logf("Request %d failed: %v", i+1, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	})
}

// TestClientPool_CircuitBreaker 测试熔断器功能
func TestClientPool_CircuitBreaker(t *testing.T) {
	// 创建客户端池，设置更严格的熔断参数
	pool := NewClientPool[string](2, 3*time.Second, RoundRobin)

	// 注册中间件
	pool.RegisterMiddleware(middleware.TraceMiddleware[string]())
	pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())

	// 添加正常客户端和会失败的客户端
	pool.AddClient("normal_client", 1)
	pool.AddClient("failing_client", 1)

	clientMap := map[string]*HTTPClient{
		"normal_client":  {Name: "normal_client", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		"failing_client": {Name: "failing_client", Client: &http.Client{Timeout: 1 * time.Millisecond}, URL: "https://httpstat.us/500"}, // 会超时失败
	}

	testFn := func(ctx context.Context, clientName string) error {
		if client, ok := clientMap[clientName]; ok {
			return client.Get(ctx)
		}
		return fmt.Errorf("client %s not found", clientName)
	}

	fmt.Println("\n=== Testing Circuit Breaker ===")

	// 触发失败，使failing_client被熔断
	for i := 0; i < 5; i++ {
		ctx := context.Background()
		err := pool.Do(ctx, testFn)
		t.Logf("Request %d: %v", i+1, err)
		time.Sleep(200 * time.Millisecond)
	}

	// 等待熔断器恢复
	fmt.Println("Waiting for circuit breaker recovery...")
	time.Sleep(4 * time.Second)

	// 再次测试
	for i := 0; i < 3; i++ {
		ctx := context.Background()
		err := pool.Do(ctx, testFn)
		t.Logf("After recovery request %d: %v", i+1, err)
		time.Sleep(500 * time.Millisecond)
	}
}

// TestClientPool_Concurrency 测试并发安全
func TestClientPool_Concurrency(t *testing.T) {
	pool := NewClientPool[string](3, 5*time.Second, WeightedRandom)

	// 注册中间件
	pool.RegisterMiddleware(middleware.TraceMiddleware[string]())
	pool.RegisterMiddleware(middleware.PrometheusMiddleware[string]())
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[string](10, 20, 1*time.Second))

	// 添加客户端
	for i := 1; i <= 3; i++ {
		pool.AddClient(fmt.Sprintf("concurrent_client_%d", i), i)
	}

	clientMap := make(map[string]*HTTPClient)
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("concurrent_client_%d", i)
		clientMap[name] = &HTTPClient{
			Name:   name,
			Client: &http.Client{Timeout: 10 * time.Second},
			URL:    "https://www.bilibili.com",
		}
	}

	testFn := func(ctx context.Context, clientName string) error {
		if client, ok := clientMap[clientName]; ok {
			return client.Get(ctx)
		}
		return fmt.Errorf("client %s not found", clientName)
	}

	fmt.Println("\n=== Testing Concurrency ===")

	var wg sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 3

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				ctx := context.Background()
				err := pool.Do(ctx, testFn)
				if err != nil {
					t.Logf("Goroutine %d, Request %d failed: %v", goroutineID, j+1, err)
				} else {
					t.Logf("Goroutine %d, Request %d succeeded", goroutineID, j+1)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("Concurrency test completed")
}

// TestClientPool_PanicRecovery 测试panic恢复
func TestClientPool_PanicRecovery(t *testing.T) {
	pool := NewClientPool[string](3, 5*time.Second, RoundRobin)
	pool.RegisterMiddleware(middleware.TraceMiddleware[string]())

	pool.AddClient("panic_client", 1)

	panicFn := func(ctx context.Context, clientName string) error {
		if clientName == "panic_client" {
			panic("test panic")
		}
		return nil
	}

	fmt.Println("\n=== Testing Panic Recovery ===")

	ctx := context.Background()
	err := pool.Do(ctx, panicFn)
	if err == nil {
		t.Error("Expected error from panic, but got nil")
	} else {
		fmt.Printf("Successfully recovered from panic: %v\n", err)
	}
}

// TestClientPool_TraceID 测试链路追踪
func TestClientPool_TraceID(t *testing.T) {
	pool := NewClientPool[string](3, 5*time.Second, RoundRobin)
	pool.RegisterMiddleware(middleware.TraceMiddleware[string]())

	pool.AddClient("trace_client", 1)

	testFn := func(ctx context.Context, clientName string) error {
		traceID := middleware.GetTraceID(ctx)
		fmt.Printf("Business logic: traceID=%s, client=%s\n", traceID, clientName)
		return nil
	}

	fmt.Println("\n=== Testing Trace ID ===")

	// 测试自动生成traceID
	ctx1 := context.Background()
	err := pool.Do(ctx1, testFn)
	if err != nil {
		t.Errorf("Request with auto traceID failed: %v", err)
	}

	// 测试传入已有traceID
	ctx2 := context.WithValue(context.Background(), middleware.GetTraceID, "existing-trace-123")
	err = pool.Do(ctx2, testFn)
	if err != nil {
		t.Errorf("Request with existing traceID failed: %v", err)
	}
}

// BenchmarkClientPool 性能测试
func BenchmarkClientPool(b *testing.B) {
	pool := NewClientPool[string](3, 5*time.Second, RoundRobin)

	// 只添加必要的中间件避免影响性能测试
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[string](1000, 2000, 100*time.Millisecond))

	pool.AddClient("bench_client", 1)

	testFn := func(ctx context.Context, clientName string) error {
		// 模拟轻量级操作
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			_ = pool.Do(ctx, testFn)
		}
	})
}
