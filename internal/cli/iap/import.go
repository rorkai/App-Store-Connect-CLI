package iap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// iapImportMaxProducts bounds one run so a single invocation cannot turn a
// mistyped file into an unbounded sequence of irreversible product creates.
const iapImportMaxProducts = 50

// iapImportMaxFileSize bounds the JSON payload the command will read.
const iapImportMaxFileSize = 4 << 20

const (
	iapImportStatusCreated = "created"
	iapImportStatusSkipped = "skipped"
	iapImportStatusPlanned = "planned"
	iapImportStatusFailed  = "failed"
)

type iapImportDocument struct {
	Products []iapImportProduct `json:"products"`
}

type iapImportProduct struct {
	Type             string                  `json:"type"`
	ReferenceName    string                  `json:"referenceName"`
	ProductID        string                  `json:"productId"`
	FamilySharable   bool                    `json:"familySharable"`
	Localizations    []iapImportLocalization `json:"localizations"`
	ReviewScreenshot string                  `json:"reviewScreenshot"`
}

type iapImportLocalization struct {
	Locale      string `json:"locale"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// iapImportPlannedProduct is one validated product plus the screenshot path
// already resolved and contained inside the import root.
type iapImportPlannedProduct struct {
	product        iapImportProduct
	screenshotName string
}

// IAPImportCommand returns the iap import subcommand.
func IAPImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("import", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	filePath := fs.String("file", "", "Path to the JSON import file (required)")
	dryRun := fs.Bool("dry-run", false, "Validate the file and print the plan without creating anything")
	confirm := fs.Bool("confirm", false, "Confirm creating in-app purchases (required unless --dry-run)")
	skipExisting := fs.Bool("skip-existing", false, "Skip product IDs that already exist on the app instead of failing")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc iap import --app \"APP_ID\" --file \"./iap.json\" --confirm",
		ShortHelp:  "Create in-app purchases in bulk from a JSON file.",
		LongHelp: `Create in-app purchases in bulk from a JSON file.

Each product creates an in-app purchase, then its version-scoped localizations,
then an optional App Store review screenshot. Pricing, availability, and offers
are not part of this command.

The file directory is the import root. Every "reviewScreenshot" path resolves
relative to it unless absolute, and must stay inside it.

File schema (unknown fields are rejected, at most ` + fmt.Sprintf("%d", iapImportMaxProducts) + ` products):
  {
    "products": [
      {
        "type": "CONSUMABLE",
        "referenceName": "Pro",
        "productId": "com.example.pro",
        "familySharable": false,
        "localizations": [
          {"locale": "en-US", "name": "Pro", "description": "Remove limits"}
        ],
        "reviewScreenshot": "shots/pro.png"
      }
    ]
  }

"type" must be one of: ` + strings.Join(asc.ValidIAPTypes, ", ") + `.
"familySharable" defaults to false, "localizations" may be empty, and
"reviewScreenshot" is optional.

A product ID that already exists on the app fails the run before anything is
created. Use --skip-existing to skip and report those products instead. If a
create fails partway through, the run stops and the receipt lists every product
already created; nothing is rolled back.

Examples:
  asc iap import --app "APP_ID" --file "./iap.json" --dry-run
  asc iap import --app "APP_ID" --file "./iap.json" --confirm
  asc iap import --app "APP_ID" --file "./iap.json" --skip-existing --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("iap import does not accept positional arguments: %s", strings.Join(args, " "))
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError("--file")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			if err := shared.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			root, planned, err := loadIAPImportPlan(pathValue)
			if err != nil {
				return err
			}
			defer root.Close()

			client, existing, err := prepareIAPImport(ctx, resolvedAppID, planned, *skipExisting)
			if err != nil {
				return err
			}

			result := &asc.InAppPurchaseImportResult{
				AppID:             resolvedAppID,
				File:              filepath.Clean(pathValue),
				DryRun:            *dryRun,
				SkipExisting:      *skipExisting,
				Total:             len(planned),
				CreatedProductIDs: []string{},
				Products:          make([]asc.InAppPurchaseImportProductResult, 0, len(planned)),
			}

			runErr := executeIAPImport(ctx, client, root, result, existing, planned)
			if printErr := shared.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
				return printErr
			}
			if runErr != nil {
				return shared.NewReportedError(runErr)
			}
			return nil
		},
	}
}

