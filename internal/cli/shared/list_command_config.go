package shared

import "github.com/peterbourgon/ff/v3/ffcli"

// ListCommandConfig describes common help/output behavior for list-style CLI commands.
type ListCommandConfig struct {
	Name              string
	ShortUsage        string
	ShortHelp         string
	LongHelp          string
	ErrorPrefix       string
	DeprecatedWarning string
	UsageFunc         func(*ffcli.Command) string
}
