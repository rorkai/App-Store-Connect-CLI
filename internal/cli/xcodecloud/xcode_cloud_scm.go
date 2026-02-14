package xcodecloud

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

func xcodeCloudScmListFlags(fs *flag.FlagSet) (limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	outputFlags := shared.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

func xcodeCloudScmRepoListFlags(fs *flag.FlagSet) (repoID *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	repoID = fs.String("repo-id", "", "SCM repository ID")
	limit, next, paginate, output, pretty = xcodeCloudScmListFlags(fs)
	return
}

func xcodeCloudScmProviderListFlags(fs *flag.FlagSet) (providerID *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	providerID = fs.String("provider-id", "", "SCM provider ID")
	limit, next, paginate, output, pretty = xcodeCloudScmListFlags(fs)
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	providerID := fs.String("provider-id", "", "SCM provider ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud scm providers get --provider-id \"PROVIDER_ID\"",
		ShortHelp:  "Get an SCM provider by ID.",
		LongHelp: `Get an SCM provider by ID.

Examples:
  asc xcode-cloud scm providers get --provider-id "PROVIDER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*providerID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --provider-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm providers get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetScmProvider(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm providers get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudScmProvidersRepositoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("repositories", flag.ExitOnError)

	providerID, limit, next, paginate, output, pretty := xcodeCloudScmProviderListFlags(fs)

	return &ffcli.Command{
		Name:       "repositories",
		ShortUsage: "asc xcode-cloud scm providers repositories --provider-id \"PROVIDER_ID\" [flags]",
		ShortHelp:  "List repositories for an SCM provider.",
		LongHelp: `List repositories for an SCM provider.

Examples:
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID"
  asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud scm providers repositories: --limit must be between 1 and 200")
			}
			nextURL := strings.TrimSpace(*next)
			if err := shared.ValidateNextURL(nextURL); err != nil {
				return fmt.Errorf("xcode-cloud scm providers repositories: %w", err)
			}

			idValue := strings.TrimSpace(*providerID)
			if idValue == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --provider-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm providers repositories: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.ScmRepositoriesOption{
				asc.WithScmRepositoriesLimit(*limit),
				asc.WithScmRepositoriesNextURL(nextURL),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithScmRepositoriesLimit(200))
				firstPage, err := client.GetScmProviderRepositories(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud scm providers repositories: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetScmProviderRepositories(ctx, idValue, asc.WithScmRepositoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud scm providers repositories: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetScmProviderRepositories(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm providers repositories: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudScmRepositoriesListCommand(),
			XcodeCloudScmRepositoriesGetCommand(),
			XcodeCloudScmRepositoriesGitReferencesCommand(),
			XcodeCloudScmRepositoriesPullRequestsCommand(),
			XcodeCloudScmRepositoriesRelationshipsCommand(),
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "SCM repository ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud scm repositories get --id \"REPO_ID\"",
		ShortHelp:  "Get an SCM repository by ID.",
		LongHelp: `Get an SCM repository by ID.

Examples:
  asc xcode-cloud scm repositories get --id "REPO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			repo, err := client.GetScmRepository(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories get: %w", err)
			}

			resp := &asc.ScmRepositoriesResponse{Data: []asc.ScmRepositoryResource{*repo}}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudScmRepositoriesGitReferencesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("git-references", flag.ExitOnError)

	repoID, limit, next, paginate, output, pretty := xcodeCloudScmRepoListFlags(fs)

	return &ffcli.Command{
		Name:       "git-references",
		ShortUsage: "asc xcode-cloud scm repositories git-references --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List git references for a repository.",
		LongHelp: `List git references for a repository.

Examples:
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories git-references --repo-id "REPO_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud scm repositories git-references: --limit must be between 1 and 200")
			}
			nextURL := strings.TrimSpace(*next)
			if err := shared.ValidateNextURL(nextURL); err != nil {
				return fmt.Errorf("xcode-cloud scm repositories git-references: %w", err)
			}

			idValue := strings.TrimSpace(*repoID)
			if idValue == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --repo-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories git-references: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.ScmGitReferencesOption{
				asc.WithScmGitReferencesLimit(*limit),
				asc.WithScmGitReferencesNextURL(nextURL),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithScmGitReferencesLimit(200))
				firstPage, err := client.GetScmGitReferences(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories git-references: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetScmGitReferences(ctx, idValue, asc.WithScmGitReferencesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories git-references: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetScmGitReferences(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories git-references: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

func XcodeCloudScmRepositoriesPullRequestsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pull-requests", flag.ExitOnError)

	repoID, limit, next, paginate, output, pretty := xcodeCloudScmRepoListFlags(fs)

	return &ffcli.Command{
		Name:       "pull-requests",
		ShortUsage: "asc xcode-cloud scm repositories pull-requests --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List pull requests for a repository.",
		LongHelp: `List pull requests for a repository.

Examples:
  asc xcode-cloud scm repositories pull-requests --repo-id "REPO_ID"
  asc xcode-cloud scm repositories pull-requests --repo-id "REPO_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud scm repositories pull-requests: --limit must be between 1 and 200")
			}
			nextURL := strings.TrimSpace(*next)
			if err := shared.ValidateNextURL(nextURL); err != nil {
				return fmt.Errorf("xcode-cloud scm repositories pull-requests: %w", err)
			}

			idValue := strings.TrimSpace(*repoID)
			if idValue == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --repo-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories pull-requests: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.ScmPullRequestsOption{
				asc.WithScmPullRequestsLimit(*limit),
				asc.WithScmPullRequestsNextURL(nextURL),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithScmPullRequestsLimit(200))
				firstPage, err := client.GetScmRepositoryPullRequests(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories pull-requests: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetScmRepositoryPullRequests(ctx, idValue, asc.WithScmPullRequestsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories pull-requests: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetScmRepositoryPullRequests(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories pull-requests: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

func XcodeCloudScmRepositoriesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc xcode-cloud scm repositories relationships <git-references|pull-requests> [flags]",
		ShortHelp:  "List SCM repository relationship linkages.",
		LongHelp: `List SCM repository relationship linkages.

Examples:
  asc xcode-cloud scm repositories relationships git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories relationships pull-requests --repo-id "REPO_ID"`,
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
	fs := flag.NewFlagSet("git-references", flag.ExitOnError)

	repoID, limit, next, paginate, output, pretty := xcodeCloudScmRepoListFlags(fs)

	return &ffcli.Command{
		Name:       "git-references",
		ShortUsage: "asc xcode-cloud scm repositories relationships git-references --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List git reference relationship linkages for a repository.",
		LongHelp: `List git reference relationship linkages for a repository.

Examples:
  asc xcode-cloud scm repositories relationships git-references --repo-id "REPO_ID"
  asc xcode-cloud scm repositories relationships git-references --repo-id "REPO_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud scm repositories relationships git-references: --limit must be between 1 and 200")
			}
			nextURL := strings.TrimSpace(*next)
			if err := shared.ValidateNextURL(nextURL); err != nil {
				return fmt.Errorf("xcode-cloud scm repositories relationships git-references: %w", err)
			}

			idValue := strings.TrimSpace(*repoID)
			if idValue == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --repo-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories relationships git-references: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(nextURL),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetScmRepositoryGitReferencesRelationships(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories relationships git-references: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetScmRepositoryGitReferencesRelationships(ctx, idValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories relationships git-references: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetScmRepositoryGitReferencesRelationships(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories relationships git-references: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

func XcodeCloudScmRepositoriesRelationshipsPullRequestsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pull-requests", flag.ExitOnError)

	repoID, limit, next, paginate, output, pretty := xcodeCloudScmRepoListFlags(fs)

	return &ffcli.Command{
		Name:       "pull-requests",
		ShortUsage: "asc xcode-cloud scm repositories relationships pull-requests --repo-id \"REPO_ID\" [flags]",
		ShortHelp:  "List pull request relationship linkages for a repository.",
		LongHelp: `List pull request relationship linkages for a repository.

Examples:
  asc xcode-cloud scm repositories relationships pull-requests --repo-id "REPO_ID"
  asc xcode-cloud scm repositories relationships pull-requests --repo-id "REPO_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud scm repositories relationships pull-requests: --limit must be between 1 and 200")
			}
			nextURL := strings.TrimSpace(*next)
			if err := shared.ValidateNextURL(nextURL); err != nil {
				return fmt.Errorf("xcode-cloud scm repositories relationships pull-requests: %w", err)
			}

			idValue := strings.TrimSpace(*repoID)
			if idValue == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --repo-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories relationships pull-requests: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(nextURL),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetScmRepositoryPullRequestsRelationships(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories relationships pull-requests: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetScmRepositoryPullRequestsRelationships(ctx, idValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud scm repositories relationships pull-requests: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetScmRepositoryPullRequestsRelationships(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm repositories relationships pull-requests: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "SCM git reference ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud scm git-references get --id \"REF_ID\"",
		ShortHelp:  "Get an SCM git reference by ID.",
		LongHelp: `Get an SCM git reference by ID.

Examples:
  asc xcode-cloud scm git-references get --id "REF_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm git-references get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetScmGitReference(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm git-references get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "SCM pull request ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud scm pull-requests get --id \"PR_ID\"",
		ShortHelp:  "Get an SCM pull request by ID.",
		LongHelp: `Get an SCM pull request by ID.

Examples:
  asc xcode-cloud scm pull-requests get --id "PR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud scm pull-requests get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetScmPullRequest(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud scm pull-requests get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func xcodeCloudScmProvidersList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud scm providers: --limit must be between 1 and 200")
	}
	nextURL := strings.TrimSpace(next)
	if err := shared.ValidateNextURL(nextURL); err != nil {
		return fmt.Errorf("xcode-cloud scm providers: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud scm providers: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	opts := []asc.ScmProvidersOption{
		asc.WithScmProvidersLimit(limit),
		asc.WithScmProvidersNextURL(nextURL),
	}

	if paginate {
		paginateOpts := append(opts, asc.WithScmProvidersLimit(200))
		firstPage, err := client.GetScmProviders(requestCtx, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud scm providers: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetScmProviders(ctx, asc.WithScmProvidersNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("xcode-cloud scm providers: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetScmProviders(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud scm providers: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
}

func xcodeCloudScmRepositoriesList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud scm repositories: --limit must be between 1 and 200")
	}
	nextURL := strings.TrimSpace(next)
	if err := shared.ValidateNextURL(nextURL); err != nil {
		return fmt.Errorf("xcode-cloud scm repositories: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud scm repositories: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	opts := []asc.ScmRepositoriesOption{
		asc.WithScmRepositoriesLimit(limit),
		asc.WithScmRepositoriesNextURL(nextURL),
	}

	if paginate {
		paginateOpts := append(opts, asc.WithScmRepositoriesLimit(200))
		firstPage, err := client.GetScmRepositories(requestCtx, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud scm repositories: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetScmRepositories(ctx, asc.WithScmRepositoriesNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("xcode-cloud scm repositories: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetScmRepositories(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud scm repositories: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
}
