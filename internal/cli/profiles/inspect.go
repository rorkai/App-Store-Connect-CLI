package profiles

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type profileCertificate struct {
	CommonName   string    `json:"commonName,omitempty"`
	SerialNumber string    `json:"serialNumber,omitempty"`
	SHA1         string    `json:"sha1"`
	SHA256       string    `json:"sha256"`
	NotBefore    time.Time `json:"notBefore,omitempty"`
	NotAfter     time.Time `json:"notAfter,omitempty"`
}

type profileInspectResult struct {
	Path                   string               `json:"path"`
	UUID                   string               `json:"uuid,omitempty"`
	Name                   string               `json:"name,omitempty"`
	AppIDName              string               `json:"appIdName,omitempty"`
	TeamID                 string               `json:"teamId,omitempty"`
	TeamName               string               `json:"teamName,omitempty"`
	BundleID               string               `json:"bundleId,omitempty"`
	ApplicationIdentifier  string               `json:"applicationIdentifier,omitempty"`
	Platforms              []string             `json:"platforms,omitempty"`
	CreatedAt              time.Time            `json:"createdAt,omitempty"`
	ExpiresAt              time.Time            `json:"expiresAt,omitempty"`
	Expired                bool                 `json:"expired"`
	TimeToLive             int                  `json:"timeToLive,omitempty"`
	ProvisionedDevices     []string             `json:"provisionedDevices,omitempty"`
	ProvisionedDeviceCount int                  `json:"provisionedDeviceCount"`
	ProvisionsAllDevices   bool                 `json:"provisionsAllDevices"`
	Certificates           []profileCertificate `json:"certificates,omitempty"`
	Entitlements           map[string]any       `json:"entitlements,omitempty"`
}

// ProfilesInspectCommand returns the profiles inspect subcommand.
func ProfilesInspectCommand() *ffcli.Command {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)

	sourcePath := fs.String("path", "", "Path to a .mobileprovision file to inspect")
	showEntitlements := fs.Bool("entitlements", false, "Include entitlement key/value rows in table or markdown output")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "inspect",
		ShortUsage: "asc profiles inspect --path \"./profile.mobileprovision\" [flags]",
		ShortHelp:  "Inspect a local provisioning profile.",
		LongHelp: `Inspect a local provisioning profile.

This command decodes the embedded plist from a .mobileprovision file and prints
the profile identifiers, dates, certificate fingerprints, devices, and
entitlements.

Examples:
  asc profiles inspect --path "./profile.mobileprovision"
  asc profiles inspect --path "./profile.mobileprovision" --output json
  asc profiles inspect --path "./profile.mobileprovision" --entitlements`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pathValue := strings.TrimSpace(*sourcePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return shared.MissingRequiredUsageError()
			}

			file, err := shared.OpenExistingNoFollow(pathValue)
			if err != nil {
				return fmt.Errorf("profiles inspect: open input: %w", err)
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				return fmt.Errorf("profiles inspect: read input: %w", err)
			}

			parsed, err := parseMobileProvision(data)
			if err != nil {
				return fmt.Errorf("profiles inspect: %w", err)
			}

			result := buildProfileInspectResult(pathValue, parsed, time.Now())
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderProfileInspectResult(result, *showEntitlements, false) },
				func() error { return renderProfileInspectResult(result, *showEntitlements, true) },
			)
		},
	}
}

func buildProfileInspectResult(path string, parsed *mobileProvision, now time.Time) *profileInspectResult {
	result := &profileInspectResult{
		Path:                   path,
		UUID:                   strings.TrimSpace(parsed.UUID),
		Name:                   strings.TrimSpace(parsed.Name),
		AppIDName:              strings.TrimSpace(parsed.AppIDName),
		TeamID:                 strings.TrimSpace(parsed.TeamID()),
		TeamName:               strings.TrimSpace(parsed.TeamName),
		BundleID:               strings.TrimSpace(parsed.BundleID()),
		ApplicationIdentifier:  strings.TrimSpace(parsed.ApplicationIdentifier()),
		Platforms:              append([]string(nil), parsed.Platform...),
		CreatedAt:              parsed.CreationDate,
		ExpiresAt:              parsed.ExpirationDate,
		Expired:                isExpired(parsed.ExpirationDate, now),
		TimeToLive:             parsed.TimeToLive,
		ProvisionedDevices:     append([]string(nil), parsed.ProvisionedDevices...),
		ProvisionedDeviceCount: len(parsed.ProvisionedDevices),
		ProvisionsAllDevices:   parsed.ProvisionsAllDevices,
		Entitlements:           sanitizeMap(parsed.Entitlements),
	}
	sort.Strings(result.Platforms)
	sort.Strings(result.ProvisionedDevices)

	for _, certData := range parsed.DeveloperCertificates {
		if len(certData) == 0 {
			continue
		}
		result.Certificates = append(result.Certificates, inspectDeveloperCertificate(certData))
	}
	return result
}

func renderProfileInspectResult(result *profileInspectResult, showEntitlements bool, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Field", "Value"},
		[][]string{
			{"Path", result.Path},
			{"UUID", result.UUID},
			{"Name", result.Name},
			{"App ID Name", result.AppIDName},
			{"Team ID", result.TeamID},
			{"Team Name", result.TeamName},
			{"Bundle ID", result.BundleID},
			{"Application Identifier", result.ApplicationIdentifier},
			{"Platforms", strings.Join(result.Platforms, ", ")},
			{"Created At", formatInspectTime(result.CreatedAt)},
			{"Expires At", formatInspectTime(result.ExpiresAt)},
			{"Expired", fmt.Sprintf("%t", result.Expired)},
			{"Provisioned Devices", fmt.Sprintf("%d", result.ProvisionedDeviceCount)},
			{"Provisions All Devices", fmt.Sprintf("%t", result.ProvisionsAllDevices)},
			{"Certificates", fmt.Sprintf("%d", len(result.Certificates))},
		},
	)

	if len(result.Certificates) > 0 {
		rows := make([][]string, 0, len(result.Certificates))
		for _, cert := range result.Certificates {
			rows = append(rows, []string{
				cert.CommonName,
				cert.SHA256,
				formatInspectTime(cert.NotAfter),
			})
		}
		render([]string{"Certificate", "SHA-256", "Not After"}, rows)
	}

	if showEntitlements {
		render([]string{"Entitlement", "Value"}, entitlementRows(result.Entitlements))
	}
	return nil
}

func entitlementRows(entitlements map[string]any) [][]string {
	keys := make([]string, 0, len(entitlements))
	for key := range entitlements {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, formatInspectValue(entitlements[key])})
	}
	return rows
}

func formatInspectTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatInspectValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case []string:
		return strings.Join(v, ", ")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, formatInspectValue(item))
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+"="+formatInspectValue(v[key]))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func sanitizeMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = sanitizeValue(value)
	}
	return out
}

func sanitizeValue(value any) any {
	switch v := value.(type) {
	case nil, string, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v
	case []byte:
		return base64.StdEncoding.EncodeToString(v)
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, sanitizeValue(item))
		}
		return out
	case map[string]any:
		return sanitizeMap(v)
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out = append(out, sanitizeValue(rv.Index(i).Interface()))
		}
		return out
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		for _, key := range rv.MapKeys() {
			out[fmt.Sprint(key.Interface())] = sanitizeValue(rv.MapIndex(key).Interface())
		}
		return out
	default:
		return fmt.Sprint(value)
	}
}
