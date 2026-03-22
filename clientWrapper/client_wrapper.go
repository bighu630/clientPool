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
	// 不可变字段，初始化后不再改变，无需加锁
	id     string
	client T   // 客户端
	weight int // 权重

	// 可变字段，需要加锁保护
	mu          sync.Mutex
	failCount   int       // 连续失败次数
	lastFail    time.Time // 最后一次失败时间
	unavailable bool      // 是否可用
}

func NewClientWrapper[T any](client T, id string, weight int) ClientWrapped[T] {
	return &clientWrapped[T]{
		id:     id,
		client: client,
		weight: weight,
	}
}

// GetClientId 返回客户端ID（不可变字段，无需加锁）
func (c *clientWrapped[T]) GetClientId() string {
	return c.id
}

func (c *clientWrapped[T]) ResetAvailable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount = 0
	c.unavailable = false
}

func (c *clientWrapped[T]) MarkFail(maxFail int) {
	if maxFail == 0 {
		return
	}
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

// GetClient 返回客户端实例（不可变字段，无需加锁）
func (c *clientWrapped[T]) GetClient() T {
	return c.client
}

// GetWight 返回权重（不可变字段，无需加锁）
func (c *clientWrapped[T]) GetWight() int {
	return c.weight
}

func (c *clientWrapped[T]) IsUnavailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.unavailable && c.failCount > 0
}
