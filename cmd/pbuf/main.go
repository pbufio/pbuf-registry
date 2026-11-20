package main

import (
	"log"
	"os"

	"github.com/pbufio/pbuf-registry/internal/cli/cmd"
)

const (
	noFlags  = 0
	noPrefix = ""
)

func main() {
	log.SetFlags(noFlags)
	log.SetPrefix(noPrefix)
	log.SetOutput(os.Stderr)

	err := cmd.NewRootCmd().Execute()
	if err != nil {
		log.Fatalf("failed to execute command: %v", err)
	}
}
