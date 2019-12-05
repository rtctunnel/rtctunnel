package peer

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/apex/log"
	"github.com/pkg/errors"
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/rtctunnel/rtctunnel/signal"
	"github.com/xtaci/smux/v2"
)

// Conn wraps an RTCPeerConnection so connections can be made and accepted.
type Conn struct {
	keypair       crypt.KeyPair
	peerPublicKey crypt.Key

	pc   RTCPeerConnection
	sess *smux.Session

	closeCond *Cond
	closeErr  error

	dataChannelOpenCond *Cond
	dataChannel         RTCDataChannel
}

// Accept accepts a new connection over the datachannel.
func (conn *Conn) Accept() (stream net.Conn, port int, err error) {
	stream, err = conn.sess.AcceptStream()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to accept smux stream: %w", err)
	}

	var portData [8]byte
	_, err = io.ReadFull(stream, portData[:])
	if err != nil {
		stream.Close()
		return nil, 0, fmt.Errorf("failed to read port from smux stream: %w", err)
	}

	port = int(binary.BigEndian.Uint64(portData[:]))

	log.WithField("peer", conn.peerPublicKey).
		WithField("port", port).
		Info("accepted connection")

	return stream, port, err
}

// Open opens a new connection over the datachannel.
func (conn *Conn) Open(port int) (stream net.Conn, err error) {
	stream, err = conn.sess.OpenStream()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open smux stream")
	}

	var portData [8]byte
	binary.BigEndian.PutUint64(portData[:], uint64(port))

	_, err = stream.Write(portData[:])
	if err != nil {
		stream.Close()
		return nil, errors.Wrapf(err, "failed to write port to smux stream")
	}

	log.WithField("peer", conn.peerPublicKey).
		WithField("port", port).
		Info("opened connection")

	return stream, nil
}

// Close closes the peer connection
func (conn *Conn) Close() error {
	return conn.closeWithError(conn.closeErr)
}

func (conn *Conn) closeWithError(err error) error {
	conn.closeCond.Do(func() {
		if conn.sess != nil {
			e := conn.sess.Close()
			conn.sess = nil
			if err == nil {
				err = e
			}
		}
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
	}

	log.WithField("peer", peerPublicKey).
		Info("creating webrtc peer connection")

	connected := NewCond()

	var err error
	conn.pc, err = NewRTCPeerConnection()
	if err != nil {
		conn.Close()
		return nil, errors.Wrapf(err, "failed to create webrtc peer connection")
	}
	conn.pc.OnICEConnectionStateChange(func(state string) {
		switch state {
		case "connected":
			connected.Signal()
		case "closed":
			_ = conn.closeWithError(context.Canceled)
		}
	})

	if keypair.Public.String() < peerPublicKey.String() {
		conn.dataChannel, err = conn.pc.CreateDataChannel("mux")
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating webrtc datachannel: %w", err))
		}

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

		dcconn, err := WrapDataChannel(conn.dataChannel)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error wrapping webrtc datachannel: %w", err))
		}

		conn.sess, err = smux.Server(dcconn, nil)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating smux server: %w", err))
		}
	} else {
		pending := make(chan RTCDataChannel, 1)
		conn.pc.OnDataChannel(func(dc RTCDataChannel) {
			select {
			case pending <- dc:
			default:
			}
		})

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

		select {
		case <-time.After(time.Minute):
			return nil, conn.closeWithError(fmt.Errorf("failed to receive webrtc datachannel in time: %w", err))
		case conn.dataChannel = <-pending:
		}

		dcconn, err := WrapDataChannel(conn.dataChannel)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error wrapping webrtc datachannel: %w", err))
		}

		conn.sess, err = smux.Client(dcconn, nil)
		if err != nil {
			return nil, conn.closeWithError(fmt.Errorf("error creating smux client: %w", err))
		}
	}

	select {
	case <-time.After(time.Minute):
		return nil, conn.closeWithError(fmt.Errorf("failed to connect in time: %w", err))
	case <-connected.C:
	}

	return conn, nil
}
