package clientPool

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/bighu630/clientPool/clientWrapper"
	"github.com/bighu630/clientPool/middleware"
)

var NoAvailableClientError = errors.New("no available client")

type BalancerType string

const (
	RoundRobin     BalancerType = "round_robin"
	WeightedRandom BalancerType = "weighted_random"
	Random         BalancerType = "random"
)

type ClientPool[T any] struct {
	mu              sync.RWMutex
	clients         []clientWrapper.ClientWrapped[T]
	index           int
	rand            *rand.Rand
	maxFails        int           // 最大失败次数
	cooldown        time.Duration // 熔断恢复时间
	defaultBalancer BalancerType
	middlewares     []middleware.Middleware[T]
}

func NewClientPool[T any](maxFails int, cooldown time.Duration, defaultBalancer BalancerType) *ClientPool[T] {
	c := &ClientPool[T]{
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		maxFails:        maxFails,
		cooldown:        cooldown,
		defaultBalancer: defaultBalancer,
		middlewares:     make([]middleware.Middleware[T], 0),
	}
	c.RegisterMiddleware(middleware.RecoverMiddleware[T]())
	return c
}

func (c *ClientPool[T]) GetClientPool() []clientWrapper.ClientWrapped[T] {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clients
}

// 添加client, if weight <= 0, weight = 1
func (c *ClientPool[T]) AddClient(client T, id string, weight int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if weight <= 0 {
		weight = 1
	}
	c.clients = append(c.clients, clientWrapper.NewClientWrapper(client, id, weight))
}

// middleware需要有序添加
func (c *ClientPool[T]) RegisterMiddleware(middleware middleware.Middleware[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, middleware)
}

func (c *ClientPool[T]) executeWithMiddleware(ctx context.Context, client clientWrapper.ClientWrapped[T], fn func(ctx context.Context, client T) error) error {
	handler := func(ctx context.Context, client clientWrapper.ClientWrapped[T]) error {
		return fn(ctx, client.GetClient())
	}
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		next := handler
		m := c.middlewares[i]
		handler = func(ctx context.Context, client clientWrapper.ClientWrapped[T]) error {
			return m.Execute(ctx, client, next)
		}
	}
	return handler(ctx, client)
}

func (c *ClientPool[T]) Do(ctx context.Context, fn func(ctx context.Context, client T) error) error {
	switch c.defaultBalancer {
	case RoundRobin:
		return c.DoRoundRobinClient(ctx, fn)
	case WeightedRandom:
		return c.DoWeightedRandomClient(ctx, fn)
	default:
		return c.DoRandomClient(ctx, fn)
	}
}

func (c *ClientPool[T]) doWithClient(ctx context.Context, cw clientWrapper.ClientWrapped[T], fn func(ctx context.Context, client T) error) error {
	err := c.executeWithMiddleware(ctx, cw, fn)
	if err != nil {
		// 中间件自身的错误（如限流超时）不应标记客户端失败
		if !middleware.IsMiddlewareError(err) {
			cw.MarkFail(c.maxFails)
		}
	} else {
		cw.MarkSuccess()
	}
	return err
}

// 随机选择可用的client
func (c *ClientPool[T]) DoRandomClient(ctx context.Context, fn func(ctx context.Context, client T) error) error {
	cw, err := c.random()
	if err != nil {
		return err
	}
	return c.doWithClient(ctx, cw, fn)
}

// 轮询选择可用的client
func (c *ClientPool[T]) DoRoundRobinClient(ctx context.Context, fn func(ctx context.Context, client T) error) error {
	cw, err := c.roundRobin()
	if err != nil {
		return err
	}
	return c.doWithClient(ctx, cw, fn)
}

// 按权重随机选择可用的client
func (c *ClientPool[T]) DoWeightedRandomClient(ctx context.Context, fn func(ctx context.Context, client T) error) error {
	cw, err := c.weightedRandom()
	if err != nil {
		return err
	}
	return c.doWithClient(ctx, cw, fn)
}

// Close 关闭池中所有实现了 io.Closer 的客户端
func (c *ClientPool[T]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var errs []error
	for _, cw := range c.clients {
		if closer, ok := any(cw.GetClient()).(io.Closer); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	c.clients = nil
	return errors.Join(errs...)
}
