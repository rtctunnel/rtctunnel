package peer

import (
	"bufio"
	"io"
	"testing"

	"github.com/rtctunnel/rtctunnel/channels"
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/rtctunnel/rtctunnel/signal"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

func TestConn(t *testing.T) {
	ch, err := channels.Get("memory://test")
	assert.NoError(t, err)
	options := []signal.Option{signal.WithChannel(ch)}

	key1 := crypt.GenerateKeyPair()
	key2 := crypt.GenerateKeyPair()

	var eg errgroup.Group
	eg.Go(func() error {
		conn1, err := Open(key1, key2.Public, options...)
		if err != nil {
			return err
		}

		conn2, port, err := conn1.Accept()
		if err != nil {
			return err
		}
		defer conn2.Close()

		assert.Equal(t, 9000, port)
		io.WriteString(conn2, "hello world\n")
		return nil
	})
	eg.Go(func() error {
		conn2, err := Open(key2, key1.Public, options...)
		if err != nil {
			return err
		}

		conn1, err := conn2.Open(9000)
		if err != nil {
			return err
		}
		defer conn1.Close()

		s := bufio.NewScanner(conn1)
		assert.True(t, s.Scan())
		assert.Equal(t, "hello world", s.Text())
		return nil
	})
	assert.NoError(t, eg.Wait())
}
