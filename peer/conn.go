package peer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/rtctunnel/rtctunnel/signal"
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
			log.WithField("label", lbl).Info("ignoring datachannel")
			continue
		}
		name := lbl[:idx]
		port, err := strconv.Atoi(lbl[idx+1:])
		if err != nil || name != "rtctunnel" {
			log.WithField("label", lbl).Info("ignoring datachannel")
			continue
		}

		stream, err := WrapDataChannel(dc)
		if errors.Is(err, ErrClosedByPeer) {
			log.WithField("label", lbl).Info("datachannel was closed by peer")
			continue
		} else if err != nil {
			dc.Close()
			return nil, 0, err
		}

		log.WithField("peer", conn.peerPublicKey).
			WithField("port", port).
			Info("accepted connection")

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

	log.WithField("peer", conn.peerPublicKey).
		WithField("port", port).
		Info("opened connection")

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

	log.WithField("peer", peerPublicKey).
		Info("creating webrtc peer connection")

	connected := NewCond()

	var err error
	conn.pc, err = NewRTCPeerConnection()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create webrtc peer connection: %w", err)
	}
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
		dc, err := conn.pc.CreateDataChannel("rtctunnel:init")
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating init datachannel: %w", err))
		}
		defer dc.Close()

		// we create the offer
		offer, err := conn.pc.CreateOffer()
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating webrtc offer: %w", err))
		}

		err = signal.Send(keypair, peerPublicKey, []byte(offer), options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error sending webrtc offer: %w", err))
		}

		answerSDPBytes, err := signal.Recv(keypair, peerPublicKey, options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error receiving webrtc answer: %w", err))
		}
		answerSDP := string(answerSDPBytes)

		err = conn.pc.SetAnswer(answerSDP)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error setting webrtc answer: %w", err))
		}

	} else {
		offerSDPBytes, err := signal.Recv(keypair, peerPublicKey, options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error receiving webrtc offer: %w", err))
		}
		offerSDP := string(offerSDPBytes)

		err = conn.pc.SetOffer(offerSDP)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error setting webrtc offer: %w", err))
		}

		answer, err := conn.pc.CreateAnswer()
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating webrtc answer: %w", err))
		}

		err = signal.Send(keypair, peerPublicKey, []byte(answer), options...)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error sending webrtc answer: %w", err))
		}
	}

	select {
	case <-time.After(time.Minute):
		return nil, conn.closeWithError(fmt.Errorf("failed to connect in time: %w", err))
	case <-connected.C:
	}

	return conn, nil
}
