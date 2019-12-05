package peer

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pion/webrtc/v2"
	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/rtctunnel/crypt"
)

type Network struct {
	local crypt.KeyPair
	ch    *Channel
}

func NewNetwork(local crypt.KeyPair, options ...Option) *Network {
	cfg := getConfig(options...)
	return &Network{
		local: local,
		ch:    NewChannel(cfg.ch),
	}
}

func (network *Network) Connect(ctx context.Context, remote crypt.Key, port int) (*Conn, error) {
	return newConn(ctx, network, remote, port)
}

type Conn struct {
	network              *Network
	pc                   *webrtc.PeerConnection
	incomingR, outgoingR io.ReadCloser
	incomingW, outgoingW io.WriteCloser

	closeSignal *Signal
	closeErr    error

	haveLocalOffer  *Signal
	haveRemoteOffer *Signal
	stable          *Signal

	dataChannelSignal *Signal
	dataChannel       *webrtc.DataChannel

	peerCloseSignal *Signal
	iceCloseSignal  *Signal
}

func newConn(ctx context.Context, network *Network, remote crypt.Key, port int) (*Conn, error) {
	c := &Conn{
		network: network,

		closeSignal: NewSignal(),

		haveLocalOffer:  NewSignal(),
		haveRemoteOffer: NewSignal(),
		stable:          NewSignal(),

		dataChannelSignal: NewSignal(),
		peerCloseSignal:   NewSignal(),
		iceCloseSignal:    NewSignal(),
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

			err = c.dataChannel.Send(packet[:n])
			if err != nil {
				c.closeWithError(err)
				return
			}
		}
	}()

	err := c.createPeerConnection(ctx, remote, port)
	if err != nil {
		return nil, c.closeWithError(err)
	}

	err = c.establishDataChannel(ctx, remote)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Conn) Read(b []byte) (n int, err error) {
	return c.incomingR.Read(b)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	return c.outgoingW.Write(b)
}

func (c *Conn) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	err := c.closeWithError(nil)

	// wait for everything to finish closing
	select {
	case <-c.peerCloseSignal.C:
	case <-ctx.Done():
	}
	select {
	case <-c.iceCloseSignal.C:
	case <-ctx.Done():
	}

	return err
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
	c.closeSignal.Do(func() {
		var closers []io.Closer
		if c.dataChannel != nil {
			closers = append(closers, c.dataChannel)
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
		c.closeErr = err
	})
	return err
}

