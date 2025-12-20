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
			Buckets: []float64{0.1, 0.2, 0.5, 1.0, 5.0},
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

type PrometheusClientKey struct{} // 弃用
type PrometheusMethodKey struct{}

// 从 context 获取 client label
func GetPrometheusClientLabel(ctx context.Context, client any) []string {
	labels := []string{}
	if v := ctx.Value(PrometheusClientKey{}); v != nil {
		labels = append(labels, fmt.Sprintf("%v", v))
	}
	if v := ctx.Value(PrometheusMethodKey{}); v != nil {
		labels = append(labels, fmt.Sprintf("%v", v))
	}
	return labels
}

// GetPrometheusMethodName 从 context 获取方法名
func GetPrometheusMethodName(ctx context.Context) string {
	if v := ctx.Value(PrometheusMethodKey{}); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return "unknown"
}

// PrometheusMiddleware 实现
func NewPrometheusMiddleware[T any]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error {
		var labels []string
		labels = append(labels, client.GetClientId())
		ctxLabels := GetPrometheusClientLabel(ctx, client)
		if len(ctxLabels) > 0 {
			labels = append(labels, ctxLabels...)
		}
		start := time.Now()
		requestsTotal.WithLabelValues(labels...).Inc()

		err := next(ctx, client)

		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(labels...).Observe(duration)

		if err != nil {
			requestErrors.WithLabelValues(labels...).Inc()
		}

		return err
	})
}
