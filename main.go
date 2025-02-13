package main

import (
	"github.com/rs/zerolog/log"

	"github.com/rtctunnel/rtctunnel/cmd"
)

func main() {
	err := cmd.RootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to execute command")
	}
}
