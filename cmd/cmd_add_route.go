package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/rtctunnel/rtctunnel/internal/app"
	"github.com/rtctunnel/rtctunnel/internal/crypt"
)

var (
	localPort, remotePort int
	localPeer, remotePeer string
	routeType             string

	addRouteCmd = &cobra.Command{
		Use:   "add-route",
		Short: "add a new route for network traffic",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := app.LoadConfig(options.configFile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to load config")
			}

			if localPort == 0 {
				cmd.Usage()
				log.Fatal().Msg("local-port is required")
			}
			if remotePort == 0 {
				cmd.Usage()
				log.Fatal().Msg("remote-port is required")
			}
			if localPeer == "" {
				localPeer = cfg.KeyPair.Public.String()
			}
			localPeerKey, err := crypt.NewKey(localPeer)
			if err != nil {
				cmd.Usage()
				log.Fatal().Err(err).Msg("invalid local peer key")
			}
			if remotePeer == "" {
				cmd.Usage()
				log.Fatal().Msg("remote-peer is required")
			}
			remotePeerKey, err := crypt.NewKey(remotePeer)
			if err != nil {
				cmd.Usage()
				log.Fatal().Err(err).Msg("invalid remote peer key")
			}

			log.Info().
				Str("config-file", options.configFile).
				Int("local-port", localPort).
				Str("local-peer", localPeer).
				Str("remote-peer", remotePeer).
				Int("remote-port", remotePort).
				Str("type", routeType).
				Msg("adding route")

			err = cfg.AddRoute(localPort, localPeerKey, remotePeerKey, remotePort, app.RouteType(routeType))
			if err != nil {
				log.Fatal().Err(err).Msg("failed to add route")
			}

			err = cfg.Save(options.configFile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to save config")
			}
		},
	}
)

func init() {
	addRouteCmd.PersistentFlags().IntVarP(&localPort, "local-port", "", 0, "the local port to start listening on")
	addRouteCmd.PersistentFlags().StringVarP(&localPeer, "local-peer", "", "", "the local peer")
	addRouteCmd.PersistentFlags().StringVarP(&remotePeer, "remote-peer", "", "", "the remote peer")
	addRouteCmd.PersistentFlags().IntVarP(&remotePort, "remote-port", "", 0, "the remote port to connect to")
	addRouteCmd.PersistentFlags().StringVarP(&routeType, "type", "", "TCP", "the route type (TCP or UDP)")
}
