package app_events

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppEventVideoClipsCommand returns the app event video clips command group.
func newAppEventVideoClipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("video-clips", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "video-clips",
		ShortUsage: "asc app-events video-clips <subcommand> [flags]",
		ShortHelp:  "Manage in-app event video clips.",
		LongHelp: `Manage in-app event video clips.

Examples:
  asc app-events video-clips list --event-id "EVENT_ID"
  asc app-events video-clips create --localization-id "LOC_ID" --path "./clip.mov" --asset-type EVENT_CARD
  asc app-events video-clips set-poster-frame --clip-id "CLIP_ID" --time-code "00:00:05:00"`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppEventVideoClipsListCommand(d),
			newAppEventVideoClipsRelationshipsCommand(d),
			newAppEventVideoClipsGetCommand(d),
			newAppEventVideoClipsCreateCommand(d),
			newAppEventVideoClipsSetPosterFrameCommand(d),
			newAppEventVideoClipsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppEventVideoClipsRelationshipsCommand returns the app event video clips links subcommand.
func newAppEventVideoClipsRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("video-clips links", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc app-events video-clips links [flags]",
		ShortHelp:  "List video clip relationships for an in-app event localization.",
		LongHelp: `List video clip relationships for an in-app event localization.

Examples:
  asc app-events video-clips links --localization-id "LOC_ID"
  asc app-events video-clips links --event-id "EVENT_ID" --locale "en-US"
  asc app-events video-clips links --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-events video-clips links", 200); err != nil {
				return err
			}

			trimmedNext := strings.TrimSpace(*next)
			if trimmedNext == "" && strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --event-id or --localization-id is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-events video-clips links: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedLocalizationID := strings.TrimSpace(*localizationID)
			if trimmedNext == "" {
				resolvedLocalizationID, err = resolveAppEventLocalizationID(requestCtx, d, client, *eventID, resolvedLocalizationID, *locale)
				if err != nil {
					return fmt.Errorf("app-events video-clips links: %w", err)
				}
			}

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](200))
				firstPage, err := client.GetAppEventVideoClipsRelationships(requestCtx, resolvedLocalizationID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events video-clips links: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventVideoClipsRelationships(ctx, resolvedLocalizationID, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events video-clips links: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEventVideoClipsRelationships(requestCtx, resolvedLocalizationID, opts...)
			if err != nil {
				return fmt.Errorf("app-events video-clips links: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppEventVideoClipsListCommand returns the app event video clips list subcommand.
func newAppEventVideoClipsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("video-clips list", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events video-clips list [flags]",
		ShortHelp:  "List video clips for an in-app event localization.",
		LongHelp: `List video clips for an in-app event localization.

Examples:
  asc app-events video-clips list --localization-id "LOC_ID"
  asc app-events video-clips list --event-id "EVENT_ID" --locale "en-US"
  asc app-events video-clips list --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-events video-clips list", 200); err != nil {
				return err
			}
			if strings.TrimSpace(*next) == "" && strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --event-id or --localization-id is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-events video-clips list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedLocalizationID := strings.TrimSpace(*localizationID)
			if strings.TrimSpace(*next) == "" {
				resolvedLocalizationID, err = resolveAppEventLocalizationID(requestCtx, d, client, *eventID, resolvedLocalizationID, *locale)
				if err != nil {
					return fmt.Errorf("app-events video-clips list: %w", err)
				}
			}

			opts := []asc.AppEventVideoClipsOption{
				asc.Limit[asc.AppEventVideoClipsOption](*limit),
				asc.NextURL[asc.AppEventVideoClipsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppEventVideoClipsOption](200))
				firstPage, err := client.GetAppEventVideoClips(requestCtx, resolvedLocalizationID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events video-clips list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventVideoClips(ctx, resolvedLocalizationID, asc.NextURL[asc.AppEventVideoClipsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events video-clips list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEventVideoClips(requestCtx, resolvedLocalizationID, opts...)
			if err != nil {
				return fmt.Errorf("app-events video-clips list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppEventVideoClipsGetCommand returns the app event video clips get subcommand.
func newAppEventVideoClipsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "video-clips view",
		Name:        "view",
		ShortUsage:  "asc app-events video-clips view --clip-id \"CLIP_ID\"",
		ShortHelp:   "View an in-app event video clip by ID.",
		LongHelp: `View an in-app event video clip by ID.

Examples:
  asc app-events video-clips view --clip-id "CLIP_ID"`,
		IDFlag:             "clip-id",
		IDUsage:            "App event video clip ID",
		IDType:             "appEventVideoClips",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEventVideoClip(ctx, id)
		},
	})
}

// newAppEventVideoClipsCreateCommand returns the app event video clips create subcommand.
func newAppEventVideoClipsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("video-clips create", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	path := fs.String("path", "", "Path to video clip file")
	assetType := fs.String("asset-type", "", "Asset type: "+strings.Join(asc.ValidAppEventAssetTypes, ", "))
	fs.String("time-code", "", "Poster frame timecode (e.g., 00:00:05:00 or 00:00:05.000)")
	fs.String("preview-frame-time-code", "", "Alias of --time-code")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-events video-clips create [flags]",
		ShortHelp:  "Upload a video clip for an in-app event localization.",
		LongHelp: `Upload a video clip for an in-app event localization.

Examples:
  asc app-events video-clips create --localization-id "LOC_ID" --path "./clip.mov" --asset-type EVENT_CARD
  asc app-events video-clips create --event-id "EVENT_ID" --locale "en-US" --path "./clip.mov" --asset-type EVENT_DETAILS_PAGE --time-code "00:00:05.000"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				return d.RequiredFlag("--path")
			}
			if strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --event-id or --localization-id is required")
				return shared.MissingRequiredUsageError("")
			}

			normalizedAssetType, err := normalizeAppEventAssetType(*assetType)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			timeCode, err := videoClipCreateTimeCode(d, fs)
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-events video-clips create: %w", err)
			}

			requestCtx, cancel := d.ResolvedTimeout(ctx, appEventAssetUploadDefaultTimeout)
			defer cancel()

			resolvedLocalizationID, err := resolveAppEventLocalizationID(requestCtx, d, client, *eventID, *localizationID, *locale)
			if err != nil {
				return fmt.Errorf("app-events video-clips create: %w", err)
			}

			file, info, err := openAssetFile(pathValue)
			if err != nil {
				return fmt.Errorf("app-events video-clips create: %w", err)
			}
			defer file.Close()

			resp, err := client.CreateAppEventVideoClip(requestCtx, resolvedLocalizationID, info.Name(), info.Size(), normalizedAssetType, timeCode)
			if err != nil {
				return fmt.Errorf("app-events video-clips create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("app-events video-clips create: no upload operations returned")
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), resp.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("app-events video-clips create: upload failed: %w", err)
			}

			uploaded := true
			updateAttrs := asc.AppEventVideoClipUpdateAttributes{
				Uploaded: &uploaded,
			}
			_, err = client.UpdateAppEventVideoClip(requestCtx, resp.Data.ID, updateAttrs)
			if err != nil {
				return fmt.Errorf("app-events video-clips create: failed to commit upload: %w", err)
			}

			finalResp, err := waitForAppEventVideoClipDelivery(requestCtx, d, client, resp.Data.ID)
			if err != nil {
				return fmt.Errorf("app-events video-clips create: %w", err)
			}
			if finalResp != nil {
				return d.PrintOutput(finalResp, *output.Output, *output.Pretty)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppEventVideoClipsSetPosterFrameCommand returns the set-poster-frame subcommand.
func newAppEventVideoClipsSetPosterFrameCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("video-clips set-poster-frame", flag.ExitOnError)

	clipID := shared.BindResourceIDFlag(fs, "clip-id", "appEventVideoClips", "App event video clip ID")
	timeCode := fs.String("time-code", "", "Poster frame timecode (e.g., 00:00:05:00 or 00:00:05.000)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set-poster-frame",
		ShortUsage: `asc app-events video-clips set-poster-frame --clip-id "CLIP_ID" --time-code "00:00:05:00"`,
		ShortHelp:  "Set the poster frame timecode for an in-app event video clip.",
		LongHelp: `Set the poster frame timecode for an in-app event video clip.

Accepted timecode formats are HH:MM:SS:FF (frames) and HH:MM:SS.mmm
(milliseconds).

Examples:
  asc app-events video-clips set-poster-frame --clip-id "CLIP_ID" --time-code "00:00:05:00"
  asc app-events video-clips set-poster-frame --clip-id "CLIP_ID" --time-code "00:00:05.000"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*clipID)
			if id == "" {
				return d.RequiredFlag("--clip-id")
			}
			tc := strings.TrimSpace(*timeCode)
			if tc == "" {
				return d.RequiredFlag("--time-code")
			}
			if err := validateVideoClipTimeCode(d, "time-code", tc); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-events video-clips set-poster-frame: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppEventVideoClip(requestCtx, id, asc.AppEventVideoClipUpdateAttributes{PreviewFrameTimeCode: &tc})
			if err != nil {
				return fmt.Errorf("app-events video-clips set-poster-frame: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// videoClipCreateTimeCode returns the validated time code from whichever
// spelling was passed, or "" when neither was. It reads visited flags so an
// explicit empty value is rejected instead of treated as omitted.
func videoClipCreateTimeCode(d *shared.Deps, fs *flag.FlagSet) (string, error) {
	var provided []*flag.Flag
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "time-code" || f.Name == "preview-frame-time-code" {
			provided = append(provided, f)
		}
	})
	if len(provided) == 0 {
		return "", nil
	}
	if len(provided) > 1 {
		return "", d.UsageError("--time-code and --preview-frame-time-code are mutually exclusive")
	}
	timeCode := strings.TrimSpace(provided[0].Value.String())
	return timeCode, validateVideoClipTimeCode(d, provided[0].Name, timeCode)
}

func validateVideoClipTimeCode(d *shared.Deps, flagName, timeCode string) error {
	if !assets.ValidPreviewFrameTimeCode(timeCode) {
		return d.UsageErrorf("--%s must be in HH:MM:SS:FF or HH:MM:SS.mmm format (e.g., 00:00:05:00 or 00:00:05.000)", flagName)
	}
	return nil
}

// newAppEventVideoClipsDeleteCommand returns the app event video clips delete subcommand.
func newAppEventVideoClipsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "video-clips delete",
		Name:        "delete",
		ShortUsage:  "asc app-events video-clips delete --clip-id \"CLIP_ID\" --confirm",
		ShortHelp:   "Delete an in-app event video clip.",
		LongHelp: `Delete an in-app event video clip.

Examples:
  asc app-events video-clips delete --clip-id "CLIP_ID" --confirm`,
		IDFlag:             "clip-id",
		IDUsage:            "App event video clip ID",
		IDType:             "appEventVideoClips",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppEventVideoClip(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}
