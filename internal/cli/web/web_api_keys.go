package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
	webref "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web/reference"
)

const webAPIKeyDownloadAttempts = 3

var (
	newWebAPIKeyClientFn = webcore.NewClient
	createWebAPIKeyFn    = func(ctx context.Context, client *webcore.Client, attrs webcore.APIKeyCreateAttributes) (*webcore.APIKey, error) {
		return client.CreateAPIKey(ctx, attrs)
	}
	downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
		return client.DownloadAPIKey(ctx, keyID)
	}
	getWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKey, error) {
		return client.GetAPIKey(ctx, keyID)
	}
	listWebAPIKeysFn = func(ctx context.Context, client *webcore.Client) ([]webcore.APIKeyListItem, error) {
		return client.ListAPIKeys(ctx)
	}
	listWebAPIKeysByKindFn = func(ctx context.Context, client *webcore.Client, kind string) ([]webcore.APIKeyListItem, error) {
		return client.ListAPIKeysByKind(ctx, kind)
	}
	revokeWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID, kind string) error {
		return client.RevokeAPIKey(ctx, keyID, kind)
	}
	waitWebAPIKeyRetryFn = waitForWebAPIKeyRetry
)

type webAPIKeyCreateResult struct {
	KeyID          string   `json:"keyId"`
	Name           string   `json:"name"`
	IssuerID       string   `json:"issuerId"`
	TeamID         string   `json:"teamId,omitempty"`
	Roles          []string `json:"roles"`
	Active         bool     `json:"active"`
	AllAppsVisible bool     `json:"allAppsVisible"`
	KeyType        string   `json:"keyType"`
	P8Path         string   `json:"p8Path"`
}

// WebAPIKeysCommand returns the web-session API keys command group.
func WebAPIKeysCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web api-keys", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "api-keys",
		ShortUsage: "asc web api-keys <subcommand> [flags]",
		ShortHelp:  "Manage App Store Connect API keys via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage App Store Connect API keys using a cached Apple Account web session.

Examples:
  asc web api-keys list --output json
  asc web api-keys view --key-id KEY_ID
  asc web api-keys create --name "Release automation"
  asc web api-keys create-individual --user-id USER_UUID --output-dir ~/.asc/keys --confirm
  asc web api-keys revoke --key-id KEY_ID --type team --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAPIKeysListCommand(),
			WebAPIKeysViewCommand(),
			WebAPIKeysCreateCommand(),
			WebAPIKeysCreateIndividualCommand(),
			WebAPIKeysRevokeCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAPIKeysListCommand lists team and individual API keys visible to the web session.
func WebAPIKeysListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web api-keys list", flag.ExitOnError)

	authFlags := bindWebSessionFlagsWithSessionFromEnv(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web api-keys list [flags]",
		ShortHelp:  "List App Store Connect API keys via a web session.",
		LongHelp: `WEB SESSION WORKFLOWS

List team and individual App Store Connect API keys visible to the Apple Account
web session. Output includes key ID, name, kind, roles, and active state. Creation
date is omitted because the existing key-list readers do not expose it. Key material is never printed. Individual keys may have empty roles
on this list; use "asc web auth capabilities --key-id" to resolve actor-backed
roles for one key.

By default, this command uses the cached web session. Pass --session-from-env
to use ASC_WEB_SESSION in memory without persistence. This mode does not read
or write the session cache or keychain.

The underlying readers already follow every pagination link, so this command
always returns the complete visible set and does not accept --paginate.

Examples:
  asc web api-keys list
  asc web api-keys list --session-from-env --output json
  asc web api-keys list --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebAPIKeyClientFn(session)

			var keys []webcore.APIKeyListItem
			err = withWebSpinner("Loading App Store Connect API keys", func() error {
				var listErr error
				keys, listErr = listWebAPIKeysFn(requestCtx, client)
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web api-keys list")
			}
			if keys == nil {
				keys = []webcore.APIKeyListItem{}
			}

			result := &asc.WebAPIKeysListResult{Keys: make([]asc.WebAPIKeyListItem, 0, len(keys))}
			for _, key := range keys {
				result.Keys = append(result.Keys, webAPIKeyListItemFromClient(key))
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// WebAPIKeysViewCommand inspects one team API key by ID.
func WebAPIKeysViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web api-keys view", flag.ExitOnError)

	keyID := fs.String("key-id", "", "API key ID")
	authFlags := bindWebSessionFlagsWithSessionFromEnv(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web api-keys view --key-id KEY_ID [flags]",
		ShortHelp:  "View one App Store Connect API key via a web session.",
		LongHelp: `WEB SESSION WORKFLOWS

Inspect one team API key by ID using the iris v1 key resource. Output includes
key ID, name, issuer ID, roles, and active state. Creation date is omitted
because the existing GetAPIKey reader does not expose it. Key material is never
printed.

Individual keys appear in "asc web api-keys list" but are not loaded by this
command.

By default, this command uses the cached web session. Pass --session-from-env
to use ASC_WEB_SESSION in memory without persistence. This mode does not read
or write the session cache or keychain.

Examples:
  asc web api-keys view --key-id KEY_ID
  asc web api-keys view --key-id KEY_ID --session-from-env --output json
  asc web api-keys view --key-id KEY_ID --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			keyIDValue := strings.TrimSpace(*keyID)
			if keyIDValue == "" {
				return shared.UsageError("--key-id is required")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebAPIKeyClientFn(session)

			var details *webcore.APIKey
			err = withWebSpinner("Loading App Store Connect API key", func() error {
				var getErr error
				details, getErr = getWebAPIKeyFn(requestCtx, client, keyIDValue)
				return getErr
			})
			if err != nil {
				return withWebAuthHint(err, "web api-keys view")
			}
			if details == nil {
				return fmt.Errorf("web api-keys view failed: response did not include a key")
			}

			result := webAPIKeyGetResultFromClient(details)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

const (
	webAPIKeyRevokeStatusRevoked         = "revoked"
	webAPIKeyRevokeStatusAlreadyInactive = "already_inactive"
)

// WebAPIKeysRevokeCommand revokes one team or individual API key.
func WebAPIKeysRevokeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web api-keys revoke", flag.ExitOnError)

	keyID := fs.String("key-id", "", "API key ID")
	kind := fs.String("type", "", "API key type: team or individual")
	confirm := fs.Bool("confirm", false, "Confirm revoking this API key")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "revoke",
		ShortUsage: "asc web api-keys revoke --key-id KEY_ID --type team|individual --confirm [flags]",
		ShortHelp:  "Revoke a team or individual App Store Connect API key via a web session.",
		LongHelp: `WEB SESSION WORKFLOWS

Revoke one visible App Store Connect API key using the cached Apple Account web
session. The command first loads only the requested key type and fails closed
unless exactly one matching key is present. An already inactive key is a
verified no-op. An active key is revoked with one type-specific web request and
the same key list is reloaded to verify that it is inactive.

The operation requires --confirm. Key material is never printed. Use the
opaque ID returned by "asc web api-keys list" and pass --type team or
--type individual explicitly because the two key families use different web
hosts.

Examples:
  asc web api-keys revoke --key-id KEY_ID --type team --confirm
  asc web api-keys revoke --key-id KEY_ID --type individual --confirm --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			resolvedKeyID := strings.TrimSpace(*keyID)
			if resolvedKeyID == "" {
				return shared.UsageError("--key-id is required")
			}
			resolvedKind, err := normalizeWebAPIKeyKind(*kind)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebAPIKeyClientFn(session)

			var before []webcore.APIKeyListItem
			err = withWebSpinner("Loading API key before revocation", func() error {
				var listErr error
				before, listErr = listWebAPIKeysByKindFn(requestCtx, client, resolvedKind)
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web api-keys revoke")
			}
			selected, err := findWebAPIKeyForRevoke(before, resolvedKeyID, resolvedKind)
			if err != nil {
				return err
			}
			if !selected.Active {
				return shared.PrintOutput(&asc.WebAPIKeyRevokeResult{
					KeyID:   resolvedKeyID,
					Kind:    resolvedKind,
					Changed: false,
					Active:  false,
					Status:  webAPIKeyRevokeStatusAlreadyInactive,
				}, *output.Output, *output.Pretty)
			}

			err = withWebSpinner("Revoking App Store Connect API key", func() error {
				return revokeWebAPIKeyFn(requestCtx, client, resolvedKeyID, resolvedKind)
			})
			if err != nil {
				if isUnknownWebAPIKeyRevokeError(err) {
					return unknownWebAPIKeyRevokeError(err, "the revoke request")
				}
				return withWebAuthHint(err, "web api-keys revoke")
			}

			var after []webcore.APIKeyListItem
			err = withWebSpinner("Verifying API key revocation", func() error {
				var listErr error
				after, listErr = listWebAPIKeysByKindFn(requestCtx, client, resolvedKind)
				return listErr
			})
			if err != nil {
				return unknownWebAPIKeyRevokeError(err, "post-state verification")
			}
			verified, err := findWebAPIKeyForRevoke(after, resolvedKeyID, resolvedKind)
			if err != nil {
				return unknownWebAPIKeyRevokeError(err, "post-state verification")
			}
			if verified.Active {
				return unknownWebAPIKeyRevokeError(
					fmt.Errorf("key %q remains active", resolvedKeyID),
					"post-state verification",
				)
			}

			return shared.PrintOutput(&asc.WebAPIKeyRevokeResult{
				KeyID:   resolvedKeyID,
				Kind:    resolvedKind,
				Changed: true,
				Active:  false,
				Status:  webAPIKeyRevokeStatusRevoked,
			}, *output.Output, *output.Pretty)
		},
	}
}

func isUnknownWebAPIKeyRevokeError(err error) bool {
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status >= http.StatusInternalServerError
	}
	return true
}

func unknownWebAPIKeyRevokeError(err error, phase string) error {
	return fmt.Errorf(
		"web api-keys revoke outcome is unknown after %s; no automatic retry was sent: %w",
		phase,
		withWebAuthHint(err, "web api-keys revoke"),
	)
}

func normalizeWebAPIKeyKind(value string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(value))
	switch kind {
	case webcore.APIKeyKindTeam, webcore.APIKeyKindIndividual:
		return kind, nil
	default:
		return "", fmt.Errorf("--type must be team or individual")
	}
}

func findWebAPIKeyForRevoke(keys []webcore.APIKeyListItem, keyID, kind string) (webcore.APIKeyListItem, error) {
	var selected webcore.APIKeyListItem
	found := false
	for _, key := range keys {
		if strings.TrimSpace(key.KeyID) != keyID {
			continue
		}
		if strings.TrimSpace(key.Kind) != kind {
			return webcore.APIKeyListItem{}, fmt.Errorf("web api-keys revoke failed: key %q was listed as type %q, not %q", keyID, key.Kind, kind)
		}
		if found {
			return webcore.APIKeyListItem{}, fmt.Errorf("web api-keys revoke failed: key %q appeared more than once in the %s key list", keyID, kind)
		}
		selected = key
		found = true
	}
	if !found {
		return webcore.APIKeyListItem{}, fmt.Errorf("web api-keys revoke failed: key %q of type %q was not found", keyID, kind)
	}
	return selected, nil
}

func webAPIKeyListItemFromClient(key webcore.APIKeyListItem) asc.WebAPIKeyListItem {
	item := asc.WebAPIKeyListItem{
		KeyID:    key.KeyID,
		Name:     key.Name,
		Kind:     key.Kind,
		Roles:    append([]string(nil), key.Roles...),
		Active:   key.Active,
		KeyType:  key.KeyType,
		LastUsed: key.LastUsed,
	}
	if key.GeneratedBy != nil {
		item.GeneratedBy = &asc.WebAPIKeyActor{ID: key.GeneratedBy.ID, Name: key.GeneratedBy.Name}
	}
	if key.RevokedBy != nil {
		item.RevokedBy = &asc.WebAPIKeyActor{ID: key.RevokedBy.ID, Name: key.RevokedBy.Name}
	}
	return item
}

func webAPIKeyGetResultFromClient(key *webcore.APIKey) *asc.WebAPIKeyGetResult {
	if key == nil {
		return &asc.WebAPIKeyGetResult{}
	}
	return &asc.WebAPIKeyGetResult{
		KeyID:          key.KeyID,
		Name:           key.Name,
		IssuerID:       key.IssuerID,
		Roles:          append([]string(nil), key.Roles...),
		Active:         key.Active,
		AllAppsVisible: key.AllAppsVisible,
		CanDownload:    key.CanDownload,
		KeyType:        key.KeyType,
		LastUsed:       key.LastUsed,
		RevokingDate:   key.RevokingDate,
	}
}

// WebAPIKeysCreateCommand creates a team API key and saves its one-time P8.
func WebAPIKeysCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web api-keys create", flag.ExitOnError)

	name := fs.String("name", "", "API key name")
	role := fs.String("role", "ADMIN", "API key role")
	outputDir := fs.String("output-dir", ".", "Directory for AuthKey_<KEY_ID>.p8")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web api-keys create --name NAME [--role ROLE] [--output-dir DIR] [flags]",
		ShortHelp:  "Create a team API key and save its one-time P8.",
		LongHelp: `WEB SESSION WORKFLOWS

Create an all-apps App Store Connect team API key using a cached Apple Account
web session. The one-time P8 is saved as AuthKey_<KEY_ID>.p8 with mode 0600.
The P8 contents are never written to command output.

Account Holder or Admin access is required. The role defaults to ADMIN and is a
role identifier such as ADMIN or APP_MANAGER. Matching is case-insensitive:
lowercase input such as app_manager is sent to App Store Connect as
APP_MANAGER. Documented roles that cannot be selected for a team API key are
rejected. Roles missing from the bundled snapshot are sent to App Store Connect
with a warning instead of being rejected client-side.

Examples:
  asc web api-keys create --name "Release automation"
  asc web api-keys create --name "CI uploads" --role APP_MANAGER --output-dir ~/.asc/keys --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return shared.UsageError("--name is required")
			}
			roleValue, err := normalizeWebAPIKeyRole(*role)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			outputDirValue := strings.TrimSpace(*outputDir)
			if outputDirValue == "" {
				return shared.UsageError("--output-dir is required")
			}
			outputRoot, err := rootfs.New(outputDirValue)
			if err != nil {
				return shared.UsageErrorf("invalid --output-dir: %v", err)
			}
			if err := outputRoot.MkdirAll(".", 0o700); err != nil {
				return fmt.Errorf("prepare API key output directory: %w", err)
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebAPIKeyClientFn(session)

			var created *webcore.APIKey
			err = withWebSpinner("Creating App Store Connect API key", func() error {
				var createErr error
				created, createErr = createWebAPIKeyFn(requestCtx, client, webcore.APIKeyCreateAttributes{
					Nickname: nameValue,
					Role:     roleValue,
				})
				return createErr
			})
			if err != nil {
				return withWebAuthHint(err, "web api-keys create")
			}
			if created == nil || strings.TrimSpace(created.KeyID) == "" {
				return fmt.Errorf("web api-keys create failed: create response did not include a key id")
			}

			fileName := fmt.Sprintf("AuthKey_%s.p8", strings.TrimSpace(created.KeyID))
			p8Path := filepath.Join(outputRoot.Path(), fileName)
			if err := outputRoot.CreateNewFile(fileName, nil, 0o600); err != nil {
				return fmt.Errorf(
					"API key %q (%s) was created, but destination %q could not be reserved before its one-time P8 download; the P8 was not downloaded: %w",
					nameValue,
					created.KeyID,
					p8Path,
					err,
				)
			}

			var p8 []byte
			err = withWebSpinner("Downloading one-time API key P8", func() error {
				var downloadErr error
				p8, downloadErr = downloadWebAPIKeyWithRetry(requestCtx, client, created.KeyID)
				return downloadErr
			})
			if err != nil {
				downloadFailure := fmt.Errorf(
					"API key %q (%s) was created, but its one-time P8 could not be downloaded: %w",
					nameValue,
					created.KeyID,
					withWebAuthHint(err, "web api-keys create"),
				)
				if removeErr := removeReservedWebAPIKeyP8(outputRoot, fileName); removeErr != nil {
					return errors.Join(
						downloadFailure,
						fmt.Errorf("remove empty reserved P8 file %q: %w", p8Path, removeErr),
					)
				}
				return downloadFailure
			}

			if err := outputRoot.WriteFile(fileName, p8, 0o600); err != nil {
				return fmt.Errorf(
					"API key %q (%s) was created and its one-time P8 was downloaded, but saving %q failed; the P8 cannot be downloaded again: %w",
					nameValue,
					created.KeyID,
					p8Path,
					err,
				)
			}

			var details *webcore.APIKey
			err = withWebSpinner("Loading API key issuer ID", func() error {
				var getErr error
				details, getErr = getWebAPIKeyFn(requestCtx, client, created.KeyID)
				return getErr
			})
			if err != nil {
				return fmt.Errorf(
					"API key %q (%s) was created and its P8 was saved to %q, but the issuer ID could not be loaded: %w",
					nameValue,
					created.KeyID,
					p8Path,
					err,
				)
			}
			if details == nil || strings.TrimSpace(details.IssuerID) == "" {
				return fmt.Errorf(
					"API key %q (%s) was created and its P8 was saved to %q, but the response did not include an issuer ID",
					nameValue,
					created.KeyID,
					p8Path,
				)
			}

			result := &webAPIKeyCreateResult{
				KeyID:          created.KeyID,
				Name:           firstNonEmpty(details.Name, created.Name, nameValue),
				IssuerID:       details.IssuerID,
				TeamID:         strings.TrimSpace(session.TeamID),
				Roles:          firstNonEmptyRoles(details.Roles, created.Roles, []string{roleValue}),
				Active:         details.Active,
				AllAppsVisible: details.AllAppsVisible,
				KeyType:        firstNonEmpty(details.KeyType, created.KeyType, "PUBLIC_API"),
				P8Path:         p8Path,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebAPIKeyCreateTable(result) },
				func() error { return renderWebAPIKeyCreateMarkdown(result) },
			)
		},
	}
}

