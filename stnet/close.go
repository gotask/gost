package stnet

import (
	"sync/atomic"
)

type Closer struct {
	lock int32
}

func NewCloser(isclose bool) *Closer {
	if isclose {
		return &Closer{1}
	}
	return &Closer{}
}

func (c *Closer) Close() {
	atomic.StoreInt32(&c.lock, 1)
}

func (c *Closer) Open() {
	atomic.StoreInt32(&c.lock, 0)
}

func (c *Closer) IsClose() bool {
	return atomic.LoadInt32(&c.lock) > 0
}
