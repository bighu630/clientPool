package middleware

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type traceKey struct{}

// 从 context 获取 traceId
func GetTraceID(ctx context.Context) string {
	if v := ctx.Value(traceKey{}); v != nil {
		if tid, ok := v.(string); ok {
			return tid
		}
	}
	return ""
}

// TraceMiddleware 负责生成或传递 traceId
func TraceMiddleware[T ~string]() Middleware[T] {
	return WrapMiddleware(func(ctx context.Context, client T, next func(ctx context.Context, client T) error) error {
		traceID := GetTraceID(ctx)
		if traceID == "" {
			traceID = uuid.NewString() // 生成新的 TraceID
			ctx = context.WithValue(ctx, traceKey{}, traceID)
		}

		fmt.Printf("[Trace] traceID=%s client=%s -> start\n", traceID, client)
		err := next(ctx, client)
		fmt.Printf("[Trace] traceID=%s client=%s <- end (err=%v)\n", traceID, client, err)

		return err
	})
}
