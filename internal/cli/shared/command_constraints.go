package shared

import (
	"flag"
	"maps"
	"slices"
)

// CommandConstraints describes declared validation, not every runtime check.
type CommandConstraints struct {
	Coverage     string           `json:"coverage"`
	Rules        []ConstraintRule `json:"rules"`
	DynamicRules []string         `json:"dynamicRules,omitempty"`
}

// ConstraintRule describes a condition on canonical flag values or presence.
type ConstraintRule struct {
	Kind            string            `json:"kind"`
	Flags           []string          `json:"flags,omitempty"`
	ProvidedFlags   []string          `json:"providedFlags,omitempty"`
	WhenNonEmpty    string            `json:"whenNonEmpty,omitempty"`
	WhenTrue        string            `json:"whenTrue,omitempty"`
	Values          []string          `json:"values,omitempty"`
	Aliases         map[string]string `json:"aliases,omitempty"`
	CaseInsensitive bool              `json:"caseInsensitive,omitempty"`
	TrimSpace       bool              `json:"trimSpace,omitempty"`
	EmptyValue      string            `json:"emptyValue,omitempty"`
	Minimum         *int              `json:"minimum,omitempty"`
	Maximum         *int              `json:"maximum,omitempty"`
	Unit            string            `json:"unit,omitempty"`
}

func setCommandConstraints(fs *flag.FlagSet, constraints CommandConstraints) {
	if f := fs.Lookup("output"); f != nil {
		if output, ok := f.Value.(*validatedOutputValue); ok {
			output.constraints = &constraints
		}
	}
}

// CommandConstraintsFor returns only source-declared rules. Custom validators
// remain unclassified, so coverage is never complete.
func CommandConstraintsFor(fs *flag.FlagSet) CommandConstraints {
	result := CommandConstraints{Coverage: "unknown", Rules: []ConstraintRule{}}
	for _, f := range VisibleHelpFlags(fs) {
		output, ok := f.Value.(*validatedOutputValue)
		if !ok {
			continue
		}
		if declared := output.constraints; declared != nil {
			result.DynamicRules = append(result.DynamicRules, declared.DynamicRules...)
			for _, rule := range declared.Rules {
				rule.Flags = slices.Clone(rule.Flags)
				rule.ProvidedFlags = slices.Clone(rule.ProvidedFlags)
				rule.Values = slices.Clone(rule.Values)
				rule.Aliases = maps.Clone(rule.Aliases)
				if rule.Minimum != nil {
					rule.Minimum = new(*rule.Minimum)
				}
				if rule.Maximum != nil {
					rule.Maximum = new(*rule.Maximum)
				}
				result.Rules = append(result.Rules, rule)
			}
		}
		values := canonicalOutputFormats(output.allowed)
		rule := ConstraintRule{Kind: "values", Flags: []string{"--" + f.Name}, Values: values, CaseInsensitive: true, TrimSpace: true, EmptyValue: "json"}
		if slices.Contains(values, "markdown") {
			rule.Aliases = map[string]string{"md": "markdown"}
		}
		result.Rules = append(result.Rules, rule)
		if fs.Lookup("pretty") != nil {
			result.Rules = append(result.Rules, ConstraintRule{Kind: "requiresValue", Flags: []string{"--" + f.Name}, Values: []string{"json"}, WhenTrue: "--pretty"})
		}
	}
	if len(result.Rules) != 0 || len(result.DynamicRules) != 0 {
		result.Coverage = "partial"
	}
	return result
}
