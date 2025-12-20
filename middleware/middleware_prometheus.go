package middleware

import (
	"context"
	"fmt"
	"time"

	cw "github.com/bighu630/clientPool/clientWrapper"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "middleware_requests_total",
			Help: "Total number of requests handled by middleware",
		},
		[]string{"client", "method"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "middleware_request_duration_seconds",
			Help:    "Histogram of request processing duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"client", "method"},
	)

	requestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "middleware_request_errors_total",
			Help: "Total number of errors returned by handler",
		},
		[]string{"client", "method"},
	)
)

func init() {
	// 注册指标
	prometheus.MustRegister(requestsTotal, requestDuration, requestErrors)
}

type PrometheusClientKey struct{}

// PrometheusMethodKey 用于在 context 中传递方法名，用于监控统计
type PrometheusMethodKey struct{}

// 从 context 获取 client label
func GetPrometheusClientLabel(ctx context.Context, client any) string {
	if v := ctx.Value(PrometheusClientKey{}); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("%v", client)
}

// GetPrometheusMethodName 从 context 获取方法名
func GetPrometheusMethodName(ctx context.Context) string {
	if v := ctx.Value(PrometheusMethodKey{}); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return "unknown"
}

// PrometheusMiddleware 实现
func PrometheusMiddleware[T any]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error {
		// label := GetPrometheusClientLabel(ctx, client)
		label := client.GetClientId()
		method := GetPrometheusMethodName(ctx)
		start := time.Now()
		requestsTotal.WithLabelValues(label, method).Inc()

		err := next(ctx, client)

		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(label, method).Observe(duration)

		if err != nil {
			requestErrors.WithLabelValues(label, method).Inc()
		}

		return err
	})
}
