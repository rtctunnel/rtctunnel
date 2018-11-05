package main

import (
	"context"
	"sync"
)

type opResult struct {
	data string
	err  error
}
type opWaiter struct {
	addr string
	ch   chan string
}
type op struct {
	ctx    context.Context
	addr   string
	data   string
	result chan opResult
}

type operator struct {
	pubc   chan op
	unpubc chan opWaiter
	subc   chan op
	unsubc chan opWaiter

	pendingPubs map[string]map[chan string]struct{}
	pendingSubs map[string]map[chan string]struct{}

	once   sync.Once
	closer chan struct{}
}

func newOperator() *operator {
	o := &operator{
		pubc:   make(chan op),
		unpubc: make(chan opWaiter),
		subc:   make(chan op),
		unsubc: make(chan opWaiter),

		pendingPubs: make(map[string]map[chan string]struct{}),
		pendingSubs: make(map[string]map[chan string]struct{}),

		closer: make(chan struct{}),
	}
	go o.run()
	return o
}

func (o *operator) Close() error {
	o.once.Do(func() {
		close(o.closer)
	})
	return nil
}

func (o *operator) Pub(ctx context.Context, addr, data string) error {
	c := make(chan opResult, 1)
	select {
	case o.pubc <- op{ctx: ctx, addr: addr, data: data, result: c}:
	case <-o.closer:
		return context.Canceled
	}

	select {
	case r := <-c:
		return r.err
	case <-o.closer:
		return context.Canceled
	}
}

func (o *operator) Sub(ctx context.Context, addr string) (string, error) {
	c := make(chan opResult, 1)
	select {
	case o.subc <- op{ctx: ctx, addr: addr, result: c}:
	case <-o.closer:
		return "", context.Canceled
	}

	select {
	case r := <-c:
		return r.data, r.err
	case <-o.closer:
		return "", context.Canceled
	}
}

func (o *operator) run() {
	for {
		select {
		case op := <-o.pubc:
			_ = o.pubNow(op.addr, op.data, op.result) || o.pubWait(op.ctx, op.addr, op.data, op.result)
		case opw := <-o.unpubc:
			_ = o.unpubNow(opw.addr, opw.ch)

		case op := <-o.subc:
			_ = o.subNow(op.addr, op.result) || o.subWait(op.ctx, op.addr, op.result)
		case opw := <-o.unsubc:
			_ = o.unsubNow(opw)

		case <-o.closer:
			return
		}
	}
}

func (o *operator) pubNow(addr, data string, result chan opResult) bool {
	scs, ok := o.pendingSubs[addr]
	if ok {
		for sc := range scs {
			select {
			case sc <- data:
				result <- opResult{}
				return true
			default:
			}
		}
	}
	return false
}

func (o *operator) pubWait(ctx context.Context, addr, data string, result chan opResult) bool {
	pcs, ok := o.pendingPubs[addr]
	if !ok {
		pcs = make(map[chan string]struct{})
		o.pendingPubs[addr] = pcs
	}

	ch := make(chan string)
	pcs[ch] = struct{}{}
	go func() {
		select {
		case <-ctx.Done():
			result <- opResult{err: ctx.Err()}
		case ch <- data:
			result <- opResult{}
		}

		select {
		case o.unpubc <- opWaiter{addr: addr, ch: ch}:
		case <-o.closer:
		}
	}()

	return true
}

func (o *operator) unpubNow(addr string, c chan string) bool {
	cs, ok := o.pendingPubs[addr]
	if ok {
		delete(cs, c)
		if len(cs) == 0 {
			delete(o.pendingPubs, addr)
		}
	}
	return true
}

func (o *operator) subNow(addr string, result chan opResult) bool {
	cs, ok := o.pendingPubs[addr]
	if ok {
		for c := range cs {
			select {
			case data := <-c:
				result <- opResult{data: data}
				return true
			default:
			}
		}
	}
	return false
}

func (o *operator) subWait(ctx context.Context, addr string, result chan opResult) bool {
	cs, ok := o.pendingSubs[addr]
	if !ok {
		cs = make(map[chan string]struct{})
		o.pendingSubs[addr] = cs
	}

	c := make(chan string)
	cs[c] = struct{}{}
	go func() {
		select {
		case <-ctx.Done():
			result <- opResult{err: ctx.Err()}
		case data := <-c:
			result <- opResult{data: data}
		}

		select {
		case o.unsubc <- opWaiter{addr: addr, ch: c}:
		case <-o.closer:
		}
	}()

	return true
}

func (o *operator) unsubNow(opw opWaiter) bool {
	cs, ok := o.pendingSubs[opw.addr]
	if ok {
		delete(cs, opw.ch)
		if len(cs) == 0 {
			delete(o.pendingSubs, opw.addr)
		}
	}
	return true
}
