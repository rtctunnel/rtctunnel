package peer

import (
	"context"
	"github.com/rtctunnel/rtctunnel/channels"
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/stretchr/testify/assert"
	"io"
	"sync"
	"testing"
	"time"
)

func TestNetwork_Connect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	k1, k2 := crypt.GenerateKeyPair(), crypt.GenerateKeyPair()
	ch := channels.Must(channels.Get("memory://test"))

	send := []byte("Hello World")

	var c1, c2 *Conn

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		var err error

		n1 := NewNetwork(k1, WithChannel(ch))
		c1, err = n1.Connect(ctx, k2.Public, 1)
		if !assert.NoError(t, err) {
			return
		}

		_, err = c1.Write(send)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()

		var err error

		n2 := NewNetwork(k2, WithChannel(ch))
		c2, err = n2.Connect(ctx, k1.Public, 1)
		if !assert.NoError(t, err) {
			return
		}

		buf := make([]byte, len(send))
		_, err = io.ReadFull(c2, buf)
		assert.NoError(t, err)
		assert.Equal(t, send, buf)
	}()
	wg.Wait()

	if c1 != nil {
		c1.Close()
	}
	if c2 != nil {
		c2.Close()
	}
}
