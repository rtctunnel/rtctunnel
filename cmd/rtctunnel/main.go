package main

import (
	"github.com/rs/zerolog/log"
)

func main() {
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to execute command")
	}
}
