package shared

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// IDGetCommandConfig configures a standard "get by ID" command.
type IDGetCommandConfig struct {
	FlagSetName string
	Name        string
	ShortUsage  string
	ShortHelp   string
	LongHelp    string

	IDFlag  string
	IDUsage string

	ErrorPrefix string

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	Fetch          func(context.Context, *asc.Client, string) (any, error)
}

// BuildIDGetCommand builds a standard "get by ID" command.
func BuildIDGetCommand(config IDGetCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	idFlagName := strings.TrimSpace(config.IDFlag)
	if idFlagName == "" {
		idFlagName = "id"
	}
	idUsage := strings.TrimSpace(config.IDUsage)
	if idUsage == "" {
		idUsage = "Resource ID"
	}

	id := fs.String(idFlagName, "", idUsage)
	output := BindOutputFlags(fs)

	timeout := config.ContextTimeout
	if timeout == nil {
		timeout = ContextWithTimeout
	}

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
				return UsageErrorf("--%s is required", idFlagName)
			}

			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := timeout(ctx)
			defer cancel()

			resp, err := config.Fetch(requestCtx, client, idValue)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			return PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PaginatedListCommandConfig configures a standard list command with
// --limit/--next/--paginate and a required parent resource ID.
type PaginatedListCommandConfig struct {
	FlagSetName string
	Name        string
	ShortUsage  string
	ShortHelp   string
	LongHelp    string

	ParentFlag  string
	ParentUsage string

	LimitMax int

	ErrorPrefix string

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	FetchPage      func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error)
}

// BuildPaginatedListCommand builds a list command that supports --next and
// --paginate semantics shared by many resources.
func BuildPaginatedListCommand(config PaginatedListCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	parentFlagName := strings.TrimSpace(config.ParentFlag)
	if parentFlagName == "" {
		parentFlagName = "id"
	}
	parentUsage := strings.TrimSpace(config.ParentUsage)
	if parentUsage == "" {
		parentUsage = "Parent resource ID"
	}
	limitMax := config.LimitMax
	if limitMax <= 0 {
		limitMax = 200
	}

	parentID := fs.String(parentFlagName, "", parentUsage)
	limit := fs.Int("limit", 0, fmt.Sprintf("Maximum results per page (1-%d)", limitMax))
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := BindOutputFlags(fs)

	timeout := config.ContextTimeout
	if timeout == nil {
		timeout = ContextWithTimeout
	}

	return &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > limitMax) {
				return fmt.Errorf("%s: --limit must be between 1 and %d", config.ErrorPrefix, limitMax)
			}
			if err := ValidateNextURL(*next); err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			resolvedParentID := strings.TrimSpace(*parentID)
			if resolvedParentID == "" && strings.TrimSpace(*next) == "" {
				return UsageErrorf("--%s is required", parentFlagName)
			}

			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := timeout(ctx)
			defer cancel()

			if *paginate {
				resp, err := PaginateWithSpinner(requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return config.FetchPage(ctx, client, resolvedParentID, limitMax, *next)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return config.FetchPage(ctx, client, resolvedParentID, 0, nextURL)
					},
				)
				if err != nil {
					return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
				}

				return PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := config.FetchPage(requestCtx, client, resolvedParentID, *limit, *next)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			return PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PricePointEqualizationsCommandConfig configures a standard equalizations list
// command for app, IAP, and subscription price points.
type PricePointEqualizationsCommandConfig struct {
	FlagSetName string
	Name        string

	ShortUsage  string
	BaseExample string
	Subject     string

	ParentFlag  string
	ParentUsage string
	LimitMax    int

	ErrorPrefix string

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	FetchPage      func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error)
}

// BuildPricePointEqualizationsCommand builds a standard equalizations list
// command with shared help/examples and --limit/--next/--paginate semantics.
func BuildPricePointEqualizationsCommand(config PricePointEqualizationsCommandConfig) *ffcli.Command {
	baseUsage := strings.TrimSpace(config.ShortUsage)
	if baseUsage == "" {
		baseUsage = "asc equalizations"
	}

	baseExample := strings.TrimSpace(config.BaseExample)
	if baseExample == "" {
		baseExample = baseUsage
	}

	commandPath := commandPathFromUsage(baseUsage)
	if commandPath == "" {
		commandPath = baseUsage
	}

	subject := strings.TrimSpace(config.Subject)
	if subject == "" {
		subject = "a price point"
	}

	return BuildPaginatedListCommand(PaginatedListCommandConfig{
		FlagSetName: config.FlagSetName,
		Name:        config.Name,
		ShortUsage:  baseUsage + " [--limit N] [--next URL] [--paginate]",
		ShortHelp:   fmt.Sprintf("List equalized price points for %s.", subject),
		LongHelp: fmt.Sprintf(`List equalized price points for %s.

Examples:
  %s
  %s --limit 175
  %s --paginate
  %s --next "NEXT_URL"`, subject, baseExample, baseExample, baseExample, commandPath),
		ParentFlag:     config.ParentFlag,
		ParentUsage:    config.ParentUsage,
		LimitMax:       config.LimitMax,
		ErrorPrefix:    config.ErrorPrefix,
		ContextTimeout: config.ContextTimeout,
		FetchPage:      config.FetchPage,
	})
}

// ConfirmDeleteCommandConfig configures a standard delete command requiring
// --id and --confirm.
type ConfirmDeleteCommandConfig struct {
	FlagSetName string
	Name        string
	ShortUsage  string
	ShortHelp   string
	LongHelp    string

	IDFlag  string
	IDUsage string

	ErrorPrefix string

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	Delete         func(context.Context, *asc.Client, string) error
	Result         func(string) any
}

// BuildConfirmDeleteCommand builds a standard delete command requiring --id and
// --confirm and printing a caller-provided result payload.
func BuildConfirmDeleteCommand(config ConfirmDeleteCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	idFlagName := strings.TrimSpace(config.IDFlag)
	if idFlagName == "" {
		idFlagName = "id"
	}
	idUsage := strings.TrimSpace(config.IDUsage)
	if idUsage == "" {
		idUsage = "Resource ID"
	}

	id := fs.String(idFlagName, "", idUsage)
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := BindOutputFlags(fs)

	timeout := config.ContextTimeout
	if timeout == nil {
		timeout = ContextWithTimeout
	}

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
				return UsageErrorf("--%s is required", idFlagName)
			}
			if !*confirm {
				return UsageError("--confirm is required")
			}

			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := timeout(ctx)
			defer cancel()

			if err := config.Delete(requestCtx, client, idValue); err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			result := config.Result(idValue)
			return PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
