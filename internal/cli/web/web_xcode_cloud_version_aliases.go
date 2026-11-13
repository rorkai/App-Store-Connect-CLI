package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"unicode/utf16"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var newVersionAliasIDFn = newUUID

const webVersionAliasMaxNameUTF16 = 40

func webVersionAliasesGroup() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings version-aliases", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "version-aliases",
		ShortUsage: "asc web xcode-cloud settings version-aliases <subcommand> [flags]",
		ShortHelp:  "Manage Xcode Cloud custom version aliases.",
		UsageFunc:  shared.DefaultUsageFunc,
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			webVersionAliasesList(),
			webVersionAliasView(),
			webVersionAliasCreate(),
			webVersionAliasUpdate(),
			webVersionAliasDelete(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func webVersionAliasesList() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings version-aliases list", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web xcode-cloud settings version-aliases list --product-id ID [flags]",
		ShortHelp:  "List up to 100 Xcode Cloud custom version aliases.",
		LongHelp: `WEB SESSION WORKFLOWS

List up to 100 custom version aliases for an Xcode Cloud product. The captured
response contract does not expose continuation metadata, so this command
intentionally does not claim full pagination.



Example:
  asc web xcode-cloud settings version-aliases list --product-id "UUID" --apple-id "user@example.com"`,
		UsageFunc: shared.DefaultUsageFunc,
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return shared.MissingRequiredUsageError("--product-id")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud settings version-aliases list failed: session has no public provider ID")
			}

			response, err := newCIClientFn(session).GetCIVersionAliases(requestCtx, teamID, pid)
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud settings version-aliases list")
			}
			result := &asc.WebXcodeCloudVersionAliasesResult{
				ProductID:      pid,
				VersionAliases: make([]asc.WebXcodeCloudVersionAlias, 0, len(response.Items)),
			}
			for _, item := range response.Items {
				result.VersionAliases = append(result.VersionAliases, webVersionAliasOutput(item))
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return asc.PrintTable(result) },
				func() error { return asc.PrintMarkdown(result) },
			)
		},
	}
}

func webVersionAliasOutput(item webcore.CIVersionAlias) asc.WebXcodeCloudVersionAlias {
	return asc.WebXcodeCloudVersionAlias{
		ID:             item.ID,
		Name:           item.Name,
		Type:           item.Type,
		Locked:         item.Locked,
		BuildName:      item.BuildName,
		BuildSupported: item.BuildSupported,
	}
}

func webVersionAliasResult(productID, action string, item *webcore.CIVersionAlias) *asc.WebXcodeCloudVersionAliasResult {
	result := &asc.WebXcodeCloudVersionAliasResult{ProductID: productID, Action: action}
	if item == nil {
		return result
	}
	result.ID = item.ID
	result.Name = item.Name
	result.Type = item.Type
	result.Locked = item.Locked
	result.BuildName = item.BuildName
	result.BuildSupported = item.BuildSupported
	return result
}

func webVersionAliasDeleteResult(productID, aliasID string) *asc.WebXcodeCloudVersionAliasDeleteResult {
	return &asc.WebXcodeCloudVersionAliasDeleteResult{
		ProductID: productID,
		ID:        aliasID,
		Deleted:   true,
	}
}

func webVersionAliasType(value string) (string, error) {
	typeValue := strings.ToLower(strings.TrimSpace(value))
	switch typeValue {
	case "macos_version", "xcode_version":
		return typeValue, nil
	case "":
		return "", fmt.Errorf("type is required")
	default:
		return "", fmt.Errorf("type must be macos_version or xcode_version")
	}
}

func webVersionAliasName(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("must not be blank")
	}
	if len(utf16.Encode([]rune(value))) > webVersionAliasMaxNameUTF16 {
		return "", fmt.Errorf("must be at most %d UTF-16 code units", webVersionAliasMaxNameUTF16)
	}
	return strings.TrimSpace(value), nil
}

func webVersionAliasNameFlag(flagName, value string) (string, error) {
	name, err := webVersionAliasName(value)
	if err == nil {
		return name, nil
	}
	if strings.TrimSpace(value) == "" {
		fmt.Fprintf(os.Stderr, "Error: --%s is required\n", flagName)
		return "", shared.MissingRequiredUsageError("--" + flagName)
	}
	return "", shared.UsageError(fmt.Sprintf("--%s %v", flagName, err))
}

