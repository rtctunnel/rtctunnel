package peer

import "sync"

type Signal struct {
	C    chan struct{}
	once sync.Once
}

func NewSignal() *Signal {
	return &Signal{C: make(chan struct{})}
}

func (s *Signal) Do(f func()) {
	s.once.Do(func() {
		f()
		close(s.C)
	})
}

func (s *Signal) Set() {
	s.Do(func() {})
}