func (c *Conn) createPeerConnection(ctx context.Context, remote crypt.Key, port int) error {
	s := webrtc.SettingEngine{
		LoggerFactory: new(loggerFactory),
	}
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
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Str("state", state.String()).
			Msg("peerconnection connection state changed")
		switch state {
		case webrtc.PeerConnectionStateClosed:
			c.peerCloseSignal.Set()
		}
	})
	c.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Interface("candidate", candidate).
			Msg("peerconnection received ice candidate")
	})
	c.pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Str("state", state.String()).
			Msg("peerconnection ice connection state changed")
		switch state {
		case webrtc.ICEConnectionStateClosed:
			_ = c.closeWithError(fmt.Errorf("peerconnection unexpectedly closed"))
			c.iceCloseSignal.Set()
		case webrtc.ICEConnectionStateFailed:
			_ = c.closeWithError(fmt.Errorf("peerconnection failed"))
		}
	})
	c.pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Str("state", state.String()).
			Msg("peerconnection ice gathering state changed")
	})
	c.pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Str("state", state.String()).
			Msg("peerconnection signaling state changed")
		switch state {
		case webrtc.SignalingStateHaveLocalOffer:
			c.haveLocalOffer.Set()
		case webrtc.SignalingStateHaveRemoteOffer:
			c.haveRemoteOffer.Set()
		case webrtc.SignalingStateStable:
			c.stable.Set()
		}
	})
	c.pc.OnDataChannel(func(recv *webrtc.DataChannel) {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Interface("transport", recv.Transport()).
			Msg("received datachannel")
		c.dataChannel = recv
		c.dataChannelSignal.Set()
	})

	if c.network.local.Public.String() < remote.String() {
		err = c.establishOfferingPeerConnection(ctx, remote, port)
	} else {
		err = c.establishAnsweringPeerConnection(ctx, remote)
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) establishOfferingPeerConnection(ctx context.Context, remote crypt.Key, port int) error {
	var err error

	ordered := true
	protocol := "rtctunnel"
	c.dataChannel, err = c.pc.CreateDataChannel(fmt.Sprintf("rtctunnel-%d", port), &webrtc.DataChannelInit{
		Ordered:  &ordered,
		Protocol: &protocol,
	})
	if err != nil {
		return c.closeWithError(fmt.Errorf("error creating webrtc data channel (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err))
	}
	c.dataChannelSignal.Set()

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

	select {
	case <-ctx.Done():
		return c.closeWithError(ctx.Err())
	case <-c.closeSignal.C:
		return c.closeErr
	case <-c.haveLocalOffer.C:
	}

	err = c.network.ch.Send(ctx, c.network.local, remote, offer)
	if err != nil {
		return c.closeWithError(fmt.Errorf("error sending webrtc offer (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err))
	}

	answer, err := c.network.ch.Recv(ctx, c.network.local, remote)
	if err != nil {
		return c.closeWithError(fmt.Errorf("error receiving webrtc answer (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err))
	}

	err = c.pc.SetRemoteDescription(answer)
	if err != nil {
		return c.closeWithError(fmt.Errorf("error setting webrtc remote description for answer (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err))
	}

	select {
	case <-ctx.Done():
		return c.closeWithError(ctx.Err())
	case <-c.closeSignal.C:
		return c.closeErr
	case <-c.stable.C:
	}

	return nil
}

func (c *Conn) establishAnsweringPeerConnection(ctx context.Context, remote crypt.Key) error {
	log.Debug().
		Str("local", c.network.local.Public.String()).
		Str("remote", remote.String()).
		Msg("receiving offer")

	offer, err := c.network.ch.Recv(ctx, c.network.local, remote)
	if err != nil {
		return c.closeWithError(fmt.Errorf("error receiving webrtc offer (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err))
	}

	err = c.pc.SetRemoteDescription(offer)
	if err != nil {
		return c.closeWithError(fmt.Errorf("error setting webrtc remote description for offer (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err))
	}

	select {
	case <-ctx.Done():
		return c.closeWithError(ctx.Err())
	case <-c.closeSignal.C:
		return c.closeErr
	case <-c.haveRemoteOffer.C:
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

	err = c.network.ch.Send(ctx, c.network.local, remote, answer)
	if err != nil {
		return c.closeWithError(fmt.Errorf("error sending webrtc answer (local=%s remote=%s): %w",
			c.network.local.Public.String(), remote.String(), err))
	}

	select {
	case <-ctx.Done():
		return c.closeWithError(ctx.Err())
	case <-c.closeSignal.C:
		return c.closeErr
	case <-c.stable.C:
	}

	return nil
}

func (c *Conn) establishDataChannel(ctx context.Context, remote crypt.Key) error {
	select {
	case <-ctx.Done():
		return c.closeWithError(ctx.Err())
	case <-c.dataChannelSignal.C:
	}

	c.dataChannel.OnClose(func() {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Msg("datachannel closed")
		_ = c.closeWithError(context.Canceled)
	})
	c.dataChannel.OnError(func(err error) {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Err(err).
			Msg("datachannel error")
		if err != nil {
			_ = c.closeWithError(err)
		}
	})
	c.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Bytes("message", msg.Data).
			Msg("datachannel message")
		_, err := c.incomingW.Write(msg.Data)
		if err != nil {
			_ = c.closeWithError(err)
		}
	})

	opened := NewSignal()
	c.dataChannel.OnOpen(func() {
		log.Debug().
			Str("local", c.network.local.Public.String()).
			Str("remote", remote.String()).
			Msg("datachannel opened")
		// for reasons I don't understand, when opened the data channel is not immediately available for use
		time.Sleep(50 * time.Millisecond)
		opened.Set()
	})

	select {
	case <-ctx.Done():
		return c.closeWithError(ctx.Err())
	case <-opened.C:
	}

	return nil
}