func webVersionAliasBuildPresent(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return false
	}
	var stringValue string
	if json.Unmarshal(trimmed, &stringValue) == nil {
		return stringValue != ""
	}
	return false
}

func webVersionAliasEffectiveValues(request webcore.CIVersionAliasRequest) (string, error) {
	name, err := webVersionAliasName(request.Name)
	if err != nil {
		return "", fmt.Errorf("existing version alias name %w", err)
	}
	if !webVersionAliasBuildPresent(request.Build) {
		return "", fmt.Errorf("existing version alias build must be a nonempty string; pass --build to replace it")
	}
	return name, nil
}

func webVersionAliasBuild(value string) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return json.RawMessage(encoded)
}

func webVersionAliasRequestFromItem(item *webcore.CIVersionAlias) webcore.CIVersionAliasRequest {
	if item == nil {
		return webcore.CIVersionAliasRequest{Build: webVersionAliasBuild("")}
	}
	return webcore.CIVersionAliasRequest{
		Name:   item.Name,
		Type:   item.Type,
		Build:  append(json.RawMessage(nil), item.Build...),
		Locked: item.Locked,
	}
}

func webVersionAliasMatches(item *webcore.CIVersionAlias, request webcore.CIVersionAliasRequest) bool {
	if item == nil {
		return false
	}
	return item.Name == request.Name &&
		item.Type == request.Type &&
		item.Locked == request.Locked &&
		webVersionAliasJSONEqual(item.Build, request.Build)
}

func webVersionAliasJSONEqual(left, right json.RawMessage) bool {
	if len(bytes.TrimSpace(left)) == 0 && len(bytes.TrimSpace(right)) == 0 {
		return true
	}
	var leftValue any
	var rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return bytes.Equal(bytes.TrimSpace(left), bytes.TrimSpace(right))
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func versionAliasFlagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(value *flag.Flag) {
		provided = provided || value.Name == name
	})
	return provided
}

func webVersionAliasTeamID(session *webcore.AuthSession, operation string) (string, error) {
	teamID := strings.TrimSpace(session.PublicProviderID)
	if teamID == "" {
		return "", fmt.Errorf("xcode-cloud settings version-aliases %s failed: session has no public provider ID", operation)
	}
	return teamID, nil
}

func webVersionAliasPrint(result *asc.WebXcodeCloudVersionAliasResult, output *string, pretty *bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		*output,
		*pretty,
		func() error { return asc.PrintTable(result) },
		func() error { return asc.PrintMarkdown(result) },
	)
}

// webVersionAliasViewOutput keeps JSON detail reads at Apple's response
// boundary while reusing the safe scalar renderers for human output.
type webVersionAliasViewOutput struct {
	raw    json.RawMessage
	scalar *asc.WebXcodeCloudVersionAliasResult
}

func (v webVersionAliasViewOutput) MarshalJSON() ([]byte, error) {
	if len(bytes.TrimSpace(v.raw)) == 0 {
		return []byte("null"), nil
	}
	if !json.Valid(v.raw) {
		return nil, fmt.Errorf("invalid raw version alias response")
	}
	return v.raw, nil
}

func webVersionAliasViewPrint(raw json.RawMessage, result *asc.WebXcodeCloudVersionAliasResult, output *string, pretty *bool) error {
	view := webVersionAliasViewOutput{
		raw:    append(json.RawMessage(nil), raw...),
		scalar: result,
	}
	return shared.PrintOutputWithRenderers(
		view,
		*output,
		*pretty,
		func() error { return asc.PrintTable(view.scalar) },
		func() error { return asc.PrintMarkdown(view.scalar) },
	)
}

func webVersionAliasDeletePrint(result *asc.WebXcodeCloudVersionAliasDeleteResult, output *string, pretty *bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		*output,
		*pretty,
		func() error { return asc.PrintTable(result) },
		func() error { return asc.PrintMarkdown(result) },
	)
}

