package encryption

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewEncryptionCommand returns the encryption command group, built on d.
func NewEncryptionCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "encryption",
		ShortUsage: "asc encryption <subcommand> [flags]",
		ShortHelp:  "Manage app encryption declarations and documents.",
		LongHelp: `Manage app encryption declarations and documents.

Examples:
  asc encryption declarations list --app "APP_ID"
  asc encryption declarations view --id "DECL_ID"
  asc encryption declarations create --app "APP_ID" --app-description "Uses TLS" --contains-proprietary-cryptography=false --contains-third-party-cryptography=true --available-on-french-store=true
  asc encryption declarations exempt-declare --plist ./Info.plist
  asc encryption declarations assign-builds --id "DECL_ID" --build-id "BUILD_ID"
  asc encryption documents view --id "DOC_ID"
  asc encryption documents upload --declaration "DECL_ID" --file ./export.pdf`,
		FlagSet:   d.NewFlagSet("encryption", flag.ExitOnError),
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newEncryptionDeclarationsCommand(d),
			newEncryptionDocumentsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newEncryptionDeclarationsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "declarations",
		ShortUsage: "asc encryption declarations <subcommand> [flags]",
		ShortHelp:  "Manage app encryption declarations.",
		LongHelp: `Manage app encryption declarations.

Examples:
  asc encryption declarations list --app "APP_ID"
  asc encryption declarations view --id "DECL_ID"
  asc encryption declarations create --app "APP_ID" --app-description "Uses TLS" --contains-proprietary-cryptography=false --contains-third-party-cryptography=true --available-on-french-store=true
  asc encryption declarations exempt-declare --plist ./Info.plist
  asc encryption declarations assign-builds --id "DECL_ID" --build-id "BUILD_ID"`,
		FlagSet:   d.NewFlagSet("declarations", flag.ExitOnError),
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newEncryptionDeclarationsListCommand(d),
			newEncryptionDeclarationsGetCommand(d),
			newEncryptionDeclarationsAppCommand(d),
			newEncryptionDeclarationsDeclarationDocumentCommand(d),
			newEncryptionDeclarationsCreateCommand(d),
			newEncryptionDeclarationsExemptDeclareCommand(d),
			newEncryptionDeclarationsAssignBuildsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newEncryptionDeclarationsListCommand(d *shared.Deps) *ffcli.Command {
	var (
		builds                                         *string
		buildLimit                                     *int
		fieldsValue, documentFieldsValue, includeValue []string
		included                                       asc.IncludedFields
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "encryption declarations list",
		Name:        "list",
		ShortUsage:  "asc encryption declarations list --app \"APP_ID\" [flags]",
		ShortHelp:   "List encryption declarations for an app.",
		LongHelp: `List encryption declarations for an app.

Examples:
  asc encryption declarations list --app "APP_ID"
  asc encryption declarations list --app "APP_ID" --include appEncryptionDeclarationDocument --document-fields "fileName,fileSize"
  asc encryption declarations list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"build-id", "fields", "document-fields", "include", "build-limit"},
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			next := fs.Lookup("next").Value
			builds = fs.String("build-id", "", "Filter by build IDs (comma-separated)")
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(encryptionDeclarationFieldList(), ", "))
			documentFields := fs.String("document-fields", "", "Document fields to include: "+strings.Join(encryptionDocumentFieldList(), ", "))
			include := fs.String("include", "", "Include relationships: "+strings.Join(encryptionDeclarationIncludeList(), ", "))
			includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{"app": "apps", "builds": "builds"})
			buildLimit = fs.Int("build-limit", 0, "Maximum included builds per declaration (1-50)")
			return func() error {
				if *buildLimit != 0 && (*buildLimit < 1 || *buildLimit > 50) {
					return d.UsageError("encryption declarations list: --build-limit must be between 1 and 50")
				}

				var err error
				fieldsValue, err = normalizeEncryptionDeclarationFields(*fields)
				if err != nil {
					return d.UsageErrorf("encryption declarations list: %v", err)
				}
				documentFieldsValue, err = normalizeEncryptionDocumentFields(*documentFields, "--document-fields")
				if err != nil {
					return d.UsageErrorf("encryption declarations list: %v", err)
				}
				includeValue, err = normalizeEncryptionDeclarationInclude(*include)
				if err != nil {
					return d.UsageErrorf("encryption declarations list: %v", err)
				}
				included, err = includedFields.ResolveWith(d, "encryption declarations list", next.String(), includeValue)
				return err
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, appID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEncryptionDeclarations(ctx, appID,
				asc.WithAppEncryptionDeclarationsBuildIDs(shared.SplitCSV(*builds)),
				asc.WithAppEncryptionDeclarationsFields(fieldsValue),
				asc.WithAppEncryptionDeclarationsDocumentFields(documentFieldsValue),
				asc.WithAppEncryptionDeclarationsInclude(includeValue),
				asc.WithIncludedFields[asc.AppEncryptionDeclarationsOption](included),
				asc.Limit[asc.AppEncryptionDeclarationsOption](limit),
				asc.WithAppEncryptionDeclarationsBuildLimit(*buildLimit),
				asc.NextURL[asc.AppEncryptionDeclarationsOption](next),
			)
		},
	})
}

func newEncryptionDeclarationsGetCommand(d *shared.Deps) *ffcli.Command {
	var (
		buildLimit                                     *int
		fieldsValue, documentFieldsValue, includeValue []string
		included                                       asc.IncludedFields
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "encryption declarations view",
		Name:        "view",
		ShortUsage:  "asc encryption declarations view --id \"DECL_ID\"",
		ShortHelp:   "View an encryption declaration by ID.",
		LongHelp: `View an encryption declaration by ID.

Examples:
  asc encryption declarations view --id "DECL_ID"
  asc encryption declarations view --id "DECL_ID" --include appEncryptionDeclarationDocument --document-fields "fileName,fileSize"`,
		IDUsage:            "Encryption declaration ID (required)",
		IDType:             "appEncryptionDeclarations",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(encryptionDeclarationFieldList(), ", "))
			documentFields := fs.String("document-fields", "", "Document fields to include: "+strings.Join(encryptionDocumentFieldList(), ", "))
			include := fs.String("include", "", "Include relationships: "+strings.Join(encryptionDeclarationIncludeList(), ", "))
			includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{"app": "apps", "builds": "builds"})
			buildLimit = fs.Int("build-limit", 0, "Maximum included builds (1-50)")
			return func() (err error) {
				if *buildLimit != 0 && (*buildLimit < 1 || *buildLimit > 50) {
					return d.UsageError("encryption declarations view: --build-limit must be between 1 and 50")
				}
				if fieldsValue, err = normalizeEncryptionDeclarationFields(*fields); err != nil {
					return d.UsageErrorf("encryption declarations view: %v", err)
				}
				if documentFieldsValue, err = normalizeEncryptionDocumentFields(*documentFields, "--document-fields"); err != nil {
					return d.UsageErrorf("encryption declarations view: %v", err)
				}
				if includeValue, err = normalizeEncryptionDeclarationInclude(*include); err != nil {
					return d.UsageErrorf("encryption declarations view: %v", err)
				}
				included, err = includedFields.ResolveWith(d, "encryption declarations view", "", includeValue)
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEncryptionDeclaration(
				ctx, id,
				asc.WithAppEncryptionDeclarationsFields(fieldsValue),
				asc.WithAppEncryptionDeclarationsDocumentFields(documentFieldsValue),
				asc.WithAppEncryptionDeclarationsInclude(includeValue),
				asc.WithIncludedFields[asc.AppEncryptionDeclarationsOption](included),
				asc.WithAppEncryptionDeclarationsBuildLimit(*buildLimit),
			)
		},
	})
}

func newEncryptionDeclarationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("encryption declarations create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	appDescription := fs.String("app-description", "", "Description of encryption usage (required)")
	containsProprietary := fs.Bool("contains-proprietary-cryptography", false, "App contains proprietary cryptography (required)")
	containsThirdParty := fs.Bool("contains-third-party-cryptography", false, "App contains third-party cryptography (required)")
	availableOnFrenchStore := fs.Bool("available-on-french-store", false, "App is available on the French store (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc encryption declarations create --app \"APP_ID\" [flags]",
		ShortHelp:  "Create a new encryption declaration.",
		LongHelp: `Create a new encryption declaration.

Examples:
  asc encryption declarations create --app "APP_ID" --app-description "Uses TLS" --contains-proprietary-cryptography=false --contains-third-party-cryptography=true --available-on-french-store=true`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			descriptionValue := strings.TrimSpace(*appDescription)
			if descriptionValue == "" {
				return d.RequiredFlag("--app-description")
			}
			if !visited["contains-proprietary-cryptography"] {
				return d.RequiredFlag("--contains-proprietary-cryptography")
			}
			if !visited["contains-third-party-cryptography"] {
				return d.RequiredFlag("--contains-third-party-cryptography")
			}
			if !visited["available-on-french-store"] {
				return d.RequiredFlag("--available-on-french-store")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("encryption declarations create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.AppEncryptionDeclarationCreateAttributes{
				AppDescription:                  descriptionValue,
				ContainsProprietaryCryptography: *containsProprietary,
				ContainsThirdPartyCryptography:  *containsThirdParty,
				AvailableOnFrenchStore:          *availableOnFrenchStore,
			}

			resp, err := client.CreateAppEncryptionDeclaration(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("encryption declarations create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newEncryptionDeclarationsExemptDeclareCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("encryption declarations exempt-declare", flag.ExitOnError)

	plistPath := fs.String("plist", "", "Path to Info.plist to update with ITSAppUsesNonExemptEncryption=false")

	return &ffcli.Command{
		Name:       "exempt-declare",
		ShortUsage: "asc encryption declarations exempt-declare [--plist ./Info.plist]",
		ShortHelp:  "Guide local Info.plist exemption for exempt encryption.",
		LongHelp: `Guide local Info.plist exemption for exempt encryption.

The App Store Connect API does not support creating an encryption declaration
with all cryptography flags set to false. For apps that use no encryption (or
only exempt encryption like HTTPS/TLS), the correct approach is to set
ITSAppUsesNonExemptEncryption to false in your Info.plist.

This command can update your Info.plist automatically, or print the required
entry for you to add manually.

This command only updates local project metadata. It does not create or attach
an App Store Connect encryption declaration. For uploaded builds, use
"asc validate" to verify ASC-side readiness, including encryption state.

Examples:
  asc encryption declarations exempt-declare
  asc encryption declarations exempt-declare --plist ./MyApp/Info.plist`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			plistValue := strings.TrimSpace(*plistPath)

			if visited["plist"] {
				if plistValue == "" {
					return d.UsageError("--plist must not be empty")
				}
				return updatePlistExemption(d.IO.ErrOut, plistValue)
			}

			fmt.Fprintln(d.IO.ErrOut, `To declare encryption exemption, add the following to your Info.plist:

  <key>ITSAppUsesNonExemptEncryption</key>
  <false/>

Or pass --plist to update it automatically:

  asc encryption declarations exempt-declare --plist ./MyApp/Info.plist

This eliminates the encryption compliance dialog on each TestFlight and
App Store submission. Most apps that only use HTTPS/TLS qualify as exempt.

This command only updates local project metadata. It does not create or attach
an App Store Connect encryption declaration.

For uploaded builds in App Store Connect, verify readiness with:
  asc validate --app "APP_ID" --version "1.0"

If a build still reports non-exempt encryption incorrectly, update the build:
  asc builds update --build-id "BUILD_ID" --uses-non-exempt-encryption=false

For details, see:
  https://developer.apple.com/documentation/bundleresources/information-property-list/itsappusesnonexemptencryption`)

			return nil
		},
	}
}

