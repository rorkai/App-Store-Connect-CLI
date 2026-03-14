package asc

import (
	"encoding/json"
	"testing"
)

func TestExtractPreReleaseVersionMap(t *testing.T) {
	included := json.RawMessage(`[
		{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}},
		{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"TV_OS"}},
		{"type":"otherType","id":"other-1","attributes":{}}
	]`)

	m := extractPreReleaseVersionMap(included)
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	if m["prv-1"].Version != "1.2.3" || m["prv-1"].Platform != "IOS" {
		t.Fatalf("unexpected prv-1: %+v", m["prv-1"])
	}
	if m["prv-2"].Platform != "TV_OS" {
		t.Fatalf("unexpected prv-2 platform: %s", m["prv-2"].Platform)
	}
}

func TestExtractPreReleaseVersionMapEmpty(t *testing.T) {
	m := extractPreReleaseVersionMap(nil)
	if m != nil {
		t.Fatalf("expected nil for empty included, got %v", m)
	}
}

func TestBuildPreReleaseVersionID(t *testing.T) {
	rels := json.RawMessage(`{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}`)
	id := buildPreReleaseVersionID(rels)
	if id != "prv-1" {
		t.Fatalf("expected prv-1, got %q", id)
	}
}

func TestBuildPreReleaseVersionIDEmpty(t *testing.T) {
	id := buildPreReleaseVersionID(nil)
	if id != "" {
		t.Fatalf("expected empty string, got %q", id)
	}
}

func TestBuildsRowsWithPreReleaseVersion(t *testing.T) {
	resp := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				Type:          "builds",
				ID:            "build-1",
				Attributes:    BuildAttributes{Version: "9", UploadedDate: "2026-03-13", ProcessingState: "VALID"},
				Relationships: json.RawMessage(`{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}`),
			},
		},
		Included: json.RawMessage(`[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"TV_OS"}}]`),
	}

	headers, rows := buildsRows(resp)
	if len(headers) != 8 {
		t.Fatalf("expected 8 headers (with Version+Platform), got %d: %v", len(headers), headers)
	}
	if headers[1] != "Build" || headers[2] != "Version" || headers[3] != "Platform" {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row[1] != "9" {
		t.Fatalf("expected build number 9, got %q", row[1])
	}
	if row[2] != "1.2.3" {
		t.Fatalf("expected marketing version 1.2.3, got %q", row[2])
	}
	if row[3] != "TV_OS" {
		t.Fatalf("expected platform TV_OS, got %q", row[3])
	}
}

func TestBuildsRowsWithoutPreReleaseVersion(t *testing.T) {
	resp := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				Type:       "builds",
				ID:         "build-1",
				Attributes: BuildAttributes{Version: "9", UploadedDate: "2026-03-13", ProcessingState: "VALID"},
			},
		},
	}

	headers, rows := buildsRows(resp)
	if len(headers) != 6 {
		t.Fatalf("expected 6 headers (backward compat), got %d: %v", len(headers), headers)
	}
	if headers[1] != "Version" {
		t.Fatalf("expected Version header at index 1, got %q", headers[1])
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0][1] != "9" {
		t.Fatalf("expected version 9, got %q", rows[0][1])
	}
}