func webVersionAliasView() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings version-aliases view", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	aliasID := fs.String("id", "", "Version alias ID (required)")

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web xcode-cloud settings version-aliases view --product-id ID --id ID [flags]",
		ShortHelp:  "View one Xcode Cloud custom version alias.",
		LongHelp: `WEB SESSION WORKFLOWS

View one custom version alias for an Xcode Cloud product. JSON output preserves
Apple's raw detail response, including nested build and workflow payloads;
table and markdown output render safe scalar fields.

Example:
  asc web xcode-cloud settings version-aliases view --product-id "UUID" --id "ALIAS_UUID" --apple-id "user@example.com"`,
		UsageFunc: shared.DefaultUsageFunc,
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return shared.MissingRequiredUsageError("--product-id")
			}
			id := strings.TrimSpace(*aliasID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID, err := webVersionAliasTeamID(session, "view")
			if err != nil {
				return err
			}
			raw, item, err := newCIClientFn(session).GetCIVersionAliasRaw(requestCtx, teamID, pid, id)
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud settings version-aliases view")
			}
			return webVersionAliasViewPrint(raw, webVersionAliasResult(pid, "", item), output.Output, output.Pretty)
		},
	}
}

func webVersionAliasCreate() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings version-aliases create", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	name := fs.String("name", "", "Version alias name (required; maximum 40 UTF-16 code units)")
	typeValue := fs.String("type", "", "Version alias type: macos_version or xcode_version (required)")
	build := fs.String("build", "", "Version alias build value (required)")
	locked := fs.Bool("locked", false, "Create the alias as locked")
	confirm := fs.Bool("confirm", false, "Confirm creating the version alias (required)")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web xcode-cloud settings version-aliases create --product-id ID --name NAME --type TYPE --build BUILD --confirm [flags]",
		ShortHelp:  "Create an Xcode Cloud custom version alias.",
		LongHelp: `WEB SESSION WORKFLOWS

Create a custom version alias. Apple supports macos_version and xcode_version
aliases. Name must be nonblank and at most 40 UTF-16 code units; the value sent
to Apple is trimmed. Build must be nonempty, while Apple validates whether the
build is supported by the product. Apple may require the
can_configure_locked_version_aliases capability for locked aliases.

Example:
  asc web xcode-cloud settings version-aliases create --product-id "UUID" --type xcode_version --name "Stable" --build "latest:stable" --confirm --apple-id "user@example.com"`,
		UsageFunc: shared.DefaultUsageFunc,
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return shared.MissingRequiredUsageError("--product-id")
			}
			normalizedName, err := webVersionAliasNameFlag("name", *name)
			if err != nil {
				return err
			}
			if *build == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return shared.MissingRequiredUsageError("--build")
			}
			normalizedType, err := webVersionAliasType(*typeValue)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: --type %v\n", err)
				return shared.UsageError("--type " + err.Error())
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID, err := webVersionAliasTeamID(session, "create")
			if err != nil {
				return err
			}
			aliasID := strings.TrimSpace(newVersionAliasIDFn())
			if aliasID == "" {
				return fmt.Errorf("xcode-cloud settings version-aliases create failed: generated empty version alias ID")
			}
			request := webcore.CIVersionAliasRequest{
				Name:   normalizedName,
				Type:   normalizedType,
				Build:  webVersionAliasBuild(*build),
				Locked: *locked,
			}
			client := newCIClientFn(session)
			if err := client.PutCIVersionAlias(requestCtx, teamID, pid, aliasID, request); err != nil {
				return verifyVersionAliasWrite(ctx, client, teamID, pid, aliasID, request, "create", err, output.Output, output.Pretty)
			}
			item, err := client.GetCIVersionAlias(requestCtx, teamID, pid, aliasID)
			if err != nil {
				return fmt.Errorf("xcode-cloud settings version-aliases create may have succeeded but verification failed: %w", withWebAuthHint(err, "xcode-cloud settings version-aliases create"))
			}
			if !webVersionAliasMatches(item, request) {
				return fmt.Errorf("xcode-cloud settings version-aliases create verification failed: remote alias does not match the requested values")
			}
			return webVersionAliasPrint(webVersionAliasResult(pid, "created", item), output.Output, output.Pretty)
		},
	}
}

