//+build !js

package peer

import (
	"io"
	"strings"

	webrtc "github.com/pion/webrtc/v2"
	"github.com/pkg/errors"
)

type nativeRTCDataChannel struct {
	native *webrtc.DataChannel
}

func (dc nativeRTCDataChannel) Close() error {
	return dc.native.Close()
}

func (dc nativeRTCDataChannel) OnClose(handler func()) {
	dc.native.OnClose(handler)
}

func (dc nativeRTCDataChannel) OnMessage(handler func([]byte)) {
	dc.native.OnMessage(func(msg webrtc.DataChannelMessage) {
		handler(msg.Data)
	})
}

func (dc nativeRTCDataChannel) OnOpen(handler func()) {
	dc.native.OnOpen(func() {
		handler()
	})
}

func (dc nativeRTCDataChannel) Send(data []byte) error {
	return dc.native.Send(data)
}

type nativeRTCPeerConnection struct {
	*webrtc.PeerConnection
}

func (pc nativeRTCPeerConnection) CreateDataChannel(label string) (RTCDataChannel, error) {
	dc, err := pc.PeerConnection.CreateDataChannel(label, nil)
	if err != nil {
		return nil, err
	}
	return nativeRTCDataChannel{dc}, nil
}

func (pc nativeRTCPeerConnection) OnICEConnectionStateChange(handler func(state string)) {
	pc.PeerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		handler(strings.ToLower(state.String()))
	})
}

func (pc nativeRTCPeerConnection) OnDataChannel(handler func(RTCDataChannel)) {
	pc.PeerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		handler(nativeRTCDataChannel{dc})
	})
}

func (pc nativeRTCPeerConnection) CreateAnswer() (string, error) {
	sdp, err := pc.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	err = pc.PeerConnection.SetLocalDescription(sdp)
	if err != nil {
		return "", err
	}
	return sdp.SDP, nil
}

func (pc nativeRTCPeerConnection) CreateOffer() (string, error) {
	sdp, err := pc.PeerConnection.CreateOffer(nil)
	if err != nil {
		return "", err
	}
	err = pc.PeerConnection.SetLocalDescription(sdp)
	if err != nil {
		return "", err
	}
	return sdp.SDP, nil
}

func (pc nativeRTCPeerConnection) SetAnswer(answer string) error {
	return pc.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer,
	})
}

func (pc nativeRTCPeerConnection) SetOffer(offer string) error {
	return pc.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offer,
	})
}

func NewRTCPeerConnection() (RTCPeerConnection, error) {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{
			URLs: []string{
				"stun:stun.l.google.com:19302",
				"stun:stun1.l.google.com:19302",
				"stun:stun2.l.google.com:19302",
				"stun:stun3.l.google.com:19302",
				"stun:stun4.l.google.com:19302",
			},
		}},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating peer connection")
	}
	return nativeRTCPeerConnection{pc}, nil
}

func Pipe() (io.ReadCloser, io.WriteCloser) {
	return io.Pipe()
}
