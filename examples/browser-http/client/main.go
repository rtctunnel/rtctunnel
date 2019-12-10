//+build js

package main

import (
	"bufio"
	"io/ioutil"
	"net/http"
	"strings"
	"syscall/js"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/rtctunnel/rtctunnel/ext/js/localstorage"
	"github.com/rtctunnel/rtctunnel/peer"
)

const (
	localStorageKeypairKey = "rtctunnel/examples/browser-http/client/keypair"
	localStoragePeerKey    = "rtctunnel/examples/browser-http/client/peerpublickey"
)

var keypair crypt.KeyPair

func main() {
	saved := localstorage.Get(localStorageKeypairKey)
	if saved == "" {
		keypair = crypt.GenerateKeyPair()
		localstorage.Set(localStorageKeypairKey, keypair.Private.String()+"|"+keypair.Public.String())
	} else {
		private, err := crypt.NewKey(strings.Split(saved, "|")[0])
		if err != nil {
			panic(err)
		}
		public, err := crypt.NewKey(strings.Split(saved, "|")[1])
		if err != nil {
			panic(err)
		}
		keypair = crypt.KeyPair{Private: private, Public: public}
	}

	onload()

	for range time.Tick(time.Second) {}
}

func onload() {
	doc := js.Global().Get("document")
	body := doc.Call("getElementsByTagName", "body").Index(0)
	body.Get("style").Set("fontFamily", "monospace")

	p := doc.Call("createElement", "p")
	p.Set("innerHTML", "your public key: "+keypair.Public.String())
	body.Call("appendChild", p)

	form := doc.Call("createElement", "form")
	label := doc.Call("createElement", "label")
	label.Call("appendChild", doc.Call("createTextNode", "enter peer key:"))
	input := doc.Call("createElement", "input")
	input.Set("id", "peerPublicKey")
	input.Set("type", "text")
	input.Set("value", localstorage.Get(localStoragePeerKey))
	label.Call("appendChild", doc.Call("createTextNode", " "))
	label.Call("appendChild", input)
	form.Call("appendChild", label)
	button := doc.Call("createElement", "input")
	button.Set("type", "submit")
	form.Call("appendChild", doc.Call("createTextNode", " "))
	form.Call("appendChild", button)
	body.Call("appendChild", form)

	form.Set("onsubmit", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		onsubmitpeerkey(args[0])
		return false
	}))
}

func onsubmitpeerkey(evt js.Value) {
	evt.Call("preventDefault")

	value := js.Global().Get("document").Call("getElementById", "peerPublicKey").Get("value").String()
	peerPublicKey, err := crypt.NewKey(value)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid peer key")
		return
	}
	localstorage.Set(localStoragePeerKey, peerPublicKey.String())

	doc := js.Global().Get("document")
	body := doc.Call("getElementsByTagName", "body").Index(0)
	p := doc.Call("createElement", "p")
	p.Set("innerHTML", "connecting")
	body.Call("appendChild", p)

	go openConnection(peerPublicKey)
}

type transport struct {
	conn *peer.Conn
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	c, err := t.conn.Open(80)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	log.Info().Str("url", req.URL.String()).Msg("sending request")

	err = req.Write(c)
	if err != nil {
		return nil, err
	}


	log.Info().Str("url", req.URL.String()).Msg("reading response")

	cbr := bufio.NewReader(c)
	res, err := http.ReadResponse(cbr, req)
	if err != nil {
		return nil, err
	}

	log.Info().Int("status_code", res.StatusCode).Msg("received response")

	return res, nil
}

func openConnection(peerPublicKey crypt.Key) {
	conn, err := peer.Open(keypair, peerPublicKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create peer connection")
		return
	}
	defer conn.Close()

	client := &http.Client{
		Transport: transport{conn},
	}
	resp, err := client.Get("http://peer/")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to make http request")
		return
	}
	defer resp.Body.Close()

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		js.Global().Call("alert", err.Error())
		return
	}

	doc := js.Global().Get("document")
	body := doc.Call("getElementsByTagName", "body").Index(0)
	body.Call("appendChild", doc.Call("createTextNode", string(bs)))
}
