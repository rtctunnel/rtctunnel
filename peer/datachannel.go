package peer

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/apex/log"
)

var ErrClosedByPeer = errors.New("closed by peer")

type dataChannelAddr struct{}

func (addr dataChannelAddr) Network() string {
	return "webrtc"
}

func (addr dataChannelAddr) String() string {
	return "webrtc://datachannel"
}

// A DataChannel implements the net.Conn interface over a webrtc data channel
type DataChannel struct {
	dc RTCDataChannel
	rr io.ReadCloser
	rw io.WriteCloser

	openCond  *Cond
	closeCond *Cond
	closeErr  error
}

// WrapDataChannel wraps an rtc data channel and implements the net.Conn
// interface
func WrapDataChannel(rtcDataChannel RTCDataChannel) (*DataChannel, error) {
	rr, rw := Pipe()

	dc := &DataChannel{
		dc: rtcDataChannel,
		rr: rr,
		rw: rw,

		openCond:  NewCond(),
		closeCond: NewCond(),
	}
	dc.dc.OnClose(func() {
		_ = dc.closeWithError(ErrClosedByPeer)
	})
	dc.dc.OnOpen(func() {
		// for reasons I don't understand, when opened the data channel is not immediately available for use
		time.Sleep(50 * time.Millisecond)
		dc.openCond.Signal()
	})
	dc.dc.OnMessage(func(data []byte) {
		log.WithField("data", data).
			Debug("datachannel message")

		if rw != nil {
			_, err := rw.Write(data)
			if err != nil {
				_ = dc.closeWithError(err)
				rw = nil
			}
		}
	})

	select {
	case <-dc.closeCond.C:
		err := dc.closeErr
		if err == nil {
			err = errors.New("datachannel closed for unknown reasons")
		}
		return nil, err
	case <-dc.openCond.C:
	}

	return dc, nil
}

func (dc *DataChannel) Read(b []byte) (n int, err error) {
	return dc.rr.Read(b)
}

func (dc *DataChannel) Write(b []byte) (n int, err error) {
	err = dc.dc.Send(b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (dc *DataChannel) Close() error {
	return dc.closeWithError(nil)
}

func (dc *DataChannel) LocalAddr() net.Addr {
	return dataChannelAddr{}
}

func (dc *DataChannel) RemoteAddr() net.Addr {
	return dataChannelAddr{}
}

func (dc *DataChannel) SetDeadline(t time.Time) error {
	panic("SetDeadline not implemented")
}

func (dc *DataChannel) SetReadDeadline(t time.Time) error {
	panic("SetReadDeadline not implemented")
}

func (dc *DataChannel) SetWriteDeadline(t time.Time) error {
	panic("SetWriteDeadline not implemented")
}

func (dc *DataChannel) closeWithError(err error) error {
	dc.closeCond.Do(func() {
		e := dc.rr.Close()
		if err == nil {
			err = e
		}
		e = dc.rw.Close()
		if err == nil {
			err = e
		}
		e = dc.dc.Close()
		if err == nil {
			err = e
		}
		dc.closeErr = err
	})
	return err
}
