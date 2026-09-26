package asc

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestSigningKeychainInstallResultJSONContainsOnlyPublicReceiptFields(t *testing.T) {
	result := &SigningKeychainInstallResult{
		Action:            "installed",
		KeychainPath:      "/tmp/release.keychain-db",
		CertificateSHA256: strings.Repeat("A", 64),
		CertificateSHA1:   strings.Repeat("B", 40),
		TeamID:            "TEAM12345",
		SearchListUpdated: true,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"action", "certificateSha1", "certificateSha256", "keychainPath", "searchListUpdated", "teamId"}
	got := make([]string, 0, len(decoded))
	for key := range decoded {
		got = append(got, key)
	}
	sort.Strings(got)
	if !slices.Equal(got, wantKeys) {
		t.Fatalf("keys = %v, want %v", got, wantKeys)
	}
}

func TestSigningKeychainInstallResultRendererRegistered(t *testing.T) {
	ensureOutputRegistryPopulated()
	handler := requireOutputHandlerFor[SigningKeychainInstallResult](t, "SigningKeychainInstallResult")
	result := &SigningKeychainInstallResult{
		Action:            "installed",
		KeychainPath:      "/tmp/release.keychain-db",
		CertificateSHA256: strings.Repeat("A", 64),
		CertificateSHA1:   strings.Repeat("B", 40),
		TeamID:            "TEAM12345",
		SearchListUpdated: true,
	}
	headers, rows, err := handler(result)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleRowEquals(t, headers, rows,
		[]string{"Action", "Keychain Path", "Certificate SHA-256", "Certificate SHA-1", "Team ID", "Search List Updated"},
		[]string{"installed", "/tmp/release.keychain-db", strings.Repeat("A", 64), strings.Repeat("B", 40), "TEAM12345", "true"})
}

func TestSigningKeychainActionResultRendererShowsPartition(t *testing.T) {
	ensureOutputRegistryPopulated()
	handler := requireOutputHandlerFor[SigningKeychainActionResult](t, "SigningKeychainActionResult")
	headers, rows, err := handler(&SigningKeychainActionResult{Action: "set-partition-list", KeychainPath: "/tmp/release.keychain-db", Partition: "apple-tool:,apple:"})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleRowEquals(t, headers, rows,
		[]string{"Action", "Keychain Path", "Partition"},
		[]string{"set-partition-list", "/tmp/release.keychain-db", "apple-tool:,apple:"})

	headers, rows, err = handler(&SigningKeychainActionResult{Action: "lock", KeychainPath: "/tmp/release.keychain-db"})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleRowEquals(t, headers, rows, []string{"Action", "Keychain Path"}, []string{"lock", "/tmp/release.keychain-db"})
}

func TestSigningKeychainListResultRendererRegistered(t *testing.T) {
	ensureOutputRegistryPopulated()
	handler := requireOutputHandlerFor[SigningKeychainListResult](t, "SigningKeychainListResult")
	headers, rows, err := handler(&SigningKeychainListResult{Keychains: []SigningKeychainInfo{{Path: "/tmp/release.keychain-db", Exists: true, InSearchList: true, Locked: true, Identities: []SigningKeychainIdentity{{SHA256: strings.Repeat("a", 64)}}}}})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleRowEquals(t, headers, rows,
		[]string{"Path", "Exists", "Search List", "Locked", "Identities"},
		[]string{"/tmp/release.keychain-db", "true", "true", "true", "1"})
}
