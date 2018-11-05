package main

import (
	"github.com/rtctunnel/rtctunnel/crypt"
	"github.com/apex/log"
	"github.com/spf13/cobra"
)

func init() {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new RTCTunnel config and stores it to disk",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig(options.configFile)
			if err == nil {
				log.WithField("config-file", options.configFile).
					Fatal("config file already exists. remove it if you want to re-initialize")
			}

			cfg = new(Config)
			cfg.KeyPair = crypt.GenerateKeyPair()

			log.WithFields(log.Fields{
				"public-key":  cfg.KeyPair.Public,
				"config-file": options.configFile,
			}).Info("saving config file")

			err = cfg.Save(options.configFile)
			if err != nil {
				log.WithError(err).Fatal("failed to save config file")
			}
		},
	}
	rootCmd.AddCommand(initCmd)

}
