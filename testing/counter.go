package testing

import "sync"

// Counter provides a thread-safe counter for tracking function calls
type Counter struct {
	mu    sync.Mutex
	count int
}

// Inc increments the counter by 1
func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

// Total returns the current count
func (c *Counter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// Reset resets the counter to 0
func (c *Counter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count = 0
}
