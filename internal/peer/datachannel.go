package peer

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/rs/zerolog/log"
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
	rr ContextReadCloser
	rw ContextWriteCloser

	openCond  *Cond
	closeCond *Cond
	closeErr  error
}

// WrapDataChannel wraps an rtc data channel and implements the net.Conn
// interface
func WrapDataChannel(rtcDataChannel RTCDataChannel) (*DataChannel, error) {
	rr, rw := io.Pipe()

	dc := &DataChannel{
		dc: rtcDataChannel,
		rr: ContextReadCloser{Context: context.Background(), ReadCloser: rr},
		rw: ContextWriteCloser{Context: context.Background(), WriteCloser: rw},

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
		log.Debug().Bytes("data", data).
			Msg("datachannel message")

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
	var err error
	if e := dc.SetReadDeadline(t); e != nil {
		err = e
	}
	if e := dc.SetWriteDeadline(t); e != nil {
		err = e
	}
	return err
}

func (dc *DataChannel) SetReadDeadline(t time.Time) error {
	return dc.rr.SetReadDeadline(t)
}

func (dc *DataChannel) SetWriteDeadline(t time.Time) error {
	return dc.rw.SetWriteDeadline(t)
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

type ContextReadCloser struct {
	context.Context
	io.ReadCloser
	cancel func()
}

func (cr ContextReadCloser) Close() error {
	err := cr.ReadCloser.Close()
	if cr.cancel != nil {
		cr.cancel()
		cr.cancel = nil
	}
	return err
}

func (cr ContextReadCloser) SetReadDeadline(t time.Time) error {
	if cr.cancel != nil {
		cr.cancel()
		cr.cancel = nil
	}
	cr.Context, cr.cancel = context.WithDeadline(context.Background(), t)
	return nil
}

func (cr ContextReadCloser) Read(p []byte) (n int, err error) {
	done := make(chan struct{})
	go func() {
		n, err = cr.ReadCloser.Read(p)
		close(done)
	}()
	select {
	case <-done:
		return n, err
	case <-cr.Context.Done():
		return 0, cr.Context.Err()
	}
}

type ContextWriteCloser struct {
	context.Context
	io.WriteCloser
	cancel func()
}

func (cw ContextWriteCloser) Close() error {
	err := cw.WriteCloser.Close()
	if cw.cancel != nil {
		cw.cancel()
		cw.cancel = nil
	}
	return err
}

func (cw ContextWriteCloser) SetWriteDeadline(t time.Time) error {
	if cw.cancel != nil {
		cw.cancel()
		cw.cancel = nil
	}
	cw.Context, cw.cancel = context.WithDeadline(context.Background(), t)
	return nil
}

func (cw ContextWriteCloser) Write(p []byte) (n int, err error) {
	done := make(chan struct{})
	go func() {
		n, err = cw.WriteCloser.Write(p)
		close(done)
	}()
	select {
	case <-done:
		return n, err
	case <-cw.Context.Done():
		return 0, cw.Context.Err()
	}
}
