//go:build js
// +build js

package peer

import (
	"encoding/json"
	"errors"
	"fmt"

	"syscall/js"
)

type M = map[string]interface{}
type S = []interface{}

type jsRTCDataChannel struct {
	object js.Value
}

func (dc jsRTCDataChannel) Close() error {
	dc.object.Call("close")
	return nil
}

func (dc jsRTCDataChannel) Label() string {
	return dc.object.Get("label").String()
}

func (dc jsRTCDataChannel) OnClose(handler func()) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go handler()
		return js.Undefined()
	})
	dc.object.Set("onclose", f)
}

func (dc jsRTCDataChannel) OnMessage(handler func([]byte)) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		consolelog("RTCPeerConnection::onmessage", args[0].Get("data"))
		evt := args[0]
		src := js.Global().Get("Uint8Array").New(evt.Get("data"))
		dst := make([]byte, src.Length())
		js.CopyBytesToGo(dst, src)
		go handler(dst)
		return js.Undefined()
	})
	dc.object.Set("onmessage", f)
}

func (dc jsRTCDataChannel) OnOpen(handler func()) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go handler()
		return js.Undefined()
	})
	dc.object.Set("onopen", f)
}

func (dc jsRTCDataChannel) Send(data []byte) error {
	return jsExceptionToGoError(func() {
		dst := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(dst, data)
		dc.object.Call("send", dst)
	})
}

type jsRTCPeerConnection struct {
	object           js.Value
	negotiationready *Cond
	closed           *Cond
}

func (pc *jsRTCPeerConnection) AddICECandidate(candidate string) error {
	return jsExceptionToGoError(func() {
		var obj M
		json.Unmarshal([]byte(candidate), &obj)
		pc.object.Call("addIceCandidate", obj)
	})
}

func (pc *jsRTCPeerConnection) Close() error {
	pc.closed.Signal()
	return jsExceptionToGoError(func() {
		pc.object.Call("close")
	})
}

func (pc *jsRTCPeerConnection) CreateDataChannel(label string) (RTCDataChannel, error) {
	dc := pc.object.Call("createDataChannel", label)
	consolelog("RTCPeerConnection::CreateDataChannel::result", dc)
	return jsRTCDataChannel{dc}, nil
}

func (pc *jsRTCPeerConnection) OnICECandidate(handler func(string)) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		candidate := args[0].Get("candidate")
		consolelog("RTCPeerConnection::onicecandidate", candidate)
		if candidate.Truthy() {
			go handler(js.Global().Get("JSON").Call("stringify", candidate).String())
		} else {
			go handler("")
		}
		return js.Undefined()
	})
	pc.object.Set("onicecandidate", f)
}

func (pc *jsRTCPeerConnection) OnICEConnectionStateChange(handler func(state string)) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go handler(pc.object.Get("iceConnectionState").String())
		return js.Undefined()
	})
	pc.object.Set("oniceconnectionstatechange", f)
}

func (pc *jsRTCPeerConnection) OnDataChannel(handler func(RTCDataChannel)) {
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		obj := args[0].Get("channel")
		dc := jsRTCDataChannel{obj}
		go handler(dc)
		return js.Undefined()
	})
	pc.object.Set("ondatachannel", f)
}

func (pc *jsRTCPeerConnection) CreateAnswer() (string, error) {
	consolelog("RTCPeerConnection::CreateAnswer")
	promise := pc.object.Call("createAnswer")
	return pc.handleLocalSDPPromise(promise)
}

func (pc *jsRTCPeerConnection) CreateOffer() (string, error) {
	pc.negotiationready.Wait()
	consolelog("RTCPeerConnection::CreateOffer")
	promise := pc.object.Call("createOffer")
	return pc.handleLocalSDPPromise(promise)
}

func (pc *jsRTCPeerConnection) SetAnswer(answer string) error {
	consolelog("RTCPeerConnection::SetAnswer", answer)
	promise := pc.object.Call("setRemoteDescription", M{
		"type": "answer",
		"sdp":  answer,
	})
	return pc.handleRemoteSDPPromise(promise)
}

func (pc *jsRTCPeerConnection) SetOffer(offer string) error {
	consolelog("RTCPeerConnection::SetOffer", offer)
	promise := pc.object.Call("setRemoteDescription", M{
		"type": "offer",
		"sdp":  offer,
	})
	return pc.handleRemoteSDPPromise(promise)
}

func (pc *jsRTCPeerConnection) handleRemoteSDPPromise(promise js.Value) error {
	errc := make(chan error, 1)
	promise.
		Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			go func() {
				errc <- nil
			}()
			return nil
		})).
		Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			err := args[0]
			go func() {
				errc <- errors.New(err.String())
			}()
			return nil
		}))
	return <-errc
}

func (pc *jsRTCPeerConnection) handleLocalSDPPromise(promise js.Value) (string, error) {
	sdpc := make(chan string, 1)
	errc := make(chan error, 2)
	promise.
		Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			desc := args[0]
			go func() {
				sdpc <- desc.Get("sdp").String()
			}()
			return pc.object.Call("setLocalDescription", desc)
		})).
		Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			err := args[0]
			go func() {
				errc <- errors.New(err.String())
			}()
			return js.Undefined()
		}))

	select {
	case sdp := <-sdpc:
		return sdp, nil
	case err := <-errc:
		return "", err
	case <-pc.closed.C:
		return "", errors.New("closed")
	}
}

func NewRTCPeerConnection() (RTCPeerConnection, error) {
	obj := js.Global().Get("RTCPeerConnection").New(M{
		"iceServers": S{
			M{
				"urls": "stun:stun.l.google.com:19302",
			},
		},
	})
	pc := &jsRTCPeerConnection{
		object:           obj,
		closed:           NewCond(),
		negotiationready: NewCond(),
	}
	obj.Set("onsignalingstatechange", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		consolelog("RTCPeerConnection::onsignalingstatechange", args[0].Get("target").Get("signalingState"))
		return nil
	}))
	obj.Set("oniceconnectionstatechange", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		consolelog("RTCPeerConnection::oniceconnectionstatechange", args[0].Get("target").Get("iceConnectionState"))
		return nil
	}))
	obj.Set("onnegotiationneeded", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		consolelog("RTCPeerConnection::onnegotiationneeded", args[0].Get("target"))
		pc.negotiationready.Signal()
		return nil
	}))
	obj.Set("onconnectionstatechange", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		consolelog("RTCPeerConnection::onconnectionstatechange", args[0].Get("target").Get("connectionState"))
		cs := args[0].Get("target").Get("connectionState").String()
		switch cs {
		case "failed", "disconnected":
			pc.Close()
		}
		return nil
	}))
	return pc, nil
}

func consolelog(args ...interface{}) {
	js.Global().Get("console").Call("log", args...)
}

func jsExceptionToGoError(f func()) error {
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", err)
			}
		}()
		f()
	}()
	return err
}
