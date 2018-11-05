package main

import (
	"os"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
)

func main() {
	log.SetHandler(text.New(os.Stderr))

	err := rootCmd.Execute()
	if err != nil {
		log.WithError(err).Fatal("failed to execute command")
	}
}
