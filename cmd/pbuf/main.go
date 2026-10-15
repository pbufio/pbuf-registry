package main

import (
	"io"
	"log"
	"os"

	"github.com/pbufio/pbuf-registry/internal/cli/cmd"
	"go.uber.org/automaxprocs/maxprocs"
)

const (
	noFlags  = 0
	noPrefix = ""
)

func main() {
	log.SetFlags(noFlags)
	log.SetPrefix(noPrefix)
	log.SetOutput(os.Stdout)

	err := cmd.NewRootCmd().Execute()
	if err != nil {
		log.Fatalf("failed to execute command: %v", err)
	}
}

func init() {
	log.SetOutput(io.Discard)
	_, err := maxprocs.Set(maxprocs.Logger(log.Printf))
	if err != nil {
		log.SetOutput(os.Stdout)
		log.Printf("failed to set maxprocs: %v", err)
	}
}
