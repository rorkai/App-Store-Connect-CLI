package shared

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// WriteCommandConfig configures a create or update command that sends one
// request built from string flags.
type WriteCommandConfig struct {
	Name       string
	ShortUsage string
	ShortHelp  string
	LongHelp   string

	// IDFlag names the required ID flag: the parent resource of a create,
	// the resource itself of an update. It defaults to "id".
	IDFlag  string
	IDUsage string
	// IDType is the API resource type named by the ID flag; see
	// IDGetCommandConfig.IDType.
	IDType string

	// Fields are the request's flags, checked in this order after the ID.
	Fields []AttributeFlag

	// Deps supplies the client, streams and timeout. It defaults to
	// DefaultDeps().
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
	set      func(string)
}

// RequiredStringFlag binds a string flag that must not be blank to target.
func RequiredStringFlag(target *string, name, usage string) AttributeFlag {
	return AttributeFlag{name: name, usage: usage, required: true, set: func(value string) {
		*target = value
	}}
}

// OptionalStringFlag binds a string flag to target, which stays nil while the
// flag is blank or not given.
func OptionalStringFlag(target **string, name, usage string) AttributeFlag {
	return AttributeFlag{name: name, usage: usage, set: func(value string) {
		*target = nil
		if value != "" {
			*target = &value
		}
	}}
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
	d := builderDeps(config.Deps)
	fs := d.NewFlagSet(config.Name, flag.ExitOnError)

	idFlagName := builderDefault(config.IDFlag, "id")
	id := bindBuilderIDFlag(fs, idFlagName, config.IDType, config.IDUsage)
	values := make([]*string, len(config.Fields))
	updateFlags := make([]any, len(config.Fields))
	for i, field := range config.Fields {
		values[i] = fs.String(field.name, "", field.usage)
		updateFlags[i] = values[i]
	}
	output := d.BindOutputFlags(fs)
	errorPrefix := commandErrorPrefixFromUsage(config.ShortUsage)

	return &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--" + idFlagName)
			}
			given := false
			for i, field := range config.Fields {
				value := strings.TrimSpace(*values[i])
				if value == "" && field.required {
					return d.RequiredFlag("--" + field.name)
				}
				field.set(value)
				given = given || value != ""
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