func normalizeWebAPIKeyRole(value string) (string, error) {
	role := strings.ToUpper(strings.TrimSpace(value))
	if role == "" {
		return "", fmt.Errorf("--role is required")
	}
	if !isWebAPIKeyRoleIdentifier(role) {
		return "", fmt.Errorf("--role must be a role identifier such as ADMIN or APP_MANAGER (letters, digits, and underscores; case-insensitive)")
	}
	if err := classifyWebAPIKeyRole(role); err != nil {
		return "", err
	}
	return role, nil
}

func isWebAPIKeyRoleIdentifier(role string) bool {
	if role == "" {
		return false
	}
	for i := 0; i < len(role); i++ {
		c := role[i]
		if c >= 'A' && c <= 'Z' {
			continue
		}
		if i > 0 && (c == '_' || (c >= '0' && c <= '9')) {
			continue
		}
		return false
	}
	return true
}

func classifyWebAPIKeyRole(role string) error {
	view, err := webref.Resolve("team", []string{role})
	if err != nil {
		return fmt.Errorf("load team API key role reference: %w", err)
	}
	if view == nil || view.KeyNotes == nil {
		return fmt.Errorf("load team API key role reference: missing team key notes")
	}
	for _, selectable := range view.KeyNotes.SelectableRoles {
		if role == selectable {
			return nil
		}
	}
	for _, unknown := range view.UnknownRoles {
		if unknown == role {
			fmt.Fprintf(os.Stderr, "Warning: --role %s is not a documented selectable team API key role; continuing so App Store Connect can accept or reject it.\n", role)
			return nil
		}
	}
	return fmt.Errorf("--role %s is not a selectable team API key role", role)
}

