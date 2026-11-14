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
	// ResolveApp reads the ID flag through ResolveAppID, which falls back to
	// ASC_APP_ID and the config file; a missing app is reported as
	// "--<flag> is required (or set ASC_APP_ID)".
	ResolveApp bool
	// RequiredDiagnostic reports a missing ID flag with the required-input
	// diagnostic instead of a plain usage error.
	RequiredDiagnostic bool

	ErrorPrefix string
	// FetchErrorPrefix wraps errors returned by Fetch. It defaults to
	// ErrorPrefix.
	FetchErrorPrefix string

	// Flags registers command-specific flags and returns their validation,
	// which runs after the ID check. The returned function may be nil.
	Flags func(*flag.FlagSet) func() error

	// Deps supplies the client, streams and timeout. It defaults to
	// DefaultDeps().
	Deps *Deps

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	Fetch          func(context.Context, *asc.Client, string) (any, error)
}

// BuildIDGetCommand builds a standard "get by ID" command.
func BuildIDGetCommand(config IDGetCommandConfig) *ffcli.Command {
	d := builderDeps(config.Deps)
	fs := d.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	idFlagName := strings.TrimSpace(config.IDFlag)
	if idFlagName == "" {
		idFlagName = "id"
	}
	idUsage := strings.TrimSpace(config.IDUsage)
	if idUsage == "" {
		idUsage = "Resource ID"
	}

	id := bindBuilderIDFlag(fs, idFlagName, config.IDType, idUsage)
	output := d.BindOutputFlags(fs)
	validate := bindBuilderFlags(fs, config.Flags)

	timeout := builderTimeout(d, config.ContextTimeout)
	fetchErrorPrefix := builderFetchErrorPrefix(config.FetchErrorPrefix, config.ErrorPrefix)

	return &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := readBuilderFlag(*id, config.ResolveApp)
			if idValue == "" {
				return missingBuilderInput(d, config.RequiredDiagnostic, config.ResolveApp, idFlagName)
			}
			if err := validate(); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := timeout(ctx)
			defer cancel()

			resp, err := config.Fetch(requestCtx, client, idValue)
			if err != nil {
				return fmt.Errorf("%s: %w", fetchErrorPrefix, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PaginatedListCommandConfig configures a standard list command with
// --limit/--next/--paginate and, unless TopLevel is set, a required parent
// resource ID.
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
	// TopLevel lists a collection that has no parent: no parent flag is
	// registered, and FetchPage receives an empty parent ID.
	TopLevel bool
	// ResolveApp reads the parent flag through ResolveAppID, which falls back
	// to ASC_APP_ID and the config file; a missing app is reported as
	// "--<flag> is required (or set ASC_APP_ID)".
	ResolveApp bool
	// LookupParent turns the parent ID into the ID FetchPage receives. It
	// runs after validation, once the client exists, and its error is
	// returned unwrapped. With --next it is skipped and FetchPage receives an
	// empty parent ID.
	LookupParent func(context.Context, *asc.Client, string) (string, error)
	// RequiredDiagnostic reports a missing parent flag with the
	// required-input diagnostic instead of a plain usage error.
	RequiredDiagnostic bool

	LimitMax int
	// PaginateIgnoresLimit makes --paginate request LimitMax items per page
	// even when --limit is set.
	PaginateIgnoresLimit bool
	// NoSpinner makes --paginate fetch without the stderr spinner. The first
	// page's error is then wrapped with FetchErrorPrefix, later pages' errors
	// with ErrorPrefix.
	NoSpinner bool

	// NextConflicts names flags that cannot be combined with --next.
	NextConflicts []string
	// NextBeforeLimit checks --next and its conflicts before --limit.
	NextBeforeLimit bool
	// Flags registers command-specific flags and returns their validation,
	// which runs after the --limit, --next and parent checks. The returned
	// function may be nil.
	Flags func(*flag.FlagSet) func() error

	ErrorPrefix string
	// FetchErrorPrefix wraps the error of a single-page request. It defaults
	// to ErrorPrefix.
	FetchErrorPrefix string

	// Deps supplies the client, streams and timeout. It defaults to
	// DefaultDeps().
	Deps *Deps

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	FetchPage      func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error)
}

// BuildPaginatedListCommand builds a list command that supports --next and
// --paginate semantics shared by many resources.
func BuildPaginatedListCommand(config PaginatedListCommandConfig) *ffcli.Command {
	d := builderDeps(config.Deps)
	fs := d.NewFlagSet(config.FlagSetName, flag.ExitOnError)

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

	parentID := new(string)
	if !config.TopLevel {
		parentID = bindBuilderIDFlag(fs, parentFlagName, config.ParentType, parentUsage)
	}
	limit := fs.Int("limit", 0, fmt.Sprintf("Maximum results per page (1-%d)", limitMax))
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)
	validate := bindBuilderFlags(fs, config.Flags)

	timeout := builderTimeout(d, config.ContextTimeout)
	fetchErrorPrefix := builderFetchErrorPrefix(config.FetchErrorPrefix, config.ErrorPrefix)

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
		checkLimit := func() error {
			if *limit != 0 && (*limit < 1 || *limit > limitMax) {
				return UsageErrorf("%s: --limit must be between 1 and %d", usageErrorPrefix(), limitMax)
			}
			return nil
		}
		if !config.NextBeforeLimit {
			if err := checkLimit(); err != nil {
				return err
			}
		}
		if err := ValidateNextURL(*next); err != nil {
			return UsageErrorf("%s: %v", usageErrorPrefix(), err)
		}
		if err := RejectNextFlagConflicts(fs, *next, usageErrorPrefix(), config.NextConflicts...); err != nil {
			return err
		}
		if err := checkLimit(); err != nil {
			return err
		}

		resolvedParentID := readBuilderFlag(*parentID, config.ResolveApp)
		if !config.TopLevel && resolvedParentID == "" && strings.TrimSpace(*next) == "" {
			return missingBuilderInput(d, config.RequiredDiagnostic, config.ResolveApp, parentFlagName)
		}
		if err := validate(); err != nil {
			return err
		}

		client, err := d.Client()
		if err != nil {
			return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
		}

		requestCtx, cancel := timeout(ctx)
		defer cancel()

		if config.LookupParent != nil {
			lookedUpID := ""
			if strings.TrimSpace(*next) == "" {
				if lookedUpID, err = config.LookupParent(requestCtx, client, resolvedParentID); err != nil {
					return err
				}
			}
			resolvedParentID = lookedUpID
		}

		if *paginate {
			firstPageLimit := *limit
			if firstPageLimit == 0 || config.PaginateIgnoresLimit {
				firstPageLimit = limitMax
			}
			fetchFirst := func(ctx context.Context) (asc.PaginatedResponse, error) {
				return config.FetchPage(ctx, client, resolvedParentID, firstPageLimit, *next)
			}
			fetchNext := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return config.FetchPage(ctx, client, resolvedParentID, 0, nextURL)
			}

			var resp asc.PaginatedResponse
			if config.NoSpinner {
				firstPage, err := fetchFirst(requestCtx)
				if err != nil {
					return fmt.Errorf("%s: %w", fetchErrorPrefix, err)
				}
				resp, err = asc.PaginateAll(requestCtx, firstPage, fetchNext)
				if err != nil {
					return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
				}
			} else {
				resp, err = PaginateWithSpinner(requestCtx, fetchFirst, fetchNext)
				if err != nil {
					return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
				}
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		}

		resp, err := config.FetchPage(requestCtx, client, resolvedParentID, *limit, *next)
		if err != nil {
			return fmt.Errorf("%s: %w", fetchErrorPrefix, err)
		}

		return d.PrintOutput(resp, *output.Output, *output.Pretty)
	}

	return cmd
}

