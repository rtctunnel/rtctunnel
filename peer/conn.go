package peer

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pion/webrtc/v2"
	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/rtctunnel/crypt"
)

type Network struct {
	local  crypt.KeyPair
	signal *Signal
}

func NewNetwork(local crypt.KeyPair, options ...Option) *Network {
	cfg := getConfig(options...)
	return &Network{
		local:  local,
		signal: NewSignal(cfg.ch),
	}
}

func (network *Network) Connect(ctx context.Context, remote crypt.Key, port int) (*Conn, error) {
	return newConn(ctx, network, remote)
}

type Conn struct {
	network              *Network
	pc                   *webrtc.PeerConnection
	dc                   *webrtc.DataChannel
	incomingR, outgoingR io.ReadCloser
	incomingW, outgoingW io.WriteCloser

	closeOnce  sync.Once
	closeError error
	closed     chan struct{}

	openOnce sync.Once
	opened   chan struct{}
}

func newConn(ctx context.Context, network *Network, remote crypt.Key) (*Conn, error) {
	c := &Conn{
		network: network,

		closed: make(chan struct{}),
		opened: make(chan struct{}),
	}

	c.incomingR, c.incomingW = io.Pipe()
	c.outgoingR, c.outgoingW = io.Pipe()

	go func() {
		for {
			var packet [8192]byte
			n, err := c.outgoingR.Read(packet[:])
			if err != nil {
				c.closeWithError(err)
				return
			}

			log.Info().Bytes("data", packet[:n]).
				Str("local", network.local.Public.String()).
				Str("remote", remote.String()).
				Msg("datachannel sending")

			err = c.dc.Send(packet[:n])
			if err != nil {
				c.closeWithError(err)
				return
			}
		}
	}()

	dcs := make(chan *webrtc.DataChannel, 1)
	err := c.createPeerConnection(ctx, remote, dcs)
	if err != nil {
		return nil, c.closeWithError(err)
	}

	select {
	case <-c.closed:
		return nil, c.closeError
	case <-c.opened:
		return c, nil
	}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	return c.incomingR.Read(b)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	return c.outgoingW.Write(b)
}

func (c *Conn) Close() error {
	return c.closeWithError(nil)
}

func (c *Conn) LocalAddr() net.Addr {
	panic("implement me")
}

func (c *Conn) RemoteAddr() net.Addr {
	panic("implement me")
}

func (c *Conn) SetDeadline(t time.Time) error {
	panic("implement me")
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	panic("implement me")
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	panic("implement me")
}

func (c *Conn) closeWithError(err error) error {
	c.closeOnce.Do(func() {
		var closers []io.Closer
		if c.dc != nil {
			closers = append(closers, c.dc)
		}
		if c.outgoingW != nil {
			closers = append(closers, c.outgoingW)
		}
		if c.incomingW != nil {
			closers = append(closers, c.incomingW)
		}
		if c.pc != nil {
			closers = append(closers, c.pc)
		}

		for _, closer := range closers {
			e := closer.Close()
			if err == nil {
				err = e
			}
		}
		if err == nil {
			err = context.Canceled
		}
		c.closeError = err
		close(c.closed)
	})
	return err
}

