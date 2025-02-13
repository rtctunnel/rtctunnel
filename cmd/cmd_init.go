package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/rtctunnel/rtctunnel/internal/app"
	"github.com/rtctunnel/rtctunnel/internal/crypt"
)

var (
	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Creates a new RTCTunnel config and stores it to disk",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := app.LoadConfig(options.configFile)
			if err == nil {
				log.Fatal().
					Str("config-file", options.configFile).
					Msg("config file already exists. remove it if you want to re-initialize")
			}

			cfg = new(app.Config)
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
)