// loadIAPImportPlan reads and validates the import file through a root
// anchored at the file's own directory, so neither the document nor any
// screenshot it names can select a file outside the operator-selected tree.
func loadIAPImportPlan(path string) (rootfs.Root, []iapImportPlannedProduct, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return rootfs.Root{}, nil, shared.UsageErrorf("iap import: resolve --file %q: %v", path, err)
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return rootfs.Root{}, nil, shared.UsageErrorf("iap import: %v", err)
	}

	data, err := root.ReadFileLimited(filepath.Base(absolute), iapImportMaxFileSize)
	if err != nil {
		_ = root.Close()
		return rootfs.Root{}, nil, shared.UsageErrorf("iap import: read --file %q: %v", path, err)
	}

	planned, err := parseIAPImportDocument(root, data)
	if err != nil {
		_ = root.Close()
		return rootfs.Root{}, nil, err
	}
	return root, planned, nil
}

func parseIAPImportDocument(root rootfs.Root, data []byte) ([]iapImportPlannedProduct, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var document iapImportDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, shared.UsageErrorf("iap import: invalid import file: %v", err)
	}
	if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return nil, shared.UsageError("iap import: invalid import file: expected a single JSON object")
	}

	if len(document.Products) == 0 {
		return nil, shared.UsageError("iap import: the import file must contain at least one product")
	}
	if len(document.Products) > iapImportMaxProducts {
		return nil, shared.UsageErrorf("iap import: the import file contains %d products; at most %d are allowed per run", len(document.Products), iapImportMaxProducts)
	}

	planned := make([]iapImportPlannedProduct, 0, len(document.Products))
	seen := make(map[string]struct{}, len(document.Products))
	for index, raw := range document.Products {
		product, screenshotName, err := validateIAPImportProduct(root, raw, index)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[product.ProductID]; exists {
			return nil, shared.UsageErrorf("iap import: products[%d]: duplicate productId %q", index, product.ProductID)
		}
		seen[product.ProductID] = struct{}{}
		planned = append(planned, iapImportPlannedProduct{product: product, screenshotName: screenshotName})
	}
	return planned, nil
}

func validateIAPImportProduct(root rootfs.Root, raw iapImportProduct, index int) (iapImportProduct, string, error) {
	product := iapImportProduct{
		Type:           strings.ToUpper(strings.TrimSpace(raw.Type)),
		ReferenceName:  strings.TrimSpace(raw.ReferenceName),
		ProductID:      strings.TrimSpace(raw.ProductID),
		FamilySharable: raw.FamilySharable,
	}
	if product.ProductID == "" {
		return iapImportProduct{}, "", shared.UsageErrorf("iap import: products[%d]: productId is required", index)
	}
	if product.ReferenceName == "" {
		return iapImportProduct{}, "", shared.UsageErrorf("iap import: products[%d]: referenceName is required", index)
	}
	if !slices.Contains(asc.ValidIAPTypes, product.Type) {
		return iapImportProduct{}, "", shared.UsageErrorf("iap import: products[%d]: type must be one of: %s", index, strings.Join(asc.ValidIAPTypes, ", "))
	}

	product.Localizations = make([]iapImportLocalization, 0, len(raw.Localizations))
	locales := make(map[string]struct{}, len(raw.Localizations))
	for localizationIndex, rawLocalization := range raw.Localizations {
		localization := iapImportLocalization{
			Locale:      strings.TrimSpace(rawLocalization.Locale),
			Name:        strings.TrimSpace(rawLocalization.Name),
			Description: strings.TrimSpace(rawLocalization.Description),
		}
		if localization.Locale == "" {
			return iapImportProduct{}, "", shared.UsageErrorf("iap import: products[%d].localizations[%d]: locale is required", index, localizationIndex)
		}
		if localization.Name == "" {
			return iapImportProduct{}, "", shared.UsageErrorf("iap import: products[%d].localizations[%d]: name is required", index, localizationIndex)
		}
		if _, exists := locales[localization.Locale]; exists {
			return iapImportProduct{}, "", shared.UsageErrorf("iap import: products[%d]: duplicate locale %q", index, localization.Locale)
		}
		locales[localization.Locale] = struct{}{}
		product.Localizations = append(product.Localizations, localization)
	}

	screenshot := strings.TrimSpace(raw.ReviewScreenshot)
	if screenshot == "" {
		return product, "", nil
	}
	product.ReviewScreenshot = screenshot
	name, err := resolveIAPImportScreenshotName(root, screenshot)
	if err != nil {
		return iapImportProduct{}, "", shared.UsageErrorf("iap import: products[%d]: reviewScreenshot %q: %v", index, screenshot, err)
	}
	return product, name, nil
}

