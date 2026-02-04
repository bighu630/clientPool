package middleware

import (
	"context"
	"time"

	cw "github.com/bighu630/clientPool/clientWrapper"
)

func NewTimeoutMiddleware[T any](timeout time.Duration) Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return next(ctx, client)
	})
}
