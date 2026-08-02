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
	case "agent-event-batch":
		_, err = conformance.LoadAgentEventBatch(args[1])
	case "agent-event-ack":
		_, err = conformance.LoadAgentEventBatchAck(args[1])
	case "agent-plugin-reconcile":
		_, err = conformance.LoadAgentPluginReconcileCommand(args[1])
	case "agent-plugin-status":
		_, err = conformance.LoadAgentPluginStatus(args[1])
	case "agent-policy-reconcile":
		_, err = conformance.LoadAgentPolicyReconcileCommand(args[1])
	case "agent-traffic-snapshot":
		_, err = conformance.LoadAgentTrafficSnapshot(args[1])
	case "agent-access-event":
		_, err = conformance.LoadAgentAccessEvent(args[1])
	case "node-plugin-info":
		_, err = conformance.LoadNodePluginInfo(args[1])
	case "center-plugin-info":
		_, err = conformance.LoadCenterPluginInfo(args[1])
	case "center-plugin-activation":
		_, err = conformance.LoadCenterPluginActivation(args[1])
	case "center-plugin-status":
		_, err = conformance.LoadCenterPluginStatus(args[1])
	case "center-plugin-ui":
		_, err = conformance.LoadCenterPluginUIRequest(args[1])
	case "center-plugin-nodes":
		_, err = conformance.LoadCenterPluginNodes(args[1])
	case "center-plugin-services":
		_, err = conformance.LoadCenterPluginServices(args[1])
	case "center-plugin-subscription":
		_, err = conformance.LoadCenterPluginSubscription(args[1])
	case "center-plugin-events":
		_, err = conformance.LoadCenterPluginEvents(args[1])
	case "center-plugin-published-events":
		_, err = conformance.LoadCenterPluginPublishedEvents(args[1])
	case "plugin-release":
		_, err = conformance.VerifyPluginRelease(args[1])
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
	fmt.Fprintln(writer, "usage: relayward-conformance <manifest|plugin-release|envelope|agent-register|agent-envelope|agent-event-batch|agent-event-ack|agent-plugin-reconcile|agent-plugin-status|agent-policy-reconcile|agent-traffic-snapshot|agent-access-event|node-plugin-info|center-plugin-info|center-plugin-activation|center-plugin-status|center-plugin-ui|center-plugin-nodes|center-plugin-services|center-plugin-subscription|center-plugin-events|center-plugin-published-events> <path>")
	fmt.Fprintln(writer, "       relayward-conformance version")
}
