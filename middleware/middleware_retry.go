package middleware

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
	cw "github.com/bighu630/clientPool/clientWrapper"
)

func NewRetryMiddleware[T any]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error {
		return retry.Do(func() error { return next(ctx, client) }, retry.LastErrorOnly(true), retry.Delay(200*time.Millisecond), retry.Attempts(6))
	})
}
