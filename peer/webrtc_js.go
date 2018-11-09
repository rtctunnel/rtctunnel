//+build js

package peer

import (
	"errors"
	"io"
	"sync"

	"github.com/gopherjs/gopherjs/js"
)

type jsRTCDataChannel struct {
	object *js.Object
}

func (dc jsRTCDataChannel) OnMessage(handler func([]byte)) {
	dc.object.Set("onmessage", func(evt *js.Object) {
		bs := js.Global.Get("Uint8Array").New(evt.Get("data")).Interface().([]byte)
		handler(bs)
	})
}

func (dc jsRTCDataChannel) OnOpen(handler func()) {
	dc.object.Set("onopen", func(evt *js.Object) {
		handler()
	})
}

func (dc jsRTCDataChannel) Send(data []byte) error {
	dc.object.Call("send", data)
	return nil
}

type jsRTCPeerConnection struct {
	object   *js.Object
	iceready chan struct{}
}

func (pc *jsRTCPeerConnection) Close() error {
	pc.object.Call("close")
	return nil
}

func (pc *jsRTCPeerConnection) CreateDataChannel(label string) (RTCDataChannel, error) {
	dc := pc.object.Call("createDataChannel", label)
	return jsRTCDataChannel{dc}, nil
}

func (pc *jsRTCPeerConnection) OnICEConnectionStateChange(handler func(state string)) {
	pc.object.Set("oniceconnectionstatechange", func(evt *js.Object) {
		handler(pc.object.Get("iceConnectionState").String())
	})
}

func (pc *jsRTCPeerConnection) OnDataChannel(handler func(RTCDataChannel)) {
	pc.object.Set("ondatachannel", func(evt *js.Object) {
		obj := evt.Get("channel")
		dc := jsRTCDataChannel{obj}
		handler(dc)
	})
}

func (pc *jsRTCPeerConnection) CreateAnswer() (string, error) {
	consolelog("RTCPeerConnection::CreateAnswer")
	promise := pc.object.Call("createAnswer")
	return pc.handleLocalSDPPromise(promise)
}

func (pc *jsRTCPeerConnection) CreateOffer() (string, error) {
	consolelog("RTCPeerConnection::CreateOffer")
	promise := pc.object.Call("createOffer")
	return pc.handleLocalSDPPromise(promise)
}

func (pc *jsRTCPeerConnection) SetAnswer(answer string) error {
	consolelog("RTCPeerConnection::SetAnswer", answer)
	pc.object.Call("setRemoteDescription", js.M{
		"type": "answer",
		"sdp":  answer,
	})
	return nil
}

func (pc *jsRTCPeerConnection) SetOffer(offer string) error {
	consolelog("RTCPeerConnection::SetOffer", offer)
	pc.object.Call("setRemoteDescription", js.M{
		"type": "offer",
		"sdp":  offer,
	})
	return nil
}

func (pc *jsRTCPeerConnection) handleLocalSDPPromise(promise *js.Object) (string, error) {
	sdpc := make(chan string, 1)
	errc := make(chan error, 1)
	promise.Call("then", func(desc *js.Object) *js.Object {
		go func() {
			<-pc.iceready
			sdpc <- pc.object.Get("localDescription").Get("sdp").String()
		}()
		return pc.object.Call("setLocalDescription", desc)
	}).Call("catch", func(err *js.Object) {
		go func() {
			errc <- errors.New(err.String())
		}()
	})

	select {
	case sdp := <-sdpc:
		return sdp, nil
	case err := <-errc:
		return "", err
	}
}

func NewRTCPeerConnection() (RTCPeerConnection, error) {
	obj := js.Global.Get("RTCPeerConnection").New(js.M{
		"iceServers": js.S{
			js.M{
				"urls": "stun:stun.l.google.com:19302",
			},
		},
	})
	pc := &jsRTCPeerConnection{
		object:   obj,
		iceready: make(chan struct{}),
	}
	obj.Set("onicecandidate", func(evt *js.Object) {
		if !evt.Get("candidate").Bool() {
			consolelog("RTCPeerConnection::iceready")
			close(pc.iceready)
		}
	})
	return pc, nil
}

type jsPipe struct {
	ch chan []byte

	mu  sync.Mutex
	buf []byte

	once   sync.Once
	closer chan struct{}
}

func (p *jsPipe) Close() error {
	p.once.Do(func() {
		close(p.closer)
	})
	return nil
}

func (p *jsPipe) Read(bs []byte) (int, error) {
	for {
		n := p.readbuf(bs)
		if n > 0 {
			return n, nil
		}

		select {
		case <-p.closer:
			return 0, io.EOF
		case buf := <-p.ch:
			p.buf = buf
		}
	}
}

func (p *jsPipe) readbuf(bs []byte) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.buf)
	if n > 0 {
		copy(bs, p.buf)
		if n > len(bs) {
			p.buf = p.buf[len(bs):]
			return len(bs)
		} else {
			p.buf = nil
			return n
		}
	}

	return 0
}

func (p *jsPipe) Write(bs []byte) (int, error) {
	select {
	case <-p.closer:
		return 0, io.EOF
	case p.ch <- bs:
		return len(bs), nil
	}
}

func Pipe() (io.ReadCloser, io.WriteCloser, error) {
	p := &jsPipe{
		ch:     make(chan []byte),
		closer: make(chan struct{}),
	}
	return p, p, nil
}

func consolelog(args ...interface{}) {
	js.Global.Get("console").Call("log", args...)
}
