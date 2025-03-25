package operator

import (
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/rtctunnel/pkg/channels"
)

func init() {
	channels.RegisterFactory("operator", func(addr string) (channels.Channel, error) {
		return New(strings.Replace(addr, "operator://", "https://", 1)), nil
	})
}

// DefaultClient is the client to use for making http requests
var DefaultClient = &http.Client{
	Timeout: 30 * time.Second,
}

// An operatorChannel signals over a custom http server.
type operatorChannel struct {
	url string
}

// New creates a new operatorChannel.
func New(url string) channels.Channel {
	return &operatorChannel{url: url}
}

// Recv receives a message at the given key.
func (c *operatorChannel) Recv(key string) (data string, err error) {
	log.Debug().Str("url", c.url).Str("key", key).Msg("[operator] receive")

	uv := url.Values{
		"address": {key},
	}
	for {
		req, _ := http.NewRequest("GET", c.url+"/sub?"+uv.Encode(), nil)
		resp, err := c.do(req)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				log.Warn().Msg("[operator] timed-out, retrying")
				continue
			}
			return "", err
		}
		if resp.StatusCode == http.StatusGatewayTimeout {
			log.Warn().Msg("[operator] timed-out, retrying")
			resp.Body.Close()
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return "", errors.New(resp.Status)
		}

		bs, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		log.Debug().Str("key", key).Str("data", string(bs)).Msg("[operator] received")

		return string(bs), nil
	}
}

// Send sends a message to the given key with the given data.
func (c *operatorChannel) Send(key, data string) error {
	log.Debug().Str("url", c.url).Str("key", key).Str("data", data).Msg("[operator] send")

	uv := url.Values{
		"address": {key},
		"data":    {data},
	}
	for {
		req, _ := http.NewRequest("POST", c.url+"/pub", strings.NewReader(uv.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := c.do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusGatewayTimeout {
			resp.Body.Close()
			continue
		}

		log.Debug().Int("status_code", resp.StatusCode).Str("status", resp.Status).Msg("[operator] sent")

		ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		return nil
	}
}

func (c *operatorChannel) do(req *http.Request) (*http.Response, error) {

	if runtime.GOOS == "js" {
		req.Header.Set("js.fetch:mode", "cors")
	}

	for {
		res, err := DefaultClient.Do(req)
		if err != nil && strings.Contains(c.url, "https://") && strings.Contains(err.Error(), "server gave HTTP response") {
			c.url = strings.Replace(c.url, "https://", "http://", 1)
			continue
		}
		return res, err
	}
}
