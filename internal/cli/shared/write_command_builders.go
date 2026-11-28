package shared

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// WriteCommandConfig configures a create or update command that sends one
// request built from string flags.
type WriteCommandConfig struct {
	// FlagSetName names the flag set, which parse errors show. It defaults
	// to Name.
	FlagSetName string
	Name        string
	ShortUsage  string
	ShortHelp   string
	LongHelp    string

	// ErrorPrefix wraps the client error and the request error. It defaults
	// to the command path in ShortUsage without "asc ".
	ErrorPrefix string

	// IDFlag names the required ID flag: the parent resource of a create,
	// the resource itself of an update. It defaults to "id".
	IDFlag  string
	IDUsage string
	// IDType is the API resource type named by the ID flag; see
	// IDGetCommandConfig.IDType.
	IDType string
	// TopLevel creates a resource that has no parent: no ID flag is
	// registered, and Send receives an empty ID.
	TopLevel bool

	// Fields are the request's flags, checked in this order after the ID.
	Fields []AttributeFlag

	// Deps supplies the client, streams and timeout.
	Deps *Deps

	// Send makes the request for the ID flag's value once every field has
	// been written to its target.
	Send func(context.Context, *asc.Client, string) (any, error)
}

// AttributeFlag is a string flag whose trimmed value a create or update
// command writes to a request field.
type AttributeFlag struct {
	name     string
	usage    string
	required bool
	// explicit fields count as given when they appear on the command line,
	// even blank, instead of when they are not blank.
	explicit bool
	// maxLength, when positive, is the most characters the trimmed value
	// may have.
	maxLength int
	set       func(value string, given bool)
}

// WithMaxLength returns f with a limit of maxLength characters on its trimmed
// value, checked before any request.
func (f AttributeFlag) WithMaxLength(maxLength int) AttributeFlag {
	f.maxLength = maxLength
	return f
}

// RequiredStringFlag binds a string flag that must not be blank to target.
func RequiredStringFlag(target *string, name, usage string) AttributeFlag {
	return AttributeFlag{name: name, usage: usage, required: true, set: setString(target)}
}

// StringFlag binds a string flag to target, which is blank while the flag is
// blank or not given.
func StringFlag(target *string, name, usage string) AttributeFlag {
	return AttributeFlag{name: name, usage: usage, set: setString(target)}
}

// OptionalStringFlag binds a string flag to target, which stays nil while the
// flag is blank or not given.
func OptionalStringFlag(target **string, name, usage string) AttributeFlag {
	return AttributeFlag{name: name, usage: usage, set: setGivenString(target)}
}

// ExplicitStringFlag binds a string flag to target, which stays nil unless the
// flag appears on the command line, even with a blank value.
func ExplicitStringFlag(target **string, name, usage string) AttributeFlag {
	return AttributeFlag{name: name, usage: usage, explicit: true, set: setGivenString(target)}
}

func setString(target *string) func(string, bool) {
	return func(value string, _ bool) {
		*target = value
	}
}

func setGivenString(target **string) func(string, bool) {
	return func(value string, given bool) {
		*target = nil
		if given {
			*target = &value
		}
	}
}

// BuildCreateCommand builds a create command that checks the ID flag and the
// required fields, sends one request and prints its response.
func BuildCreateCommand(config WriteCommandConfig) *ffcli.Command {
	return buildWriteCommand(config, "create")
}

// BuildUpdateCommand builds an update command like BuildCreateCommand that
// also requires at least one of its fields.
func BuildUpdateCommand(config WriteCommandConfig) *ffcli.Command {
	return buildWriteCommand(config, "update")
}

func buildWriteCommand(config WriteCommandConfig, verb string) *ffcli.Command {
	d := config.Deps
	fs := d.NewFlagSet(builderDefault(config.FlagSetName, config.Name), flag.ExitOnError)

	idFlagName := builderDefault(config.IDFlag, "id")
	var id *string
	constraints := CommandConstraints{}
	required := ConstraintRule{Kind: "required"}
	update := ConstraintRule{Kind: "anyOf"}
	if !config.TopLevel {
		id = bindBuilderIDFlag(fs, idFlagName, config.IDType, config.IDUsage)
		required.Flags = append(required.Flags, "--"+idFlagName)
	}
	values := make([]*string, len(config.Fields))
	updateFlags := make([]any, len(config.Fields))
	for i, field := range config.Fields {
		values[i] = fs.String(field.name, "", field.usage)
		updateFlags[i] = values[i]
		name := "--" + field.name
		if field.required {
			required.Flags = append(required.Flags, name)
		}
		if field.explicit {
			update.ProvidedFlags = append(update.ProvidedFlags, name)
		} else {
			update.Flags = append(update.Flags, name)
		}
		if field.maxLength > 0 {
			constraints.Rules = append(constraints.Rules, ConstraintRule{Kind: "maxLength", Flags: []string{name}, Maximum: new(field.maxLength), Unit: "unicodeCodePoints", TrimSpace: true})
		}
	}
	if len(required.Flags) != 0 {
		constraints.Rules = append(constraints.Rules, required)
	}
	if verb == "update" {
		constraints.Rules = append(constraints.Rules, update)
	}
	output := d.BindOutputFlags(fs)
	setCommandConstraints(fs, constraints)
	errorPrefix := builderDefault(config.ErrorPrefix, commandErrorPrefixFromUsage(config.ShortUsage))

	return &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := ""
			if !config.TopLevel {
				idValue = strings.TrimSpace(*id)
				if idValue == "" {
					return d.RequiredFlag("--" + idFlagName)
				}
			}
			onCommandLine := map[string]bool{}
			fs.Visit(func(f *flag.Flag) { onCommandLine[f.Name] = true })
			given := false
			for i, field := range config.Fields {
				value := strings.TrimSpace(*values[i])
				if value == "" && field.required {
					return d.RequiredFlag("--" + field.name)
				}
				if length := utf8.RuneCountInString(value); field.maxLength > 0 && length > field.maxLength {
					return d.UsageErrorf("--%s must be at most %d characters (got %d)", field.name, field.maxLength, length)
				}
				fieldGiven := value != ""
				if field.explicit {
					fieldGiven = onCommandLine[field.name]
				}
				field.set(value, fieldGiven)
				given = given || fieldGiven
			}
			if verb == "update" && !given {
				return d.UpdateFlagRequired(fs, updateFlags...)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := config.Send(requestCtx, client, idValue)
			if err != nil {
				return fmt.Errorf("%s: failed to %s: %w", errorPrefix, verb, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