func (c *Conn) createPeerConnection(ctx context.Context, remote crypt.Key, dcs chan *webrtc.DataChannel) error {
	s := webrtc.SettingEngine{}
	deadline, ok := ctx.Deadline()
	if ok {
		s.SetConnectionTimeout(time.Until(deadline), 0)
		s.SetCandidateSelectionTimeout(time.Until(deadline))
	}

	var err error

	c.pc, err = webrtc.NewAPI(webrtc.WithSettingEngine(s)).NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return fmt.Errorf("error establishing webrtc peer connection (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err)
	}

	c.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		//log.Info().
		//	Str("local", network.local.Public.String()).
		//	Str("remote", remote.String()).
		//	Str("state", state.String()).
		//	Msg("peerconnection connection state changed")
	})
	c.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		//log.Info().
		//	Str("local", network.local.Public.String()).
		//	Str("remote", remote.String()).
		//	Interface("candidate", candidate).
		//	Msg("peerconnection received ice candidate")
	})
	c.pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		//log.Info().
		//	Str("local", network.local.Public.String()).
		//	Str("remote", remote.String()).
		//	Str("state", state.String()).
		//	Msg("peerconnection ice connection state changed")
		switch state {
		case webrtc.ICEConnectionStateClosed:
			_ = c.closeWithError(fmt.Errorf("peerconnection unexpectedly closed"))
		case webrtc.ICEConnectionStateFailed:
			_ = c.closeWithError(fmt.Errorf("peerconnection failed"))
		}
	})
	c.pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		//log.Info().
		//	Str("local", network.local.Public.String()).
		//	Str("remote", remote.String()).
		//	Str("state", state.String()).
		//	Msg("peerconnection ice gathering state changed")
	})
	c.pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		//log.Info().
		//	Str("local", network.local.Public.String()).
		//	Str("remote", remote.String()).
		//	Str("state", state.String()).
		//	Msg("peerconnection signaling state changed")
	})
	c.pc.OnDataChannel(func(recv *webrtc.DataChannel) {
		log.Info().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Msg("received datachannel")
		select {
		case dcs <- recv:
		default:
		}
	})

	if c.network.local.Public.String() < remote.String() {
		log.Info().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Msg("creating offer")
		offer, err := c.pc.CreateOffer(nil)
		if err != nil {
			return c.closeWithError(fmt.Errorf("error creating webrtc offer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		err = c.pc.SetLocalDescription(offer)
		if err != nil {
			return c.closeWithError(fmt.Errorf("error setting webrtc local description for offer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		err = c.network.signal.Send(ctx, c.network.local, remote, []byte(offer.SDP))
		if err != nil {
			return c.closeWithError(fmt.Errorf("error sending webrtc offer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		rawAnswer, err := c.network.signal.Recv(ctx, c.network.local, remote)
		if err != nil {
			return c.closeWithError(fmt.Errorf("error receiving webrtc answer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		err = c.pc.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  string(rawAnswer),
		})
		if err != nil {
			return c.closeWithError(fmt.Errorf("error setting webrtc remote description for answer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}
	} else {
		log.Info().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Msg("receiving offer")

		rawOffer, err := c.network.signal.Recv(ctx, c.network.local, remote)
		if err != nil {
			return c.closeWithError(fmt.Errorf("error receiving webrtc offer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		err = c.pc.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  string(rawOffer),
		})
		if err != nil {
			return c.closeWithError(fmt.Errorf("error setting webrtc remote description for offer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		answer, err := c.pc.CreateAnswer(nil)
		if err != nil {
			return c.closeWithError(fmt.Errorf("error creating webrtc answer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		err = c.pc.SetLocalDescription(answer)
		if err != nil {
			return c.closeWithError(fmt.Errorf("error setting webrtc local description for answer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}

		err = c.network.signal.Send(ctx, c.network.local, remote, []byte(answer.SDP))
		if err != nil {
			return c.closeWithError(fmt.Errorf("error sending webrtc answer (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}
	}

	return nil
}

func (c *Conn) createDataChannel(ctx context.Context, remote crypt.Key, dcs chan *webrtc.DataChannel) error {
	var err error

	if c.network.local.Public.String() < remote.String() {
		c.dc, err = c.pc.CreateDataChannel("rtctunnel", nil)
		if err != nil {
			return c.closeWithError(fmt.Errorf("error creating webrtc data channel (local=%s remote=%s): %w",
				c.network.local.Public.String(), remote.String(), err))
		}
	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c.dc = <-dcs:
		}
	}

	c.dc.OnClose(func() {
		log.Info().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Msg("datachannel closed")
		_ = c.closeWithError(context.Canceled)
	})
	c.dc.OnError(func(err error) {
		log.Info().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Err(err).
			Msg("datachannel error")
		if err != nil {
			_ = c.closeWithError(err)
		}
	})
	c.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Info().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Bytes("message", msg.Data).
			Msg("datachannel message")
		_, err := c.incomingW.Write(msg.Data)
		if err != nil {
			_ = c.closeWithError(err)
		}
	})
	c.dc.OnOpen(func() {
		log.Info().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Msg("datachannel opened")
		c.openOnce.Do(func() {
			close(c.opened)
		})
	})

	return nil
}
