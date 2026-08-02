package main

import (
	"os"

	"github.com/Relayward/relayward-sdk/pluginfixture"
)

var version = "0.0.0-dev"

func main() {
	os.Exit(pluginfixture.Run(version))
}
