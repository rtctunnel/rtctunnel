package peer

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/apex/log"
	"github.com/pkg/errors"
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/rtctunnel/rtctunnel/signal"
	"github.com/xtaci/smux"
)

// Conn wraps an RTCPeerConnection so connections can be made and accepted.
type Conn struct {
	keypair       crypt.KeyPair
	peerPublicKey crypt.Key

	pc   RTCPeerConnection
	dc   RTCDataChannel
	sess *smux.Session
}

// Accept accepts a new connection over the datachannel.
func (conn *Conn) Accept() (stream net.Conn, port int, err error) {
	stream, err = conn.sess.AcceptStream()
	if err != nil {
		return nil, 0, errors.New("failed to accept smux stream")
	}

	var portData [8]byte
	_, err = io.ReadFull(stream, portData[:])
	if err != nil {
		stream.Close()
		return nil, 0, errors.New("failed to read port from smux stream")
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
	var err error
	if conn.sess != nil {
		err = conn.sess.Close()
		conn.sess = nil
	}
	if conn.pc != nil {
		err = conn.pc.Close()
		conn.pc = nil
	}
	return err
}

// Open opens a new Connection.
func Open(keypair crypt.KeyPair, peerPublicKey crypt.Key) (*Conn, error) {
	conn := &Conn{
		keypair:       keypair,
		peerPublicKey: peerPublicKey,
	}

	log.WithField("peer", peerPublicKey).
		Info("creating webrtc peer connection")

	var err error
	conn.pc, err = NewRTCPeerConnection()
	if err != nil {
		conn.Close()
		return nil, errors.Wrapf(err, "failed to create webrtc peer connection")
	}
	connectionReady := make(chan struct{})
	conn.pc.OnICEConnectionStateChange(func(state string) {
		log.WithField("peer", conn.peerPublicKey).
			WithField("state", state).
			Info("ice state change")
		if state == "connected" {
			close(connectionReady)
		}
	})

	if keypair.Public.String() < peerPublicKey.String() {
		conn.dc, err = conn.pc.CreateDataChannel("mux")
		if err != nil {
			conn.Close()
			return nil, errors.Wrapf(err, "failed to create datachannel")
		}

		// we create the offer
		offer, err := conn.pc.CreateOffer()
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create webrtc offer")
		}

		log.WithField("sdp", offer).Info("sending offer")

		err = signal.Send(keypair, peerPublicKey, []byte(offer))
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to send webrtc offer")
		}

		answerSDPBytes, err := signal.Recv(keypair, peerPublicKey)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to receive webrtc answer")
		}
		answerSDP := string(answerSDPBytes)

		log.WithField("sdp", answerSDP).Info("received answer")

		err = conn.pc.SetAnswer(answerSDP)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to set webrtc answer")
		}

		dcconn, err := WrapDataChannel(conn.dc)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create datachannel connection wrapper")
		}

		conn.sess, err = smux.Server(dcconn, nil)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create smux server")
		}
	} else {
		pending := make(chan RTCDataChannel, 1)
		conn.pc.OnDataChannel(func(dc RTCDataChannel) {
			pending <- dc
		})

		offerSDPBytes, err := signal.Recv(keypair, peerPublicKey)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to receive webrtc offer")
		}
		offerSDP := string(offerSDPBytes)

		log.WithField("sdp", offerSDP).Info("received offer")

		err = conn.pc.SetOffer(offerSDP)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to set webrtc offer")
		}

		answer, err := conn.pc.CreateAnswer()
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create webrtc answer")
		}

		log.WithField("sdp", answer).Info("sending answer")

		err = signal.Send(keypair, peerPublicKey, []byte(answer))
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to send webrtc answer")
		}

		select {
		case <-time.After(time.Minute):
			conn.Close()
			return nil, errors.Wrap(err, "failed to receive a new datachannel in time")
		case conn.dc = <-pending:
		}

		dcconn, err := WrapDataChannel(conn.dc)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create datachannel connection wrapper")
		}

		conn.sess, err = smux.Client(dcconn, nil)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create smux client")
		}
	}

	select {
	case <-time.After(time.Minute):
		conn.Close()
		return nil, errors.Wrap(err, "failed to connect in time")
	case <-connectionReady:
	}

	return conn, nil
}
