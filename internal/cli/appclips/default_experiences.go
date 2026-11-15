package appclips

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppClipDefaultExperiencesCommand returns the default experiences command group.
func newAppClipDefaultExperiencesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("default-experiences", flag.ExitOnError)

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
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipDefaultExperiencesListCommand(d),
			newAppClipDefaultExperiencesGetCommand(d),
			newAppClipDefaultExperienceReviewDetailCommand(d),
			newAppClipDefaultExperienceReleaseWithAppStoreVersionCommand(d),
			newAppClipDefaultExperienceRelationshipsCommand(d),
			newAppClipDefaultExperienceHeaderImageCommand(d),
			newAppClipDefaultExperiencesCreateCommand(d),
			newAppClipDefaultExperiencesUpdateCommand(d),
			newAppClipDefaultExperiencesDeleteCommand(d),
			newAppClipDefaultExperienceLocalizationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipDefaultExperiencesListCommand lists default experiences.
func newAppClipDefaultExperiencesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appClipID := shared.BindResourceIDFlag(fs, "app-clip-id", "appClips", "App Clip ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
			if err := d.ValidatePaginationFlags(ctx, fs, "app-clips default-experiences list", 200); err != nil {
				return err
			}

			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--app-clip-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppClipDefaultExperiencesOption{
				asc.Limit[asc.AppClipDefaultExperiencesOption](*limit),
				asc.NextURL[asc.AppClipDefaultExperiencesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppClipDefaultExperiencesOption](200))
				firstPage, err := client.GetAppClipDefaultExperiences(requestCtx, appClipValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.AppClipDefaultExperiencesResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceAttributes]{}}
						return d.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips default-experiences list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipDefaultExperiences(ctx, appClipValue, asc.NextURL[asc.AppClipDefaultExperiencesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips default-experiences list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipDefaultExperiences(requestCtx, appClipValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipDefaultExperiencesResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceAttributes]{}}
					return d.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips default-experiences list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipDefaultExperiencesGetCommand gets a default experience by ID.
func newAppClipDefaultExperiencesGetCommand(d *shared.Deps) *ffcli.Command {
	var (
		includeValue []string
		included     asc.IncludedFields
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-clips default-experiences view --experience-id \"EXP_ID\"",
		ShortHelp:  "View a default experience by ID.",
		LongHelp: `View a default experience by ID.

Examples:
  asc app-clips default-experiences view --experience-id "EXP_ID"`,
		IDFlag:             "experience-id",
		IDUsage:            "Default experience ID",
		IDType:             "appClipDefaultExperiences",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			include := fs.String("include", "", "Include relationships: "+strings.Join(appClipDefaultExperienceIncludeList(), ", "))
			includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{
				"appClip":                               "appClips",
				"releaseWithAppStoreVersion":            "appStoreVersions",
				"appClipDefaultExperienceLocalizations": "appClipDefaultExperienceLocalizations",
				"appClipAppStoreReviewDetail":           "appClipAppStoreReviewDetails",
			})
			return func() (err error) {
				if includeValue, err = normalizeAppClipDefaultExperienceInclude(*include); err != nil {
					return d.UsageErrorf("app-clips default-experiences view: %v", err)
				}
				included, err = includedFields.ResolveWith(d, "app-clips default-experiences view", "", includeValue)
				return err
			}
		},
		Deps: d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClipDefaultExperience(ctx, id, asc.WithAppClipDefaultExperienceInclude(includeValue), asc.WithIncludedFields[asc.AppClipDefaultExperienceOption](included))
		},
	})
}

