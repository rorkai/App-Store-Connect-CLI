package xcodecloud

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

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
	cmd := xcodeCloudScmProvidersListCommand(
		"providers",
		"asc xcode-cloud scm providers [flags]",
		"Manage SCM providers.",
		`Manage SCM providers.

Examples:
  asc xcode-cloud scm providers list
  asc xcode-cloud scm providers view --provider-id "PROVIDER_ID"
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID"`,
	)
	cmd.Subcommands = []*ffcli.Command{
		XcodeCloudScmProvidersListCommand(),
		XcodeCloudScmProvidersGetCommand(),
		XcodeCloudScmProvidersRepositoriesCommand(),
	}
	return cmd
}

func XcodeCloudScmProvidersListCommand() *ffcli.Command {
	return xcodeCloudScmProvidersListCommand(
		"list",
		"asc xcode-cloud scm providers list [flags]",
		"List SCM providers.",
		`List SCM providers.

Examples:
  asc xcode-cloud scm providers list
  asc xcode-cloud scm providers list --paginate`,
	)
}

func xcodeCloudScmProvidersListCommand(name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud scm providers",
		FetchErrorPrefix:     "xcode-cloud scm providers",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetScmProviders(ctx, asc.Limit[asc.ScmProvidersOption](limit), asc.NextURL[asc.ScmProvidersOption](next))
		},
	})
}

func XcodeCloudScmProvidersGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud scm providers view --provider-id \"PROVIDER_ID\"",
		ShortHelp:  "View an SCM provider by ID.",
		LongHelp: `View an SCM provider by ID.

Examples:
  asc xcode-cloud scm providers view --provider-id "PROVIDER_ID"`,
		IDFlag:           "provider-id",
		IDUsage:          "SCM provider ID",
		IDType:           "scmProviders",
		FetchErrorPrefix: "xcode-cloud scm providers view",
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
		Name:       "repositories",
		ShortUsage: "asc xcode-cloud scm providers repositories --provider-id \"PROVIDER_ID\" [flags]",
		ShortHelp:  "List repositories for an SCM provider.",
		LongHelp: `List repositories for an SCM provider.

Examples:
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID"
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID" --paginate`,
		ParentFlag:       "provider-id",
		ParentUsage:      "SCM provider ID",
		ParentType:       "scmProviders",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud scm providers repositories",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, providerID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.ScmRepositoriesOption{
				asc.Limit[asc.ScmRepositoriesOption](limit),
				asc.NextURL[asc.ScmRepositoriesOption](next),
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
	cmd := xcodeCloudScmRepositoriesListCommand(
		"repositories",
		"asc xcode-cloud scm repositories [flags]",
		"Manage SCM repositories.",
		`Manage SCM repositories.

Examples:
  asc xcode-cloud scm repositories list
  asc xcode-cloud scm repositories view --id "REPO_ID"
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"`,
	)
	cmd.UsageFunc = shared.VisibleUsageFunc
	cmd.Subcommands = []*ffcli.Command{
		XcodeCloudScmRepositoriesListCommand(),
		XcodeCloudScmRepositoriesGetCommand(),
		XcodeCloudScmRepositoriesGitReferencesCommand(),
		XcodeCloudScmRepositoriesPullRequestsCommand(),
		XcodeCloudScmRepositoriesRelationshipsCommand(),
	}
	return cmd
}

func XcodeCloudScmRepositoriesListCommand() *ffcli.Command {
	return xcodeCloudScmRepositoriesListCommand(
		"list",
		"asc xcode-cloud scm repositories list [flags]",
		"List SCM repositories.",
		`List SCM repositories.

Examples:
  asc xcode-cloud scm repositories list
  asc xcode-cloud scm repositories list --paginate`,
	)
}

func xcodeCloudScmRepositoriesListCommand(name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud scm repositories",
		FetchErrorPrefix:     "xcode-cloud scm repositories",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetScmRepositories(ctx, asc.Limit[asc.ScmRepositoriesOption](limit), asc.NextURL[asc.ScmRepositoriesOption](next))
		},
	})
}

func XcodeCloudScmRepositoriesGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud scm repositories view --id \"REPO_ID\"",
		ShortHelp:  "View an SCM repository by ID.",
		LongHelp: `View an SCM repository by ID.

Examples:
  asc xcode-cloud scm repositories view --id "REPO_ID"`,
		IDFlag:           "id",
		IDUsage:          "SCM repository ID",
		IDType:           "scmRepositories",
		FetchErrorPrefix: "xcode-cloud scm repositories view",
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
		Name:       "git-references",
		ShortUsage: "asc xcode-cloud scm repositories git-references --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List git references for a repository.",
		LongHelp: `List git references for a repository.

Examples:
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID" --paginate`,
		ParentFlag:       "repo-id",
		ParentUsage:      "SCM repository ID",
		ParentType:       "scmRepositories",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud scm repositories git-references",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.ScmGitReferencesOption{
				asc.Limit[asc.ScmGitReferencesOption](limit),
				asc.NextURL[asc.ScmGitReferencesOption](next),
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
		Name:       "pull-requests",
		ShortUsage: "asc xcode-cloud scm repositories pull-requests --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List pull requests for a repository.",
		LongHelp: `List pull requests for a repository.

Examples:
  asc xcode-cloud scm repositories pull-requests --repo-id "REPO_ID"
  asc xcode-cloud scm repositories pull-requests --repo-id "REPO_ID" --paginate`,
		ParentFlag:       "repo-id",
		ParentUsage:      "SCM repository ID",
		ParentType:       "scmRepositories",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud scm repositories pull-requests",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.ScmPullRequestsOption{
				asc.Limit[asc.ScmPullRequestsOption](limit),
				asc.NextURL[asc.ScmPullRequestsOption](next),
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
		Name:       "git-references",
		ShortUsage: "asc xcode-cloud scm repositories links git-references --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List git reference relationship linkages for a repository.",
		LongHelp: `List git reference relationship linkages for a repository.

Examples:
  asc xcode-cloud scm repositories links git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories links git-references --repo-id "REPO_ID" --paginate`,
		ParentFlag:       "repo-id",
		ParentUsage:      "SCM repository ID",
		ParentType:       "scmRepositories",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud scm repositories links git-references",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](limit),
				asc.NextURL[asc.LinkagesOption](next),
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
		Name:       "pull-requests",
		ShortUsage: "asc xcode-cloud scm repositories links pull-requests --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List pull request relationship linkages for a repository.",
		LongHelp: `List pull request relationship linkages for a repository.

Examples:
  asc xcode-cloud scm repositories links pull-requests --repo-id "REPO_ID"
  asc xcode-cloud scm repositories links pull-requests --repo-id "REPO_ID" --paginate`,
		ParentFlag:       "repo-id",
		ParentUsage:      "SCM repository ID",
		ParentType:       "scmRepositories",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud scm repositories links pull-requests",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, repoID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](limit),
				asc.NextURL[asc.LinkagesOption](next),
			}
			resp, err := client.GetScmRepositoryPullRequestsRelationships(ctx, repoID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
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
  asc xcode-cloud scm git-references view --id "REF_ID"`,
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
		Name:       "view",
		ShortUsage: "asc xcode-cloud scm git-references view --id \"REF_ID\"",
		ShortHelp:  "View an SCM git reference by ID.",
		LongHelp: `View an SCM git reference by ID.

Examples:
  asc xcode-cloud scm git-references view --id "REF_ID"`,
		IDFlag:           "id",
		IDUsage:          "SCM git reference ID",
		IDType:           "scmGitReferences",
		FetchErrorPrefix: "xcode-cloud scm git-references view",
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
  asc xcode-cloud scm pull-requests view --id "PR_ID"`,
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
		Name:       "view",
		ShortUsage: "asc xcode-cloud scm pull-requests view --id \"PR_ID\"",
		ShortHelp:  "View an SCM pull request by ID.",
		LongHelp: `View an SCM pull request by ID.

Examples:
  asc xcode-cloud scm pull-requests view --id "PR_ID"`,
		IDFlag:           "id",
		IDUsage:          "SCM pull request ID",
		IDType:           "scmPullRequests",
		FetchErrorPrefix: "xcode-cloud scm pull-requests view",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetScmPullRequest(ctx, id)
		},
	})
}
