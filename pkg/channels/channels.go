package channels

import (
	"fmt"
	"net/url"
	"sync"
)

// A Channel facilitates signaling.
type Channel interface {
	Send(key, data string) error
	Recv(key string) (data string, err error)
}

// A Factory returns a Channel from an address
type Factory = func(addr string) (Channel, error)

var channelFactories = struct {
	sync.Mutex
	m map[string]Factory
}{
	m: make(map[string]Factory),
}

// RegisterFactory registers a new Factory
func RegisterFactory(scheme string, factory Factory) {
	channelFactories.Lock()
	channelFactories.m[scheme] = factory
	channelFactories.Unlock()
}

// Get returns a channel for the given address
func Get(addr string) (Channel, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	channelFactories.Lock()
	factory, ok := channelFactories.m[u.Scheme]
	channelFactories.Unlock()
	if !ok {
		return nil, fmt.Errorf("no channel factory registered for %s", u.Scheme)
	}

	return factory(addr)
}

// Must panics if there's an error
func Must(ch Channel, err error) Channel {
	if err != nil {
		panic(err)
	}
	return ch
}
