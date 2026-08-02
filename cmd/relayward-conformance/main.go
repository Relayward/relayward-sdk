package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Relayward/relayward-sdk/conformance"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(stdout, version)
		return 0
	}
	if len(args) != 2 {
		printUsage(stderr)
		return 2
	}

	var err error
	switch args[0] {
	case "manifest":
		_, err = conformance.LoadManifest(args[1])
	case "envelope":
		_, err = conformance.LoadEnvelope(args[1])
	case "agent-register":
		_, err = conformance.LoadAgentRegisterRequest(args[1])
	case "agent-envelope":
		_, err = conformance.LoadAgentEnvelope(args[1])
	default:
		printUsage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", args[0], err)
		return 1
	}
	fmt.Fprintf(stdout, "%s valid: %s\n", args[0], args[1])
	return 0
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage: relayward-conformance <manifest|envelope|agent-register|agent-envelope> <path>")
	fmt.Fprintln(writer, "       relayward-conformance version")
}
