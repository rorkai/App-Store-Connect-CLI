package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadTestdata(t *testing.T) (*Spec, *Bindings) {
	t.Helper()
	spec, err := LoadSpec(filepath.Join("testdata", "spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := LoadBindings(filepath.Join("testdata", "bindings"))
	if err != nil {
		t.Fatal(err)
	}
	return spec, bindings
}

func TestGenerateMatchesGolden(t *testing.T) {
	spec, bindings := loadTestdata(t)
	got, err := Generate(spec, bindings.Families[0])
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "widgets_gen.go.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("generated file differs from testdata/widgets_gen.go.golden:\n%s", got)
	}
}

func TestBindingsEncodeIsCanonical(t *testing.T) {
	_, bindings := loadTestdata(t)
	got, err := bindings.Families[0].Encode()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "bindings", "Widgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("Encode differs from testdata/bindings/Widgets.json:\n%s", got)
	}
}

func TestLoadBindingsRejects(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Gadgets.json"), []byte(`{"tag": "Widgets", "file": "widgets_gen.go"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(t.TempDir(), "bindings")
	if err := os.Mkdir(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })
	tests := []struct {
		name, path, want string
	}{
		{"a tag that differs from its file name", dir, "must be named Widgets.json"},
		{"a file instead of a directory", filepath.Join(dir, "Gadgets.json"), "not a directory"},
		{"a directory it cannot read", unreadable, "permission denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := LoadBindings(tt.path); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestGenerateRejectsWhatItCannotModel(t *testing.T) {
	spec, _ := loadTestdata(t)
	tests := []struct {
		name string
		fam  *Family
	}{
		{"unknown operation", &Family{Operations: []*OpBind{{Operation: "widgets_missing", Name: "X"}}}},
		{"param count differs from path", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X"}}}},
		{"relationship change without ids", &Family{Operations: []*OpBind{{Operation: "widgets_gadgets_replaceToManyRelationship", Name: "X", Params: []string{"id"}}}}},
		{"type without a rule", &Family{Types: []string{"WidgetThing"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Generate(spec, tt.fam); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestGoName(t *testing.T) {
	tests := map[string]string{
		"referenceName": "ReferenceName",
		"apiKeyId":      "APIKeyID",
		"iosVersion":    "IOSVersion",
		"sha256":        "SHA256",
		"supportUrl":    "SupportURL",
	}
	for in, want := range tests {
		if got := goName(in); got != want {
			t.Errorf("goName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultName(t *testing.T) {
	spec, _ := loadTestdata(t)
	tests := map[string]string{
		"widgets_getCollection":                     "GetWidgets",
		"widgets_getInstance":                       "GetWidget",
		"widgets_createInstance":                    "CreateWidget",
		"widgets_updateInstance":                    "UpdateWidget",
		"widgets_deleteInstance":                    "DeleteWidget",
		"widgets_gadgets_getToManyRelated":          "GetWidgetGadgets",
		"widgets_gadgets_replaceToManyRelationship": "UpdateWidgetGadgetsRelationship",
	}
	for id, want := range tests {
		op, ok := spec.Operation(id)
		if !ok {
			t.Fatalf("no operation %s", id)
		}
		if got := defaultName(spec, op); got != want {
			t.Errorf("defaultName(%s) = %q, want %q", id, got, want)
		}
	}
	if got := defaultName(spec, &Operation{ID: "gadgetsV2_parts_createToManyRelationship"}); got != "AddGadgetPartsV2" {
		t.Errorf("versioned collection without a schema: got %q", got)
	}
}

// TestCheckedInClientIsUpToDate fails when a generated file in internal/asc
// differs from what the snapshot and the bindings produce, or no binding
// produces it any more.
func TestCheckedInClientIsUpToDate(t *testing.T) {
	spec, err := LoadSpec(filepath.Join("..", "..", "docs", "openapi", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := LoadBindings(filepath.Join("..", "asc", "openapi_bindings"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAll(spec, bindings, filepath.Join("..", "asc"), true); err != nil {
		t.Fatal(err)
	}
}