// readBuilderFlag reads a parent or ID flag; with resolveApp the value goes
// through ResolveAppID unchanged, as the hand-written app commands did.
func readBuilderFlag(value string, resolveApp bool) string {
	if resolveApp {
		return ResolveAppID(value)
	}
	return strings.TrimSpace(value)
}

// missingBuilderInput reports a missing parent or ID flag; an app flag names
// its ASC_APP_ID fallback.
func missingBuilderInput(d *Deps, diagnostic, resolveApp bool, name string) error {
	if !resolveApp {
		return missingBuilderFlag(d, diagnostic, name)
	}
	message := fmt.Sprintf("--%s is required (or set ASC_APP_ID)", name)
	if !diagnostic {
		return UsageError(message)
	}
	fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", message)
	return MissingRequiredUsageError("--" + name)
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
	// RequiredDiagnostic reports a missing ID flag or --confirm with the
	// required-input diagnostic instead of a plain usage error.
	RequiredDiagnostic bool

	ErrorPrefix string
	// DeleteErrorPrefix wraps errors returned by Delete. It defaults to
	// ErrorPrefix.
	DeleteErrorPrefix string

	// Deps supplies the client, streams and timeout. It defaults to
	// DefaultDeps().
	Deps *Deps

	ContextTimeout func(context.Context) (context.Context, context.CancelFunc)
	Delete         func(context.Context, *asc.Client, string) error
	Result         func(string) any
}

