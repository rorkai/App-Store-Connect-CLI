package status

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const (
	untilReviewDone      = "review-done"
	untilReadyForSale    = "ready-for-sale"
	untilProcessed       = "processed"
	untilTestFlightReady = "testflight-ready"
	untilChange          = "change"
)

var untilConditions = []string{untilReviewDone, untilReadyForSale, untilProcessed, untilTestFlightReady, untilChange}

func validateUntil(until string, includes includeSet) error {
	section, included := "", true
	switch until {
	case untilReviewDone, untilReadyForSale:
		section, included = "appstore", includes.appstore
	case untilProcessed:
		section, included = "builds", includes.builds
	case untilTestFlightReady:
		section, included = "testflight", includes.testflight
	case untilChange:
	default:
		return fmt.Errorf("--until must be one of: %s", strings.Join(untilConditions, ", "))
	}
	if !included {
		return fmt.Errorf("--until %s requires the %s section in --include", until, section)
	}
	return nil
}

// untilCheck is one snapshot evaluated against an --until condition. An empty
// outcome means the condition has not reached a terminal outcome yet.
type untilCheck struct {
	outcome  string
	state    string
	positive bool
}

func evaluateUntil(until string, resp *dashboardResponse, changed bool) untilCheck {
	switch until {
	case untilChange:
		if changed {
			return untilCheck{outcome: "changed", positive: true}
		}
	case untilReviewDone, untilReadyForSale:
		state := ""
		if resp.AppStore != nil {
			state = strings.ToUpper(strings.TrimSpace(resp.AppStore.State))
		}
		switch {
		case until == untilReadyForSale && shared.IsLiveAppStoreVersionState(state):
			return untilCheck{outcome: untilReadyForSale, state: state, positive: true}
		case until == untilReviewDone && isApprovedAppStoreState(state):
			return untilCheck{outcome: "approved", state: state, positive: true}
		case state == "REJECTED" || state == "METADATA_REJECTED" || state == "INVALID_BINARY":
			return untilCheck{outcome: "rejected", state: state}
		case state == "DEVELOPER_REJECTED":
			return untilCheck{outcome: "developer-rejected", state: state}
		}
		return untilCheck{state: state}
	case untilProcessed:
		state := ""
		if resp.Builds != nil && resp.Builds.Latest != nil {
			state = strings.ToUpper(strings.TrimSpace(resp.Builds.Latest.ProcessingState))
		}
		switch state {
		case "VALID":
			return untilCheck{outcome: untilProcessed, state: state, positive: true}
		case "FAILED", "INVALID":
			return untilCheck{outcome: "failed", state: state}
		}
		return untilCheck{state: state}
	case untilTestFlightReady:
		state := ""
		if resp.TestFlight != nil {
			state = strings.ToUpper(strings.TrimSpace(resp.TestFlight.InternalBuildState))
		}
		switch state {
		case "READY_FOR_BETA_TESTING", "IN_BETA_TESTING":
			return untilCheck{outcome: untilTestFlightReady, state: state, positive: true}
		case "PROCESSING_EXCEPTION", "MISSING_EXPORT_COMPLIANCE", "EXPIRED":
			return untilCheck{outcome: "blocked", state: state}
		}
		return untilCheck{state: state}
	}
	return untilCheck{}
}

func isApprovedAppStoreState(state string) bool {
	switch state {
	case "ACCEPTED", "PENDING_DEVELOPER_RELEASE", "PENDING_APPLE_RELEASE", "PROCESSING_FOR_DISTRIBUTION", "PROCESSING_FOR_APP_STORE", "PREORDER_READY_FOR_SALE", "PENDING_CONTRACT":
		return true
	}
	return shared.IsLiveAppStoreVersionState(state)
}

func finishUntil(until string, check untilCheck, polls int, output string, pretty bool) error {
	result := &asc.StatusUntilResult{
		Until:   until,
		Reached: check.outcome != "",
		Outcome: check.outcome,
		State:   check.state,
		Polls:   polls,
	}
	if !result.Reached {
		result.Outcome = "pending"
	}

	format := strings.ToLower(strings.TrimSpace(output))
	if format == "" {
		format = shared.DefaultOutputFormat()
	}
	if format != "json" {
		fmt.Fprintln(os.Stdout)
	}
	if err := shared.PrintOutput(result, format, pretty); err != nil {
		return err
	}
	if check.positive {
		return nil
	}

	state := ""
	if check.state != "" {
		state = fmt.Sprintf(" (state %s)", shared.SanitizeTerminal(check.state))
	}
	if result.Reached {
		message := fmt.Sprintf("status: --until %s ended with outcome %s%s", until, result.Outcome, state)
		fmt.Fprintln(os.Stderr, message)
		return shared.NewStderrReportedError(shared.NewValidationError(errors.New(message)))
	}
	message := fmt.Sprintf("status: --until %s not reached after %d polls%s", until, polls, state)
	fmt.Fprintln(os.Stderr, message)
	return shared.NewPendingError(message)
}

// resolveNextCommands omits a command whose values cannot be shell-quoted
// rather than print an approximation.
func resolveNextCommands(resp *dashboardResponse, appID string, platform string) []asc.StatusNextCommand {
	commands := []asc.StatusNextCommand{}
	rootFlags, ok := shared.RootFlagsForReinvocation()
	if !ok {
		return commands
	}
	add := func(reason string, mutates bool, args ...string) {
		if mutates && readonly.Enabled() {
			return
		}
		parts := append([]string{"asc"}, rootFlags...)
		for _, arg := range args {
			quoted, quotable := shared.ShellQuote(arg)
			if !quotable {
				return
			}
			parts = append(parts, quoted)
		}
		commands = append(commands, asc.StatusNextCommand{Command: strings.Join(parts, " "), Reason: reason, Mutates: mutates})
	}

	var build *latestBuild
	if resp.Builds != nil {
		build = resp.Builds.Latest
	}
	if build != nil && strings.EqualFold(build.ProcessingState, "PROCESSING") {
		add("Wait for the latest build to finish processing.", false, "builds", "wait", "--build-id", build.ID)
	}

	version := resp.AppStore
	if version == nil || version.VersionID == "" {
		return commands
	}
	switch strings.ToUpper(strings.TrimSpace(version.State)) {
	case "PREPARE_FOR_SUBMISSION":
		add("Check App Store submission readiness.", false, "validate", "--app", appID, "--version-id", version.VersionID)
		if build != nil && !build.appStoreRestricted && strings.EqualFold(build.ProcessingState, "VALID") && build.Version == version.Version && build.Platform == version.Platform {
			add("Attach the latest build and submit the version for App Review.", true, "review", "submit", "--app", appID, "--version-id", version.VersionID, "--build-id", build.ID, "--confirm")
		}
	case "WAITING_FOR_REVIEW", "IN_REVIEW":
		args := []string{"status", "--app", appID}
		if platform != "" {
			args = append(args, "--platform", platform)
		}
		add("Wait for the App Review decision.", false, append(args, "--until", untilReviewDone)...)
	case "PENDING_DEVELOPER_RELEASE":
		add("Release the approved version.", true, "versions", "release", "--version-id", version.VersionID, "--confirm")
	case "REJECTED", "METADATA_REJECTED", "INVALID_BINARY", "DEVELOPER_REJECTED":
		add("Diagnose what blocks resubmission.", false, "review", "doctor", "--app", appID, "--version-id", version.VersionID)
	}
	return commands
}
