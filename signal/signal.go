package signal

import (
	"github.com/mr-tron/base58"
	"github.com/rtctunnel/rtctunnel/channels"
	_ "github.com/rtctunnel/rtctunnel/channels/apprtc"   // for the default apprtc channel
	_ "github.com/rtctunnel/rtctunnel/channels/operator" // for the operator channel
	"github.com/rtctunnel/rtctunnel/crypt"
)

type config struct {
	channel channels.Channel
}

var defaultOptions = []Option{
	WithChannel(channels.Must(channels.Get("apprtc://"))),
}

func getConfig(options ...Option) (*config, error) {
	cfg := new(config)
	for _, o := range defaultOptions {
		err := o(cfg)
		if err != nil {
			return nil, err
		}
	}
	for _, o := range options {
		err := o(cfg)
		if err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// An Option customizes the config.
type Option func(cfg *config) error

// WithChannel sets the channel option.
func WithChannel(ch channels.Channel) Option {
	return func(cfg *config) error {
		cfg.channel = ch
		return nil
	}
}

// SetDefaultOptions sets the default options
func SetDefaultOptions(options ...Option) {
	defaultOptions = options
}

// Send sends a message to a peer. Messages are encrypted and authenticated.
func Send(keypair crypt.KeyPair, peerPublicKey crypt.Key, data []byte, options ...Option) error {
	cfg, err := getConfig(options...)
	if err != nil {
		return err
	}
	encrypted := keypair.Encrypt(peerPublicKey, data)
	address := peerPublicKey.String() + "/" + keypair.Public.String()
	encoded := base58.Encode(encrypted)
	return cfg.channel.Send(address, encoded)
}

// Recv receives a message from a peer. Messages are encrypted and authenticated.
func Recv(keypair crypt.KeyPair, peerPublicKey crypt.Key, options ...Option) (data []byte, err error) {
	cfg, err := getConfig(options...)
	if err != nil {
		return nil, err
	}
	address := keypair.Public.String() + "/" + peerPublicKey.String()
	encoded, err := cfg.channel.Recv(address)
	if err != nil {
		return nil, err
	}
	decoded, err := base58.Decode(encoded)
	if err != nil {
		return nil, err
	}
	decrypted, err := keypair.Decrypt(peerPublicKey, decoded)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}
