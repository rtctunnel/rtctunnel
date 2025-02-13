package app

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"github.com/rtctunnel/rtctunnel/internal/crypt"
)

type RouteType string

const (
	RouteTypeTCP RouteType = "TCP"
	RouteTypeUDP RouteType = "UDP"
)

// A Route is a route from a local port to a remote port over a webrtc connection.
type Route struct {
	LocalPort  int
	LocalPeer  crypt.Key
	RemotePeer crypt.Key
	RemotePort int
	Type       RouteType
}

// A Config is the configuration for the RTCTunnel.
type Config struct {
	KeyPair       crypt.KeyPair
	Routes        []Route `json:",omitempty"`
	SignalChannel string  `json:"signalchannel,omitempty"`
}

// LoadConfig loads the config off of the disk.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var cfg Config
	switch filepath.Ext(path) {
	case ".json":
		err = json.Unmarshal(bs, &cfg)
		if err != nil {
			return nil, err
		}
	default:
		err = yaml.Unmarshal(bs, &cfg)
		if err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// AddRoute adds a route to the config. It also validates the route and removes duplicates.
func (cfg *Config) AddRoute(localPort int, localPeer, remotePeer crypt.Key, remotePort int, routeType RouteType) error {
	nr := Route{
		LocalPort:  localPort,
		LocalPeer:  localPeer,
		RemotePeer: remotePeer,
		RemotePort: remotePort,
		Type:       routeType,
	}

	for _, r := range cfg.Routes {
		if nr == r {
			return nil
		}
	}
	cfg.Routes = append(cfg.Routes, nr)
	return nil
}

// Save saves the config file
func (cfg *Config) Save(path string) error {
	var bs []byte
	var err error
	switch filepath.Ext(path) {
	case ".json":
		bs, err = json.Marshal(cfg)
	default:
		bs, err = yaml.Marshal(cfg)
	}
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, bs, 0600)
	if err != nil {
		return err
	}

	return nil
}
