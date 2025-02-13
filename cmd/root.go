package cmd

import (
	"path/filepath"

	"github.com/kirsle/configdir"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	options struct {
		bindAddress string
		configFile  string
		logLevel    string
	}
	RootCmd = &cobra.Command{
		Use:   "",
		Short: "RTCTunnel creates network tunnels over WebRTC",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			lvl, err := zerolog.ParseLevel(options.logLevel)
			if err != nil {
				return err
			}
			zerolog.SetGlobalLevel(lvl)
			return nil
		},
	}
)

func init() {
	RootCmd.AddCommand(runCmd)
	RootCmd.AddCommand(initCmd)
	RootCmd.AddCommand(infoCmd)
	RootCmd.AddCommand(addRouteCmd)

	RootCmd.PersistentFlags().StringVar(&options.bindAddress, "bind-address", "127.0.0.1", "the ip address to bind")
	RootCmd.PersistentFlags().StringVar(&options.configFile, "config-file", defaultConfigFile(), "the config file")
	RootCmd.PersistentFlags().StringVar(&options.logLevel, "log-level", "info", "the log level to use")
}

func defaultConfigFile() string {
	dir := configdir.LocalConfig("rtctunnel")
	if err := configdir.MakePath(dir); err != nil {
		log.Fatal().Msg("failed to create config folder")
	}

	return filepath.Join(dir, "rtctunnel.yaml")
}
