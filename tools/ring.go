package tools

import (
	"container/ring"
	"sync"
)

// Ring encapsulate of ring.Ring
type Ring struct {
	ring     *ring.Ring
	lock     sync.RWMutex
	capacity int
}

// NewRing constructor
func NewRing(capacity int) *Ring {
	return &Ring{
		capacity: capacity,
	}
}

// Add add element
func (r *Ring) Add(p interface{}) {
	if r.capacity <= 0 {
		return
	}
	r.lock.Lock()
	defer r.lock.Unlock()

	item := ring.New(1)
	item.Value = p

	if r.ring.Len() == 0 || r.capacity == 1 {
		r.ring = item
	} else {
		if r.ring.Len() == r.capacity {
			r.delCurrent()
		}
		r.ring.Prev().Link(item)
	}
}

func (r *Ring) delCurrent() {
	if r.ring.Len() > 1 {
		r.ring = r.ring.Prev()
		r.ring.Unlink(1)
		r.ring = r.ring.Next()
	} else {
		r.ring = nil
	}
}

// Do iterate and fileter out processed elements
func (r *Ring) Do(do func(interface{}) bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	for i := 0; i < r.ring.Len(); i++ {
		if do(r.ring.Value) {
			i--
			r.delCurrent()
		} else {
			r.ring = r.ring.Next()
		}
	}
}
