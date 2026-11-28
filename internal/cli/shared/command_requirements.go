package shared

import (
	"flag"
	"slices"
)

// CommandRuntimeRequirements declares host and external-tool requirements.
// It does not report whether the current host meets them.
type CommandRuntimeRequirements struct {
	Host         string   `json:"host"`
	Tools        []string `json:"tools"`
	ToolCoverage string   `json:"toolCoverage"`
}

// DeclareCommandRuntimeRequirements attaches metadata to the command's existing
// output binding without retaining the command or reading parsed flag values.
func DeclareCommandRuntimeRequirements(fs *flag.FlagSet, requirements CommandRuntimeRequirements) {
	for _, f := range VisibleHelpFlags(fs) {
		if output, ok := f.Value.(*validatedOutputValue); ok {
			requirements.Tools = append([]string{}, requirements.Tools...)
			output.requirements = &requirements
			return
		}
	}
}

// CommandRuntimeRequirementsFor returns nil when requirements are undeclared.
func CommandRuntimeRequirementsFor(fs *flag.FlagSet) *CommandRuntimeRequirements {
	for _, f := range VisibleHelpFlags(fs) {
		if output, ok := f.Value.(*validatedOutputValue); ok && output.requirements != nil {
			result := *output.requirements
			result.Tools = slices.Clone(result.Tools)
			return &result
		}
	}
	return nil
}
