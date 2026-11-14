package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func webXcodeCloudNextBuildNumberCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "settings",
		ShortUsage: "asc web xcode-cloud settings <subcommand> [flags]",
		ShortHelp:  "Manage Xcode Cloud product settings.",
		UsageFunc:  shared.DefaultUsageFunc,
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			webNextBuildNumberGroup(),
			webVersionAliasesGroup(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func webNextBuildNumberGroup() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings next-build-number", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "next-build-number",
		ShortUsage: "asc web xcode-cloud settings next-build-number <subcommand> [flags]",
		ShortHelp:  "Manage the next Xcode Cloud build number.",
		UsageFunc:  shared.DefaultUsageFunc,
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			webNextBuildNumberShow(),
			webNextBuildNumberSet(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func webNextBuildNumberShow() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings next-build-number show", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")

	return &ffcli.Command{
		Name:       "show",
		ShortUsage: "asc web xcode-cloud settings next-build-number show --product-id ID [flags]",
		ShortHelp:  "Show the next Xcode Cloud build number.",
		LongHelp: `WEB SESSION WORKFLOWS

Show the persistent next build number configured for an Xcode Cloud product.



Example:
  asc web xcode-cloud settings next-build-number show --product-id "UUID" --apple-id "user@example.com"`,
		UsageFunc: shared.DefaultUsageFunc,
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				return shared.RequiredFlag("--product-id")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud settings next-build-number show failed: session has no public provider ID")
			}

			current, err := newCIClientFn(session).GetCINextBuildNumber(requestCtx, teamID, pid)
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud settings next-build-number show")
			}
			result := &asc.WebXcodeCloudNextBuildNumberResult{
				ProductID:       pid,
				NextBuildNumber: current.NextBuildNumber,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return asc.PrintTable(result) },
				func() error { return asc.PrintMarkdown(result) },
			)
		},
	}
}

func webNextBuildNumberSet() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings next-build-number set", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	value := fs.Int("value", 0, "Next build number (required; must exceed the current value)")
	confirm := fs.Bool("confirm", false, "Confirm changing the persistent next build number (required)")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web xcode-cloud settings next-build-number set --product-id ID --value NUMBER --confirm [flags]",
		ShortHelp:  "Set the next Xcode Cloud build number.",
		LongHelp: `WEB SESSION WORKFLOWS

Set the persistent next build number for an Xcode Cloud product. The new value
must exceed the current value. The command reads the setting again after the
update and succeeds only when the requested value is confirmed. Changing this
setting requires the App Store Connect Admin or App Manager role.



Example:
  asc web xcode-cloud settings next-build-number set --product-id "UUID" --value 102 --confirm --apple-id "user@example.com"`,
		UsageFunc: shared.DefaultUsageFunc,
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				return shared.RequiredFlag("--product-id")
			}
			if *value <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --value must be greater than 0")
				return shared.UsageError("--value must be greater than 0")
			}
			if !*confirm {
				return shared.RequiredFlag("--confirm")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud settings next-build-number set failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			current, err := client.GetCINextBuildNumber(requestCtx, teamID, pid)
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud settings next-build-number set")
			}
			if *value <= current.NextBuildNumber {
				return fmt.Errorf("next build number must be greater than current value %d", current.NextBuildNumber)
			}
			writeErr := client.SetCINextBuildNumber(requestCtx, teamID, pid, *value)
			if writeErr != nil && !isAmbiguousNextBuildNumberWriteFailure(writeErr) {
				return withNextBuildNumberSetHint(writeErr)
			}

			verifyCtx := requestCtx
			verifyCancel := func() {}
			if writeErr != nil {
				verifyCtx, verifyCancel = newWebRequestContext(ctx)
			}
			defer verifyCancel()

			updated, err := client.GetCINextBuildNumber(verifyCtx, teamID, pid)
			if err != nil {
				if writeErr != nil {
					return fmt.Errorf("xcode-cloud settings next-build-number set may have succeeded but reconciliation failed: write error: %w; re-read error: %w", writeErr, err)
				}
				verifyErr := withWebAuthHint(err, "xcode-cloud settings next-build-number verification")
				return fmt.Errorf("xcode-cloud settings next-build-number set may have succeeded but verification failed: %w", verifyErr)
			}
			if updated.NextBuildNumber != *value {
				if writeErr != nil {
					if updated.NextBuildNumber == current.NextBuildNumber {
						return fmt.Errorf("xcode-cloud settings next-build-number set failed: remote still reports %d; the write was not applied: %w", updated.NextBuildNumber, writeErr)
					}
					return fmt.Errorf("xcode-cloud settings next-build-number set is unverified: remote reports %d, which is neither the previous value %d nor the requested value %d: %w", updated.NextBuildNumber, current.NextBuildNumber, *value, writeErr)
				}
				return fmt.Errorf("xcode-cloud settings next-build-number set verification failed: got %d, expected %d", updated.NextBuildNumber, *value)
			}

			previous := current.NextBuildNumber
			result := &asc.WebXcodeCloudNextBuildNumberResult{
				ProductID:               pid,
				PreviousNextBuildNumber: &previous,
				NextBuildNumber:         updated.NextBuildNumber,
				Updated:                 true,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return asc.PrintTable(result) },
				func() error { return asc.PrintMarkdown(result) },
			)
		},
	}
}

// isAmbiguousNextBuildNumberWriteFailure reports failures where the request was
// handed to the transport but no response established whether Apple applied it.
func isAmbiguousNextBuildNumberWriteFailure(err error) bool {
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) {
		return false
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func withNextBuildNumberSetHint(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
		return fmt.Errorf("xcode-cloud settings next-build-number set failed: changing the next build number requires the App Store Connect Admin or App Manager role: %w", err)
	}
	return withWebAuthHint(err, "xcode-cloud settings next-build-number set")
}
