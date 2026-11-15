package builds

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	buildWaitPendingStatus     = "pending"
	buildWaitPhaseDiscovery    = "discovery"
	buildWaitPhaseProcessing   = "processing"
	buildWaitReportPendingHint = "add --report-pending to print this state on stdout and exit 7"
)

// buildsWaitTimeout turns an expired builds wait --timeout into either the
// default failure, which now carries the last known state and a resume
// command, or, with --report-pending, a pending result on stdout that exits
// with the pending code.
type buildsWaitTimeout struct {
	d             *shared.Deps
	fs            *flag.FlagSet
	started       time.Time
	timeout       time.Duration
	reportPending bool
	output        shared.OutputFlags
	selector      appBuildWaitSelector
	sinceValue    string
	observed      buildWaitObservation
}

// discoveryTimeout reports a wait that never found a build for its selector.
func (t *buildsWaitTimeout) discoveryTimeout() error {
	prefix := fmt.Sprintf("builds wait: timed out resolving build selector after %s", t.timeout.Round(time.Second))

	upload := t.observed.upload
	// A failed upload never becomes a build, so the wait cannot be resumed.
	if failure := shared.BuildUploadFailureError(upload); failure != nil {
		return fmt.Errorf("%s; %w", prefix, failure)
	}

	result := t.newResult(buildWaitPhaseDiscovery)
	result.AppID = t.selector.AppID
	result.BuildNumber = t.selector.BuildNumber
	state := "no build or build upload matching the selector is visible yet"
	if upload != nil {
		result.Upload = buildWaitPendingUpload(upload)
		state = describeBuildWaitPendingUpload(result.Upload)
	}

	selectorFlags := []buildsWaitResumeFlag{{name: "app", value: t.selector.AppID}}
	if t.selector.Latest {
		selectorFlags = append(selectorFlags, buildsWaitResumeFlag{name: "latest", boolean: true})
	} else {
		selectorFlags = append(selectorFlags, buildsWaitResumeFlag{name: "build-number", value: t.selector.BuildNumber})
	}
	selectorFlags = appendOptionalResumeFlag(selectorFlags, "version", t.selector.Version)
	selectorFlags = appendOptionalResumeFlag(selectorFlags, "platform", t.selector.Platform)
	selectorFlags = appendOptionalResumeFlag(selectorFlags, "since", t.sinceValue)

	return t.finish(prefix, state, result, selectorFlags)
}

// processingTimeout reports a wait whose build had not finished processing.
// The resume command selects the build by ID so a newer upload cannot replace
// it.
func (t *buildsWaitTimeout) processingTimeout(buildID string) error {
	prefix := fmt.Sprintf("builds wait: timed out waiting for build %s after %s", buildID, t.timeout.Round(time.Second))

	result := t.newResult(buildWaitPhaseProcessing)
	result.AppID = t.selector.AppID
	result.BuildID = strings.TrimSpace(buildID)
	result.BuildNumber = t.selector.BuildNumber
	if build := t.observed.build; build != nil {
		if number := strings.TrimSpace(build.Data.Attributes.Version); number != "" {
			result.BuildNumber = number
		}
		result.ProcessingState = strings.ToUpper(strings.TrimSpace(build.Data.Attributes.ProcessingState))
	}

	state := fmt.Sprintf("build %q has not reported a processing state yet", shared.SanitizeTerminal(result.BuildID))
	if result.ProcessingState != "" {
		state = fmt.Sprintf(
			"build %q is %s; processing has not finished",
			shared.SanitizeTerminal(result.BuildID),
			shared.SanitizeTerminal(result.ProcessingState),
		)
	}

	return t.finish(prefix, state, result, []buildsWaitResumeFlag{{name: "build-id", value: result.BuildID}})
}

func (t *buildsWaitTimeout) newResult(phase string) *asc.BuildWaitPendingResult {
	return &asc.BuildWaitPendingResult{
		Status:   buildWaitPendingStatus,
		Phase:    phase,
		Version:  t.selector.Version,
		Platform: t.selector.Platform,
		Elapsed:  t.d.Now().Sub(t.started).Round(time.Second).String(),
		Timeout:  t.timeout.String(),
	}
}

func (t *buildsWaitTimeout) finish(prefix, state string, result *asc.BuildWaitPendingResult, selectorFlags []buildsWaitResumeFlag) error {
	result.Summary = buildWaitPendingSentence(state)
	resume, resumable := t.resumeCommand(selectorFlags)
	if resumable {
		result.ResumeCommand = resume
	}

	if !t.reportPending {
		message := prefix + "; " + state
		if resumable {
			message += "; resume with: " + resume
		}
		return shared.NewValidationError(errors.New(message + "; " + buildWaitReportPendingHint))
	}

	format, err := shared.ValidateOutputFormat(*t.output.Output, *t.output.Pretty)
	if err != nil {
		return err
	}
	if err := t.d.PrintOutput(result, format, *t.output.Pretty); err != nil {
		return err
	}

	notice := "Build is still pending after " + result.Elapsed
	if resumable {
		notice += "; resume with: " + resume
	}
	fmt.Fprintln(t.d.IO.ErrOut, notice)
	return shared.NewPendingError("builds wait: " + notice)
}

