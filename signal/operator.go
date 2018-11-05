package signal

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/apex/log"
)

type OperatorChannel struct {
	addr string
}

func NewOperatorChannel(addr string) *OperatorChannel {
	return &OperatorChannel{addr: addr}
}

func (c *OperatorChannel) Recv(key string) (data string, err error) {
	log.WithFields(log.Fields{
		"key": key,
	}).Info("[operator] receive")

	uv := url.Values{
		"address": {key},
	}
	for {
		resp, err := http.Get("http://" + c.addr + "/sub?" + uv.Encode())
		if err != nil {
			return "", err
		}
		if resp.StatusCode == http.StatusGatewayTimeout {
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

		log.WithFields(log.Fields{
			"key":  key,
			"data": string(bs),
		}).Info("[operator] received")

		return string(bs), nil
	}
}

func (c *OperatorChannel) Send(key, data string) error {
	log.WithFields(log.Fields{
		"key":  key,
		"data": data,
	}).Info("[operator] send")

	uv := url.Values{
		"address": {key},
		"data":    {data},
	}
	for {
		resp, err := http.Get("http://" + c.addr + "/pub?" + uv.Encode())
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusGatewayTimeout {
			resp.Body.Close()
			continue
		}

		ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		return nil
	}
}