// resolveIAPImportScreenshotName converts a document-supplied screenshot path
// into a name below the import root. Absolute paths are accepted only when
// they already point inside that root; everything else is rejected rather than
// read from outside the operator-selected tree.
func resolveIAPImportScreenshotName(root rootfs.Root, value string) (string, error) {
	name := value
	if filepath.IsAbs(value) {
		relative, err := filepath.Rel(root.Path(), filepath.Clean(value))
		if err != nil {
			return "", fmt.Errorf("must stay inside %s", root.Path())
		}
		name = relative
	}
	if err := rootfs.ValidateRelative(name); err != nil {
		return "", fmt.Errorf("must stay inside %s", root.Path())
	}
	if err := root.CheckContained(name); err != nil {
		return "", err
	}
	resolved, err := root.Resolve(name)
	if err != nil {
		return "", err
	}
	if err := asc.ValidateImageFile(resolved); err != nil {
		return "", err
	}
	return name, nil
}

// prepareIAPImport resolves the client and the app's existing product IDs, and
// rejects a conflicting run before the receipt exists, because nothing has
// been created yet and the error alone is the complete result.
func prepareIAPImport(ctx context.Context, appID string, planned []iapImportPlannedProduct, skipExisting bool) (*asc.Client, map[string]struct{}, error) {
	client, err := iapQueryClientFactory()
	if err != nil {
		return nil, nil, fmt.Errorf("iap import: %w", err)
	}

	existing, err := fetchExistingIAPProductIDs(ctx, client, appID, planned)
	if err != nil {
		return nil, nil, fmt.Errorf("iap import: %w", err)
	}
	if skipExisting {
		return client, existing, nil
	}

	conflicts := make([]string, 0)
	for _, item := range planned {
		if _, ok := existing[item.product.ProductID]; ok {
			conflicts = append(conflicts, item.product.ProductID)
		}
	}
	if len(conflicts) > 0 {
		return nil, nil, fmt.Errorf(
			"iap import: product ID(s) already exist on app %s: %s (use --skip-existing to skip them)",
			appID, strings.Join(conflicts, ", "),
		)
	}
	return client, existing, nil
}

func executeIAPImport(
	ctx context.Context,
	client *asc.Client,
	root rootfs.Root,
	result *asc.InAppPurchaseImportResult,
	existing map[string]struct{},
	planned []iapImportPlannedProduct,
) error {
	for _, item := range planned {
		product := item.product
		entry := asc.InAppPurchaseImportProductResult{
			ProductID:     product.ProductID,
			ReferenceName: product.ReferenceName,
			Type:          product.Type,
		}

		if _, ok := existing[product.ProductID]; ok {
			entry.Status = iapImportStatusSkipped
			result.Skipped++
			result.Products = append(result.Products, entry)
			continue
		}
		if result.DryRun {
			entry.Status = iapImportStatusPlanned
			result.Products = append(result.Products, entry)
			continue
		}

		if err := importIAPProduct(ctx, client, root, result, &entry, item); err != nil {
			entry.Status = iapImportStatusFailed
			entry.Error = err.Error()
			result.Products = append(result.Products, entry)
			result.FailedProductID = product.ProductID
			result.Error = err.Error()
			return fmt.Errorf("iap import: product %q: %w", product.ProductID, err)
		}

		entry.Status = iapImportStatusCreated
		result.Products = append(result.Products, entry)
	}

	return nil
}

