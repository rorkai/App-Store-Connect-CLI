package asc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSigningSyncResultPreservesSingleTargetJSONShape(t *testing.T) {
	result := &SigningSyncResult{
		Operation:       "push",
		RepoURL:         "file:///tmp/signing.git",
		BundleID:        "com.example.app",
		ProfileType:     "IOS_APP_STORE",
		Files:           []string{"profiles/appstore/profile.mobileprovision"},
		IdentityPresent: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal signing sync result: %v", err)
	}
	want := `{"operation":"push","repoUrl":"file:///tmp/signing.git","bundleId":"com.example.app","profileType":"IOS_APP_STORE","files":["profiles/appstore/profile.mobileprovision"],"identityPresent":false}`
	if string(data) != want {
		t.Fatalf("single-target JSON = %s, want %s", data, want)
	}
}

func TestSigningSyncResultBatchJSONOmitsSingularBundleID(t *testing.T) {
	result := &SigningSyncResult{
		Operation:       "push",
		RepoURL:         "file:///tmp/signing.git",
		BundleID:        "com.example.app",
		ProfileType:     "IOS_APP_STORE",
		Files:           []string{},
		IdentityPresent: false,
		BundleIDs:       []string{"com.example.app"},
		Targets: []SigningSyncTargetResult{{
			BundleID:       "com.example.app",
			ProfileType:    "IOS_APP_STORE",
			ProfilePath:    "profiles/appstore/com.example.app--profile.mobileprovision",
			ProfilePaths:   []string{"profiles/appstore/com.example.app--profile.mobileprovision"},
			ProfileCreated: false,
			Files:          []string{"profiles/appstore/com.example.app--profile.mobileprovision"},
		}},
	}
	result.MarkBatch()

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal batch signing sync result: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode batch signing sync result: %v", err)
	}
	if _, ok := decoded["bundleId"]; ok {
		t.Fatalf("batch JSON unexpectedly contains singular bundleId: %s", data)
	}
	for _, field := range []string{`"bundleIds":["com.example.app"]`, `"targets":[`, `"profilePaths":["profiles/appstore/com.example.app--profile.mobileprovision"]`, `"profileCreated":false`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("batch JSON missing %s: %s", field, data)
		}
	}
}

func TestSigningSyncResultJSONIncludesPartialCreationState(t *testing.T) {
	result := &SigningSyncResult{
		Operation:                "push",
		RepoURL:                  "file:///tmp/signing.git",
		ProfileType:              "IOS_APP_STORE",
		Files:                    []string{},
		BundleIDs:                []string{"com.example.app"},
		CertificateIDs:           []string{"certificate-1"},
		CertificateCreationState: "created",
		ProfileCreationState:     "unknown",
		PublicationState:         "unknown",
		Partial:                  true,
		Targets: []SigningSyncTargetResult{{
			BundleID:                 "com.example.app",
			ProfileType:              "IOS_APP_STORE",
			CertificateCreationState: "created",
			ProfileCreationState:     "unknown",
		}},
	}
	result.MarkBatch()

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"certificateIds":["certificate-1"]`,
		`"certificateCreationState":"created"`,
		`"profileCreationState":"unknown"`,
		`"publicationState":"unknown"`,
		`"partial":true`,
	} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("partial JSON missing %s: %s", field, data)
		}
	}
	if strings.Count(string(data), `"profileCreationState":"unknown"`) != 2 {
		t.Fatalf("partial JSON missing per-target profile state: %s", data)
	}
}

func TestSigningSyncResultRendererRegisteredAndRenders(t *testing.T) {
	ensureOutputRegistryPopulated()
	handler := requireOutputHandlerFor[SigningSyncResult](t, "SigningSyncResult")
	result := &SigningSyncResult{Targets: []SigningSyncTargetResult{{
		BundleID:       "com.example.app",
		ProfileType:    "IOS_APP_STORE",
		ProfilePath:    "profiles/appstore/com.example.app--profile.mobileprovision",
		ProfileCreated: true,
		Files:          []string{"certs/distribution/certificate.cer", "profiles/appstore/profile.mobileprovision"},
	}}}
	result.MarkBatch()

	headers, rows, err := handler(result)
	if err != nil {
		t.Fatalf("signing sync rows handler: %v", err)
	}
	assertSingleRowEquals(t, headers,
		rows,
		[]string{"Bundle ID", "Profile Type", "Profile Path", "Profile Created", "Files"},
		[]string{"com.example.app", "IOS_APP_STORE", "profiles/appstore/com.example.app--profile.mobileprovision", "true", "certs/distribution/certificate.cer, profiles/appstore/profile.mobileprovision"})

	for _, renderer := range []struct {
		name string
		fn   func(any) error
	}{
		{name: "table", fn: PrintTable},
		{name: "markdown", fn: PrintMarkdown},
	} {
		t.Run(renderer.name, func(t *testing.T) {
			assertRenderedNonJSONContains(t, renderer.fn, result,
				"com.example.app", "IOS_APP_STORE", "profile.mobileprovision", "true")
		})
	}
}

func TestSigningSyncResultRendererRendersSingleTargetSummaries(t *testing.T) {
	ensureOutputRegistryPopulated()
	handler := requireOutputHandlerFor[SigningSyncResult](t, "SigningSyncResult")

	tests := []struct {
		name   string
		result *SigningSyncResult
		want   []string
	}{
		{
			name: "push",
			result: &SigningSyncResult{
				Operation:       "push",
				RepoURL:         "file:///tmp/signing.git",
				BundleID:        "com.example.app",
				ProfileType:     "IOS_APP_STORE",
				Files:           []string{"profiles/appstore/profile.mobileprovision"},
				IdentityPresent: true,
			},
			want: []string{
				"push",
				"file:///tmp/signing.git",
				"com.example.app",
				"IOS_APP_STORE",
				"profiles/appstore/profile.mobileprovision",
				"true",
			},
		},
		{
			name: "pull",
			result: &SigningSyncResult{
				Operation:       "pull",
				RepoURL:         "file:///tmp/signing.git",
				Files:           []string{"profiles/appstore/profile.mobileprovision"},
				IdentityPresent: false,
			},
			want: []string{
				"pull",
				"file:///tmp/signing.git",
				"profiles/appstore/profile.mobileprovision",
				"false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, rows, err := handler(tt.result)
			if err != nil {
				t.Fatalf("signing sync rows handler: %v", err)
			}
			wantHeaders := []string{"Operation", "Repo URL", "Bundle ID", "Profile Type", "Files", "Identity Present"}
			if len(rows) != 1 {
				t.Fatalf("single-target rows = %d, want 1: %v", len(rows), rows)
			}
			if len(headers) != len(wantHeaders) {
				t.Fatalf("single-target headers = %v, want %v", headers, wantHeaders)
			}
			for index, wantHeader := range wantHeaders {
				if headers[index] != wantHeader {
					t.Fatalf("single-target headers = %v, want %v", headers, wantHeaders)
				}
			}
			for _, want := range tt.want {
				if !strings.Contains(strings.Join(rows[0], " "), want) {
					t.Fatalf("single-target row = %v, want it to contain %q", rows[0], want)
				}
			}
			for _, renderer := range []struct {
				name string
				fn   func(any) error
			}{
				{name: "table", fn: PrintTable},
				{name: "markdown", fn: PrintMarkdown},
			} {
				t.Run(renderer.name, func(t *testing.T) {
					assertRenderedNonJSONContains(t, renderer.fn, tt.result, tt.want...)
				})
			}
		})
	}
}
