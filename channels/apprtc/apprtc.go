package apprtc

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/apex/log"
	"github.com/gorilla/websocket"
	"github.com/rtctunnel/rtctunnel/channels"
)

func init() {
	channels.RegisterFactory("apprtc", func(addr string) (channels.Channel, error) {
		return New(), nil
	})
}

// An apprtcChannel signals over apprtc.
type apprtcChannel struct {
}

// New creates a new apprtcChannel.
func New() channels.Channel {
	return &apprtcChannel{}
}

// Recv receives a message at the given key.
func (c *apprtcChannel) Recv(key string) (data string, err error) {
	conn, err := c.getConnection(key, "recv")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	for {
		var packet struct {
			Message string `json:"msg"`
			Error   string `json:"error"`
		}
		err = conn.ReadJSON(&packet)
		if err != nil {
			return "", fmt.Errorf("error receiving packet: %w", err)
		}

		if packet.Error != "" {
			return "", fmt.Errorf("apprtc returned an error: %s", packet.Error)
		}

		return packet.Message, nil
	}
}

// Send sends a message to the given key with the given data.
func (c *apprtcChannel) Send(key, data string) error {
	conn, err := c.getConnection(key, "send")
	if err != nil {
		return err
	}
	defer conn.Close()

	log.WithFields(log.Fields{
		"data": data,
	}).Info("[apprtc] send")
	err = conn.WriteJSON(map[string]interface{}{
		"cmd": "send",
		"msg": data,
	})
	if err != nil {
		return fmt.Errorf("error sending over websocket: %w", err)
	}

	return nil
}

func (c *apprtcChannel) getConnection(roomID, clientID string) (*websocket.Conn, error) {
	url := "wss://apprtc-ws.webrtc.org/ws"
	log.WithFields(log.Fields{
		"url": url,
	}).Info("[apprtc] connect")
	conn, resp, err := websocket.DefaultDialer.Dial(url, http.Header{
		"Origin": {"https://appr.tc"},
	})
	if err != nil {
		var msg string
		if resp.Body != nil {
			bs, _ := ioutil.ReadAll(resp.Body)
			msg = string(bs)
		}
		return nil, fmt.Errorf("error connecting to webrtc (msg=%s): %w", msg, err)
	}

	log.WithFields(log.Fields{
		"roomID":   roomID,
		"clientID": clientID,
	}).Info("[apprtc] register")
	err = conn.WriteJSON(map[string]interface{}{
		"cmd":      "register",
		"roomid":   roomID,
		"clientid": clientID,
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("error registering send client: %w", err)
	}

	return conn, err
}
