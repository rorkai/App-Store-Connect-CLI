package offercodes

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type activeUpdateCommandConfig struct {
	FlagSetName string
	Name        string
	ShortUsage  string
	ShortHelp   string
	LongHelp    string
	IDFlag      string
	IDUsage     string
	ErrorPrefix string
	Update      func(context.Context, *asc.Client, string, *bool) (any, error)
}

func newActiveUpdateCommand(config activeUpdateCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	id := fs.String(config.IDFlag, "", config.IDUsage)
	active := fs.String("active", "", "Set active (true/false)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintf(os.Stderr, "Error: --%s is required\n", config.IDFlag)
				return shared.MissingRequiredUsageError()
			}

			activeValue, err := shared.ParseOptionalBoolFlag("--active", *active)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}
			if activeValue == nil {
				fmt.Fprintln(os.Stderr, "Error: --active is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := config.Update(requestCtx, client, trimmedID, activeValue)
			if err != nil {
				return fmt.Errorf("%s: failed to update: %w", config.ErrorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
