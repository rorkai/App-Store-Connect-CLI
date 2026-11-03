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
	// IDType is the API resource type named by the ID flag. When set, the flag
	// also accepts that resource's self-link and rejects links of other types.
	IDType string

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

	id := bindBuilderIDFlag(fs, idFlagName, config.IDType, idUsage)
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
	// ParentType is the API resource type named by the parent flag. When set,
	// the flag also accepts that resource's self-link.
	ParentType string

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

	parentID := bindBuilderIDFlag(fs, parentFlagName, config.ParentType, parentUsage)
	limit := fs.Int("limit", 0, fmt.Sprintf("Maximum results per page (1-%d)", limitMax))
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := BindOutputFlags(fs)

	timeout := config.ContextTimeout
	if timeout == nil {
		timeout = ContextWithTimeout
	}

	cmd := &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
	}

	usageErrorPrefix := func() string {
		if prefix := commandErrorPrefixFromUsage(cmd.ShortUsage); prefix != "" {
			return prefix
		}
		return config.ErrorPrefix
	}

	cmd.Exec = func(ctx context.Context, args []string) error {
		if *limit != 0 && (*limit < 1 || *limit > limitMax) {
			return UsageErrorf("%s: --limit must be between 1 and %d", usageErrorPrefix(), limitMax)
		}
		if err := ValidateNextURL(*next); err != nil {
			return UsageErrorf("%s: %v", usageErrorPrefix(), err)
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
			firstPageLimit := *limit
			if firstPageLimit == 0 {
				firstPageLimit = limitMax
			}

			resp, err := PaginateWithSpinner(
				requestCtx,
				func(ctx context.Context) (asc.PaginatedResponse, error) {
					return config.FetchPage(ctx, client, resolvedParentID, firstPageLimit, *next)
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
	}

	return cmd
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
	ParentType  string
	LimitMax    int

	ErrorPrefix string

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	// FetchPage receives the --include values for a first page and nil for a
	// links.next page, which already carries them. The client ignores the
	// sparse fields on a links.next page for the same reason.
	FetchPage func(context.Context, *asc.Client, string, int, string, []string, asc.IncludedFields) (asc.PaginatedResponse, error)
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

	var include *string
	var includedFields *IncludedFieldFlags
	var included asc.IncludedFields
	cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
		FlagSetName: config.FlagSetName,
		Name:        config.Name,
		ShortUsage:  baseUsage + " [--limit N] [--next URL] [--paginate]",
		ShortHelp:   fmt.Sprintf("List equalized price points for %s.", subject),
		LongHelp: fmt.Sprintf(`List equalized price points for %s.

Use --include territory to return each price point's territory and currency.

Examples:
  %s
  %s --include territory
  %s --limit 175
  %s --paginate
  %s --next "NEXT_URL"`, subject, baseExample, baseExample, baseExample, baseExample, commandPath),
		ParentFlag:     config.ParentFlag,
		ParentUsage:    config.ParentUsage,
		ParentType:     config.ParentType,
		LimitMax:       config.LimitMax,
		ErrorPrefix:    config.ErrorPrefix,
		ContextTimeout: config.ContextTimeout,
		FetchPage: func(ctx context.Context, client *asc.Client, parentID string, limit int, next string) (asc.PaginatedResponse, error) {
			var values []string
			if strings.TrimSpace(next) == "" {
				values = SplitCSV(*include)
			}
			return config.FetchPage(ctx, client, parentID, limit, next, values, included)
		},
	})
	include = cmd.FlagSet.String("include", "", "Relationships to include (comma-separated): territory")
	includedFields = BindIncludedFieldFlags(cmd.FlagSet, map[string]string{"territory": "territories"})
	exec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		for _, value := range SplitCSV(*include) {
			if value != "territory" {
				return UsageError("--include must be: territory")
			}
		}
		prefix := commandErrorPrefixFromUsage(cmd.ShortUsage)
		if prefix == "" {
			prefix = config.ErrorPrefix
		}
		next := cmd.FlagSet.Lookup("next").Value.String()
		if err := RejectNextFlagConflicts(cmd.FlagSet, next, prefix, "include"); err != nil {
			return err
		}
		var err error
		if included, err = includedFields.Resolve(prefix, next, SplitCSV(*include)); err != nil {
			return err
		}
		return exec(ctx, args)
	}
	return cmd
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
	// IDType is the API resource type named by the ID flag; see
	// IDGetCommandConfig.IDType.
	IDType string

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

	id := bindBuilderIDFlag(fs, idFlagName, config.IDType, idUsage)
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

// bindBuilderIDFlag registers a builder's ID flag, accepting the resource's
// self-link when the builder config names its resource type.
func bindBuilderIDFlag(fs *flag.FlagSet, name, resourceType, usage string) *string {
	if strings.TrimSpace(resourceType) == "" {
		return fs.String(name, "", usage)
	}
	return BindResourceIDFlag(fs, name, resourceType, usage)
}