// buildsWaitResumeFlag is one selector flag of a resume command.
type buildsWaitResumeFlag struct {
	name    string
	value   string
	boolean bool
}

func appendOptionalResumeFlag(flags []buildsWaitResumeFlag, name, value string) []buildsWaitResumeFlag {
	if strings.TrimSpace(value) == "" {
		return flags
	}
	return append(flags, buildsWaitResumeFlag{name: name, value: value})
}

// buildsWaitResumePassthroughFlags are the flags a resume command repeats when
// the caller set them explicitly. Selector flags are rebuilt from the resolved
// selector instead.
var buildsWaitResumePassthroughFlags = map[string]bool{
	"fail-on-invalid": true,
	"output":          true,
	"poll-interval":   true,
	"pretty":          true,
	"report-pending":  true,
	"timeout":         true,
}

// resumeCommand renders the invocation that continues this wait, keeping the
// explicitly set root flags and wait options. ok is false when a value cannot
// be rendered as one copyable shell argument, so no approximation is printed.
func (t *buildsWaitTimeout) resumeCommand(selectorFlags []buildsWaitResumeFlag) (string, bool) {
	rootFlags, ok := shared.RootFlagsForReinvocation()
	if !ok {
		return "", false
	}
	parts := []string{"asc"}
	parts = append(parts, rootFlags...)
	parts = append(parts, "builds", "wait")

	appendValue := func(name, value string) bool {
		quoted, quotable := shared.ShellQuote(value)
		if !quotable {
			return false
		}
		parts = append(parts, "--"+name, quoted)
		return true
	}

	for _, selectorFlag := range selectorFlags {
		if selectorFlag.boolean {
			parts = append(parts, "--"+selectorFlag.name)
			continue
		}
		if !appendValue(selectorFlag.name, selectorFlag.value) {
			return "", false
		}
	}

	if t.fs != nil {
		// flag.Visit walks only explicitly set flags, in lexical order, so the
		// rendered command is deterministic.
		t.fs.Visit(func(f *flag.Flag) {
			if !ok || !buildsWaitResumePassthroughFlags[f.Name] {
				return
			}
			if boolFlag, isBool := f.Value.(interface{ IsBoolFlag() bool }); isBool && boolFlag.IsBoolFlag() {
				if f.Value.String() == "true" {
					parts = append(parts, "--"+f.Name)
				}
				return
			}
			ok = appendValue(f.Name, f.Value.String())
		})
	}
	if !ok {
		return "", false
	}
	return strings.Join(parts, " "), true
}

func buildWaitPendingUpload(upload *asc.BuildUploadResponse) *asc.BuildWaitPendingUpload {
	attributes := upload.Data.Attributes
	result := &asc.BuildWaitPendingUpload{
		ID:          strings.TrimSpace(upload.Data.ID),
		Version:     strings.TrimSpace(attributes.CFBundleShortVersionString),
		BuildNumber: strings.TrimSpace(attributes.CFBundleVersion),
		Platform:    strings.TrimSpace(string(attributes.Platform)),
	}
	if attributes.State != nil && attributes.State.State != nil {
		result.State = strings.ToUpper(strings.TrimSpace(*attributes.State.State))
	}
	if attributes.UploadedDate != nil {
		result.UploadedDate = strings.TrimSpace(*attributes.UploadedDate)
	}
	return result
}

func describeBuildWaitPendingUpload(upload *asc.BuildWaitPendingUpload) string {
	details := make([]string, 0, 3)
	if upload.Version != "" {
		details = append(details, "version "+upload.Version)
	}
	if upload.BuildNumber != "" {
		details = append(details, "build "+upload.BuildNumber)
	}
	if upload.Platform != "" {
		details = append(details, upload.Platform)
	}
	identity := ""
	if len(details) > 0 {
		identity = " (" + shared.SanitizeTerminal(strings.Join(details, ", ")) + ")"
	}
	state := upload.State
	if state == "" {
		state = "UNKNOWN"
	}
	return fmt.Sprintf(
		"build upload %q%s is %s and not yet visible as a build",
		shared.SanitizeTerminal(upload.ID),
		identity,
		shared.SanitizeTerminal(state),
	)
}

// buildWaitPendingSentence turns a lower-case state clause into the summary
// sentence of the pending result.
func buildWaitPendingSentence(clause string) string {
	first, size := utf8.DecodeRuneInString(clause)
	if first == utf8.RuneError {
		return clause
	}
	return string(unicode.ToUpper(first)) + clause[size:] + "."
}
