package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	resolveWebAuthCredentialsFn = shared.ResolveAuthCredentials
	newWebAuthClientFn          = webcore.NewClient
	lookupWebAuthKeyFn          = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return client.LookupAPIKeyRoles(ctx, keyID)
	}
)

type webAuthCapabilitiesResult struct {
	KeyID        string                 `json:"keyId"`
	Name         string                 `json:"name,omitempty"`
	Kind         string                 `json:"kind"`
	Roles        []string               `json:"roles"`
	RoleSource   string                 `json:"roleSource"`
	Active       bool                   `json:"active"`
	KeyType      string                 `json:"keyType,omitempty"`
	LastUsed     string                 `json:"lastUsed,omitempty"`
	Lookup       string                 `json:"lookup"`
	ResolvedFrom string                 `json:"resolvedFrom"`
	Profile      string                 `json:"profile,omitempty"`
	GeneratedBy  *webcoreKeyActorResult `json:"generatedBy,omitempty"`
	RevokedBy    *webcoreKeyActorResult `json:"revokedBy,omitempty"`
}

type webcoreKeyActorResult struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// WebAuthCapabilitiesCommand returns exact key-role lookup via App Store Connect web-session endpoints.
func WebAuthCapabilitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth capabilities", flag.ExitOnError)

	authFlags := bindWebSessionFlags(fs)
	keyID := fs.String("key-id", "", "API key ID to inspect (optional; defaults to the currently selected CLI API key)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "capabilities",
		ShortUsage: "asc web auth capabilities [--key-id ID] [flags]",
		ShortHelp:  "[experimental] Show exact web-visible API key roles.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Return exact role metadata for an App Store Connect API key using Apple web-session endpoints.
Unlike "asc auth capabilities", which probes effective public-API access, this command reads the web-visible key role assignment directly.

If --key-id is omitted, the command resolves the current API key ID from local asc API auth and uses the active web session only for the exact web lookup.

` + webWarningText + `

Examples:
  asc web auth capabilities
  asc web auth capabilities --output json
  asc web auth capabilities --key-id "39MX87M9Y4" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web auth capabilities does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			resolvedKeyID := strings.TrimSpace(*keyID)
			resolvedFrom := "flag"
			profile := ""
			if resolvedKeyID == "" {
				resolved, err := resolveWebAuthCredentialsFn("")
				if err != nil {
					return shared.UsageErrorf("unable to resolve current API key ID; run 'asc auth login' or provide --key-id (%v)", err)
				}
				resolvedKeyID = strings.TrimSpace(resolved.KeyID)
				profile = strings.TrimSpace(resolved.Profile)
				resolvedFrom = "auth"
			}
			if resolvedKeyID == "" {
				return shared.UsageError("unable to resolve current API key ID; run 'asc auth login' or provide --key-id")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}

			client := newWebAuthClientFn(session)
			var lookup *webcore.APIKeyRoleLookup
			err = withWebSpinner("Loading exact API key roles", func() error {
				var innerErr error
				lookup, innerErr = lookupWebAuthKeyFn(requestCtx, client, resolvedKeyID)
				return innerErr
			})
			if err != nil {
				return wrapWebAuthCapabilitiesError(resolvedKeyID, err)
			}

			result := webAuthCapabilitiesResult{
				KeyID:        lookup.KeyID,
				Name:         lookup.Name,
				Kind:         lookup.Kind,
				Roles:        append([]string(nil), lookup.Roles...),
				RoleSource:   lookup.RoleSource,
				Active:       lookup.Active,
				KeyType:      lookup.KeyType,
				LastUsed:     lookup.LastUsed,
				Lookup:       lookup.Lookup,
				ResolvedFrom: resolvedFrom,
				Profile:      profile,
				GeneratedBy:  convertKeyActor(lookup.GeneratedBy),
				RevokedBy:    convertKeyActor(lookup.RevokedBy),
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebAuthCapabilitiesTable(result) },
				func() error { return renderWebAuthCapabilitiesMarkdown(result) },
			)
		},
	}
}

func convertKeyActor(actor *webcore.KeyActor) *webcoreKeyActorResult {
	if actor == nil {
		return nil
	}
	return &webcoreKeyActorResult{
		ID:   actor.ID,
		Name: actor.Name,
	}
}

func wrapWebAuthCapabilitiesError(keyID string, err error) error {
	if errors.Is(err, webcore.ErrAPIKeyNotFound) {
		return fmt.Errorf("web auth capabilities failed: key %q not found in App Store Connect web key lists", keyID)
	}
	if errors.Is(err, webcore.ErrAPIKeyNotVisible) {
		return fmt.Errorf("web auth capabilities failed: key %q is not visible in the accessible App Store Connect web key lists (team key list may be unavailable to this account)", keyID)
	}
	if errors.Is(err, webcore.ErrAPIKeyRolesUnresolved) {
		return fmt.Errorf("web auth capabilities failed: exact roles could not be resolved for key %q", keyID)
	}
	return withWebAuthHint(err, "web auth capabilities")
}

func renderWebAuthCapabilitiesTable(result webAuthCapabilitiesResult) error {
	asc.RenderTable(webAuthCapabilitiesHeaders(), webAuthCapabilitiesRows(result))
	return nil
}

func renderWebAuthCapabilitiesMarkdown(result webAuthCapabilitiesResult) error {
	asc.RenderMarkdown(webAuthCapabilitiesHeaders(), webAuthCapabilitiesRows(result))
	return nil
}

func webAuthCapabilitiesHeaders() []string {
	return []string{"KEY ID", "KIND", "ACTIVE", "ROLES", "NAME", "LOOKUP", "RESOLVED FROM", "PROFILE"}
}

func webAuthCapabilitiesRows(result webAuthCapabilitiesResult) [][]string {
	return [][]string{{
		result.KeyID,
		result.Kind,
		fmt.Sprintf("%t", result.Active),
		strings.Join(result.Roles, ", "),
		result.Name,
		result.Lookup,
		result.ResolvedFrom,
		result.Profile,
	}}
}
