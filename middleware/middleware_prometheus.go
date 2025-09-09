package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "middleware_requests_total",
			Help: "Total number of requests handled by middleware",
		},
		[]string{"client"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "middleware_request_duration_seconds",
			Help:    "Histogram of request processing duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"client"},
	)

	requestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "middleware_request_errors_total",
			Help: "Total number of errors returned by handler",
		},
		[]string{"client"},
	)
)

func init() {
	// 注册指标
	prometheus.MustRegister(requestsTotal, requestDuration, requestErrors)
}

type PrometheusClientKey struct{}

// 从 context 获取 client label
func GetPrometheusClientLabel(ctx context.Context, client any) string {
	if v := ctx.Value(PrometheusClientKey{}); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("%v", client)
}

// PrometheusMiddleware 实现
func PrometheusMiddleware[T any]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client T, next func(ctx context.Context, client T) error) error {
		label := GetPrometheusClientLabel(ctx, client)
		start := time.Now()
		requestsTotal.WithLabelValues(label).Inc()

		err := next(ctx, client)

		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(label).Observe(duration)

		if err != nil {
			requestErrors.WithLabelValues(label).Inc()
		}

		return err
	})
}
