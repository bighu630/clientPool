package clientpool

import "time"

func (c *ClientPool[T]) roundRobin() (*clientWrapper[T], error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var client *clientWrapper[T]
	if len(c.clients) == 0 {
		return client, NotAvailableClientError
	}
	for i := 0; i < len(c.clients); i++ {
		cw := c.clients[c.index%len(c.clients)]
		c.index++
		if cw.unavailable && time.Since(cw.lastFail) > c.cooldown {
			cw.resetAvailable()
		}
		if !cw.isUnavailable() {
			return cw, nil
		}
	}
	return client, NotAvailableClientError
}

func (c *ClientPool[T]) weightedRandom() (*clientWrapper[T], error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var zero *clientWrapper[T]
	if len(c.clients) == 0 {
		return zero, NotAvailableClientError
	}

	// 计算总权重
	total := 0
	validClients := make([]*clientWrapper[T], 0)
	for _, cw := range c.clients {
		if cw.unavailable && time.Since(cw.lastFail) > c.cooldown {
			cw.resetAvailable()
		}
		if !cw.unavailable {
			total += cw.Weight
			validClients = append(validClients, cw)
		}
	}
	if total == 0 {
		return zero, NotAvailableClientError
	}

	// 随机挑选
	r := c.rand.Intn(total)
	sum := 0
	for _, cw := range validClients {
		sum += cw.Weight
		if r < sum {
			return cw, nil
		}
	}

	return zero, NotAvailableClientError
}

func (c *ClientPool[T]) random() (*clientWrapper[T], error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var client *clientWrapper[T]
	if len(c.clients) == 0 {
		return client, NotAvailableClientError
	}
	cw := c.clients[c.rand.Intn(len(c.clients))]
	if cw.unavailable && time.Since(cw.lastFail) > c.cooldown {
		cw.resetAvailable()
	}
	if !cw.isUnavailable() {
		return cw, nil
	}
	return client, NotAvailableClientError
}
