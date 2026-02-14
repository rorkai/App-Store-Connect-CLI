package appclips

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

// AppClipDefaultExperiencesCommand returns the default experiences command group.
func AppClipDefaultExperiencesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("default-experiences", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "default-experiences",
		ShortUsage: "asc app-clips default-experiences <subcommand> [flags]",
		ShortHelp:  "Manage App Clip default experiences.",
		LongHelp: `Manage App Clip default experiences.

Examples:
  asc app-clips default-experiences list --app-clip-id "CLIP_ID"
  asc app-clips default-experiences create --app-clip-id "CLIP_ID" --action OPEN
  asc app-clips default-experiences update --experience-id "EXP_ID" --action VIEW
  asc app-clips default-experiences delete --experience-id "EXP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipDefaultExperiencesListCommand(),
			AppClipDefaultExperiencesGetCommand(),
			AppClipDefaultExperienceReviewDetailCommand(),
			AppClipDefaultExperienceReleaseWithAppStoreVersionCommand(),
			AppClipDefaultExperienceRelationshipsCommand(),
			AppClipDefaultExperienceHeaderImageCommand(),
			AppClipDefaultExperiencesCreateCommand(),
			AppClipDefaultExperiencesUpdateCommand(),
			AppClipDefaultExperiencesDeleteCommand(),
			AppClipDefaultExperienceLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipDefaultExperiencesListCommand lists default experiences.
func AppClipDefaultExperiencesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appClipID := fs.String("app-clip-id", "", "App Clip ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips default-experiences list --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "List default experiences for an App Clip.",
		LongHelp: `List default experiences for an App Clip.

Examples:
  asc app-clips default-experiences list --app-clip-id "CLIP_ID"
  asc app-clips default-experiences list --app-clip-id "CLIP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips default-experiences list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-clips default-experiences list: %w", err)
			}

			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-clip-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppClipDefaultExperiencesOption{
				asc.WithAppClipDefaultExperiencesLimit(*limit),
				asc.WithAppClipDefaultExperiencesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppClipDefaultExperiencesLimit(200))
				firstPage, err := client.GetAppClipDefaultExperiences(requestCtx, appClipValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.AppClipDefaultExperiencesResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceAttributes]{}}
						return shared.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips default-experiences list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipDefaultExperiences(ctx, appClipValue, asc.WithAppClipDefaultExperiencesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips default-experiences list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipDefaultExperiences(requestCtx, appClipValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipDefaultExperiencesResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceAttributes]{}}
					return shared.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips default-experiences list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperiencesGetCommand gets a default experience by ID.
func AppClipDefaultExperiencesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	include := fs.String("include", "", "Include relationships: "+strings.Join(appClipDefaultExperienceIncludeList(), ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips default-experiences get --experience-id \"EXP_ID\"",
		ShortHelp:  "Get a default experience by ID.",
		LongHelp: `Get a default experience by ID.

Examples:
  asc app-clips default-experiences get --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*experienceID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			includeValue, err := normalizeAppClipDefaultExperienceInclude(*include)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperience(requestCtx, idValue, asc.WithAppClipDefaultExperienceInclude(includeValue))
			if err != nil {
				return fmt.Errorf("app-clips default-experiences get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperiencesCreateCommand creates a default experience.
func AppClipDefaultExperiencesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appClipID := fs.String("app-clip-id", "", "App Clip ID")
	action := fs.String("action", "", "Action (OPEN, VIEW, PLAY)")
	releaseVersionID := fs.String("release-version-id", "", "Release with App Store version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips default-experiences create --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "Create a default experience.",
		LongHelp: `Create a default experience.

Examples:
  asc app-clips default-experiences create --app-clip-id "CLIP_ID" --action OPEN
  asc app-clips default-experiences create --app-clip-id "CLIP_ID" --release-version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-clip-id is required")
				return flag.ErrHelp
			}

			var attrs *asc.AppClipDefaultExperienceCreateAttributes
			if strings.TrimSpace(*action) != "" {
				actionValue, err := normalizeAppClipAction(*action)
				if err != nil {
					return fmt.Errorf("app-clips default-experiences create: %w", err)
				}
				attrs = &asc.AppClipDefaultExperienceCreateAttributes{
					Action: &actionValue,
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppClipDefaultExperience(requestCtx, appClipValue, attrs, *releaseVersionID, "")
			if err != nil {
				return fmt.Errorf("app-clips default-experiences create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperiencesUpdateCommand updates a default experience.
func AppClipDefaultExperiencesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	action := fs.String("action", "", "Action (OPEN, VIEW, PLAY)")
	releaseVersionID := fs.String("release-version-id", "", "Release with App Store version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-clips default-experiences update --experience-id \"EXP_ID\" [flags]",
		ShortHelp:  "Update a default experience.",
		LongHelp: `Update a default experience.

Examples:
  asc app-clips default-experiences update --experience-id "EXP_ID" --action VIEW
  asc app-clips default-experiences update --experience-id "EXP_ID" --release-version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["action"] && !visited["release-version-id"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			var attrs *asc.AppClipDefaultExperienceUpdateAttributes
			if visited["action"] {
				actionValue, err := normalizeAppClipAction(*action)
				if err != nil {
					return fmt.Errorf("app-clips default-experiences update: %w", err)
				}
				attrs = &asc.AppClipDefaultExperienceUpdateAttributes{
					Action: &actionValue,
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppClipDefaultExperience(requestCtx, experienceValue, attrs, *releaseVersionID)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperiencesDeleteCommand deletes a default experience.
func AppClipDefaultExperiencesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips default-experiences delete --experience-id \"EXP_ID\" --confirm",
		ShortHelp:  "Delete a default experience.",
		LongHelp: `Delete a default experience.

Examples:
  asc app-clips default-experiences delete --experience-id "EXP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppClipDefaultExperience(requestCtx, experienceValue); err != nil {
				return fmt.Errorf("app-clips default-experiences delete: failed to delete: %w", err)
			}

			result := &asc.AppClipDefaultExperienceDeleteResult{
				ID:      experienceValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceReviewDetailCommand fetches review detail for a default experience.
func AppClipDefaultExperienceReviewDetailCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-detail", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "review-detail",
		ShortUsage: "asc app-clips default-experiences review-detail --experience-id \"EXP_ID\"",
		ShortHelp:  "Get review detail for a default experience.",
		LongHelp: `Get review detail for a default experience.

Examples:
  asc app-clips default-experiences review-detail --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences review-detail: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReviewDetail(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences review-detail: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceReleaseWithAppStoreVersionCommand fetches releaseWithAppStoreVersion for a default experience.
func AppClipDefaultExperienceReleaseWithAppStoreVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("release-with-app-store-version", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "release-with-app-store-version",
		ShortUsage: "asc app-clips default-experiences release-with-app-store-version --experience-id \"EXP_ID\"",
		ShortHelp:  "Get release with App Store version for a default experience.",
		LongHelp: `Get release with App Store version for a default experience.

Examples:
  asc app-clips default-experiences release-with-app-store-version --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences release-with-app-store-version: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReleaseWithAppStoreVersion(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences release-with-app-store-version: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
