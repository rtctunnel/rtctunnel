package main

import (
	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/spf13/cobra"
)

func init() {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new RTCTunnel config and stores it to disk",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig(options.configFile)
			if err == nil {
				log.Fatal().Err(err).Msg("config file already exists. remove it if you want to re-initialize")
			}

			cfg = new(Config)
			cfg.KeyPair = crypt.GenerateKeyPair()

			log.Info().
				Str("public-key", cfg.KeyPair.Public.String()).
				Str("config-file", options.configFile).
				Msg("saving config file")

			err = cfg.Save(options.configFile)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to save config file")
			}
		},
	}
	rootCmd.AddCommand(initCmd)

}
