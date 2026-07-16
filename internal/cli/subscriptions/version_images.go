package subscriptions

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

// SubscriptionsVersionImagesCommand returns version image commands.
func SubscriptionsVersionImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images", flag.ExitOnError)
	return &ffcli.Command{
		Name: "images", ShortUsage: "asc subscriptions versions images <subcommand> [flags]",
		ShortHelp: "Manage version-scoped subscription images.",
		LongHelp: `Manage version-scoped subscription images.

Examples:
  asc subscriptions versions images list --version-id "VERSION_ID"
  asc subscriptions versions images upload --version-id "VERSION_ID" --file "./image.png"`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsVersionImagesListCommand(),
			SubscriptionsVersionImagesPrimaryCommand(),
			SubscriptionsVersionImagesLinksCommand(),
			SubscriptionsVersionImagesPrimaryLinkCommand(),
			SubscriptionsVersionImagesViewCommand(),
			SubscriptionsVersionImagesUploadCommand(),
			SubscriptionsVersionImagesUpdateCommand(),
			SubscriptionsVersionImagesDeleteCommand(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// SubscriptionsVersionImagesListCommand lists related version images.
func SubscriptionsVersionImagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images list", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription version ID")
	fields := fs.String("fields", "", "Sparse fields for subscriptionImages")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: "asc subscriptions versions images list [flags]", ShortHelp: "List images for a subscription version.",
		LongHelp: "List version-scoped subscription images.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if err := validateNextFlagConflicts(
				*next,
				flagConflict{"--version-id", flagWasProvided(fs, "version-id")},
				flagConflict{"--fields", flagWasProvided(fs, "fields")},
				flagConflict{"--limit", flagWasProvided(fs, "limit")},
			); err != nil {
				return err
			}
			if err := validatePageLimit(*limit); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("subscriptions versions images list: %v", err)
			}
			fieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionVersionImageFieldsList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersionImages(requestCtx, id,
				asc.WithSubscriptionVersionImagesLimit(*limit), asc.WithSubscriptionVersionImagesNextURL(*next), asc.WithSubscriptionVersionImagesFields(fieldValues))
			if err != nil {
				return fmt.Errorf("subscriptions versions images list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersionImages(pageCtx, id, asc.WithSubscriptionVersionImagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions versions images list: %w", err)
				}
				resp = aggregated.(*asc.SubscriptionImagesV2Response)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionImagesPrimaryCommand reads the singular image relationship.
func SubscriptionsVersionImagesPrimaryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images primary", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription version ID")
	fields := fs.String("fields", "", "Sparse fields for subscriptionImages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "primary", ShortUsage: "asc subscriptions versions images primary --version-id \"VERSION_ID\"", ShortHelp: "View the singular image for a version.",
		LongHelp: "View the singular image relationship for a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			fieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionVersionImageFieldsList())
			if err != nil {
				return err
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images primary: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersionImage(requestCtx, id, asc.WithSubscriptionImageV2Fields(fieldValues))
			if err != nil {
				return fmt.Errorf("subscriptions versions images primary: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionImagesLinksCommand lists plural image linkages.
func SubscriptionsVersionImagesLinksCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images links", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc subscriptions versions images links [flags]", ShortHelp: "List raw image linkages.",
		LongHelp: "List raw plural image relationship linkages for a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if err := validateNextFlagConflicts(
				*next,
				flagConflict{"--version-id", flagWasProvided(fs, "version-id")},
				flagConflict{"--limit", flagWasProvided(fs, "limit")},
			); err != nil {
				return err
			}
			if err := validatePageLimit(*limit); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("subscriptions versions images links: %v", err)
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images links: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersionImagesRelationships(requestCtx, id, asc.WithLinkagesLimit(*limit), asc.WithLinkagesNextURL(*next))
			if err != nil {
				return fmt.Errorf("subscriptions versions images links: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersionImagesRelationships(pageCtx, id, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions versions images links: %w", err)
				}
				resp = aggregated.(*asc.LinkagesResponse)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionImagesPrimaryLinkCommand reads the singular image linkage.
func SubscriptionsVersionImagesPrimaryLinkCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images primary-link", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription version ID")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "primary-link", ShortUsage: "asc subscriptions versions images primary-link --version-id \"VERSION_ID\"", ShortHelp: "View the singular image linkage.",
		LongHelp: "View the raw singular image relationship linkage for a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images primary-link: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersionImageRelationship(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions versions images primary-link: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionImagesViewCommand views a v2 image.
func SubscriptionsVersionImagesViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images view", flag.ExitOnError)
	id := fs.String("id", "", "Subscription image ID")
	fields := fs.String("fields", "", "Sparse fields for subscriptionImages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: "asc subscriptions versions images view --id \"IMAGE_ID\"", ShortHelp: "View a version-scoped image.",
		LongHelp: "View a version-scoped subscription image by ID.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			imageID := strings.TrimSpace(*id)
			if imageID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			fieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionVersionImageFieldsList())
			if err != nil {
				return err
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionImageV2(requestCtx, imageID, asc.WithSubscriptionImageV2Fields(fieldValues))
			if err != nil {
				return fmt.Errorf("subscriptions versions images view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionImagesUploadCommand reserves, uploads, and commits a v2 image.
func SubscriptionsVersionImagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images upload", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription version ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "upload", ShortUsage: "asc subscriptions versions images upload [flags]", ShortHelp: "Upload a version-scoped subscription image.",
		LongHelp: "Reserve, upload, and commit an image for a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			id, pathValue := strings.TrimSpace(*versionID), strings.TrimSpace(*filePath)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError("--file")
			}
			file, info, err := openSubscriptionImageFile(pathValue)
			if err != nil {
				return fmt.Errorf("subscriptions versions images upload: %w", err)
			}
			defer file.Close()
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images upload: %w", err)
			}
			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()
			reservation, err := client.CreateSubscriptionImageV2(requestCtx, id, info.Name(), info.Size())
			if err != nil {
				return fmt.Errorf("subscriptions versions images upload: failed to reserve: %w", err)
			}
			if reservation == nil || len(reservation.Data.Attributes.UploadOperations) == 0 {
				reservedID := "unknown"
				if reservation != nil && strings.TrimSpace(reservation.Data.ID) != "" {
					reservedID = reservation.Data.ID
				}
				return fmt.Errorf("subscriptions versions images upload: reserved image %s returned no upload operations", reservedID)
			}
			if err := uploadSubscriptionVersionImage(requestCtx, file, info.Size(), reservation.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("subscriptions versions images upload: upload failed for reserved image %s: %w", reservation.Data.ID, err)
			}
			uploaded := true
			resp, err := client.UpdateSubscriptionImageV2(requestCtx, reservation.Data.ID, asc.SubscriptionImageV2UpdateAttributes{Uploaded: &uploaded})
			if err != nil {
				return fmt.Errorf("subscriptions versions images upload: failed to commit reserved image %s: %w", reservation.Data.ID, err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionImagesUpdateCommand updates the uploaded state.
func SubscriptionsVersionImagesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images update", flag.ExitOnError)
	id := fs.String("id", "", "Subscription image ID")
	var uploaded shared.OptionalBool
	fs.Var(&uploaded, "uploaded", "Mark upload complete: true or false")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: "asc subscriptions versions images update --id \"IMAGE_ID\" --uploaded true", ShortHelp: "Update a version-scoped image.",
		LongHelp: "Update the uploaded state for a version-scoped subscription image.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			imageID := strings.TrimSpace(*id)
			if imageID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !uploaded.IsSet() {
				return shared.UsageError("--uploaded is required")
			}
			value := uploaded.Value()
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images update: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.UpdateSubscriptionImageV2(requestCtx, imageID, asc.SubscriptionImageV2UpdateAttributes{Uploaded: &value})
			if err != nil {
				return fmt.Errorf("subscriptions versions images update: failed to update: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionImagesDeleteCommand deletes a v2 image.
func SubscriptionsVersionImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images delete", flag.ExitOnError)
	id := fs.String("id", "", "Subscription image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "delete", ShortUsage: "asc subscriptions versions images delete --id \"IMAGE_ID\" --confirm", ShortHelp: "Delete a version-scoped image.",
		LongHelp: "Delete a version-scoped subscription image.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			imageID := strings.TrimSpace(*id)
			if imageID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions images delete: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			if err := client.DeleteSubscriptionImageV2(requestCtx, imageID); err != nil {
				return fmt.Errorf("subscriptions versions images delete: failed to delete: %w", err)
			}
			return shared.PrintOutput(&asc.AssetDeleteResult{ID: imageID, Deleted: true}, *output.Output, *output.Pretty)
		},
	}
}
