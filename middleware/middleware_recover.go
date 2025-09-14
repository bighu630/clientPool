package middleware

import (
	"context"
	"fmt"

	cw "github.com/bighu630/clientPool/clientWrapper"
)

func RecoverMiddleware[T any]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client cw.ClientWrapped[T], next func(ctx context.Context, client cw.ClientWrapped[T]) error) (err error) {
		// 捕获 panic
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic recovered: %v", r)
			}
		}()

		// 调用下一层
		return next(ctx, client)
	})
}