// newAppClipDefaultExperiencesCreateCommand creates a default experience.
func newAppClipDefaultExperiencesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appClipID := shared.BindResourceIDFlag(fs, "app-clip-id", "appClips", "App Clip ID")
	action := fs.String("action", "", "Action (OPEN, VIEW, PLAY)")
	releaseVersionID := shared.BindResourceIDFlag(fs, "release-version-id", "appStoreVersions", "Release with App Store version ID")
	templateID := shared.BindResourceIDFlag(fs, "template-id", "appClipDefaultExperiences", "Existing default experience ID to use as a template")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips default-experiences create --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "Create a default experience.",
		LongHelp: `Create a default experience.

Use --template-id to clone an existing default experience. Apple copies the
template's metadata (header image and localizations) onto the new experience,
so they do not have to be re-uploaded or recreated.

Examples:
  asc app-clips default-experiences create --app-clip-id "CLIP_ID" --action OPEN
  asc app-clips default-experiences create --app-clip-id "CLIP_ID" --release-version-id "VERSION_ID"
  asc app-clips default-experiences create --app-clip-id "CLIP_ID" --release-version-id "VERSION_ID" --template-id "CURRENT_EXPERIENCE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" {
				return d.RequiredFlag("--app-clip-id")
			}

			templateValue := strings.TrimSpace(*templateID)
			templateProvided := false
			fs.Visit(func(f *flag.Flag) {
				templateProvided = templateProvided || f.Name == "template-id"
			})
			if templateProvided && templateValue == "" {
				return d.UsageError("app-clips default-experiences create: --template-id must not be empty")
			}

			var attrs *asc.AppClipDefaultExperienceCreateAttributes
			if strings.TrimSpace(*action) != "" {
				actionValue, err := normalizeAppClipAction(*action)
				if err != nil {
					return d.UsageErrorf("app-clips default-experiences create: %v", err)
				}
				attrs = &asc.AppClipDefaultExperienceCreateAttributes{
					Action: &actionValue,
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppClipDefaultExperience(requestCtx, appClipValue, attrs, *releaseVersionID, templateValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipDefaultExperiencesUpdateCommand updates a default experience.
func newAppClipDefaultExperiencesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipDefaultExperiences", "Default experience ID")
	action := fs.String("action", "", "Action (OPEN, VIEW, PLAY)")
	releaseVersionID := shared.BindResourceIDFlag(fs, "release-version-id", "appStoreVersions", "Release with App Store version ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--experience-id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["action"] && !visited["release-version-id"] {
				return d.UpdateFlagRequired(fs, action, releaseVersionID)
			}

			var attrs *asc.AppClipDefaultExperienceUpdateAttributes
			if visited["action"] {
				actionValue, err := normalizeAppClipAction(*action)
				if err != nil {
					return d.UsageErrorf("app-clips default-experiences update: %v", err)
				}
				attrs = &asc.AppClipDefaultExperienceUpdateAttributes{
					Action: &actionValue,
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppClipDefaultExperience(requestCtx, experienceValue, attrs, *releaseVersionID)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipDefaultExperiencesDeleteCommand deletes a default experience.
func newAppClipDefaultExperiencesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc app-clips default-experiences delete --experience-id \"EXP_ID\" --confirm",
		ShortHelp:  "Delete a default experience.",
		LongHelp: `Delete a default experience.

Examples:
  asc app-clips default-experiences delete --experience-id "EXP_ID" --confirm`,
		IDFlag:             "experience-id",
		IDUsage:            "Default experience ID",
		IDType:             "appClipDefaultExperiences",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppClipDefaultExperience(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// newAppClipDefaultExperienceReviewDetailCommand fetches review detail for a default experience.
func newAppClipDefaultExperienceReviewDetailCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review-detail", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipDefaultExperiences", "Default experience ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--experience-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences review-detail: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReviewDetail(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences review-detail: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipDefaultExperienceReleaseWithAppStoreVersionCommand fetches releaseWithAppStoreVersion for a default experience.
func newAppClipDefaultExperienceReleaseWithAppStoreVersionCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("release-with-app-store-version", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipDefaultExperiences", "Default experience ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--experience-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences release-with-app-store-version: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReleaseWithAppStoreVersion(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences release-with-app-store-version: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