func removeReservedWebAPIKeyP8(root rootfs.Root, fileName string) error {
	opened, err := root.OpenRoot()
	if err != nil {
		return err
	}
	defer opened.Close()
	if err := opened.Remove(fileName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func downloadWebAPIKeyWithRetry(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < webAPIKeyDownloadAttempts; attempt++ {
		p8, err := downloadWebAPIKeyFn(ctx, client, keyID)
		if err == nil {
			return p8, nil
		}
		lastErr = err
		if attempt == webAPIKeyDownloadAttempts-1 || !webcore.IsAPIKeyDownloadRetryable(err) {
			break
		}
		if err := waitWebAPIKeyRetryFn(ctx, time.Duration(attempt+1)*time.Second); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func waitForWebAPIKeyRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyRoles(values ...[]string) []string {
	for _, roles := range values {
		if len(roles) > 0 {
			return append([]string(nil), roles...)
		}
	}
	return nil
}

func webAPIKeyCreateRows(result *webAPIKeyCreateResult) ([]string, [][]string) {
	headers := []string{"Key ID", "Name", "Issuer ID", "Team ID", "Roles", "P8 Path"}
	rows := [][]string{{
		result.KeyID,
		result.Name,
		result.IssuerID,
		result.TeamID,
		strings.Join(result.Roles, ", "),
		result.P8Path,
	}}
	return headers, rows
}

func renderWebAPIKeyCreateTable(result *webAPIKeyCreateResult) error {
	headers, rows := webAPIKeyCreateRows(result)
	asc.RenderTable(headers, rows)
	return nil
}

func renderWebAPIKeyCreateMarkdown(result *webAPIKeyCreateResult) error {
	headers, rows := webAPIKeyCreateRows(result)
	asc.RenderMarkdown(headers, rows)
	return nil
}