func updatePlistExemption(stderr io.Writer, plistPath string) error {
	displayPath := strings.TrimSpace(plistPath)
	if displayPath == "" {
		return fmt.Errorf("encryption declarations exempt-declare: plist path is required")
	}

	safePath, err := validatePlistPathNoSymlinkComponents(displayPath)
	if err != nil {
		return err
	}

	file, err := shared.OpenExistingNoFollow(safePath)
	if err != nil {
		return fmt.Errorf("encryption declarations exempt-declare: failed to read: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("encryption declarations exempt-declare: failed to stat: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("encryption declarations exempt-declare: %q is a directory", displayPath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("encryption declarations exempt-declare: refusing to read non-regular file %q", displayPath)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("encryption declarations exempt-declare: failed to read: %w", err)
	}

	var payload map[string]any
	format, err := plist.Unmarshal(data, &payload)
	if err != nil {
		return fmt.Errorf("encryption declarations exempt-declare: failed to decode plist: %w", err)
	}

	if payload == nil {
		payload = make(map[string]any)
	}

	if current, ok := payload["ITSAppUsesNonExemptEncryption"].(bool); ok && !current {
		fmt.Fprintf(stderr, "Info.plist already sets ITSAppUsesNonExemptEncryption=false: %s\n", displayPath)
		return nil
	}

	_, existed := payload["ITSAppUsesNonExemptEncryption"]
	payload["ITSAppUsesNonExemptEncryption"] = false

	updated, err := plist.Marshal(payload, format)
	if err != nil {
		return fmt.Errorf("encryption declarations exempt-declare: failed to encode plist: %w", err)
	}

	if _, err := shared.WriteFileNoSymlinkOverwrite(
		safePath,
		bytes.NewReader(updated),
		info.Mode().Perm(),
		".asc-plist-*.tmp",
		".asc-plist-*.bak",
	); err != nil {
		return fmt.Errorf("encryption declarations exempt-declare: failed to write: %w", err)
	}

	if existed {
		fmt.Fprintf(stderr, "Updated ITSAppUsesNonExemptEncryption=false in %s\n", displayPath)
		return nil
	}

	fmt.Fprintf(stderr, "Added ITSAppUsesNonExemptEncryption=false to %s\n", displayPath)
	return nil
}

func validatePlistPathNoSymlinkComponents(plistPath string) (string, error) {
	absPath, err := filepath.Abs(plistPath)
	if err != nil {
		return "", fmt.Errorf("encryption declarations exempt-declare: failed to resolve plist path: %w", err)
	}

	cleaned := filepath.Clean(absPath)
	volume := filepath.VolumeName(cleaned)
	rest := strings.TrimPrefix(cleaned, volume)
	current := volume
	if strings.HasPrefix(rest, string(os.PathSeparator)) {
		current += string(os.PathSeparator)
	}

	parts := strings.Split(strings.TrimPrefix(rest, string(os.PathSeparator)), string(os.PathSeparator))
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}

		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", fmt.Errorf("encryption declarations exempt-declare: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if i == len(parts)-1 {
				return "", fmt.Errorf("encryption declarations exempt-declare: refusing to read symlink %q", cleaned)
			}
			if isAllowedPlistSymlinkComponent(current) {
				continue
			}
			return "", fmt.Errorf("encryption declarations exempt-declare: refusing to follow symlink component %q", current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("encryption declarations exempt-declare: path component %q is not a directory", current)
		}
	}

	return cleaned, nil
}

