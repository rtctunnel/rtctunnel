package peer

import (
	"bufio"
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

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		n1 := NewNetwork(k1, WithChannel(ch))
		conn, err := n1.Connect(ctx, k2.Public, 1)
		if assert.NoError(t, err) {
			assert.NotNil(t, conn)

			_, err = io.WriteString(conn, "Hello World\n")
			assert.NoError(t, err)
		}
	}()
	go func() {
		defer wg.Done()

		n2 := NewNetwork(k2, WithChannel(ch))
		conn, err := n2.Connect(ctx, k1.Public, 1)
		if assert.NoError(t, err) {
			assert.NotNil(t, conn)

			line, _, err := bufio.NewReader(conn).ReadLine()
			assert.NoError(t, err)
			assert.Equal(t, "Hello World", string(line))
		}
	}()
	wg.Wait()
}
