//+build !js

package peer

import (
	"io"
	"os"
	"strings"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pkg/errors"
)

type nativeRTCDataChannel struct {
	native *webrtc.RTCDataChannel
}

func (dc nativeRTCDataChannel) OnMessage(handler func([]byte)) {
	dc.native.Lock()
	dc.native.OnMessage(func(payload datachannel.Payload) {
		var data []byte
		switch payload := payload.(type) {
		case *datachannel.PayloadBinary:
			data = payload.Data
		case *datachannel.PayloadString:
			data = payload.Data
		default:
			panic("unknown payload type")
		}
		handler(data)
	})
	dc.native.Unlock()
}

func (dc nativeRTCDataChannel) OnOpen(handler func()) {
	dc.native.Lock()
	dc.native.OnOpen(func() {
		handler()
	})
	dc.native.Unlock()
}

func (dc nativeRTCDataChannel) Send(data []byte) error {
	return dc.native.Send(datachannel.PayloadBinary{Data: data})
}

type nativeRTCPeerConnection struct {
	*webrtc.RTCPeerConnection
}

func (pc nativeRTCPeerConnection) CreateDataChannel(label string) (RTCDataChannel, error) {
	dc, err := pc.RTCPeerConnection.CreateDataChannel(label, nil)
	if err != nil {
		return nil, err
	}
	return nativeRTCDataChannel{dc}, nil
}

func (pc nativeRTCPeerConnection) OnICEConnectionStateChange(handler func(state string)) {
	pc.Lock()
	pc.RTCPeerConnection.OnICEConnectionStateChange(func(state ice.ConnectionState) {
		handler(strings.ToLower(state.String()))
	})
	pc.Unlock()
}

func (pc nativeRTCPeerConnection) OnDataChannel(handler func(RTCDataChannel)) {
	pc.Lock()
	pc.RTCPeerConnection.OnDataChannel(func(dc *webrtc.RTCDataChannel) {
		handler(nativeRTCDataChannel{dc})
	})
	pc.Unlock()
}

func (pc nativeRTCPeerConnection) CreateAnswer() (string, error) {
	sdp, err := pc.RTCPeerConnection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	return sdp.Sdp, nil
}

func (pc nativeRTCPeerConnection) CreateOffer() (string, error) {
	sdp, err := pc.RTCPeerConnection.CreateOffer(nil)
	if err != nil {
		return "", err
	}
	return sdp.Sdp, nil
}

func (pc nativeRTCPeerConnection) SetAnswer(answer string) error {
	return pc.RTCPeerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeAnswer,
		Sdp:  answer,
	})
}

func (pc nativeRTCPeerConnection) SetOffer(offer string) error {
	return pc.RTCPeerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeOffer,
		Sdp:  offer,
	})
}

func NewRTCPeerConnection() (RTCPeerConnection, error) {
	pc, err := webrtc.New(webrtc.RTCConfiguration{
		// IceServers: []webrtc.RTCIceServer{{
		// 	URLs: []string{
		// 		"stun:stun.l.google.com:19302",
		// 		"stun:stun1.l.google.com:19302",
		// 		"stun:stun2.l.google.com:19302",
		// 		"stun:stun3.l.google.com:19302",
		// 		"stun:stun4.l.google.com:19302",
		// 		"stun:stun.ekiga.net",
		// 		"stun:stun.ideasip.com",
		// 		"stun:stun.schlund.de",
		// 		"stun:stun.stunprotocol.org:3478",
		// 		"stun:stun.voiparound.com",
		// 		"stun:stun.voipbuster.com",
		// 		"stun:stun.voipstunt.com",
		// 		"stun:stun.voxgratia.org",
		// 	},
		// }},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating peer connection")
	}
	return nativeRTCPeerConnection{pc}, nil
}

func Pipe() (io.ReadCloser, io.WriteCloser, error) {
	return os.Pipe()
}
