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
	if r.timeOut > 0 {
		waitCtx, cancel := context.WithTimeout(context.Background(), r.timeOut) // reta 的timeout用自己独立的ctx
		defer cancel()
		if err := r.limiter.Wait(waitCtx); err != nil {
			return err
		}
	} else {
		if err := r.limiter.Wait(ctx); err != nil {
			return err
		}
	}
	return next(ctx, client)
}