func importIAPProduct(
	ctx context.Context,
	client *asc.Client,
	root rootfs.Root,
	result *asc.InAppPurchaseImportResult,
	entry *asc.InAppPurchaseImportProductResult,
	item iapImportPlannedProduct,
) error {
	product := item.product

	createCtx, createCancel := shared.ContextWithTimeout(ctx)
	created, err := client.CreateInAppPurchaseV2(createCtx, result.AppID, asc.InAppPurchaseV2CreateAttributes{
		Name:              product.ReferenceName,
		ProductID:         product.ProductID,
		InAppPurchaseType: product.Type,
		FamilySharable:    product.FamilySharable,
	})
	createCancel()
	if err != nil {
		return fmt.Errorf("create in-app purchase: %w", err)
	}
	// The successful create response acknowledges the product even if it omits
	// its ID. Record it before any ID-dependent step so the partial receipt
	// warns that a retry could collide with the accepted product.
	result.Created++
	result.CreatedProductIDs = append(result.CreatedProductIDs, product.ProductID)
	entry.InAppPurchaseID = strings.TrimSpace(created.Data.ID)
	if entry.InAppPurchaseID == "" {
		return fmt.Errorf("create in-app purchase: response did not include an id")
	}

	if len(product.Localizations) > 0 {
		versionID, err := resolveIAPImportVersionID(ctx, client, entry.InAppPurchaseID)
		if err != nil {
			return err
		}
		entry.VersionID = versionID

		for _, localization := range product.Localizations {
			attrs := asc.InAppPurchaseLocalizationV2CreateAttributes{
				Name:   localization.Name,
				Locale: localization.Locale,
			}
			if localization.Description != "" {
				description := localization.Description
				attrs.Description = &asc.NullableString{Value: &description}
			}
			localizationCtx, localizationCancel := shared.ContextWithTimeout(ctx)
			_, err := client.CreateInAppPurchaseLocalizationV2(localizationCtx, versionID, attrs)
			localizationCancel()
			if err != nil {
				return fmt.Errorf("create localization %q: %w", localization.Locale, err)
			}
			entry.LocalizationsCreated++
			result.LocalizationsCreated++
		}
	}

	if item.screenshotName == "" {
		return nil
	}

	screenshotID, err := uploadIAPImportReviewScreenshot(ctx, client, root, entry.InAppPurchaseID, item.screenshotName)
	entry.ReviewScreenshotID = screenshotID
	if err != nil {
		return err
	}
	result.ScreenshotsUploaded++
	return nil
}

// resolveIAPImportVersionID returns the version that owns the localizations for
// a freshly created in-app purchase, creating one when App Store Connect has
// not already provisioned it.
func resolveIAPImportVersionID(ctx context.Context, client *asc.Client, iapID string) (string, error) {
	listCtx, listCancel := shared.ContextWithTimeout(ctx)
	versions, err := client.GetInAppPurchaseVersions(listCtx, iapID, asc.WithIAPVersionsLimit(200))
	listCancel()
	if err != nil {
		return "", fmt.Errorf("list in-app purchase versions: %w", err)
	}
	for _, version := range versions.Data {
		if id := strings.TrimSpace(version.ID); id != "" {
			return id, nil
		}
	}

	createCtx, createCancel := shared.ContextWithTimeout(ctx)
	created, err := client.CreateInAppPurchaseVersion(createCtx, iapID)
	createCancel()
	if err != nil {
		return "", fmt.Errorf("create in-app purchase version: %w", err)
	}
	id := strings.TrimSpace(created.Data.ID)
	if id == "" {
		return "", fmt.Errorf("create in-app purchase version: response did not include an id")
	}
	return id, nil
}