func isAllowedPlistSymlinkComponent(path string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	switch filepath.Clean(path) {
	case "/var", "/tmp", "/etc":
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return false
		}
		expected := filepath.Join("/private", strings.TrimPrefix(filepath.Clean(path), "/"))
		return filepath.Clean(resolved) == expected
	default:
		return false
	}
}

func newEncryptionDeclarationsAssignBuildsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("encryption declarations assign-builds", flag.ExitOnError)

	declarationID := shared.BindResourceIDFlag(fs, "id", "appEncryptionDeclarations", "Encryption declaration ID (required)")
	builds := shared.BindOnceCSVFlag(fs, "build-id", "Build IDs to assign (comma-separated)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "assign-builds",
		ShortUsage: "asc encryption declarations assign-builds --id \"DECL_ID\" --build-id \"BUILD_ID[,BUILD_ID...]\"",
		ShortHelp:  "Assign builds to an encryption declaration.",
		LongHelp: `Assign builds to an encryption declaration.

Examples:
  asc encryption declarations assign-builds --id "DECL_ID" --build-id "BUILD_ID"
  asc encryption declarations assign-builds --id "DECL_ID" --build-id "BUILD_ID1,BUILD_ID2"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			declarationValue := strings.TrimSpace(*declarationID)
			if declarationValue == "" {
				return d.RequiredFlag("--id")
			}

			buildIDs := shared.SplitCSV(builds.String())
			if len(buildIDs) == 0 {
				return d.RequiredFlag("--build-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("encryption declarations assign-builds: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.AddBuildsToAppEncryptionDeclaration(requestCtx, declarationValue, buildIDs); err != nil {
				return fmt.Errorf("encryption declarations assign-builds: failed to assign builds: %w", err)
			}

			fmt.Fprintf(d.IO.ErrOut, "Successfully assigned %d build(s) to declaration %s\n", len(buildIDs), declarationValue)
			result := &asc.AppEncryptionDeclarationBuildsUpdateResult{
				DeclarationID: declarationValue,
				BuildIDs:      buildIDs,
				Action:        "assigned",
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func newEncryptionDocumentsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "documents",
		ShortUsage: "asc encryption documents <subcommand> [flags]",
		ShortHelp:  "Manage encryption declaration documents.",
		LongHelp: `Manage encryption declaration documents.

Examples:
  asc encryption documents view --id "DOC_ID"
  asc encryption documents upload --declaration "DECL_ID" --file ./export.pdf`,
		FlagSet:   d.NewFlagSet("documents", flag.ExitOnError),
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newEncryptionDocumentsGetCommand(d),
			newEncryptionDocumentsUploadCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newEncryptionDocumentsGetCommand(d *shared.Deps) *ffcli.Command {
	var fieldsValue []string

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "encryption documents view",
		Name:        "view",
		ShortUsage:  "asc encryption documents view --id \"DOC_ID\"",
		ShortHelp:   "View an encryption declaration document by ID.",
		LongHelp: `View an encryption declaration document by ID.

Examples:
  asc encryption documents view --id "DOC_ID"`,
		IDUsage:            "Document ID (required)",
		IDType:             "appEncryptionDeclarationDocuments",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(encryptionDocumentFieldList(), ", "))
			return func() (err error) {
				if fieldsValue, err = normalizeEncryptionDocumentFields(*fields, "--fields"); err != nil {
					return d.UsageErrorf("encryption documents view: %v", err)
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEncryptionDeclarationDocument(ctx, id, fieldsValue)
		},
	})
}

func newEncryptionDocumentsUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("encryption documents upload", flag.ExitOnError)

	declarationID := shared.BindResourceIDFlag(fs, "declaration", "appEncryptionDeclarations", "Encryption declaration ID (required)")
	filePath := fs.String("file", "", "Path to document file (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc encryption documents upload --declaration \"DECL_ID\" --file ./export.pdf",
		ShortHelp:  "Upload an encryption declaration document.",
		LongHelp: `Upload an encryption declaration document.

Examples:
  asc encryption documents upload --declaration "DECL_ID" --file ./export.pdf`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			declarationValue := strings.TrimSpace(*declarationID)
			if declarationValue == "" {
				return d.RequiredFlag("--declaration")
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				return d.RequiredFlag("--file")
			}

			info, err := os.Lstat(pathValue)
			if err != nil {
				return fmt.Errorf("encryption documents upload: %w", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("encryption documents upload: refusing to read symlink %q", pathValue)
			}
			if info.IsDir() {
				return fmt.Errorf("encryption documents upload: %q is a directory", pathValue)
			}
			if info.Size() <= 0 {
				return fmt.Errorf("encryption documents upload: file size must be greater than 0")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("encryption documents upload: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppEncryptionDeclarationDocument(requestCtx, declarationValue, filepath.Base(pathValue), info.Size())
			if err != nil {
				return fmt.Errorf("encryption documents upload: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("encryption documents upload: no upload operations returned")
			}

			uploadCtx, uploadCancel := d.UploadTimeout(ctx)
			err = asc.ExecuteUploadOperations(uploadCtx, pathValue, resp.Data.Attributes.UploadOperations)
			uploadCancel()
			if err != nil {
				return fmt.Errorf("encryption documents upload: upload failed: %w", err)
			}

			checksum, err := asc.ComputeFileChecksum(pathValue, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("encryption documents upload: checksum failed: %w", err)
			}

			uploaded := true
			updateAttrs := asc.AppEncryptionDeclarationDocumentUpdateAttributes{
				SourceFileChecksum: &checksum.Hash,
				Uploaded:           &uploaded,
			}

			commitCtx, commitCancel := d.UploadTimeout(ctx)
			commitResp, err := client.UpdateAppEncryptionDeclarationDocument(commitCtx, resp.Data.ID, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("encryption documents upload: failed to commit upload: %w", err)
			}

			return d.PrintOutput(commitResp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeEncryptionDeclarationFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range encryptionDeclarationFieldList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(encryptionDeclarationFieldList(), ", "))
		}
	}
	return fields, nil
}

func normalizeEncryptionDocumentFields(value, flagName string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range encryptionDocumentFieldList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(encryptionDocumentFieldList(), ", "))
		}
	}
	return fields, nil
}

func normalizeEncryptionDeclarationInclude(value string) ([]string, error) {
	include := shared.SplitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, item := range encryptionDeclarationIncludeList() {
		allowed[item] = struct{}{}
	}
	for _, item := range include {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(encryptionDeclarationIncludeList(), ", "))
		}
	}
	return include, nil
}

func encryptionDeclarationFieldList() []string {
	return []string{
		"appDescription",
		"createdDate",
		"usesEncryption",
		"exempt",
		"containsProprietaryCryptography",
		"containsThirdPartyCryptography",
		"availableOnFrenchStore",
		"platform",
		"uploadedDate",
		"documentUrl",
		"documentName",
		"documentType",
		"appEncryptionDeclarationState",
		"codeValue",
		"app",
		"builds",
		"appEncryptionDeclarationDocument",
	}
}

func encryptionDocumentFieldList() []string {
	return []string{
		"fileSize",
		"fileName",
		"assetToken",
		"downloadUrl",
		"sourceFileChecksum",
		"uploadOperations",
		"assetDeliveryState",
	}
}

func encryptionDeclarationIncludeList() []string {
	return []string{
		"app",
		"builds",
		"appEncryptionDeclarationDocument",
	}
}
