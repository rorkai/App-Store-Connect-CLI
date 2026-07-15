package main

import (
	"fmt"
	"os"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func versionInfoString() string {
	return fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, date)
}

func run(args []string) int {
	if telemetry.RunMaintenanceWorkerIfRequested(args) {
		return cmd.ExitSuccess
	}
	// Cross-invocation lookup caching is opted into at the binary entrypoint
	// so tests and library callers stay hermetic by default.
	shared.EnableAppLookupCache()
	return cmd.Run(args, versionInfoString())
}

func main() {
	os.Exit(run(os.Args[1:]))
}
