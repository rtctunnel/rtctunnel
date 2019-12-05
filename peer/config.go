package peer

import "github.com/rtctunnel/rtctunnel/channels"

type config struct {
	ch channels.Channel
}

type Option func(*config)

func getConfig(options ...Option) *config {
	cfg := new(config)
	WithChannel(channels.Must(channels.Get("operator://operator.rtctunnel.com")))(cfg)
	for _, option := range options {
		option(cfg)
	}
	return cfg
}

// WithChannel sets the channel for handshake communication.
func WithChannel(ch channels.Channel) Option {
	return func(cfg *config) {
		cfg.ch = ch
	}
}
