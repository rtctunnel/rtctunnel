package peer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rtctunnel/rtctunnel/internal/crypt"
	"github.com/rtctunnel/rtctunnel/internal/signal"
)

// Conn wraps an RTCPeerConnection so connections can be made and accepted.
type Conn struct {
	keypair       crypt.KeyPair
	peerPublicKey crypt.Key

	pc RTCPeerConnection

	closeCond *Cond
	closeErr  error

	incoming chan RTCDataChannel
}

// Accept accepts a new connection over the datachannel.
func (conn *Conn) Accept() (stream net.Conn, port int, err error) {
	for dc := range conn.incoming {
		lbl := dc.Label()
		idx := strings.LastIndexByte(lbl, ':')
		if idx < 0 {
			log.Info().Str("label", lbl).Msg("ignoring datachannel")
			continue
		}
		name := lbl[:idx]
		port, err := strconv.Atoi(lbl[idx+1:])
		if err != nil || name != "rtctunnel" {
			log.Info().Str("label", lbl).Msg("ignoring datachannel")
			continue
		}

		stream, err := WrapDataChannel(dc)
		if errors.Is(err, ErrClosedByPeer) {
			log.Info().Str("label", lbl).Msg("ignoring datachannel: closed by peer")
			continue
		} else if err != nil {
			dc.Close()
			return nil, 0, err
		}

		log.Info().
			Str("peer", conn.peerPublicKey.String()).
			Int("port", port).
			Msg("accepted connection")

		return stream, port, nil
	}
	return nil, 0, context.Canceled
}

// Open opens a new connection over the datachannel.
func (conn *Conn) Open(port int) (stream net.Conn, err error) {
	dc, err := conn.pc.CreateDataChannel(fmt.Sprintf("rtctunnel:%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to open RTCDataChannel: %w", err)
	}

	stream, err = WrapDataChannel(dc)
	if err != nil {
		dc.Close()
		return nil, err
	}

	log.Info().
		Str("peer", conn.peerPublicKey.String()).
		Int("port", port).
		Msg("opened connection")

	return stream, err
}

// Close closes the peer connection
func (conn *Conn) Close() error {
	return conn.closeWithError(conn.closeErr)
}

func (conn *Conn) closeWithError(err error) error {
	conn.closeCond.Do(func() {
		if conn.pc != nil {
			e := conn.pc.Close()
			if err == nil {
				err = e
			}
		}
	})
	if conn.closeErr != nil {
		err = conn.closeErr
	}
	return err
}

// Open opens a new Connection.
func Open(keypair crypt.KeyPair, peerPublicKey crypt.Key, options ...signal.Option) (*Conn, error) {
	conn := &Conn{
		keypair:       keypair,
		peerPublicKey: peerPublicKey,

		closeCond: NewCond(),

		incoming: make(chan RTCDataChannel, 1),
	}

	log.Info().
		Str("peer", peerPublicKey.String()).
		Msg("creating webrtc peer connection")

	connected := NewCond()

	iceReady := NewCond()
	var iceCandidates []string

	var err error
	conn.pc, err = NewRTCPeerConnection()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create webrtc peer connection: %w", err)
	}
	conn.pc.OnICECandidate(func(candidate string) {
		if candidate == "" {
			iceReady.Signal()
		} else {
			iceCandidates = append(iceCandidates, candidate)
		}
	})
	conn.pc.OnICEConnectionStateChange(func(state string) {
		switch state {
		case "connected":
			connected.Signal()
		case "closed":
			_ = conn.closeWithError(context.Canceled)
		}
	})
	conn.pc.OnDataChannel(func(dc RTCDataChannel) {
		conn.incoming <- dc
	})

	if keypair.Public.String() < peerPublicKey.String() {
		_, err := conn.pc.CreateDataChannel("rtctunnel:init")
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating init datachannel: %w", err))
		}

		// we create the offer
		offer, err := conn.pc.CreateOffer()
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating webrtc offer: %w", err))
		}

		// wait for the ice candidates
		select {
		case <-iceReady.C:
		case <-conn.closeCond.C:
			return nil, conn.closeWithError(context.Canceled)
		}

		err = sendSignal(keypair, peerPublicKey, &SignalMessage{
			SDP:           offer,
			ICECandidates: iceCandidates,
		}, options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error sending offer: %w", err))
		}

		answer, err := recvSignal(keypair, peerPublicKey, options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error receiving webrtc answer: %w", err))
		}

		err = conn.pc.SetAnswer(answer.SDP)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error setting webrtc answer: %w", err))
		}

		for _, candidate := range answer.ICECandidates {
			err = conn.pc.AddICECandidate(candidate)
			if err != nil {
				return nil, conn.closeWithError(fmt.Errorf("error adding ice candidate: %w", err))
			}
		}

	} else {
		offer, err := recvSignal(keypair, peerPublicKey, options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error receiving webrtc offer: %w", err))
		}

		err = conn.pc.SetOffer(offer.SDP)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error setting webrtc offer: %w", err))
		}

		answer, err := conn.pc.CreateAnswer()
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating webrtc answer: %w", err))
		}

		for _, candidate := range offer.ICECandidates {
			err = conn.pc.AddICECandidate(candidate)
			if err != nil {
				return nil, conn.closeWithError(fmt.Errorf("error adding ice candidate: %w", err))
			}
		}

		// wait for the ice candidates
		select {
		case <-iceReady.C:
		case <-conn.closeCond.C:
			return nil, conn.closeWithError(context.Canceled)
		}

		err = sendSignal(keypair, peerPublicKey, &SignalMessage{
			SDP:           answer,
			ICECandidates: iceCandidates,
		}, options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error marshaling signal message: %w", err))
		}
	}

	select {
	case <-time.After(time.Minute):
		return nil, conn.closeWithError(fmt.Errorf("failed to connect in time: %w", err))
	case <-connected.C:
	}

	return conn, nil
}

type SignalMessage struct {
	SDP           string
	ICECandidates []string
}

func recvSignal(keypair crypt.KeyPair, peerPublicKey crypt.Key, options ...signal.Option) (*SignalMessage, error) {
	bs, err := signal.Recv(keypair, peerPublicKey, options...)
	if err != nil {
		return nil, err
	}

	var msg SignalMessage
	err = json.Unmarshal(bs, &msg)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

func sendSignal(keypair crypt.KeyPair, peerPublicKey crypt.Key, msg *SignalMessage, options ...signal.Option) error {
	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	err = signal.Send(keypair, peerPublicKey, bs, options...)
	if err != nil {
		return err
	}
	return nil
}
