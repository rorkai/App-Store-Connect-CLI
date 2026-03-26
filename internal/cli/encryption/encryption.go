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

// EncryptionCommand returns the encryption command group.
func EncryptionCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "encryption",
		ShortUsage: "asc encryption <subcommand> [flags]",
		ShortHelp:  "Manage app encryption declarations and documents.",
		LongHelp: `Manage app encryption declarations and documents.

Examples:
  asc encryption declarations list --app "APP_ID"
  asc encryption declarations get --id "DECL_ID"
  asc encryption declarations create --app "APP_ID" --app-description "Uses TLS" --contains-proprietary-cryptography=false --contains-third-party-cryptography=true --available-on-french-store=true
  asc encryption declarations exempt-declare --plist ./Info.plist
  asc encryption declarations assign-builds --id "DECL_ID" --build "BUILD_ID"
  asc encryption documents get --id "DOC_ID"
  asc encryption documents upload --declaration "DECL_ID" --file ./export.pdf`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			EncryptionDeclarationsCommand(),
			EncryptionDocumentsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// EncryptionDeclarationsCommand returns the declarations subcommand group.
func EncryptionDeclarationsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "declarations",
		ShortUsage: "asc encryption declarations <subcommand> [flags]",
		ShortHelp:  "Manage app encryption declarations.",
		LongHelp: `Manage app encryption declarations.

Examples:
  asc encryption declarations list --app "APP_ID"
  asc encryption declarations get --id "DECL_ID"
  asc encryption declarations create --app "APP_ID" --app-description "Uses TLS" --contains-proprietary-cryptography=false --contains-third-party-cryptography=true --available-on-french-store=true
  asc encryption declarations exempt-declare --plist ./Info.plist
  asc encryption declarations assign-builds --id "DECL_ID" --build "BUILD_ID"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			EncryptionDeclarationsListCommand(),
			EncryptionDeclarationsGetCommand(),
			EncryptionDeclarationsAppCommand(),
			EncryptionDeclarationsDeclarationDocumentCommand(),
			EncryptionDeclarationsCreateCommand(),
			EncryptionDeclarationsExemptDeclareCommand(),
			EncryptionDeclarationsAssignBuildsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// EncryptionDeclarationsListCommand returns the declarations list subcommand.
func EncryptionDeclarationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	builds := fs.String("build", "", "Filter by build IDs (comma-separated)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(encryptionDeclarationFieldList(), ", "))
	documentFields := fs.String("document-fields", "", "Document fields to include: "+strings.Join(encryptionDocumentFieldList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(encryptionDeclarationIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	buildLimit := fs.Int("build-limit", 0, "Maximum included builds per declaration (1-50)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc encryption declarations list --app \"APP_ID\" [flags]",
		ShortHelp:  "List encryption declarations for an app.",
		LongHelp: `List encryption declarations for an app.

