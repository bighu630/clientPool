package middleware

import (
	"context"
	"sync"
	"time"

	cw "github.com/bighu630/clientPool/clientWrapper"
	"golang.org/x/time/rate"
)

type RateLimiterMiddleware[T any] struct {
	mu      sync.RWMutex
	limiter *rate.Limiter
	timeOut time.Duration
}

func NewRateLimiterMiddleware[T any](r, b int, timeOut time.Duration) Middleware[T] {
	return &RateLimiterMiddleware[T]{
		limiter: rate.NewLimiter(rate.Limit(r), b),
		timeOut: timeOut,
	}
}

func (r *RateLimiterMiddleware[T]) Execute(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error {
	waitCtx := ctx
	if r.timeOut > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, r.timeOut)
		defer cancel()
	}
	if err := r.limiter.Wait(waitCtx); err != nil {
		return NewMiddlewareError("rate limiter", err)
	}
	return next(ctx, client)
}
