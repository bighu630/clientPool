// Prometheus label key

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	clientpool "github.com/bighu630/clientPool"
	"github.com/bighu630/clientPool/middleware"

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
	time.Sleep(100 * time.Millisecond)

	// 创建客户端池，泛型为 *HTTPClient
	pool := clientpool.NewClientPool[*HTTPClient](3, 5*time.Second, clientpool.RoundRobin)
	fmt.Println("🔧 Registering middlewares...")
	pool.RegisterMiddleware(middleware.NewPrometheusMiddleware[*HTTPClient]())
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[*HTTPClient](5, 10, 2*time.Second))

	clients := []*HTTPClient{
		{Name: "bilibili-1", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "bilibili-2", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "bilibili-3", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
	}
	fmt.Println("➕ Adding clients to pool...")
	for i, client := range clients {
		pool.AddClient(client, client.Name, i+1)
		fmt.Printf("   Added %s with weight %d\n", client.Name, i+1)
	}

	businessLogic := func(ctx context.Context, client *HTTPClient) error {
		// 在 ctx 中注入 Prometheus label
		ctx = context.WithValue(ctx, middleware.PrometheusClientKey{}, client.Name)
		return client.Get(ctx)
	}

	fmt.Println("\n🔄 Demo 1: Round Robin Load Balancing")
	for i := range 6 {
		ctx := context.Background()
		ctx = context.WithValue(ctx, middleware.PrometheusClientKey{}, fmt.Sprintf("rr-%s", clients[i%len(clients)].Name))
		fmt.Printf("Request %d: ", i+1)
		err := pool.DoRoundRobinClient(ctx, businessLogic)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n🎲 Demo 2: Weighted Random Load Balancing")
	for i := range 6 {
		ctx := context.Background()
		ctx = context.WithValue(ctx, middleware.PrometheusClientKey{}, fmt.Sprintf("wr-%s", clients[i%len(clients)].Name))
		fmt.Printf("Request %d: ", i+1)
		err := pool.DoWeightedRandomClient(ctx, businessLogic)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n⚖️ Demo 3: Using Default Balancer (Round Robin)")
	for i := range 6 {
		ctx := context.Background()
		ctx = context.WithValue(ctx, middleware.PrometheusClientKey{}, fmt.Sprintf("def-%s", clients[i%len(clients)].Name))
		fmt.Printf("Request %d: ", i+1)
		err := pool.Do(ctx, businessLogic)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n🚨 Demo 4: Circuit Breaker Test")
	circuitPool := clientpool.NewClientPool[*HTTPClient](2, 3*time.Second, clientpool.RoundRobin)
	normal := &HTTPClient{Name: "normal", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"}
	failing := &HTTPClient{Name: "failing", Client: &http.Client{Timeout: 1 * time.Millisecond}, URL: "https://httpstat.us/500"}
	circuitPool.AddClient(normal, normal.Name, 1)
	circuitPool.AddClient(failing, failing.Name, 1)
	circuitBusinessLogic := func(ctx context.Context, client *HTTPClient) error {
		return client.Get(ctx)
	}
	for i := range 8 {
		ctx := context.Background()
		ctx = context.WithValue(ctx, middleware.PrometheusClientKey{}, fmt.Sprintf("circuit-%s", []string{"normal", "failing"}[i%2]))
		fmt.Printf("Circuit test request %d: ", i+1)
		err := circuitPool.Do(ctx, circuitBusinessLogic)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n💥 Demo 5: Panic Recovery")
	panicPool := clientpool.NewClientPool[*HTTPClient](3, 5*time.Second, clientpool.RoundRobin)
	panicClient := &HTTPClient{Name: "panic-client", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"}
	panicPool.AddClient(panicClient, panicClient.Name, 1)
	panicLogic := func(ctx context.Context, client *HTTPClient) error {
		if client.Name == "panic-client" {
			panic("intentional panic for testing")
		}
		return nil
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.PrometheusClientKey{}, "panic-client")
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

	fmt.Println("\n⏱️ Keeping server alive for 30 seconds to check metrics...")
	time.Sleep(30 * time.Second)
	fmt.Println("👋 Goodbye!")
}
