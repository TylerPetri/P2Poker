package protocol

import "sync/atomic"

type Lamport struct{ v uint64 }

func (l *Lamport) Now() uint64       { return atomic.LoadUint64(&l.v) }
func (l *Lamport) TickLocal() uint64 { return atomic.AddUint64(&l.v, 1) }
func (l *Lamport) TickRemote(remote uint64) uint64 {
	for {
		cur := atomic.LoadUint64(&l.v)
		next := cur
		if remote >= cur {
			next = remote + 1
		} else {
			next = cur + 1
		}
		if atomic.CompareAndSwapUint64(&l.v, cur, next) {
			return next
		}
	}
}
