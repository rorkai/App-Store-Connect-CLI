package shared

import (
	"io"
	"os"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/diagnosticoutput"
)

// RequireConfirm enforces --confirm on a destructive command. Without it, it
// prints "Error: --confirm is required" to stderr and returns a usage error
// with the required-input diagnostic. Commands check it after their other
// required flags and before any request, file read or prompt.
func RequireConfirm(confirm bool) error {
	if confirm {
		return nil
	}
	return missingConfirmTo(diagnosticoutput.Writer(os.Stderr))
}

// RequireConfirm is RequireConfirm with the "Error:" line written to d's
// stderr.
func (d *Deps) RequireConfirm(confirm bool) error {
	if confirm {
		return nil
	}
	return missingConfirmTo(d.IO.ErrOut)
}

func missingConfirmTo(stderr io.Writer) error {
	return WithDiagnostic(usageErrorTo(stderr, "--confirm is required"), DiagnosticRequiredInputMissing, "--confirm")
}

// RequireConfirmUnlessDryRun enforces the apply decision for commands that turn
// file input into App Store Connect mutations: the caller must either preview
// the plan with --dry-run or accept the mutation with --confirm. The "Error:"
// line goes to d's stderr.
//
// When both flags are set, --dry-run wins and nothing is mutated. That matches
// the existing convention in `asc release stage`, `asc review submit`, and
// `asc subscriptions pricing equalize`, so contradictory input resolves to the
// safe side in every command instead of being rejected in some and honored in
// others.
func (d *Deps) RequireConfirmUnlessDryRun(dryRun, confirm bool) error {
	return requireConfirmUnlessDryRunTo(d.IO.ErrOut, dryRun, confirm)
}

func requireConfirmUnlessDryRunTo(stderr io.Writer, dryRun, confirm bool) error {
	if dryRun || confirm {
		return nil
	}
	return WithDiagnostic(
		usageErrorTo(stderr, "--confirm is required unless --dry-run is set"),
		DiagnosticRequiredInputMissing,
		"--confirm",
	)
}
