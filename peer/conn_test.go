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

	var c1, c2 *Conn
	defer func() {
		if c1 != nil {
			c1.Close()
		}
		if c2 != nil {
			c2.Close()
		}
	}()

	var eg errgroup.Group
	eg.Go(func() error {
		var err error
		c1, err = Open(key1, key2.Public, options...)
		if err != nil {
			return err
		}

		stream, port, err := c1.Accept()
		if err != nil {
			return err
		}
		defer stream.Close()

		assert.Equal(t, 9000, port)
		_, err = io.WriteString(stream, "hello world\n")
		assert.NoError(t, err)
		return nil
	})
	eg.Go(func() error {
		var err error
		c2, err = Open(key2, key1.Public, options...)
		if err != nil {
			return err
		}

		stream, err := c2.Open(9000)
		if err != nil {
			return err
		}
		defer stream.Close()

		s := bufio.NewScanner(stream)
		assert.True(t, s.Scan())
		assert.Equal(t, "hello world", s.Text())
		return nil
	})
	assert.NoError(t, eg.Wait())
}
