package clientPool

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bighu630/clientPool/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPClient struct {
	Name   string
	Client *http.Client
	URL    string
}

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

func startPrometheusServer() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Prometheus metrics server started on :8080/metrics")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	time.Sleep(100 * time.Millisecond)
}

func TestClientPool_BasicFunctionality(t *testing.T) {
	startPrometheusServer()
	pool := NewClientPool[*HTTPClient](3, 5*time.Second, RoundRobin)
	pool.RegisterMiddleware(middleware.PrometheusMiddleware[*HTTPClient]())
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[*HTTPClient](5, 10, 2*time.Second))

	clients := []*HTTPClient{
		{Name: "client1", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "client2", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "client3", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
	}
	for i, client := range clients {
		pool.AddClient(client, client.Name, i+1)
	}

	testFn := func(ctx context.Context, client *HTTPClient) error {
		return client.Get(ctx)
	}

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

func TestClientPool_CircuitBreaker(t *testing.T) {
	pool := NewClientPool[*HTTPClient](2, 3*time.Second, RoundRobin)
	pool.RegisterMiddleware(middleware.PrometheusMiddleware[*HTTPClient]())

	normal := &HTTPClient{Name: "normal_client", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"}
	failing := &HTTPClient{Name: "failing_client", Client: &http.Client{Timeout: 1 * time.Millisecond}, URL: "https://httpstat.us/500"}
	pool.AddClient(normal, "normal_client", 1)
	pool.AddClient(failing, "failing_client", 1)

	testFn := func(ctx context.Context, client *HTTPClient) error {
		return client.Get(ctx)
	}

	fmt.Println("\n=== Testing Circuit Breaker ===")
	for i := 0; i < 5; i++ {
		ctx := context.Background()
		err := pool.Do(ctx, testFn)
		t.Logf("Request %d: %v", i+1, err)
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println("Waiting for circuit breaker recovery...")
	time.Sleep(4 * time.Second)
	for i := 0; i < 3; i++ {
		ctx := context.Background()
		err := pool.Do(ctx, testFn)
		t.Logf("After recovery request %d: %v", i+1, err)
		time.Sleep(500 * time.Millisecond)
	}
}

func TestClientPool_Concurrency(t *testing.T) {
	pool := NewClientPool[*HTTPClient](3, 5*time.Second, WeightedRandom)
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[*HTTPClient](10, 20, 1*time.Second))

	clients := []*HTTPClient{
		{Name: "concurrent_client_1", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "concurrent_client_2", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
		{Name: "concurrent_client_3", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"},
	}
	for i, client := range clients {
		pool.AddClient(client, client.Name, i+1)
	}

	testFn := func(ctx context.Context, client *HTTPClient) error {
		return client.Get(ctx)
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

func TestClientPool_PanicRecovery(t *testing.T) {
	pool := NewClientPool[*HTTPClient](3, 5*time.Second, RoundRobin)
	panicClient := &HTTPClient{Name: "panic_client", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"}
	pool.AddClient(panicClient, "panic_client", 1)
	panicFn := func(ctx context.Context, client *HTTPClient) error {
		if client.Name == "panic_client" {
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

func BenchmarkClientPool(b *testing.B) {
	pool := NewClientPool[*HTTPClient](3, 5*time.Second, RoundRobin)
	pool.RegisterMiddleware(middleware.NewRateLimiterMiddleware[*HTTPClient](1000, 2000, 100*time.Millisecond))
	benchClient := &HTTPClient{Name: "bench_client", Client: &http.Client{Timeout: 10 * time.Second}, URL: "https://www.bilibili.com"}
	pool.AddClient(benchClient, benchClient.Name, 1)
	testFn := func(ctx context.Context, client *HTTPClient) error {
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
