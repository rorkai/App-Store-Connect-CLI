package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// loginKeyJSON holds the credential read from a JSON API key file. KeyType is
// empty when the file does not say whether the key is in-house.
type loginKeyJSON struct {
	KeyID         string
	IssuerID      string
	PrivateKeyPEM string
	KeyType       string
}

// readLoginKeyJSON reads a JSON API key file with key_id, issuer_id and key
// (PEM text, or its base64 when is_key_content_base64 is true), plus an
// optional in_house. Other fields are reported on stderr as not applied.
// Errors name the field and never include key material.
func readLoginKeyJSON(d *shared.Deps, path string, fixPermissions bool) (loginKeyJSON, error) {
	if fixPermissions {
		changed, err := authsvc.FixPrivateKeyFilePermissions(path)
		if kind, _ := authsvc.PrivateKeyErrorKindOf(err); err != nil && kind != authsvc.PrivateKeyNotFound {
			rendered := errors.New(shared.SanitizeTerminal(fmt.Sprintf("auth login: failed to fix --key-json file permissions: %v", err)))
			return loginKeyJSON{}, shared.WithPrivateKeyDiagnostic(shared.NewErrorWithCause(rendered, err), err)
		}
		if changed {
			fmt.Fprintf(d.IO.ErrOut, "Changed --key-json file permissions to 0600: %s\n", shared.SanitizeTerminal(path))
		}
	}
	data, err := authsvc.ReadValidatedKeyFile(path)
	if err != nil {
		message := err.Error()
		if kind, _ := authsvc.PrivateKeyErrorKindOf(err); kind == authsvc.PrivateKeyPermissionsInsecure {
			message = "file is too permissive; other users can read this credential"
		}
		rendered := shared.WithPrivateKeyDiagnostic(d.InvalidInputErrorf("auth login: invalid --key-json file: %s", message), err)
		printPrivateKeyPermissionRemediation(d, err, path)
		return loginKeyJSON{}, rendered
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return loginKeyJSON{}, keyJSONError(d, "auth login: --key-json file is not a JSON object")
	}
	var loaded loginKeyJSON
	for _, field := range []struct {
		name  string
		value *string
	}{
		{"key_id", &loaded.KeyID},
		{"issuer_id", &loaded.IssuerID},
		{"key", &loaded.PrivateKeyPEM},
	} {
		raw, ok := fields[field.name]
		if ok && json.Unmarshal(raw, field.value) != nil {
			return loginKeyJSON{}, keyJSONError(d, fmt.Sprintf("auth login: --key-json field %q must be a string", field.name))
		}
		if field.name != "key" {
			*field.value = strings.TrimSpace(*field.value)
		}
		if strings.TrimSpace(*field.value) == "" {
			return loginKeyJSON{}, keyJSONError(d, fmt.Sprintf("auth login: --key-json file is missing %q", field.name))
		}
		delete(fields, field.name)
	}

	isBase64, err := keyJSONBool(d, fields, "is_key_content_base64")
	if err != nil {
		return loginKeyJSON{}, err
	}
	if isBase64 != nil && *isBase64 {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(loaded.PrivateKeyPEM))
		if err != nil {
			return loginKeyJSON{}, keyJSONError(d, `auth login: --key-json field "key" is not valid base64`)
		}
		loaded.PrivateKeyPEM = string(decoded)
	}
	if _, err := authsvc.LoadPrivateKeyFromPEM([]byte(loaded.PrivateKeyPEM)); err != nil {
		return loginKeyJSON{}, shared.WithPrivateKeyDiagnostic(d.InvalidInputErrorf(`auth login: invalid private key in --key-json field "key": %s`, err.Error()), err)
	}

	inHouse, err := keyJSONBool(d, fields, "in_house")
	if err != nil {
		return loginKeyJSON{}, err
	}
	if inHouse != nil {
		loaded.KeyType = config.CredentialKeyTypeTeam
		if *inHouse {
			loaded.KeyType = config.CredentialKeyTypeEnterprise
		}
	}

	if len(fields) > 0 {
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
		}
		slices.Sort(names)
		fmt.Fprintf(d.IO.ErrOut, "Warning: --key-json fields not applied: %s\n", shared.SanitizeTerminal(strings.Join(names, ", ")))
	}
	return loaded, nil
}

// keyJSONBool reads and removes an optional boolean field.
func keyJSONBool(d *shared.Deps, fields map[string]json.RawMessage, name string) (*bool, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, nil
	}
	delete(fields, name)
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, keyJSONError(d, fmt.Sprintf("auth login: --key-json field %q must be true or false", name))
	}
	return &value, nil
}

func keyJSONError(d *shared.Deps, message string) error {
	return shared.WithDiagnostic(d.InvalidInputError(message), shared.DiagnosticInvalidInput, "--key-json")
}
