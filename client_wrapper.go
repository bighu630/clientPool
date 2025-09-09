package clientPool

import (
	"sync"
	"time"
)

type clientWrapper[T any] struct {
	mu          sync.Mutex
	Client      T         // 客户端
	Weight      int       // 权重
	failCount   int       // 连续失败次数
	lastFail    time.Time // 最后一次失败时间
	unavailable bool      // 是否可用
}

func newClientWrapper[T any](client T, weight int) *clientWrapper[T] {
	return &clientWrapper[T]{
		Client: client,
		Weight: weight,
	}
}

func (c *clientWrapper[T]) resetAvailable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount = 0
	c.unavailable = false
}

func (c *clientWrapper[T]) markFail(maxFail int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount++
	if c.failCount >= maxFail {
		c.unavailable = true
	}
	c.lastFail = time.Now()
}

func (c *clientWrapper[T]) markSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount = 0
	c.unavailable = false
}

func (c *clientWrapper[T]) getClient() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Client
}

func (c *clientWrapper[T]) isUnavailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.unavailable
}