func webVersionAliasUpdate() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings version-aliases update", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	aliasID := fs.String("id", "", "Version alias ID (required)")
	name := fs.String("name", "", "Replacement version alias name (maximum 40 UTF-16 code units)")
	typeValue := fs.String("type", "", "Replacement version alias type: macos_version or xcode_version")
	build := fs.String("build", "", "Replacement version alias build value")
	locked := fs.Bool("locked", false, "Replacement locked state (true or false)")
	confirm := fs.Bool("confirm", false, "Confirm updating the version alias (required)")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc web xcode-cloud settings version-aliases update --product-id ID --id ID [fields] --confirm [flags]",
		ShortHelp:  "Update an Xcode Cloud custom version alias.",
		LongHelp: `WEB SESSION WORKFLOWS

Update selected fields on one custom version alias. The command reads the
existing alias first and sends Apple's complete {name,type,build,locked} body,
preserving fields whose flags were omitted. The alias name is trimmed before
writing. Apple may restrict changes to locked aliases or aliases used by
workflows.

Example:
  asc web xcode-cloud settings version-aliases update --product-id "UUID" --id "ALIAS_UUID" --name "Stable" --confirm --apple-id "user@example.com"`,
		UsageFunc: shared.DefaultUsageFunc,
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return shared.MissingRequiredUsageError("--product-id")
			}
			id := strings.TrimSpace(*aliasID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			fieldProvided := versionAliasFlagProvided(fs, "name") || versionAliasFlagProvided(fs, "type") || versionAliasFlagProvided(fs, "build") || versionAliasFlagProvided(fs, "locked")
			if !fieldProvided {
				fmt.Fprintln(os.Stderr, "Error: at least one of --name, --type, --build, or --locked is required")
				return shared.UsageError("at least one version alias field is required")
			}
			var normalizedName string
			if versionAliasFlagProvided(fs, "name") {
				var nameErr error
				normalizedName, nameErr = webVersionAliasNameFlag("name", *name)
				if nameErr != nil {
					return nameErr
				}
			}
			if versionAliasFlagProvided(fs, "build") && *build == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return shared.MissingRequiredUsageError("--build")
			}
			var normalizedType string
			if versionAliasFlagProvided(fs, "type") {
				var typeErr error
				normalizedType, typeErr = webVersionAliasType(*typeValue)
				if typeErr != nil {
					fmt.Fprintf(os.Stderr, "Error: --type %v\n", typeErr)
					return shared.UsageError("--type " + typeErr.Error())
				}
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID, err := webVersionAliasTeamID(session, "update")
			if err != nil {
				return err
			}
			client := newCIClientFn(session)
			current, err := client.GetCIVersionAlias(requestCtx, teamID, pid, id)
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud settings version-aliases update")
			}
			request := webVersionAliasRequestFromItem(current)
			if versionAliasFlagProvided(fs, "name") {
				request.Name = normalizedName
			}
			if versionAliasFlagProvided(fs, "type") {
				request.Type = normalizedType
			}
			if versionAliasFlagProvided(fs, "build") {
				request.Build = webVersionAliasBuild(*build)
			}
			if versionAliasFlagProvided(fs, "locked") {
				request.Locked = *locked
			}
			normalizedEffectiveName, err := webVersionAliasEffectiveValues(request)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			request.Name = normalizedEffectiveName
			if err := client.PutCIVersionAlias(requestCtx, teamID, pid, id, request); err != nil {
				return verifyVersionAliasWrite(ctx, client, teamID, pid, id, request, "update", err, output.Output, output.Pretty)
			}
			updated, err := client.GetCIVersionAlias(requestCtx, teamID, pid, id)
			if err != nil {
				return fmt.Errorf("xcode-cloud settings version-aliases update may have succeeded but verification failed: %w", withWebAuthHint(err, "xcode-cloud settings version-aliases update"))
			}
			if !webVersionAliasMatches(updated, request) {
				return fmt.Errorf("xcode-cloud settings version-aliases update verification failed: remote alias does not match the requested values")
			}
			return webVersionAliasPrint(webVersionAliasResult(pid, "updated", updated), output.Output, output.Pretty)
		},
	}
}

