package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

func TestBindingsEncodeKeepsTheOrderOfMethodsOfOneOperation(t *testing.T) {
	var others []*OpBind
	for i := range 30 {
		others = append(others, &OpBind{Operation: fmt.Sprintf("widgets_op%02d", 29-i), Name: fmt.Sprintf("Op%02d", i)})
	}
	update := &OpBind{Operation: "widgets_op14_updateInstance", Name: "UpdateWidget"}
	set := &OpBind{Operation: "widgets_op14_updateInstance", Name: "SetWidgetName"}
	for _, pair := range [][]*OpBind{{update, set}, {set, update}} {
		fam := &Family{Tag: "Widgets", File: "widgets_gen.go", Operations: slices.Concat(others[:10], pair[:1], others[10:], pair[1:])}
		got, err := fam.Encode()
		if err != nil {
			t.Fatal(err)
		}
		first, second := strings.Index(string(got), `"name":"`+pair[0].Name+`"`), strings.Index(string(got), `"name":"`+pair[1].Name+`"`)
		if first < 0 || second < first {
			t.Errorf("%s was bound before %s, Encode moved it after:\n%s", pair[0].Name, pair[1].Name, got)
		}
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
		name           string
		path           string
		needsFileModes bool
	}{
		{"a tag that differs from its file name", dir, false},
		{"a file instead of a directory", filepath.Join(dir, "Gadgets.json"), false},
		{"a directory it cannot read", unreadable, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needsFileModes && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("file modes do not stop this user from reading a directory")
			}
			if b, err := LoadBindings(tt.path); err == nil {
				t.Fatalf("loaded %d families, want an error", len(b.Families))
			}
		})
	}
}

