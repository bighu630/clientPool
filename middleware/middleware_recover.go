package middleware

import (
	"context"
	"fmt"
)

func RecoverMiddleware[T any]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client T, next func(ctx context.Context, client T) error) (err error) {
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
