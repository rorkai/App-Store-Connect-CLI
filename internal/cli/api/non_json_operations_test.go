package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNonJSONOperationsMatchOpenAPISnapshot derives, from the authoritative
// OpenAPI snapshot, every operation whose 2xx responses declare no plain
// `application/json` representation, and pins the exclusion table to that set.
// A snapshot refresh that adds or removes such an operation fails here instead
// of silently letting `asc api` print a non-JSON body as a JSON envelope.
func TestNonJSONOperationsMatchOpenAPISnapshot(t *testing.T) {
	snapshotPath := filepath.Join("..", "..", "..", "docs", "openapi", "latest.json")
	raw, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read OpenAPI snapshot: %v", err)
	}

	// A path item mixes operation objects with sibling keys such as the
	// path-level `parameters` array, so operations are decoded individually.
	var snapshot struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("parse OpenAPI snapshot: %v", err)
	}

	type operationSchema struct {
		Responses map[string]struct {
			Content map[string]json.RawMessage `json:"content"`
		} `json:"responses"`
	}

	wantMediaTypes := make(map[string][]string)
	for path, operations := range snapshot.Paths {
		for method, rawOperation := range operations {
			if strings.ToUpper(method) != "GET" {
				continue
			}
			var operation operationSchema
			if err := json.Unmarshal(rawOperation, &operation); err != nil {
				t.Fatalf("parse GET %s: %v", path, err)
			}
			mediaTypes := make(map[string]struct{})
			for status, response := range operation.Responses {
				if !strings.HasPrefix(status, "2") {
					continue
				}
				for mediaType := range response.Content {
					mediaTypes[mediaType] = struct{}{}
				}
			}
			if len(mediaTypes) == 0 {
				continue
			}
			if _, ok := mediaTypes["application/json"]; ok {
				continue
			}
			names := make([]string, 0, len(mediaTypes))
			for mediaType := range mediaTypes {
				names = append(names, mediaType)
			}
			sort.Strings(names)
			wantMediaTypes[path] = names
		}
	}

	if len(wantMediaTypes) == 0 {
		t.Fatal("snapshot yielded no non-JSON GET operations; the parser is probably wrong")
	}

	for path, mediaTypes := range wantMediaTypes {
		operation, ok := nonJSONOperations[path]
		if !ok {
			t.Errorf("GET %s responds only with %v but is missing from nonJSONOperations", path, mediaTypes)
			continue
		}
		if len(mediaTypes) != 1 || operation.mediaType != mediaTypes[0] {
			t.Errorf("GET %s media type = %q, snapshot declares %v", path, operation.mediaType, mediaTypes)
		}
		if !strings.HasPrefix(operation.command, "asc ") {
			t.Errorf("GET %s dedicated command = %q, want an `asc ...` command", path, operation.command)
		}
	}
	for path := range nonJSONOperations {
		if _, ok := wantMediaTypes[path]; !ok {
			t.Errorf("GET %s is in nonJSONOperations but the snapshot declares application/json for it", path)
		}
	}
}
