package main

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Prints information about the rtctunnel config",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig(options.configFile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to load config file")
			}

			fmt.Printf("public-key: %s\n", cfg.KeyPair.Public)
			fmt.Printf("routes: \n")
			for _, route := range cfg.Routes {
				fmt.Printf("  %s:%d -> %s:%d\n",
					route.LocalPeer, route.LocalPort,
					route.RemotePeer, route.RemotePort)
			}
		},
	}
	rootCmd.AddCommand(infoCmd)
}
