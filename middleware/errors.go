package middleware

import "fmt"

// MiddlewareError 表示中间件自身产生的错误（如限流超时），
// 与业务逻辑错误区分，避免误触发熔断。
type MiddlewareError struct {
	Middleware string
	Err        error
}

func (e *MiddlewareError) Error() string {
	return fmt.Sprintf("middleware [%s]: %v", e.Middleware, e.Err)
}

func (e *MiddlewareError) Unwrap() error {
	return e.Err
}

func NewMiddlewareError(name string, err error) *MiddlewareError {
	return &MiddlewareError{Middleware: name, Err: err}
}

// IsMiddlewareError 判断错误是否为中间件自身的错误
func IsMiddlewareError(err error) bool {
	_, ok := err.(*MiddlewareError)
	return ok
}
