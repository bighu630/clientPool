package clientpool

import (
	"client_pool/middleware"
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

var NotAvailableClientError = errors.New("not available client")

type BalancerType string

const (
	RoundRobin     BalancerType = "round_robin"
	WeightedRandom BalancerType = "weighted_random"
	Random         BalancerType = "random"
)

type ClientPool[T any] struct {
	mu              sync.RWMutex
	clients         []*clientWrapper[T]
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

// 添加client, if weight <= 0, weight = 1
func (c *ClientPool[T]) AddClient(client T, weight int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if weight <= 0 {
		weight = 1
	}
	c.clients = append(c.clients, newClientWrapper(client, weight))
}

// middleware需要有序添加
func (c *ClientPool[T]) RegisterMiddleware(middleware middleware.Middleware[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, middleware)
}

func (c *ClientPool[T]) executeWithMiddleware(ctx context.Context, client T, fn func(ctx context.Context, client T) error) error {
	handler := func(ctx context.Context, client T) error {
		return fn(ctx, client)
	}
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		next := handler
		m := c.middlewares[i]
		handler = func(ctx context.Context, client T) error {
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

// 轮训可用的client
func (c *ClientPool[T]) DoRandomClient(ctx context.Context, fn func(ctx context.Context, client T) error) error {
	clientWrapper, err := c.random()
	if err != nil {
		return err
	}
	err = c.executeWithMiddleware(ctx, clientWrapper.getClient(), fn)
	if err != nil {
		clientWrapper.markFail(c.maxFails)
	} else {
		clientWrapper.markSuccess()
	}
	return err
}

// 随机选择可用的client
func (c *ClientPool[T]) DoRoundRobinClient(ctx context.Context, fn func(ctx context.Context, client T) error) error {
	clientWrapper, err := c.roundRobin()
	if err != nil {
		return err
	}
	err = c.executeWithMiddleware(ctx, clientWrapper.getClient(), fn)
	if err != nil {
		clientWrapper.markFail(c.maxFails)
	} else {
		clientWrapper.markSuccess()
	}
	return err
}

// 按权重随机选择可用的client
func (c *ClientPool[T]) DoWeightedRandomClient(ctx context.Context, fn func(ctx context.Context, client T) error) error {
	clientWrapper, err := c.weightedRandom()
	if err != nil {
		return err
	}
	err = c.executeWithMiddleware(ctx, clientWrapper.getClient(), fn)
	if err != nil {
		clientWrapper.markFail(c.maxFails)
	} else {
		clientWrapper.markSuccess()
	}
	return err
}
