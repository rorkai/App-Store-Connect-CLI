package versions

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const phasedReleaseCheckWaitDelay = 5 * time.Second

type phasedReleaseCheckResult struct {
	ExitCode int
	TimedOut bool
}

var phasedReleaseGuardRunCheck = runPhasedReleaseCheck

// PhasedReleaseGuardCommand returns the guard subcommand.
func PhasedReleaseGuardCommand() *ffcli.Command {
	fs := flag.NewFlagSet("phased-release guard", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	pollInterval := fs.Duration("poll-interval", 15*time.Minute, "Time between health checks")
	checkTimeout := fs.Duration("check-timeout", 5*time.Minute, "Maximum run time for one health check; a timed-out check counts as failed")
	maxPolls := fs.Int("max-polls", 0, "Maximum polls before exiting (0 = until the release leaves ACTIVE)")
	completeOnDay := fs.Int("complete-on-day", 0, "Complete the release when a check passes on or after this rollout day (1-7, 0 = never)")
	confirm := fs.Bool("confirm", false, "Allow the guard to pause or complete the phased release")
	dryRun := fs.Bool("dry-run", false, "Run checks and report what the guard would do without changing the phased release")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "guard",
		ShortUsage: "asc versions phased-release guard --version-id VERSION_ID (--confirm | --dry-run) [flags] -- <command> [args...]",
		ShortHelp:  "Pause a phased release when a health check fails.",
		LongHelp: `Pause a phased release when a health check fails.

While the phased release is ACTIVE, the guard runs the command after -- every
--poll-interval. The command runs directly, without a shell, and its output goes
to stderr. A non-zero exit status, or running longer than --check-timeout,
pauses the phased release. With --complete-on-day, a passing check on or after
that rollout day completes the release for all users.

The check receives these environment variables:
  ASC_PHASED_RELEASE_VERSION_ID  App Store version ID
  ASC_PHASED_RELEASE_ID          Phased release ID
  ASC_PHASED_RELEASE_STATE       Current phased release state
  ASC_PHASED_RELEASE_DAY         Current rollout day
  ASC_PHASED_RELEASE_CHECK       1-based check number

The guard stops when the release is COMPLETE, PAUSED, or INACTIVE, and prints a
receipt whose outcome is one of: paused, would-pause, completed, would-complete,
already-complete, already-paused, inactive, max-polls.

Exit codes:
  0  The guard stopped without a failed check
  1  A check failed and the release was paused (or would be, with --dry-run),
     or the check could not be started
  2  Invalid usage
  Other codes follow the standard App Store Connect error mapping.

Examples:
  asc versions phased-release guard --version-id "VERSION_ID" --dry-run -- ./scripts/health-check.sh
  asc versions phased-release guard --version-id "VERSION_ID" --confirm --poll-interval 30m -- ./scripts/health-check.sh
  asc versions phased-release guard --version-id "VERSION_ID" --confirm --complete-on-day 5 -- ./scripts/health-check.sh --threshold 99.5`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			if len(args) == 0 {
				return shared.UsageError("a check command is required after --")
			}
			if *confirm && *dryRun {
				return shared.UsageError("--confirm and --dry-run are mutually exclusive")
			}
			if !*confirm && !*dryRun {
				return shared.UsageError("--confirm is required to pause or complete the phased release (or pass --dry-run to preview)")
			}
			if *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *checkTimeout <= 0 {
				return shared.UsageError("--check-timeout must be greater than 0")
			}
			if *maxPolls < 0 {
				return shared.UsageError("--max-polls must be greater than or equal to 0")
			}
			if *completeOnDay < 0 || *completeOnDay > 7 {
				return shared.UsageError("--complete-on-day must be between 1 and 7")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("phased-release guard: %w", err)
			}

			guard := phasedReleaseGuard{
				client:        client,
				command:       args,
				pollInterval:  *pollInterval,
				checkTimeout:  *checkTimeout,
				maxPolls:      *maxPolls,
				completeOnDay: *completeOnDay,
				result: &asc.PhasedReleaseGuardResult{
					VersionID: version,
					DryRun:    *dryRun,
					Checks:    []asc.PhasedReleaseGuardCheck{},
				},
			}
			if err := guard.run(ctx); err != nil {
				return fmt.Errorf("phased-release guard: %w", err)
			}
			if err := shared.PrintOutput(guard.result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if outcome := guard.result.Outcome; outcome == "paused" || outcome == "would-pause" {
				return shared.NewReportedError(fmt.Errorf("phased-release guard: health check failed; outcome %s", outcome))
			}
			return nil
		},
	}
}

type phasedReleaseGuard struct {
	client        *asc.Client
	command       []string
	pollInterval  time.Duration
	checkTimeout  time.Duration
	maxPolls      int
	completeOnDay int
	result        *asc.PhasedReleaseGuardResult
}

func (g *phasedReleaseGuard) run(ctx context.Context) error {
	polls := 0
	_, err := asc.PollUntilTolerant(ctx, g.pollInterval, func(ctx context.Context) (struct{}, bool, error) {
		resp, err := g.fetch(ctx)
		if err != nil {
			return struct{}{}, false, phasedReleaseFetchError{err}
		}
		done, err := g.step(ctx, resp)
		if err != nil || done {
			return struct{}{}, true, err
		}
		polls++
		if g.maxPolls > 0 && polls >= g.maxPolls {
			g.result.Outcome = "max-polls"
			return struct{}{}, true, nil
		}
		return struct{}{}, false, nil
	}, asc.PollOptions{Tolerate: g.tolerateFetchError})
	return err
}

