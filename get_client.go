package clientPool

import (
	"time"

	"github.com/bighu630/clientPool/clientWrapper"
)

func (c *ClientPool[T]) roundRobin() (clientWrapper.ClientWrapped[T], error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var client clientWrapper.ClientWrapped[T]
	if len(c.clients) == 0 {
		return client, NoAvailableClientError
	}
	for i := 0; i < len(c.clients); i++ {
		cw := c.clients[c.index%len(c.clients)]
		c.index++
		if cw.IsUnavailable() && time.Since(cw.GetLastFail()) > c.cooldown {
			cw.ResetAvailable()
		}
		if !cw.IsUnavailable() {
			return cw, nil
		}
	}
	return client, NoAvailableClientError
}

func (c *ClientPool[T]) weightedRandom() (clientWrapper.ClientWrapped[T], error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var zero clientWrapper.ClientWrapped[T]
	if len(c.clients) == 0 {
		return zero, NoAvailableClientError
	}

	// 计算总权重
	total := 0
	validClients := make([]clientWrapper.ClientWrapped[T], 0)
	for _, cw := range c.clients {
		if cw.IsUnavailable() && time.Since(cw.GetLastFail()) > c.cooldown {
			cw.ResetAvailable()
		}
		if !cw.IsUnavailable() {
			total += cw.GetWight()
			validClients = append(validClients, cw)
		}
	}
	if total == 0 {
		return zero, NoAvailableClientError
	}

	// 随机挑选
	r := c.rand.Intn(total)
	sum := 0
	for _, cw := range validClients {
		sum += cw.GetWight()
		if r < sum {
			return cw, nil
		}
	}

	return zero, NoAvailableClientError
}

func (c *ClientPool[T]) random() (clientWrapper.ClientWrapped[T], error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var client clientWrapper.ClientWrapped[T]
	if len(c.clients) == 0 {
		return client, NoAvailableClientError
	}
	cw := c.clients[c.rand.Intn(len(c.clients))]
	if cw.IsUnavailable() && time.Since(cw.GetLastFail()) > c.cooldown {
		cw.ResetAvailable()
	}
	if !cw.IsUnavailable() {
		return cw, nil
	}
	return client, NoAvailableClientError
}
