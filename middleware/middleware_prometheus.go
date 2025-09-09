package middleware

import (
	"context"
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

// PrometheusMiddleware 实现
func PrometheusMiddleware[T ~string]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client T, next func(ctx context.Context, client T) error) error {
		start := time.Now()
		requestsTotal.WithLabelValues(string(client)).Inc()

		err := next(ctx, client)

		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(string(client)).Observe(duration)

		if err != nil {
			requestErrors.WithLabelValues(string(client)).Inc()
		}

		return err
	})
}