// phasedReleaseFetchError marks a failed state read, the only failure the
// guard tolerates; check and update failures always end it.
type phasedReleaseFetchError struct{ err error }

func (e phasedReleaseFetchError) Error() string { return e.err.Error() }
func (e phasedReleaseFetchError) Unwrap() error { return e.err }

func (g *phasedReleaseGuard) tolerateFetchError(ctx context.Context, err error) bool {
	_, isFetch := errors.AsType[phasedReleaseFetchError](err)
	return isFetch && asc.IsTransientWaitError(ctx, err)
}

func (g *phasedReleaseGuard) fetch(ctx context.Context) (*asc.AppStoreVersionPhasedReleaseResponse, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return g.client.GetAppStoreVersionPhasedRelease(requestCtx, g.result.VersionID)
}

func (g *phasedReleaseGuard) step(ctx context.Context, resp *asc.AppStoreVersionPhasedReleaseResponse) (bool, error) {
	if stop, err := g.stopUnlessActive(resp); stop || err != nil {
		return true, err
	}

	day := g.result.CurrentDayNumber
	number := len(g.result.Checks) + 1
	env := []string{
		"ASC_PHASED_RELEASE_VERSION_ID=" + g.result.VersionID,
		"ASC_PHASED_RELEASE_ID=" + g.result.PhasedReleaseID,
		"ASC_PHASED_RELEASE_STATE=" + string(g.result.PhasedReleaseState),
		"ASC_PHASED_RELEASE_DAY=" + strconv.Itoa(day),
		"ASC_PHASED_RELEASE_CHECK=" + strconv.Itoa(number),
	}
	ran, err := phasedReleaseGuardRunCheck(ctx, g.checkTimeout, g.command, env)
	if err != nil {
		return true, err
	}
	check := asc.PhasedReleaseGuardCheck{CurrentDayNumber: day, ExitCode: ran.ExitCode, TimedOut: ran.TimedOut, Action: "none"}

	var target asc.PhasedReleaseState
	switch {
	case ran.TimedOut:
		fmt.Fprintf(os.Stderr, "phased-release guard: check %d timed out after %s on day %d\n", number, g.checkTimeout, day)
		target = asc.PhasedReleaseStatePaused
	case ran.ExitCode != 0:
		fmt.Fprintf(os.Stderr, "phased-release guard: check %d failed with exit status %d on day %d\n", number, ran.ExitCode, day)
		target = asc.PhasedReleaseStatePaused
	default:
		fmt.Fprintf(os.Stderr, "phased-release guard: check %d passed on day %d\n", number, day)
		if g.completeOnDay > 0 && day >= g.completeOnDay {
			target = asc.PhasedReleaseStateComplete
		}
	}
	if target == "" {
		g.result.Checks = append(g.result.Checks, check)
		return false, nil
	}

	action, outcome := "pause", "paused"
	if target == asc.PhasedReleaseStateComplete {
		action, outcome = "complete", "completed"
	}
	if g.result.DryRun {
		action = "would-" + action
		outcome = action
	} else {
		current, err := g.fetch(ctx)
		if err != nil {
			return true, fmt.Errorf("recheck phased release before %s: %w", action, err)
		}
		if stop, err := g.stopUnlessActive(current); stop || err != nil {
			g.result.Checks = append(g.result.Checks, check)
			return true, err
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		updated, err := g.client.UpdateAppStoreVersionPhasedRelease(requestCtx, g.result.PhasedReleaseID, target)
		cancel()
		if err != nil {
			return true, fmt.Errorf("set phased release %s: %w", target, err)
		}
		g.observe(updated)
	}
	check.Action = action
	g.result.Checks = append(g.result.Checks, check)
	g.result.Outcome = outcome
	return true, nil
}

// stopUnlessActive records the observed release and reports whether the guard
// must stop because the release is no longer ACTIVE.
func (g *phasedReleaseGuard) stopUnlessActive(resp *asc.AppStoreVersionPhasedReleaseResponse) (bool, error) {
	g.observe(resp)
	switch g.result.PhasedReleaseState {
	case asc.PhasedReleaseStateActive:
		return false, nil
	case asc.PhasedReleaseStateComplete:
		g.result.Outcome = "already-complete"
	case asc.PhasedReleaseStatePaused:
		g.result.Outcome = "already-paused"
	case asc.PhasedReleaseStateInactive:
		g.result.Outcome = "inactive"
	default:
		return true, fmt.Errorf("unexpected phased release state %q", g.result.PhasedReleaseState)
	}
	return true, nil
}

func (g *phasedReleaseGuard) observe(resp *asc.AppStoreVersionPhasedReleaseResponse) {
	g.result.PhasedReleaseID = resp.Data.ID
	g.result.PhasedReleaseState = resp.Data.Attributes.PhasedReleaseState
	g.result.CurrentDayNumber = resp.Data.Attributes.CurrentDayNumber
}

func runPhasedReleaseCheck(ctx context.Context, timeout time.Duration, argv, env []string) (phasedReleaseCheckResult, error) {
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = phasedReleaseCheckWaitDelay
	configurePhasedReleaseCheckProcess(cmd)
	err := cmd.Run()
	if err == nil {
		return phasedReleaseCheckResult{}, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return phasedReleaseCheckResult{}, ctxErr
	}
	if errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
		return phasedReleaseCheckResult{ExitCode: -1, TimedOut: true}, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return phasedReleaseCheckResult{ExitCode: exitErr.ExitCode()}, nil
	}
	return phasedReleaseCheckResult{}, fmt.Errorf("run check command: %w", err)
}
