package artifacts

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

func TestIPAInfoPrivacyManifestsAreOptIn(t *testing.T) {
	makeArchive := func(privacy any) []byte {
		var archive bytes.Buffer
		writer := zip.NewWriter(&archive)
		for name, value := range map[string]any{
			"Payload/Demo.app/Info.plist":               map[string]any{"CFBundleIdentifier": "com.example.demo"},
			"Payload/Demo.app/embedded.mobileprovision": map[string]any{"Entitlements": map[string]any{}},
			"Payload/Demo.app/PrivacyInfo.xcprivacy":    privacy,
		} {
			data, err := plist.Marshal(value, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			entry, err := writer.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return archive.Bytes()
	}
	path := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(path, makeArchive(map[string]any{"NSPrivacyTracking": true}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"default", "json", "table", "markdown", "invalid"} {
		t.Run(format, func(t *testing.T) {
			d := newTestDeps()
			args := []string{"--path", path, "--output", "json"}
			if format != "default" {
				args = []string{"--path", path, "--include-privacy-manifests", "--output", format}
			}
			if format == "invalid" {
				if err := os.WriteFile(path, makeArchive([]byte("not a dictionary")), 0o600); err != nil {
					t.Fatal(err)
				}
				args = []string{"--path", path, "--include-privacy-manifests", "--output", "json"}
			}
			err := NewIPAInfoCommand(d).ParseAndRun(context.Background(), args)
			if format == "invalid" {
				var receipt struct {
					Status  string `json:"status"`
					Privacy struct {
						Complete  bool `json:"complete"`
						Manifests []struct {
							Error string `json:"error"`
						} `json:"manifests"`
					} `json:"privacyManifests"`
				}
				if decodeErr := json.Unmarshal(d.IO.Out.(*bytes.Buffer).Bytes(), &receipt); decodeErr != nil {
					t.Fatal(decodeErr)
				}
				if err == nil || receipt.Status != "unreadable" || receipt.Privacy.Complete || len(receipt.Privacy.Manifests) != 1 || receipt.Privacy.Manifests[0].Error == "" {
					t.Fatalf("partial receipt=%s err=%v", d.IO.Out, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			output := d.IO.Out.(*bytes.Buffer).String()
			if format == "default" || format == "json" {
				var receipt map[string]json.RawMessage
				if err := json.Unmarshal([]byte(output), &receipt); err != nil {
					t.Fatal(err)
				}
				raw, ok := receipt["privacyManifests"]
				if format == "default" {
					if ok {
						t.Fatalf("default receipt changed: %s", output)
					}
					return
				}
				var inventory struct {
					Complete  bool `json:"complete"`
					Manifests []struct {
						BundleID     string         `json:"bundleId"`
						Declarations map[string]any `json:"declarations"`
					} `json:"manifests"`
				}
				if err := json.Unmarshal(raw, &inventory); err != nil {
					t.Fatal(err)
				}
				if !inventory.Complete || len(inventory.Manifests) != 1 || inventory.Manifests[0].BundleID != "com.example.demo" || inventory.Manifests[0].Declarations["NSPrivacyTracking"] != true {
					t.Fatalf("inventory=%s", raw)
				}
			} else if !strings.Contains(output, "Payload/Demo.app/PrivacyInfo.xcprivacy") || !strings.Contains(output, "NSPrivacyTracking") {
				t.Fatalf("inventory missing from %s: %s", format, output)
			}
		})
	}
}
