package proxy

import (
	"sync"
	"sync/atomic"
)

type WaitGroupCountable struct {
	sync.WaitGroup
	count int64
}

func (wg *WaitGroupCountable) Add(delta int) {
	atomic.AddInt64(&wg.count, int64(delta))
	wg.WaitGroup.Add(delta)
}

func (wg *WaitGroupCountable) Done() {
	atomic.AddInt64(&wg.count, -1)
	wg.WaitGroup.Done()
}

func (wg *WaitGroupCountable) GetCount() int {
	return int(atomic.LoadInt64(&wg.count))
}
