package offercodes

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ActiveUpdateCommandConfig describes a command that sets a resource's active attribute.
type ActiveUpdateCommandConfig struct {
	FlagSetName string
	Name        string
	ShortUsage  string
	ShortHelp   string
	LongHelp    string
	IDFlag      string
	IDUsage     string
	IDType      string
	ErrorPrefix string
	// ConfirmOff names what --active false turns off permanently; when set,
	// --active false requires --confirm.
	ConfirmOff string
	Update     func(context.Context, *asc.Client, string, *bool) (any, error)
	// Deps is what the command is built on. It defaults to shared.DefaultDeps().
	Deps *shared.Deps
}

// NewActiveUpdateCommand builds a command that updates a resource's active attribute.
func NewActiveUpdateCommand(config ActiveUpdateCommandConfig) *ffcli.Command {
	d := config.Deps
	if d == nil {
		d = shared.DefaultDeps()
	}
	fs := d.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, config.IDFlag, config.IDType, config.IDUsage)
	active := fs.String("active", "", "Set active (true/false)")
	var confirm *bool
	if config.ConfirmOff != "" {
		confirm = fs.Bool("confirm", false, "Confirm --active false, which is permanent")
	}
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --%s is required\n", config.IDFlag)
				return shared.MissingRequiredUsageError("--" + config.IDFlag)
			}

			activeValue, err := shared.ParseOptionalBoolFlag("--active", *active)
			if err != nil {
				return d.UsageErrorfCtx(ctx, "%s: %v", config.ErrorPrefix, err)
			}
			if activeValue == nil {
				return d.RequiredFlag("--active")
			}
			if confirm != nil && !*activeValue && !*confirm {
				return d.UsageErrorf("--confirm is required to turn off %s; turning it off is permanent", config.ConfirmOff)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := config.Update(requestCtx, client, trimmedID, activeValue)
			if err != nil {
				return fmt.Errorf("%s: failed to update: %w", config.ErrorPrefix, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
