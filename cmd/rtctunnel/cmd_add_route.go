package main

import (
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/apex/log"
	"github.com/spf13/cobra"
)

func init() {
	var localPort, remotePort int
	var localPeer, remotePeer string

	addRouteCmd := &cobra.Command{
		Use:   "add-route",
		Short: "add a new route for network traffic",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig(options.configFile)
			if err != nil {
				log.WithError(err).Fatal("failed to load config")
			}

			if localPort == 0 {
				cmd.Usage()
				log.Fatal("local-port is required")
			}
			if remotePort == 0 {
				cmd.Usage()
				log.Fatal("remote-port is required")
			}
			if localPeer == "" {
				localPeer = cfg.KeyPair.Public.String()
			}
			localPeerKey, err := crypt.NewKey(localPeer)
			if err != nil {
				cmd.Usage()
				log.WithError(err).Fatal("invalid local peer key")
			}
			if remotePeer == "" {
				cmd.Usage()
				log.Fatal("remote-peer is required")
			}
			remotePeerKey, err := crypt.NewKey(remotePeer)
			if err != nil {
				cmd.Usage()
				log.WithError(err).Fatal("invalid remote peer key")
			}

			log.WithField("config-file", options.configFile).
				WithField("local-port", localPort).
				WithField("local-peer", localPeer).
				WithField("remote-peer", remotePeer).
				WithField("remote-port", remotePort).
				Info("adding route")

			err = cfg.AddRoute(localPort, localPeerKey, remotePeerKey, remotePort)
			if err != nil {
				log.WithError(err).Fatal("failed to add route")
			}

			err = cfg.Save(options.configFile)
			if err != nil {
				log.WithError(err).Fatal("failed to save config")
			}
		},
	}
	addRouteCmd.PersistentFlags().IntVarP(&localPort, "local-port", "", 0, "the local port to start listening on")
	addRouteCmd.PersistentFlags().StringVarP(&localPeer, "local-peer", "", "", "the local peer")
	addRouteCmd.PersistentFlags().StringVarP(&remotePeer, "remote-peer", "", "", "the remote peer")
	addRouteCmd.PersistentFlags().IntVarP(&remotePort, "remote-port", "", 0, "the remote port to connect to")
	rootCmd.AddCommand(addRouteCmd)
}