// BuildConfirmDeleteCommand builds a standard delete command requiring --id and
// --confirm and printing a caller-provided result payload.
func BuildConfirmDeleteCommand(config ConfirmDeleteCommandConfig) *ffcli.Command {
	d := builderDeps(config.Deps)
	fs := d.NewFlagSet(config.FlagSetName, flag.ExitOnError)

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
	output := d.BindOutputFlags(fs)

	timeout := builderTimeout(d, config.ContextTimeout)
	deleteErrorPrefix := builderFetchErrorPrefix(config.DeleteErrorPrefix, config.ErrorPrefix)

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
				return missingBuilderFlag(d, config.RequiredDiagnostic, idFlagName)
			}
			if !*confirm {
				return missingBuilderFlag(d, config.RequiredDiagnostic, "confirm")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := timeout(ctx)
			defer cancel()

			if err := config.Delete(requestCtx, client, idValue); err != nil {
				return fmt.Errorf("%s: %w", deleteErrorPrefix, err)
			}

			result := config.Result(idValue)
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func builderDeps(d *Deps) *Deps {
	if d != nil {
		return d
	}
	return DefaultDeps()
}

func builderTimeout(d *Deps, timeout func(context.Context) (context.Context, context.CancelFunc)) func(context.Context) (context.Context, context.CancelFunc) {
	if timeout != nil {
		return timeout
	}
	return d.Timeout
}

func builderFetchErrorPrefix(fetchErrorPrefix, errorPrefix string) string {
	if fetchErrorPrefix != "" {
		return fetchErrorPrefix
	}
	return errorPrefix
}

func bindBuilderFlags(fs *flag.FlagSet, bind func(*flag.FlagSet) func() error) func() error {
	var validate func() error
	if bind != nil {
		validate = bind(fs)
	}
	if validate == nil {
		return func() error { return nil }
	}
	return validate
}

func missingBuilderFlag(d *Deps, diagnostic bool, name string) error {
	if !diagnostic {
		return UsageErrorf("--%s is required", name)
	}
	fmt.Fprintf(d.IO.ErrOut, "Error: --%s is required\n", name)
	return MissingRequiredUsageError("--" + name)
}

// bindBuilderIDFlag registers a builder's ID flag, accepting the resource's
// self-link when the builder config names its resource type.
func bindBuilderIDFlag(fs *flag.FlagSet, name, resourceType, usage string) *string {
	if strings.TrimSpace(resourceType) == "" {
		return fs.String(name, "", usage)
	}
	return BindResourceIDFlag(fs, name, resourceType, usage)
}
