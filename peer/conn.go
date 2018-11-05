package peer

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/rtctunnel/rtctunnel/signal"
	"github.com/apex/log"
	"github.com/hashicorp/yamux"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pkg/errors"
)

type Conn struct {
	keypair       crypt.KeyPair
	peerPublicKey crypt.Key

	pc   *webrtc.RTCPeerConnection
	dc   *webrtc.RTCDataChannel
	sess *yamux.Session
}

// Accept accepts a new connection over the datachannel.
func (conn *Conn) Accept() (stream net.Conn, port int, err error) {
	stream, err = conn.sess.Accept()
	if err != nil {
		return nil, 0, errors.Wrapf(err, "failed to accept yamux stream")
	}

	var portData [8]byte
	_, err = io.ReadFull(stream, portData[:])
	if err != nil {
		stream.Close()
		return nil, 0, errors.Wrapf(err, "failed to read port from yamux stream")
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
		return nil, errors.Wrapf(err, "failed to open yamux stream")
	}

	var portData [8]byte
	binary.BigEndian.PutUint64(portData[:], uint64(port))

	_, err = stream.Write(portData[:])
	if err != nil {
		stream.Close()
		return nil, errors.Wrapf(err, "failed to write port to yamux stream")
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

func (conn *Conn) onDataChannelMessage(payload datachannel.Payload) {
	log.WithField("peer", conn.peerPublicKey).
		WithField("datachannel", conn.dc.Label).
		WithField("payload", payload).
		Info("datachannel message")
}

func (conn *Conn) onDataChannelOpen() {
	log.WithField("peer", conn.peerPublicKey).
		WithField("datachannel", conn.dc.Label).
		Info("datachannel open")
}

func Open(keypair crypt.KeyPair, peerPublicKey crypt.Key) (*Conn, error) {
	conn := &Conn{
		keypair:       keypair,
		peerPublicKey: peerPublicKey,
	}

	log.WithField("peer", peerPublicKey).
		Info("creating webrtc peer connection")

	var err error
	conn.pc, err = webrtc.New(webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{{
			URLs: []string{"stun:stun.l.google.com:19302"},
		}},
	})
	if err != nil {
		conn.Close()
		return nil, errors.Wrapf(err, "failed to create webrtc peer connection")
	}
	connectionReady := make(chan struct{})
	conn.pc.OnICEConnectionStateChange = func(state ice.ConnectionState) {
		log.WithField("peer", conn.peerPublicKey).
			WithField("state", state).
			Info("ice state change")
		if state == ice.ConnectionStateConnected {
			close(connectionReady)
		}
	}

	if keypair.Public.String() < peerPublicKey.String() {
		conn.dc, err = conn.pc.CreateDataChannel("yamux", nil)
		if err != nil {
			conn.Close()
			return nil, errors.Wrapf(err, "failed to create datachannel")
		}
		conn.dc.Lock()
		conn.dc.Onmessage = conn.onDataChannelMessage
		conn.dc.OnOpen = conn.onDataChannelOpen
		conn.dc.Unlock()

		// we create the offer
		offer, err := conn.pc.CreateOffer(nil)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create webrtc offer")
		}

		log.WithField("sdp", offer.Sdp).Info("sending offer")

		err = signal.Send(keypair, peerPublicKey, []byte(offer.Sdp))
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

		err = conn.pc.SetRemoteDescription(webrtc.RTCSessionDescription{
			Type: webrtc.RTCSdpTypeAnswer,
			Sdp:  answerSDP,
		})
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to set webrtc answer")
		}

		dcconn, err := WrapDataChannel(conn.pc, conn.dc)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create datachannel connection wrapper")
		}

		conn.sess, err = yamux.Server(dcconn, yamux.DefaultConfig())
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create yamux server")
		}
	} else {
		pending := make(chan *webrtc.RTCDataChannel, 1)
		conn.pc.OnDataChannel = func(dc *webrtc.RTCDataChannel) {
			if dc.Label == "yamux" {
				pending <- dc
			}
		}

		offerSDPBytes, err := signal.Recv(keypair, peerPublicKey)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to receive webrtc offer")
		}
		offerSDP := string(offerSDPBytes)

		log.WithField("sdp", offerSDP).Info("received offer")

		err = conn.pc.SetRemoteDescription(webrtc.RTCSessionDescription{
			Type: webrtc.RTCSdpTypeOffer,
			Sdp:  offerSDP,
		})
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to set webrtc offer")
		}

		answer, err := conn.pc.CreateAnswer(nil)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create webrtc answer")
		}

		log.WithField("sdp", answer.Sdp).Info("sending answer")

		err = signal.Send(keypair, peerPublicKey, []byte(answer.Sdp))
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

		dcconn, err := WrapDataChannel(conn.pc, conn.dc)
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create datachannel connection wrapper")
		}

		conn.sess, err = yamux.Client(dcconn, yamux.DefaultConfig())
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to create yamux client")
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
