package peer

import (
	"net"
	"os"
	"time"

	"github.com/apex/log"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
)

type dataChannelAddr struct {
	dc *webrtc.RTCDataChannel
}

func (addr dataChannelAddr) Network() string {
	return "webrtc"
}

func (addr dataChannelAddr) String() string {
	return addr.dc.Label
}

// A DataChannel implements the net.Conn interface over a webrtc data channel
type DataChannel struct {
	pc *webrtc.RTCPeerConnection
	dc *webrtc.RTCDataChannel
	rr *os.File
}

var _ net.Conn = (*DataChannel)(nil)

// WrapDataChannel wraps an rtc data channel and implements the net.Conn
// interface
func WrapDataChannel(rtcPeerConnection *webrtc.RTCPeerConnection, rtcDataChannel *webrtc.RTCDataChannel) (*DataChannel, error) {
	rr, rw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	dc := &DataChannel{
		pc: rtcPeerConnection,
		dc: rtcDataChannel,
		rr: rr,
	}
	dc.dc.Lock()
	dc.dc.Onmessage = func(payload datachannel.Payload) {
		var data []byte
		switch payload := payload.(type) {
		case *datachannel.PayloadBinary:
			data = payload.Data
		case *datachannel.PayloadString:
			data = payload.Data
		default:
			panic("unknown payload type")
		}

		log.WithField("datachannel", rtcDataChannel.Label).
			WithField("data", data).
			Debug("datachannel message")

		if rw != nil {
			_, err := rw.Write(data)
			if err != nil {
				rw.Close()
				rw = nil
			}
		}
	}
	dc.dc.Unlock()

	return dc, nil
}

func (dc *DataChannel) Label() string {
	return dc.dc.Label
}

func (dc *DataChannel) Read(b []byte) (n int, err error) {
	return dc.rr.Read(b)
}

func (dc *DataChannel) Write(b []byte) (n int, err error) {
	err = dc.dc.Send(datachannel.PayloadBinary{Data: b})
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (dc *DataChannel) Close() error {
	//TODO: how do we close the datachannel?
	dc.rr.Close()
	return nil
}

func (dc *DataChannel) LocalAddr() net.Addr {
	return dataChannelAddr{dc.dc}
}

func (dc *DataChannel) RemoteAddr() net.Addr {
	return dataChannelAddr{dc.dc}
}

func (dc *DataChannel) SetDeadline(t time.Time) error {
	panic("not implemented")
}

func (dc *DataChannel) SetReadDeadline(t time.Time) error {
	panic("not implemented")
}

func (dc *DataChannel) SetWriteDeadline(t time.Time) error {
	panic("not implemented")
}
