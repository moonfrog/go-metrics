package metrics

import "sync/atomic"

// Counters hold an int64 value that can be incremented and decremented.
type Instant interface {
	Clear()
	Count() int64
	Dec(int64)
	Inc(int64)
	Update(int64) // same as Inc
}

// GetOrRegisterCounter returns an existing Instant or constructs and registers
// a new StandardCounter.
func GetOrRegisterInstantCounter(name string, r Registry) Instant {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewInstantCounter).(Instant)
}

// NewInstantCounter constructs a new InstantCounter.
func NewInstantCounter() Instant {
	return &InstantCounter{0}
}

// InstantCounter is the standard implementation of a Instant and uses the
// sync/atomic package to manage a single int64 value.
type InstantCounter struct {
	count int64
}

// Clear sets the counter to zero.
func (c *InstantCounter) Clear() {
	atomic.StoreInt64(&c.count, 0)
}

// Count returns the current count.
func (c *InstantCounter) Count() int64 {
	return atomic.LoadInt64(&c.count)
}

// Dec decrements the counter by the given amount.
func (c *InstantCounter) Dec(i int64) {
	atomic.AddInt64(&c.count, -i)
}

// Inc increments the counter by the given amount.
func (c *InstantCounter) Inc(i int64) {
	atomic.AddInt64(&c.count, i)
}

func (c *InstantCounter) Update(i int64) {
	c.Inc(i)
}