func uploadIAPImportReviewScreenshot(ctx context.Context, client *asc.Client, root rootfs.Root, iapID, name string) (string, error) {
	file, err := root.OpenFile(name)
	if err != nil {
		return "", fmt.Errorf("open review screenshot %q: %w", name, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect review screenshot %q: %w", name, err)
	}
	if err := asc.ValidateAssetFileInfo(name, info); err != nil {
		return "", fmt.Errorf("validate review screenshot %q: %w", name, err)
	}
	snapshot, cleanupSnapshot, err := snapshotImageFile(file, info.Size())
	if err != nil {
		return "", fmt.Errorf("snapshot review screenshot %q: %w", name, err)
	}
	defer cleanupSnapshot()

	checksum, err := asc.ComputeChecksumFromReader(snapshot, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return "", fmt.Errorf("checksum review screenshot %q: %w", name, err)
	}

	uploadCtx, uploadCancel := context.WithTimeout(ctx, asc.ResolveUploadTimeoutWithDefault(iapAssetUploadDefaultTimeout))
	defer uploadCancel()

	reservation, err := client.CreateInAppPurchaseAppStoreReviewScreenshot(uploadCtx, iapID, filepath.Base(name), info.Size())
	if err != nil {
		return "", fmt.Errorf("create review screenshot: %w", err)
	}
	if reservation == nil {
		return "", fmt.Errorf("create review screenshot: no upload operations returned")
	}
	screenshotID := strings.TrimSpace(reservation.Data.ID)
	if screenshotID == "" {
		return "", fmt.Errorf("create review screenshot: response did not include an id")
	}
	if len(reservation.Data.Attributes.UploadOperations) == 0 {
		return screenshotID, fmt.Errorf("create review screenshot: no upload operations returned")
	}

	if err := asc.UploadAssetFromFile(uploadCtx, snapshot, info.Size(), reservation.Data.Attributes.UploadOperations); err != nil {
		return screenshotID, fmt.Errorf("upload review screenshot: %w", err)
	}

	uploaded := true
	if _, err := client.UpdateInAppPurchaseAppStoreReviewScreenshot(uploadCtx, screenshotID, asc.InAppPurchaseAppStoreReviewScreenshotUpdateAttributes{
		Uploaded:           &uploaded,
		SourceFileChecksum: &checksum.Hash,
	}); err != nil {
		return screenshotID, fmt.Errorf("commit review screenshot: %w", err)
	}

	verifyCtx, verifyCancel := context.WithTimeout(ctx, asc.ResolveUploadTimeoutWithDefault(iapAssetUploadDefaultTimeout))
	defer verifyCancel()
	if _, err := waitForIAPReviewScreenshotDelivery(verifyCtx, client, screenshotID); err != nil {
		return screenshotID, fmt.Errorf("verify review screenshot: %w", err)
	}
	return screenshotID, nil
}

// fetchExistingIAPProductIDs returns the subset of planned product IDs that the
// app already has, filtering server-side so the check costs one page in the
// common case.
func fetchExistingIAPProductIDs(ctx context.Context, client *asc.Client, appID string, planned []iapImportPlannedProduct) (map[string]struct{}, error) {
	productIDs := make([]string, 0, len(planned))
	for _, item := range planned {
		productIDs = append(productIDs, item.product.ProductID)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	firstPage, err := client.GetInAppPurchasesV2(
		requestCtx,
		appID,
		asc.WithIAPProductIDs(productIDs),
		asc.WithIAPFields([]string{"productId"}),
		asc.WithIAPLimit(200),
	)
	if err != nil {
		return nil, fmt.Errorf("list existing in-app purchases: %w", err)
	}

	existing := make(map[string]struct{}, len(productIDs))
	err = asc.PaginateEach(
		requestCtx,
		firstPage,
		func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetInAppPurchasesV2(pageCtx, appID, asc.WithIAPNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.InAppPurchasesV2Response)
			if !ok {
				return fmt.Errorf("unexpected in-app purchases response type %T", page)
			}
			for _, item := range resp.Data {
				if productID := strings.TrimSpace(item.Attributes.ProductID); productID != "" {
					existing[productID] = struct{}{}
				}
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list existing in-app purchases: %w", err)
	}
	return existing, nil
}
