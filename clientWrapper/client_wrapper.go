package clientWrapper

import (
	"sync"
	"time"
)

type ClientWrapped[T any] interface {
	GetClientId() string
	ResetAvailable()
	MarkFail(maxFail int)
	MarkSuccess()
	GetLastFail() time.Time
	GetWight() int
	GetClient() T
	IsUnavailable() bool
}

type clientWrapped[T any] struct {
	mu          sync.Mutex
	Id          string
	client      T         // 客户端
	weight      int       // 权重
	failCount   int       // 连续失败次数
	lastFail    time.Time // 最后一次失败时间
	unavailable bool      // 是否可用
}

func NewClientWrapper[T any](client T, id string, weight int) ClientWrapped[T] {
	return &clientWrapped[T]{
		Id:     id,
		client: client,
		weight: weight,
	}
}

func (c *clientWrapped[T]) GetClientId() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Id
}

func (c *clientWrapped[T]) ResetAvailable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount = 0
	c.unavailable = false
}

func (c *clientWrapped[T]) MarkFail(maxFail int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount++
	if c.failCount >= maxFail {
		c.unavailable = true
	}
	c.lastFail = time.Now()
}

func (c *clientWrapped[T]) MarkSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount = 0
	c.unavailable = false
}

func (c *clientWrapped[T]) GetLastFail() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastFail
}

func (c *clientWrapped[T]) GetClient() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client
}

func (c *clientWrapped[T]) GetWight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.weight
}

func (c *clientWrapped[T]) IsUnavailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.unavailable
}
