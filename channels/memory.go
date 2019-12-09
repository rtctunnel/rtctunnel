package channels

import (
	"sync"

	"github.com/apex/log"
)

func init() {
	RegisterFactory("memory", func(addr string) (Channel, error) {
		addr = addr[len("memory://"):]
		ch, err := newMemoryChannel(addr)
		return ch, err
	})
}

var memoryChannels = struct {
	sync.RWMutex
	channels map[string]chan string
}{
	channels: map[string]chan string{},
}

type memoryChannel struct {
	prefix string
}

// newMemoryChannel creates a new memoryChannel
func newMemoryChannel(addr string) (*memoryChannel, error) {
	return &memoryChannel{prefix: addr}, nil
}

func (mch *memoryChannel) Send(key, data string) error {
	log.WithField("key", key).
		WithField("data", data).
		Debug("[MemoryChannel] sending")
	mch.getChannel(key) <- data
	return nil
}

func (mch *memoryChannel) Recv(key string) (data string, err error) {
	log.WithField("key", key).
		Debug("[MemoryChannel] receiving")
	data = <-mch.getChannel(key)
	return data, nil
}

func (mch *memoryChannel) getChannel(key string) chan string {
	key = mch.prefix + key
	memoryChannels.RLock()
	ch, ok := memoryChannels.channels[key]
	memoryChannels.RUnlock()
	if !ok {
		memoryChannels.Lock()
		ch, ok = memoryChannels.channels[key]
		if !ok {
			ch = make(chan string, 1)
			memoryChannels.channels[key] = ch
		}
		memoryChannels.Unlock()
	}
	return ch
}
