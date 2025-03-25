package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/rtctunnel/rtctunnel/internal/app"
	"github.com/rtctunnel/rtctunnel/internal/crypt"
	"github.com/rtctunnel/rtctunnel/internal/peer"
	"github.com/rtctunnel/rtctunnel/internal/signal"
	"github.com/rtctunnel/rtctunnel/pkg/channels"
	_ "github.com/rtctunnel/rtctunnel/pkg/channels/operator"
)

var (
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run the rtctunnel",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := app.LoadConfig(options.configFile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to load config file")
			}

			if !cfg.KeyPair.Private.Valid() {
				log.Fatal().Err(err).Msg("invalid config file, missing private key")
			}

			log.Info().
				Str("config-file", options.configFile).
				Str("public-key", cfg.KeyPair.Public.String()).
				Interface("routes", cfg.Routes).
				Str("signal-channel", cfg.SignalChannel).
				Msg("using config")

			if cfg.SignalChannel != "" {
				ch, err := channels.Get(cfg.SignalChannel)
				if err != nil {
					log.Fatal().Err(err).Msg("invalid signalchannel in yaml config")
				}

				signal.SetDefaultOptions(signal.WithChannel(ch))
			}

			peerConns := map[crypt.Key]*peer.Conn{}
			for _, route := range cfg.Routes {
				var peerPublicKey crypt.Key
				if route.LocalPeer == cfg.KeyPair.Public {
					peerPublicKey = route.RemotePeer
				} else if route.RemotePeer == cfg.KeyPair.Public {
					peerPublicKey = route.LocalPeer
				}

				conn, ok := peerConns[peerPublicKey]
				if !ok {
					var err error
					conn, err = peer.Open(cfg.KeyPair, peerPublicKey)
					if err != nil {
						log.Fatal().Err(err).Msg("failed to open peer connection")
					}
					peerConns[peerPublicKey] = conn

					go acceptRemote(cfg, conn)
				}

				if route.LocalPeer == cfg.KeyPair.Public {
					go localListener(conn, route)
				}
			}

			select {}
		},
	}
)

func acceptRemote(cfg *app.Config, pc *peer.Conn) {
	for {
		remote, port, err := pc.Accept()
		if err != nil {
			log.Error().Err(err).Msg("failed to accept remote connection")
			continue
		}

		var route *app.Route
		for _, r := range cfg.Routes {
			if r.RemotePeer == cfg.KeyPair.Public && r.RemotePort == port {
				route = &r
				break
			}
		}

		if route == nil {
			log.Warn().Int("port", port).Msg("remote peer attempted to connect to disallowed port")
			remote.Close()
			continue
		}

		switch route.Type {
		case app.RouteTypeTCP, "":
			acceptRemoteTCP(pc, *route, remote)
		case app.RouteTypeUDP:
			acceptRemoteUDP(pc, *route, remote)
		default:
			log.Fatal().Str("type", string(route.Type)).Msg("invalid route type")
		}

	}
}

func acceptRemoteTCP(pc *peer.Conn, route app.Route, remote net.Conn) {
	local, err := net.Dial("tcp", net.JoinHostPort(options.bindAddress, fmt.Sprint(route.RemotePort)))
	if err != nil {
		log.Warn().Err(err).Msg("failed to establish connection to local port")
		remote.Close()
	}
	go joinConns(local, remote)
}

func acceptRemoteUDP(pc *peer.Conn, route app.Route, remote net.Conn) {
	local, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP(options.bindAddress),
		Port: route.RemotePort,
	})
	if err != nil {
		log.Warn().Err(err).Msg("failed to establish connection to local port")
		remote.Close()
		return
	}
	go joinConns(local, remote)
}

func localListener(pc *peer.Conn, route app.Route) {
	log.Info().Interface("route", route).Msg("starting local listener")

	switch route.Type {
	case app.RouteTypeTCP, "":
		localTCPListener(pc, route)
	case app.RouteTypeUDP:
		localUDPListener(pc, route)
	default:
		log.Fatal().Str("type", string(route.Type)).Msg("invalid route type")
	}
}

func localTCPListener(pc *peer.Conn, route app.Route) {
	li, err := net.Listen("tcp", net.JoinHostPort(options.bindAddress, fmt.Sprint(route.LocalPort)))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create listener")
	}
	defer li.Close()

	for {
		local, err := li.Accept()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				time.Sleep(time.Second)
				continue
			}
			log.Fatal().Err(err).Msg("error accepting connection")
		}

		remote, err := pc.Open(route.RemotePort)
		if err != nil {
			local.Close()
			log.Warn().Err(err).Msg("failed to create data channel")
			continue
		}

		go joinConns(local, remote)
	}
}

func localUDPListener(pc *peer.Conn, route app.Route) {
	local, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP(options.bindAddress),
		Port: route.LocalPort,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("error listening for UDP packets")
	}

	remote, err := pc.Open(route.RemotePort)
	if err != nil {
		_ = local.Close()
		log.Fatal().Err(err).Msg("error opening remote connection")
	}

	go joinConns(local, remote)
}

func joinConns(c1, c2 net.Conn) {
	defer c1.Close()
	defer c2.Close()

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(c1, c2)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(c2, c1)
		errc <- err
	}()
	err := <-errc
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		log.Warn().Err(err).Msg("error copying data between connections")
	}
}
