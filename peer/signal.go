package peer

import (
	"context"
	"fmt"
	"github.com/mr-tron/base58"
	"github.com/rtctunnel/rtctunnel/channels"
	_ "github.com/rtctunnel/rtctunnel/channels/operator" // for the default operator channel
	"github.com/rtctunnel/rtctunnel/crypt"
)

type Signal struct {
	ch channels.Channel
}

func NewSignal(ch channels.Channel) *Signal {
	return &Signal{
		ch: ch,
	}
}

func (s *Signal) Recv(ctx context.Context, local crypt.KeyPair, remote crypt.Key) ([]byte, error) {
	address := local.Public.String() + "/" + remote.String()
	encoded, err := s.ch.Recv(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("error receiving message: %w", err)
	}

	decoded, err := base58.Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("message does not appear encoded using base58: %w", err)
	}

	decrypted, err := local.Decrypt(remote, decoded)
	if err != nil {
		return nil, fmt.Errorf("error decrypting message: %w", err)
	}

	return decrypted, nil
}

func (s *Signal) Send(ctx context.Context, local crypt.KeyPair, remote crypt.Key, data []byte) error {
	encrypted := local.Encrypt(remote, data)
	address := remote.String() + "/" + local.Public.String()
	encoded := base58.Encode(encrypted)

	err := s.ch.Send(ctx, address, encoded)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}
