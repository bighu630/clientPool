package middleware

import (
	"context"

	cw "github.com/bighu630/clientPool/clientWrapper"
)

type MiddlewareFunc[T any] func(ctx context.Context, clinet cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error

type Middleware[T any] interface {
	Execute(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error
}

type middlewareWrapper[T any] struct {
	fn MiddlewareFunc[T]
}

func (m middlewareWrapper[T]) Execute(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) error {
	return m.fn(ctx, client, next)
}

// 把函数转换为 Middleware
func WrapMiddleware[T any](fn MiddlewareFunc[T]) Middleware[T] {
	return middlewareWrapper[T]{fn: fn}
}
