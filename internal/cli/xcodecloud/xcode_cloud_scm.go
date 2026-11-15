package xcodecloud

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newXcodeCloudScmCommand returns the SCM command group for Xcode Cloud.
func newXcodeCloudScmCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("scm", flag.ExitOnError)

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
			newXcodeCloudScmProvidersCommand(d),
			newXcodeCloudScmRepositoriesCommand(d),
			newXcodeCloudScmGitReferencesCommand(d),
			newXcodeCloudScmPullRequestsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newXcodeCloudScmProvidersCommand returns the SCM providers command group.
func newXcodeCloudScmProvidersCommand(d *shared.Deps) *ffcli.Command {
	cmd := xcodeCloudScmProvidersListCommand(
		d,
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
		newXcodeCloudScmProvidersListCommand(d),
		newXcodeCloudScmProvidersGetCommand(d),
		newXcodeCloudScmProvidersRepositoriesCommand(d),
	}
	return cmd
}

func newXcodeCloudScmProvidersListCommand(d *shared.Deps) *ffcli.Command {
	return xcodeCloudScmProvidersListCommand(
		d,
		"list",
		"asc xcode-cloud scm providers list [flags]",
		"List SCM providers.",
		`List SCM providers.

Examples:
  asc xcode-cloud scm providers list
  asc xcode-cloud scm providers list --paginate`,
	)
}

func xcodeCloudScmProvidersListCommand(d *shared.Deps, name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud scm providers",
		FetchErrorPrefix:     "xcode-cloud scm providers",
		Deps:                 d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetScmProviders(ctx, asc.Limit[asc.ScmProvidersOption](limit), asc.NextURL[asc.ScmProvidersOption](next))
		},
	})
}

func newXcodeCloudScmProvidersGetCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetScmProvider(ctx, id)
		},
	})
}

func newXcodeCloudScmProvidersRepositoriesCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
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

// newXcodeCloudScmRepositoriesCommand returns the SCM repositories command group.
func newXcodeCloudScmRepositoriesCommand(d *shared.Deps) *ffcli.Command {
	cmd := xcodeCloudScmRepositoriesListCommand(
		d,
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
		newXcodeCloudScmRepositoriesListCommand(d),
		newXcodeCloudScmRepositoriesGetCommand(d),
		newXcodeCloudScmRepositoriesGitReferencesCommand(d),
		newXcodeCloudScmRepositoriesPullRequestsCommand(d),
		newXcodeCloudScmRepositoriesRelationshipsCommand(d),
	}
	return cmd
}

func newXcodeCloudScmRepositoriesListCommand(d *shared.Deps) *ffcli.Command {
	return xcodeCloudScmRepositoriesListCommand(
		d,
		"list",
		"asc xcode-cloud scm repositories list [flags]",
		"List SCM repositories.",
		`List SCM repositories.

Examples:
  asc xcode-cloud scm repositories list
  asc xcode-cloud scm repositories list --paginate`,
	)
}

func xcodeCloudScmRepositoriesListCommand(d *shared.Deps, name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud scm repositories",
		FetchErrorPrefix:     "xcode-cloud scm repositories",
		Deps:                 d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetScmRepositories(ctx, asc.Limit[asc.ScmRepositoriesOption](limit), asc.NextURL[asc.ScmRepositoriesOption](next))
		},
	})
}

func newXcodeCloudScmRepositoriesGetCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
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

func newXcodeCloudScmRepositoriesGitReferencesCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
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

func newXcodeCloudScmRepositoriesPullRequestsCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
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

func newXcodeCloudScmRepositoriesRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("links", flag.ExitOnError)

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
			newXcodeCloudScmRepositoriesRelationshipsGitReferencesCommand(d),
			newXcodeCloudScmRepositoriesRelationshipsPullRequestsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newXcodeCloudScmRepositoriesRelationshipsGitReferencesCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
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

func newXcodeCloudScmRepositoriesRelationshipsPullRequestsCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
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

// newXcodeCloudScmGitReferencesCommand returns the SCM git references command group.
func newXcodeCloudScmGitReferencesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("git-references", flag.ExitOnError)

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
			newXcodeCloudScmGitReferencesGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newXcodeCloudScmGitReferencesGetCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetScmGitReference(ctx, id)
		},
	})
}

// newXcodeCloudScmPullRequestsCommand returns the SCM pull requests command group.
func newXcodeCloudScmPullRequestsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pull-requests", flag.ExitOnError)

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
			newXcodeCloudScmPullRequestsGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newXcodeCloudScmPullRequestsGetCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetScmPullRequest(ctx, id)
		},
	})
}