func webVersionAliasDelete() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud settings version-aliases delete", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	aliasID := fs.String("id", "", "Version alias ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deleting the version alias (required)")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web xcode-cloud settings version-aliases delete --product-id ID --id ID --confirm [flags]",
		ShortHelp:  "Delete an Xcode Cloud custom version alias.",
		LongHelp: `WEB SESSION WORKFLOWS

Delete one custom version alias. The command requires --confirm and verifies
the deletion with a fresh detail read; a confirmed not-found response is the
success postcondition. Apple may restrict deletion of locked aliases or
aliases used by workflows.

Example:
  asc web xcode-cloud settings version-aliases delete --product-id "UUID" --id "ALIAS_UUID" --confirm --apple-id "user@example.com"`,
		UsageFunc: shared.DefaultUsageFunc,
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return shared.MissingRequiredUsageError("--product-id")
			}
			id := strings.TrimSpace(*aliasID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID, err := webVersionAliasTeamID(session, "delete")
			if err != nil {
				return err
			}
			client := newCIClientFn(session)
			writeErr := client.DeleteCIVersionAlias(requestCtx, teamID, pid, id)
			if writeErr != nil && !isAmbiguousCIWriteFailure(writeErr) {
				return withWebAuthHint(writeErr, "xcode-cloud settings version-aliases delete")
			}
			verifyCtx := requestCtx
			verifyCancel := func() {}
			if writeErr != nil {
				verifyCtx, verifyCancel = newWebRequestContext(ctx)
			}
			defer verifyCancel()
			_, verifyErr := client.GetCIVersionAlias(verifyCtx, teamID, pid, id)
			if isCIVersionAliasNotFound(verifyErr) {
				return webVersionAliasDeletePrint(webVersionAliasDeleteResult(pid, id), output.Output, output.Pretty)
			}
			if verifyErr != nil {
				if writeErr != nil {
					return fmt.Errorf("xcode-cloud settings version-aliases delete may have succeeded but reconciliation failed: write error: %w; re-read error: %w", writeErr, verifyErr)
				}
				return fmt.Errorf("xcode-cloud settings version-aliases delete may have succeeded but verification failed: %w", withWebAuthHint(verifyErr, "xcode-cloud settings version-aliases delete verification"))
			}
			if writeErr != nil {
				return fmt.Errorf("xcode-cloud settings version-aliases delete is unverified: alias %s still exists: %w", id, writeErr)
			}
			return fmt.Errorf("xcode-cloud settings version-aliases delete verification failed: alias %s still exists", id)
		},
	}
}

func verifyVersionAliasWrite(ctx context.Context, client *webcore.Client, teamID, productID, aliasID string, request webcore.CIVersionAliasRequest, operation string, writeErr error, output *string, pretty *bool) error {
	if !isAmbiguousCIWriteFailure(writeErr) {
		return withWebAuthHint(writeErr, "xcode-cloud settings version-aliases "+operation)
	}
	verifyCtx, verifyCancel := newWebRequestContext(ctx)
	defer verifyCancel()
	item, verifyErr := client.GetCIVersionAlias(verifyCtx, teamID, productID, aliasID)
	if verifyErr != nil {
		return fmt.Errorf("xcode-cloud settings version-aliases %s may have succeeded but reconciliation failed: write error: %w; re-read error: %w", operation, writeErr, verifyErr)
	}
	if !webVersionAliasMatches(item, request) {
		return fmt.Errorf("xcode-cloud settings version-aliases %s is unverified: remote alias does not match the requested values: %w", operation, writeErr)
	}
	return webVersionAliasPrint(webVersionAliasResult(productID, operation+"d", item), output, pretty)
}

func isCIVersionAliasNotFound(err error) bool {
	var apiErr *webcore.APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// isAmbiguousCIWriteFailure reports failures where the request reached the
// transport but no definitive response established whether Apple applied the write.
func isAmbiguousCIWriteFailure(err error) bool {
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusRequestTimeout || apiErr.Status >= http.StatusInternalServerError
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}
