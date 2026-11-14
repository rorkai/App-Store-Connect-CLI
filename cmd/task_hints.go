package cmd

import (
	"fmt"
	"io"
)

// taskHint is one curated entry of a command group's "Common tasks" map.
type taskHint struct {
	task    string
	command string
}

// unknownChildTaskHints maps a fully qualified command group to the handful of
// tasks callers most often reach for there. The nearest-match suggester already
// recovers typos; these hints answer the other failure mode, where a caller
// guesses a verb the group never had (`asc builds latest`, `asc builds status`)
// and the bare unknown-command error offers nothing to act on.
//
// Every command must be a copy-paste valid invocation of a real leaf command in
// the group, using long-form flags that command defines. Tests in
// task_hints_test.go resolve each entry against the live command tree, so keep
// the table in sync with the commands themselves rather than with memory.
//
// Prefer the canonical first-class command for a task over a generic equivalent
// that happens to produce the same answer: `asc builds info --app X --latest`
// rather than a sorted single-result list, and `asc builds next-build-number`
// rather than arithmetic on a listing.
//
// Placeholders use bare uppercase words (APP_ID, IPA_PATH) to match the nearest
// -match suggester rendered just above them on the same stderr surface. Angle
// brackets would be read as redirections by a POSIX shell, so a pasted hint
// would fail before `asc` ever ran.
var unknownChildTaskHints = map[string][]taskHint{
	"asc apps": {
		{task: "list apps", command: "asc apps list"},
		{task: "find by bundle ID", command: "asc apps list --bundle-id BUNDLE_ID"},
		{task: "view one app", command: "asc apps view --id APP_ID"},
		{task: "view app metadata", command: "asc apps info view --app APP_ID"},
	},
	"asc auth": {
		{task: "check credentials", command: "asc auth status"},
		{task: "diagnose problems", command: "asc auth doctor"},
		{task: "switch profile", command: "asc auth switch --name PROFILE"},
		{
			task:    "store an API key",
			command: "asc auth login --name NAME --key-id KEY_ID --issuer-id ISSUER_ID --private-key KEY_PATH",
		},
	},
	"asc builds": {
		{task: "list builds", command: "asc builds list --app APP_ID"},
		{task: "latest build", command: "asc builds info --app APP_ID --latest"},
		{task: "next build number", command: "asc builds next-build-number --app APP_ID"},
		{task: "upload a build", command: "asc builds upload --app APP_ID --ipa IPA_PATH"},
		{task: "wait for processing", command: "asc builds wait --app APP_ID --latest"},
	},
	"asc iap": {
		{task: "list purchases", command: "asc iap list --app APP_ID"},
		{task: "view a purchase", command: "asc iap view --id IAP_ID"},
		{task: "list versions", command: "asc iap versions list --iap-id IAP_ID"},
		{task: "pricing summary", command: "asc iap pricing summary --app APP_ID"},
	},
	"asc review": {
		{task: "review status", command: "asc review status --app APP_ID"},
		{task: "explain blockers", command: "asc review doctor --app APP_ID"},
		{
			task:    "submit for review",
			command: "asc review submit --app APP_ID --version VERSION --build-id BUILD_ID --confirm",
		},
		{task: "past submissions", command: "asc review history --app APP_ID"},
	},
	"asc subscriptions": {
		{task: "list groups", command: "asc subscriptions groups list --app APP_ID"},
		{task: "list subscriptions", command: "asc subscriptions list --app APP_ID"},
		{task: "view a subscription", command: "asc subscriptions view --id SUB_ID"},
		{task: "pricing summary", command: "asc subscriptions pricing summary --app APP_ID"},
	},
	"asc testflight": {
		{task: "list beta groups", command: "asc testflight groups list --app APP_ID"},
		{task: "list testers", command: "asc testflight testers list --app APP_ID"},
		{task: "read feedback", command: "asc testflight feedback list --app APP_ID"},
		{task: "notify testers", command: "asc testflight notifications send --build-id BUILD_ID"},
	},
	"asc testflight groups": {
		{task: "list groups", command: "asc testflight groups list --app APP_ID"},
		{task: "view a group", command: "asc testflight groups view --id GROUP_ID"},
		{task: "create a group", command: "asc testflight groups create --app APP_ID --name NAME"},
		{
			task:    "add testers",
			command: "asc testflight groups add-testers --group GROUP_ID --email EMAIL",
		},
	},
	"asc versions": {
		{task: "list versions", command: "asc versions list --app APP_ID"},
		{task: "view a version", command: "asc versions view --version-id VERSION_ID"},
		{task: "create a version", command: "asc versions create --app APP_ID --version VERSION"},
		{
			task:    "attach a build",
			command: "asc versions attach-build --version-id VERSION_ID --build-id BUILD_ID",
		},
		{task: "release a version", command: "asc versions release --version-id VERSION_ID --confirm"},
	},
}

// printUnknownChildTaskHints writes the curated task map for a command group,
// or nothing when the group has no curated entries. Callers print it after the
// error line and before the help pointer, so the error stays the first thing on
// stderr and the exit code is unaffected.
func printUnknownChildTaskHints(stderr io.Writer, commandName string) {
	hints := unknownChildTaskHints[commandName]
	if len(hints) == 0 {
		return
	}

	width := 0
	for _, hint := range hints {
		if len(hint.task) > width {
			width = len(hint.task)
		}
	}

	fmt.Fprintln(stderr, "Common tasks:")
	for _, hint := range hints {
		fmt.Fprintf(stderr, "  %-*s  %s\n", width, hint.task, hint.command)
	}
}