func TestGenerateRejectsWhatItCannotModel(t *testing.T) {
	spec, _ := loadTestdata(t)
	note := []*RelBind{{Data: "WidgetNoteCreateData", Type: "*WidgetNoteRelationships"}}
	release := []*RelBind{{Data: "WidgetReleaseCreateData", Type: "WidgetReleaseCreateRelationships"}}
	tests := []struct {
		name string
		fam  *Family
	}{
		{"unknown operation", &Family{Operations: []*OpBind{{Operation: "widgets_missing", Name: "X"}}}},
		{"param count differs from path", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X"}}}},
		{"relationship change without ids", &Family{Operations: []*OpBind{{Operation: "widgets_gadgets_replaceToManyRelationship", Name: "X", Params: []string{"id"}}}}},
		{"type without a rule", &Family{Types: []string{"WidgetThing"}}},
		{"unknown check style", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X", Params: []string{"id"}, Check: "bogus"}}}},
		{"check first without options", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X", Params: []string{"id"}, Check: checkRequire, CheckFirst: true}}}},
		{"check first with nothing to check", &Family{Operations: []*OpBind{{Operation: "widgets_parts_getToManyRelated", Name: "X", Params: []string{"id"}, Check: checkRaw, CheckFirst: true, Options: "PartsOption", Query: "buildPartsQuery"}}}},
		{"links on an operation that is not a create", &Family{Operations: []*OpBind{{Operation: "widgets_updateInstance", Name: "X", Params: []string{"id", "widgetID"}, Links: []string{"widget"}}}}},
		{"link to a relationship the schema does not define", &Family{Relationships: release, Operations: []*OpBind{{Operation: "widgetReleases_createInstance", Name: "X", Params: []string{"widgetID", "gadgetID", "otherID"}, Links: []string{"widget", "gadget", "other"}}}}},
		{"relationship linked twice", &Family{Relationships: release, Operations: []*OpBind{{Operation: "widgetReleases_createInstance", Name: "X", Params: []string{"widgetID", "gadgetID", "otherID"}, Links: []string{"widget", "gadget", "widget"}}}}},
		{"link to a to-many relationship", &Family{Relationships: note, Operations: []*OpBind{{Operation: "widgetNotes_createInstance", Name: "X", Params: []string{"widgetID", "gadgetID"}, Links: []string{"widget", "gadgets"}}}}},
		{"required relationship not linked", &Family{Relationships: release, Operations: []*OpBind{{Operation: "widgetReleases_createInstance", Name: "X", Params: []string{"widgetID"}, Links: []string{"widget"}}}}},
		{"links without a relationships record", &Family{Operations: []*OpBind{{Operation: "widgetReleases_createInstance", Name: "X", Params: []string{"widgetID", "gadgetID"}, Links: []string{"widget", "gadget"}}}}},
		{"create with relationships and no links", &Family{Relationships: release, Operations: []*OpBind{{Operation: "widgetReleases_createInstance", Name: "X"}}}},
		{"data with relationships and no record", &Family{Types: []string{"WidgetReleaseCreateData"}}},
		{"relationships record that is not a type name", &Family{Relationships: []*RelBind{{Data: "WidgetReleaseCreateData", Type: "json.RawMessage"}}, Types: []string{"WidgetReleaseCreateData"}}},
		{"relationships record of data without relationships", &Family{Relationships: []*RelBind{{Data: "WidgetCreateData", Type: "WidgetRelationships"}}, Types: []string{"WidgetRelationships"}}},
		{"single read without a query builder", &Family{Operations: []*OpBind{{Operation: "widgets_wheel_getToOneRelated", Name: "X", Params: []string{"id"}, Options: "WheelOption", Single: true}}}},
		{"single read with a next URL context", &Family{Operations: []*OpBind{{Operation: "widgets_wheel_getToOneRelated", Name: "X", Params: []string{"id"}, Options: "WheelOption", Query: "buildWheelQuery", NextContext: "wheel", Single: true}}}},
		{"single on a write", &Family{Operations: []*OpBind{{Operation: "widgets_deleteInstance", Name: "X", Params: []string{"id"}, Query: "buildWheelQuery", Single: true}}}},
		{"id list handling on a read", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X", Params: []string{"id"}, IDList: &IDList{Normalize: true}}}}},
		{"empty list message without normalizing", &Family{Operations: []*OpBind{{Operation: "widgets_wheels_createToManyRelationship", Name: "X", Params: []string{"id"}, IDs: "ids", IDList: &IDList{Message: "ids are required"}}}}},
		{"unknown id list payload", &Family{Operations: []*OpBind{{Operation: "widgets_wheels_createToManyRelationship", Name: "X", Params: []string{"id"}, IDs: "ids", IDList: &IDList{Payload: "bogus"}}}}},
		{"loop payload without an item name", &Family{Operations: []*OpBind{{Operation: "widgets_wheels_createToManyRelationship", Name: "X", Params: []string{"id"}, IDs: "ids", IDList: &IDList{Payload: payloadAppend}}}}},
		{"item name without a loop payload", &Family{Operations: []*OpBind{{Operation: "widgets_wheels_createToManyRelationship", Name: "X", Params: []string{"id"}, IDs: "ids", IDList: &IDList{Payload: payloadIndex, Item: "id"}}}}},
		{"unknown attributes style", &Family{Operations: []*OpBind{{Operation: "wizards_updateInstance", Name: "X", Params: []string{"id"}, Attrs: "bogus"}}}},
		{"attributes by value when the request requires them", &Family{Operations: []*OpBind{{Operation: "widgets_createInstance", Name: "X", Attrs: attrsValue}}}},
		{"attributes by pointer on a read", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X", Params: []string{"id"}, Attrs: attrsPointer}}}},
		{"skip empty attributes the request requires", &Family{Operations: []*OpBind{{Operation: "widgets_createInstance", Name: "X", SkipEmptyAttributes: true}}}},
		{"skip empty attributes of a pointer argument", &Family{Operations: []*OpBind{{Operation: "wizards_updateInstance", Name: "X", Params: []string{"id"}, Attrs: attrsPointer, SkipEmptyAttributes: true}}}},
		{"skip empty attributes on a read", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X", Params: []string{"id"}, SkipEmptyAttributes: true}}}},
		{"args on a read", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X", Params: []string{"id"}, Args: []ArgBind{{Name: "uploaded"}}}}}},
		{"arg for an attribute the request lacks", &Family{Operations: []*OpBind{{Operation: "wrenches_createInstance", Name: "X", Args: []ArgBind{{Name: "color"}}}}}},
		{"arg type that does not fit the attribute", &Family{Operations: []*OpBind{{Operation: "wrenches_createInstance", Name: "X", Args: []ArgBind{{Name: "fileName", Type: "int64"}}}}}},
		{"two args for one attribute", &Family{Operations: []*OpBind{{Operation: "wrenches_createInstance", Name: "X", Args: []ArgBind{{Name: "fileName"}, {Name: "name", Attribute: "fileName"}}}}}},
		{"int64 arg with a generated attributes type", &Family{Types: []string{"WrenchCreateAttributes"}, Operations: []*OpBind{{Operation: "wrenches_createInstance", Name: "X", Args: []ArgBind{{Name: "fileName"}, {Name: "fileSize", Type: "int64"}}}}}},
		{"args with an attributes style", &Family{Operations: []*OpBind{{Operation: "wrenches_updateInstance", Name: "X", Params: []string{"id"}, Attrs: attrsValue, Args: []ArgBind{{Name: "uploaded"}}}}}},
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
