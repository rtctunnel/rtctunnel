package peer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mr-tron/base58"
	"github.com/pion/webrtc/v2"
	"github.com/rtctunnel/rtctunnel/channels"
	_ "github.com/rtctunnel/rtctunnel/channels/operator" // for the default operator channel
	"github.com/rtctunnel/rtctunnel/crypt"
)

type ICEMessage struct {
	SessionDescription webrtc.SessionDescription
	Candidates         []webrtc.ICECandidate
}

type Channel struct {
	ch channels.Channel
}

func NewChannel(ch channels.Channel) *Channel {
	return &Channel{
		ch: ch,
	}
}

func (ch *Channel) Recv(ctx context.Context, local crypt.KeyPair, remote crypt.Key) (webrtc.SessionDescription, error) {
	address := local.Public.String() + "/" + remote.String()
	encoded, err := ch.ch.Recv(ctx, address)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("error receiving message: %w", err)
	}

	decoded, err := base58.Decode(encoded)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("message does not appear encoded using base58: %w", err)
	}

	decrypted, err := local.Decrypt(remote, decoded)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("error decrypting message: %w", err)
	}

	var msg webrtc.SessionDescription
	err = json.Unmarshal(decrypted, &msg)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("invalid Session Description: %w", err)
	}

	return msg, nil
}

func (ch *Channel) Send(ctx context.Context, local crypt.KeyPair, remote crypt.Key, msg webrtc.SessionDescription) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error encoding Session Description: %w", err)
	}

	encrypted := local.Encrypt(remote, data)
	address := remote.String() + "/" + local.Public.String()
	encoded := base58.Encode(encrypted)

	err = ch.ch.Send(ctx, address, encoded)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}
