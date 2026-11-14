package main

import (
	"os"
	"path/filepath"
	"testing"
)

func loadTestdata(t *testing.T) (*Spec, *Bindings) {
	t.Helper()
	spec, err := LoadSpec(filepath.Join("testdata", "spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := LoadBindings(filepath.Join("testdata", "bindings.json"))
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
	got, err := bindings.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "bindings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("Encode differs from testdata/bindings.json:\n%s", got)
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
