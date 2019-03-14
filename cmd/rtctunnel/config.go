package main

import (
	"io/ioutil"
	"os"

	"github.com/rtctunnel/rtctunnel/crypt"
	yaml "gopkg.in/yaml.v2"
)

// A Route is a route from a local port to a remote port over a webrtc connection.
type Route struct {
	LocalPort  int
	LocalPeer  crypt.Key
	RemotePeer crypt.Key
	RemotePort int
}

// A Config is the configuration for the RTCTunnel.
type Config struct {
	KeyPair       crypt.KeyPair
	Routes        []Route
	SignalChannel string `yaml:"signalchannel,omitempty"`
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

	err = yaml.Unmarshal(bs, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

// AddRoute adds a route to the config. It also validates the route and removes duplicates.
func (cfg *Config) AddRoute(localPort int, localPeer, remotePeer crypt.Key, remotePort int) error {
	nr := Route{
		LocalPort:  localPort,
		LocalPeer:  localPeer,
		RemotePeer: remotePeer,
		RemotePort: remotePort,
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
	bs, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, bs, 0600)
	if err != nil {
		return err
	}

	return nil
}
