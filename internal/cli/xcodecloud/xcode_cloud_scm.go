package xcodecloud

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func xcodeCloudScmListFlags(fs *flag.FlagSet) (limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	outputFlags := shared.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

// XcodeCloudScmCommand returns the SCM command group for Xcode Cloud.
func XcodeCloudScmCommand() *ffcli.Command {
	fs := flag.NewFlagSet("scm", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "scm",
		ShortUsage: "asc xcode-cloud scm <subcommand> [flags]",
		ShortHelp:  "Manage Xcode Cloud SCM providers and repositories.",
		LongHelp: `Manage Xcode Cloud SCM providers and repositories.

Examples:
  asc xcode-cloud scm providers list
  asc xcode-cloud scm repositories list
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudScmProvidersCommand(),
			XcodeCloudScmRepositoriesCommand(),
			XcodeCloudScmGitReferencesCommand(),
			XcodeCloudScmPullRequestsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeCloudScmProvidersCommand returns the SCM providers command group.
func XcodeCloudScmProvidersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("providers", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudScmListFlags(fs)

	return &ffcli.Command{
		Name:       "providers",
		ShortUsage: "asc xcode-cloud scm providers [flags]",
		ShortHelp:  "Manage SCM providers.",
		LongHelp: `Manage SCM providers.

Examples:
  asc xcode-cloud scm providers list
  asc xcode-cloud scm providers get --provider-id "PROVIDER_ID"
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudScmProvidersListCommand(),
			XcodeCloudScmProvidersGetCommand(),
			XcodeCloudScmProvidersRepositoriesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudScmProvidersList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudScmProvidersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudScmListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud scm providers list [flags]",
		ShortHelp:  "List SCM providers.",
		LongHelp: `List SCM providers.

Examples:
  asc xcode-cloud scm providers list
  asc xcode-cloud scm providers list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudScmProvidersList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudScmProvidersGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud scm providers get --provider-id \"PROVIDER_ID\"",
		ShortHelp:   "Get an SCM provider by ID.",
		LongHelp: `Get an SCM provider by ID.

Examples:
  asc xcode-cloud scm providers get --provider-id "PROVIDER_ID"`,
		IDFlag:      "provider-id",
		IDUsage:     "SCM provider ID",
		ErrorPrefix: "xcode-cloud scm providers get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetScmProvider(ctx, id)
		},
	})
}

func XcodeCloudScmProvidersRepositoriesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "repositories",
		Name:        "repositories",
		ShortUsage:  "asc xcode-cloud scm providers repositories --provider-id \"PROVIDER_ID\" [flags]",
		ShortHelp:   "List repositories for an SCM provider.",
		LongHelp: `List repositories for an SCM provider.

Examples:
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID"
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID" --paginate`,
		ParentFlag:  "provider-id",
		ParentUsage: "SCM provider ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud scm providers repositories",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, providerID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.ScmRepositoriesOption{
				asc.WithScmRepositoriesLimit(limit),
				asc.WithScmRepositoriesNextURL(next),
			}
			resp, err := client.GetScmProviderRepositories(ctx, providerID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

// XcodeCloudScmRepositoriesCommand returns the SCM repositories command group.
func XcodeCloudScmRepositoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("repositories", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudScmListFlags(fs)

	return &ffcli.Command{
		Name:       "repositories",
		ShortUsage: "asc xcode-cloud scm repositories [flags]",
		ShortHelp:  "Manage SCM repositories.",
		LongHelp: `Manage SCM repositories.

Examples:
  asc xcode-cloud scm repositories list
  asc xcode-cloud scm repositories get --id "REPO_ID"
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudScmRepositoriesListCommand(),
			XcodeCloudScmRepositoriesGetCommand(),
			XcodeCloudScmRepositoriesGitReferencesCommand(),
			XcodeCloudScmRepositoriesPullRequestsCommand(),
			XcodeCloudScmRepositoriesRelationshipsCommand(),
			DeprecatedXcodeCloudScmRepositoriesRelationshipsAliasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudScmRepositoriesList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudScmRepositoriesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudScmListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud scm repositories list [flags]",
		ShortHelp:  "List SCM repositories.",
		LongHelp: `List SCM repositories.

Examples:
  asc xcode-cloud scm repositories list
  asc xcode-cloud scm repositories list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudScmRepositoriesList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudScmRepositoriesGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud scm repositories get --id \"REPO_ID\"",
		ShortHelp:   "Get an SCM repository by ID.",
		LongHelp: `Get an SCM repository by ID.

Examples:
  asc xcode-cloud scm repositories get --id "REPO_ID"`,
		IDFlag:      "id",
		IDUsage:     "SCM repository ID",
		ErrorPrefix: "xcode-cloud scm repositories get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			repo, err := client.GetScmRepository(ctx, id)
			if err != nil {
				return nil, err
			}
			return &asc.ScmRepositoriesResponse{Data: []asc.ScmRepositoryResource{*repo}}, nil
		},
	})
}

func XcodeCloudScmRepositoriesGitReferencesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "git-references",
		Name:        "git-references",
		ShortUsage:  "asc xcode-cloud scm repositories git-references --repo-id \"REPO_ID\" [flags]",
		ShortHelp:   "List git references for a repository.",
		LongHelp: `List git references for a repository.

Examples:
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID" --paginate`,
		ParentFlag:  "repo-id",
		ParentUsage: "SCM repository ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud scm repositories git-references",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.ScmGitReferencesOption{
				asc.WithScmGitReferencesLimit(limit),
				asc.WithScmGitReferencesNextURL(next),
			}
			resp, err := client.GetScmGitReferences(ctx, repoID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

func XcodeCloudScmRepositoriesPullRequestsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "pull-requests",
		Name:        "pull-requests",
		ShortUsage:  "asc xcode-cloud scm repositories pull-requests --repo-id \"REPO_ID\" [flags]",
		ShortHelp:   "List pull requests for a repository.",
		LongHelp: `List pull requests for a repository.

Examples:
  asc xcode-cloud scm repositories pull-requests --repo-id "REPO_ID"
  asc xcode-cloud scm repositories pull-requests --repo-id "REPO_ID" --paginate`,
		ParentFlag:  "repo-id",
		ParentUsage: "SCM repository ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud scm repositories pull-requests",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.ScmPullRequestsOption{
				asc.WithScmPullRequestsLimit(limit),
				asc.WithScmPullRequestsNextURL(next),
			}
			resp, err := client.GetScmRepositoryPullRequests(ctx, repoID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

func XcodeCloudScmRepositoriesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("links", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc xcode-cloud scm repositories links <git-references|pull-requests> [flags]",
		ShortHelp:  "List SCM repository relationship linkages.",
		LongHelp: `List SCM repository relationship linkages.

Examples:
  asc xcode-cloud scm repositories links git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories links pull-requests --repo-id "REPO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudScmRepositoriesRelationshipsGitReferencesCommand(),
			XcodeCloudScmRepositoriesRelationshipsPullRequestsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func XcodeCloudScmRepositoriesRelationshipsGitReferencesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "git-references",
		Name:        "git-references",
		ShortUsage:  "asc xcode-cloud scm repositories links git-references --repo-id \"REPO_ID\" [flags]",
		ShortHelp:   "List git reference relationship linkages for a repository.",
		LongHelp: `List git reference relationship linkages for a repository.

Examples:
  asc xcode-cloud scm repositories links git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories links git-references --repo-id "REPO_ID" --paginate`,
		ParentFlag:  "repo-id",
		ParentUsage: "SCM repository ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud scm repositories links git-references",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(limit),
				asc.WithLinkagesNextURL(next),
			}
			resp, err := client.GetScmRepositoryGitReferencesRelationships(ctx, repoID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

func XcodeCloudScmRepositoriesRelationshipsPullRequestsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "pull-requests",
		Name:        "pull-requests",
		ShortUsage:  "asc xcode-cloud scm repositories links pull-requests --repo-id \"REPO_ID\" [flags]",
		ShortHelp:   "List pull request relationship linkages for a repository.",
		LongHelp: `List pull request relationship linkages for a repository.

Examples:
  asc xcode-cloud scm repositories links pull-requests --repo-id "REPO_ID"
  asc xcode-cloud scm repositories links pull-requests --repo-id "REPO_ID" --paginate`,
		ParentFlag:  "repo-id",
		ParentUsage: "SCM repository ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud scm repositories links pull-requests",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(limit),
				asc.WithLinkagesNextURL(next),
			}
			resp, err := client.GetScmRepositoryPullRequestsRelationships(ctx, repoID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

func DeprecatedXcodeCloudScmRepositoriesRelationshipsAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc xcode-cloud scm repositories links <git-references|pull-requests> [flags]",
		ShortHelp:  "DEPRECATED: use `asc xcode-cloud scm repositories links ...`.",
		LongHelp:   "Deprecated compatibility alias for `asc xcode-cloud scm repositories links ...`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				XcodeCloudScmRepositoriesRelationshipsGitReferencesCommand(),
				"git-references",
				"asc xcode-cloud scm repositories links git-references --repo-id \"REPO_ID\" [flags]",
				"asc xcode-cloud scm repositories links git-references",
				"Warning: `asc xcode-cloud scm repositories relationships git-references` is deprecated. Use `asc xcode-cloud scm repositories links git-references`.",
			),
			shared.DeprecatedAliasLeafCommand(
				XcodeCloudScmRepositoriesRelationshipsPullRequestsCommand(),
				"pull-requests",
				"asc xcode-cloud scm repositories links pull-requests --repo-id \"REPO_ID\" [flags]",
				"asc xcode-cloud scm repositories links pull-requests",
				"Warning: `asc xcode-cloud scm repositories relationships pull-requests` is deprecated. Use `asc xcode-cloud scm repositories links pull-requests`.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeCloudScmGitReferencesCommand returns the SCM git references command group.
func XcodeCloudScmGitReferencesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("git-references", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "git-references",
		ShortUsage: "asc xcode-cloud scm git-references <subcommand> [flags]",
		ShortHelp:  "Manage SCM git references.",
		LongHelp: `Manage SCM git references.

Examples:
  asc xcode-cloud scm git-references get --id "REF_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudScmGitReferencesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func XcodeCloudScmGitReferencesGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud scm git-references get --id \"REF_ID\"",
		ShortHelp:   "Get an SCM git reference by ID.",
		LongHelp: `Get an SCM git reference by ID.

Examples:
  asc xcode-cloud scm git-references get --id "REF_ID"`,
		IDFlag:      "id",
		IDUsage:     "SCM git reference ID",
		ErrorPrefix: "xcode-cloud scm git-references get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetScmGitReference(ctx, id)
		},
	})
}

// XcodeCloudScmPullRequestsCommand returns the SCM pull requests command group.
func XcodeCloudScmPullRequestsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pull-requests", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pull-requests",
		ShortUsage: "asc xcode-cloud scm pull-requests <subcommand> [flags]",
		ShortHelp:  "Manage SCM pull requests.",
		LongHelp: `Manage SCM pull requests.

Examples:
  asc xcode-cloud scm pull-requests get --id "PR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudScmPullRequestsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func XcodeCloudScmPullRequestsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud scm pull-requests get --id \"PR_ID\"",
		ShortHelp:   "Get an SCM pull request by ID.",
		LongHelp: `Get an SCM pull request by ID.

Examples:
  asc xcode-cloud scm pull-requests get --id "PR_ID"`,
		IDFlag:      "id",
		IDUsage:     "SCM pull request ID",
		ErrorPrefix: "xcode-cloud scm pull-requests get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetScmPullRequest(ctx, id)
		},
	})
}

func xcodeCloudScmProvidersList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	return runXcodeCloudPaginatedList(
		ctx,
		limit,
		next,
		paginate,
		output,
		pretty,
		"xcode-cloud scm providers",
		func(ctx context.Context, client *asc.Client, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetScmProviders(
				ctx,
				asc.WithScmProvidersLimit(limit),
				asc.WithScmProvidersNextURL(next),
			)
		},
		func(ctx context.Context, client *asc.Client, next string) (asc.PaginatedResponse, error) {
			return client.GetScmProviders(ctx, asc.WithScmProvidersNextURL(next))
		},
	)
}

func xcodeCloudScmRepositoriesList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	return runXcodeCloudPaginatedList(
		ctx,
		limit,
		next,
		paginate,
		output,
		pretty,
		"xcode-cloud scm repositories",
		func(ctx context.Context, client *asc.Client, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetScmRepositories(
				ctx,
				asc.WithScmRepositoriesLimit(limit),
				asc.WithScmRepositoriesNextURL(next),
			)
		},
		func(ctx context.Context, client *asc.Client, next string) (asc.PaginatedResponse, error) {
			return client.GetScmRepositories(ctx, asc.WithScmRepositoriesNextURL(next))
		},
	)
}