Examples:
  asc encryption declarations list --app "APP_ID"
  asc encryption declarations list --app "APP_ID" --include appEncryptionDeclarationDocument --document-fields "fileName,fileSize"
  asc encryption declarations list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("encryption declarations list: --limit must be between 1 and 200")
			}
			if *buildLimit != 0 && (*buildLimit < 1 || *buildLimit > 50) {
				return fmt.Errorf("encryption declarations list: --build-limit must be between 1 and 50")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("encryption declarations list: %w", err)
			}

			fieldsValue, err := normalizeEncryptionDeclarationFields(*fields)
			if err != nil {
				return fmt.Errorf("encryption declarations list: %w", err)
			}
			documentFieldsValue, err := normalizeEncryptionDocumentFields(*documentFields, "--document-fields")
			if err != nil {
				return fmt.Errorf("encryption declarations list: %w", err)
			}
			includeValue, err := normalizeEncryptionDeclarationInclude(*include)
			if err != nil {
				return fmt.Errorf("encryption declarations list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			buildIDs := shared.SplitCSV(*builds)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption declarations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppEncryptionDeclarationsOption{
				asc.WithAppEncryptionDeclarationsBuildIDs(buildIDs),
				asc.WithAppEncryptionDeclarationsFields(fieldsValue),
				asc.WithAppEncryptionDeclarationsDocumentFields(documentFieldsValue),
				asc.WithAppEncryptionDeclarationsInclude(includeValue),
				asc.WithAppEncryptionDeclarationsLimit(*limit),
				asc.WithAppEncryptionDeclarationsBuildLimit(*buildLimit),
				asc.WithAppEncryptionDeclarationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppEncryptionDeclarationsLimit(200))
				firstPage, err := client.GetAppEncryptionDeclarations(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("encryption declarations list: failed to fetch: %w", err)
				}

				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEncryptionDeclarations(ctx, resolvedAppID, asc.WithAppEncryptionDeclarationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("encryption declarations list: %w", err)
				}

				return shared.PrintOutput(pages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEncryptionDeclarations(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("encryption declarations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EncryptionDeclarationsGetCommand returns the declarations get subcommand.
func EncryptionDeclarationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations get", flag.ExitOnError)

	declarationID := fs.String("id", "", "Encryption declaration ID (required)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(encryptionDeclarationFieldList(), ", "))
	documentFields := fs.String("document-fields", "", "Document fields to include: "+strings.Join(encryptionDocumentFieldList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(encryptionDeclarationIncludeList(), ", "))
	buildLimit := fs.Int("build-limit", 0, "Maximum included builds (1-50)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc encryption declarations get --id \"DECL_ID\"",
		ShortHelp:  "Get an encryption declaration by ID.",
		LongHelp: `Get an encryption declaration by ID.

Examples:
  asc encryption declarations get --id "DECL_ID"
  asc encryption declarations get --id "DECL_ID" --include appEncryptionDeclarationDocument --document-fields "fileName,fileSize"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			declarationValue := strings.TrimSpace(*declarationID)
			if declarationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *buildLimit != 0 && (*buildLimit < 1 || *buildLimit > 50) {
				return fmt.Errorf("encryption declarations get: --build-limit must be between 1 and 50")
			}

			fieldsValue, err := normalizeEncryptionDeclarationFields(*fields)
			if err != nil {
				return fmt.Errorf("encryption declarations get: %w", err)
			}
			documentFieldsValue, err := normalizeEncryptionDocumentFields(*documentFields, "--document-fields")
			if err != nil {
				return fmt.Errorf("encryption declarations get: %w", err)
			}
			includeValue, err := normalizeEncryptionDeclarationInclude(*include)
			if err != nil {
				return fmt.Errorf("encryption declarations get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption declarations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEncryptionDeclaration(requestCtx, declarationValue,
				asc.WithAppEncryptionDeclarationsFields(fieldsValue),
				asc.WithAppEncryptionDeclarationsDocumentFields(documentFieldsValue),
				asc.WithAppEncryptionDeclarationsInclude(includeValue),
				asc.WithAppEncryptionDeclarationsBuildLimit(*buildLimit),
			)
			if err != nil {
				return fmt.Errorf("encryption declarations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EncryptionDeclarationsCreateCommand returns the declarations create subcommand.
func EncryptionDeclarationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	appDescription := fs.String("app-description", "", "Description of encryption usage (required)")
	containsProprietary := fs.Bool("contains-proprietary-cryptography", false, "App contains proprietary cryptography (required)")
	containsThirdParty := fs.Bool("contains-third-party-cryptography", false, "App contains third-party cryptography (required)")
	availableOnFrenchStore := fs.Bool("available-on-french-store", false, "App is available on the French store (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc encryption declarations create --app \"APP_ID\" [flags]",
		ShortHelp:  "Create a new encryption declaration.",
		LongHelp: `Create a new encryption declaration.

Examples:
  asc encryption declarations create --app "APP_ID" --app-description "Uses TLS" --contains-proprietary-cryptography=false --contains-third-party-cryptography=true --available-on-french-store=true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			descriptionValue := strings.TrimSpace(*appDescription)
			if descriptionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-description is required")
				return flag.ErrHelp
			}
			if !visited["contains-proprietary-cryptography"] {
				fmt.Fprintln(os.Stderr, "Error: --contains-proprietary-cryptography is required")
				return flag.ErrHelp
			}
			if !visited["contains-third-party-cryptography"] {
				fmt.Fprintln(os.Stderr, "Error: --contains-third-party-cryptography is required")
				return flag.ErrHelp
			}
			if !visited["available-on-french-store"] {
				fmt.Fprintln(os.Stderr, "Error: --available-on-french-store is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption declarations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EncryptionDeclarationsExemptDeclareCommand returns the exempt-declare subcommand.
func EncryptionDeclarationsExemptDeclareCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations exempt-declare", flag.ExitOnError)

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
"asc submit preflight" to verify ASC-side encryption state.

Examples:
  asc encryption declarations exempt-declare
  asc encryption declarations exempt-declare --plist ./MyApp/Info.plist`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("encryption declarations exempt-declare does not accept positional arguments")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			plistValue := strings.TrimSpace(*plistPath)

			if visited["plist"] {
				if plistValue == "" {
					return shared.UsageError("--plist must not be empty")
				}
				return updatePlistExemption(plistValue)
			}

			fmt.Fprintln(os.Stderr, `To declare encryption exemption, add the following to your Info.plist:

  <key>ITSAppUsesNonExemptEncryption</key>
  <false/>

Or pass --plist to update it automatically:

  asc encryption declarations exempt-declare --plist ./MyApp/Info.plist

This eliminates the encryption compliance dialog on each TestFlight and
App Store submission. Most apps that only use HTTPS/TLS qualify as exempt.

This command only updates local project metadata. It does not create or attach
an App Store Connect encryption declaration.

For uploaded builds in App Store Connect, verify encryption state with:
  asc submit preflight --app "APP_ID" --version "1.0"

If a build still reports non-exempt encryption incorrectly, update the build:
  asc builds update --build "BUILD_ID" --uses-non-exempt-encryption=false

For details, see:
  https://developer.apple.com/documentation/bundleresources/information-property-list/itsappusesnonexemptencryption`)

			return nil
		},
	}
}

func updatePlistExemption(plistPath string) error {
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
		fmt.Fprintf(os.Stderr, "Info.plist already sets ITSAppUsesNonExemptEncryption=false: %s\n", displayPath)
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
		fmt.Fprintf(os.Stderr, "Updated ITSAppUsesNonExemptEncryption=false in %s\n", displayPath)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Added ITSAppUsesNonExemptEncryption=false to %s\n", displayPath)
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

// EncryptionDeclarationsAssignBuildsCommand returns the declarations assign-builds subcommand.
func EncryptionDeclarationsAssignBuildsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations assign-builds", flag.ExitOnError)

	declarationID := fs.String("id", "", "Encryption declaration ID (required)")
	builds := fs.String("build", "", "Build IDs to assign (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "assign-builds",
		ShortUsage: "asc encryption declarations assign-builds --id \"DECL_ID\" --build \"BUILD_ID[,BUILD_ID...]\"",
		ShortHelp:  "Assign builds to an encryption declaration.",
		LongHelp: `Assign builds to an encryption declaration.

Examples:
  asc encryption declarations assign-builds --id "DECL_ID" --build "BUILD_ID"
  asc encryption declarations assign-builds --id "DECL_ID" --build "BUILD_ID1,BUILD_ID2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			declarationValue := strings.TrimSpace(*declarationID)
			if declarationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			buildIDs := shared.SplitCSV(*builds)
			if len(buildIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption declarations assign-builds: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.AddBuildsToAppEncryptionDeclaration(requestCtx, declarationValue, buildIDs); err != nil {
				return fmt.Errorf("encryption declarations assign-builds: failed to assign builds: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully assigned %d build(s) to declaration %s\n", len(buildIDs), declarationValue)
			result := &asc.AppEncryptionDeclarationBuildsUpdateResult{
				DeclarationID: declarationValue,
				BuildIDs:      buildIDs,
				Action:        "assigned",
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// EncryptionDocumentsCommand returns the documents subcommand group.
func EncryptionDocumentsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "documents",
		ShortUsage: "asc encryption documents <subcommand> [flags]",
		ShortHelp:  "Manage encryption declaration documents.",
		LongHelp: `Manage encryption declaration documents.

Examples:
  asc encryption documents get --id "DOC_ID"
  asc encryption documents upload --declaration "DECL_ID" --file ./export.pdf`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			EncryptionDocumentsGetCommand(),
			EncryptionDocumentsUploadCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// EncryptionDocumentsGetCommand returns the documents get subcommand.
func EncryptionDocumentsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption documents get", flag.ExitOnError)

	documentID := fs.String("id", "", "Document ID (required)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(encryptionDocumentFieldList(), ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc encryption documents get --id \"DOC_ID\"",
		ShortHelp:  "Get an encryption declaration document by ID.",
		LongHelp: `Get an encryption declaration document by ID.

Examples:
  asc encryption documents get --id "DOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			documentValue := strings.TrimSpace(*documentID)
			if documentValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeEncryptionDocumentFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("encryption documents get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption documents get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEncryptionDeclarationDocument(requestCtx, documentValue, fieldsValue)
			if err != nil {
				return fmt.Errorf("encryption documents get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EncryptionDocumentsUploadCommand returns the documents upload subcommand.
func EncryptionDocumentsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption documents upload", flag.ExitOnError)

	declarationID := fs.String("declaration", "", "Encryption declaration ID (required)")
	filePath := fs.String("file", "", "Path to document file (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc encryption documents upload --declaration \"DECL_ID\" --file ./export.pdf",
		ShortHelp:  "Upload an encryption declaration document.",
		LongHelp: `Upload an encryption declaration document.

Examples:
  asc encryption documents upload --declaration "DECL_ID" --file ./export.pdf`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			declarationValue := strings.TrimSpace(*declarationID)
			if declarationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --declaration is required")
				return flag.ErrHelp
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption documents upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppEncryptionDeclarationDocument(requestCtx, declarationValue, filepath.Base(pathValue), info.Size())
			if err != nil {
				return fmt.Errorf("encryption documents upload: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("encryption documents upload: no upload operations returned")
			}

			uploadCtx, uploadCancel := shared.ContextWithUploadTimeout(ctx)
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

			commitCtx, commitCancel := shared.ContextWithUploadTimeout(ctx)
			commitResp, err := client.UpdateAppEncryptionDeclarationDocument(commitCtx, resp.Data.ID, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("encryption documents upload: failed to commit upload: %w", err)
			}

			return shared.PrintOutput(commitResp, *output.Output, *output.Pretty)
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
