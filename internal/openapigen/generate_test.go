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
	for _, fam := range bindings.Families {
		got, err := Generate(spec, fam)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("testdata", fam.File+".golden"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("generated file differs from testdata/%s.golden:\n%s", fam.File, got)
		}
	}
}

func TestBindingsEncodeIsCanonical(t *testing.T) {
	_, bindings := loadTestdata(t)
	for _, fam := range bindings.Families {
		got, err := fam.Encode()
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(bindingsFile(filepath.Join("testdata", "bindings"), fam.Tag))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("Encode differs from testdata/bindings/%s.json:\n%s", fam.Tag, got)
		}
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
		{"reference to a schema that is neither an enum nor an object", &Family{Types: []string{"YoyoPlainAttributes"}}},
		{"reference to an object without properties", &Family{Types: []string{"YoyoOddAttributes"}}},
		{"record of a schema the spec lacks", &Family{Types: []string{"YoyoAttributes"}, RefTypes: []*RefType{{Schema: "YoyoMissing", Type: "Yoyo"}}}},
		{"record that is not a type name", &Family{Types: []string{"YoyoAttributes"}, RefTypes: []*RefType{{Schema: "YoyoState", Type: "*YoyoStatus"}}}},
		{"record of an object as a string", &Family{Types: []string{"YoyoAttributes"}, RefTypes: []*RefType{{Schema: "YoyoImage", Type: "string"}}}},
		{"record of a schema that is neither an enum nor an object", &Family{RefTypes: []*RefType{{Schema: "YoyoNote", Type: "YoyoText"}}}},
		{"schema recorded twice", &Family{Types: []string{"YoyoAttributes"}, RefTypes: []*RefType{{Schema: "YoyoState", Type: "YoyoStatus"}, {Schema: "YoyoState", Type: "YoyoPhase"}}}},
		{"Go type that is no type name", &Family{GoTypes: []*GoType{{Schema: "ZebraV2UpdateAttributes", Type: "*ZebraUpdateAttributes"}}}},
		{"Go type of a type the family generates", &Family{Types: []string{"ZebraV2CreateData"}, GoTypes: []*GoType{{Schema: "ZebraV2CreateData", Type: "ZebraCreateData"}}}},
		{"Go type recorded twice", &Family{GoTypes: []*GoType{{Schema: "ZebraV2UpdateData", Type: "ZebraUpdateData"}, {Schema: "ZebraV2UpdateData", Type: "ZebraData"}}}},
		{"type recorded by its Go name instead of its schema name", &Family{Types: []string{"ZoneIDAttributes"}}},
		{"param arg after an arg that is not one", wristband(ArgBind{Name: "color", Type: "WristbandColor"}, ArgBind{Name: "label", Param: true})},
		{"param arg of an attribute that is not a string", wristband(ArgBind{Name: "size", Param: true})},
		{"param arg with its own Go type", wristband(ArgBind{Name: "color", Param: true, Type: "WristbandColor"})},
		{"param arg with the inline check style", &Family{Relationships: []*RelBind{{Data: "WristbandCreateData", Type: "*WristbandCreateRelationships"}}, Operations: []*OpBind{{Operation: "wristbands_createInstance", Name: "X", Params: []string{"widgetID"}, Links: []string{"widget"}, Args: []ArgBind{{Name: "label", Param: true}}}}}},
		{"named arg type for an attribute that is no enum", wristband(ArgBind{Name: "label", Type: "WristbandLabel"})},
		{"named arg type that is not a type name", wristband(ArgBind{Name: "color", Type: "*WristbandColor"})},
		{"decode error text on a delete", &Family{Operations: []*OpBind{{Operation: "widgets_deleteInstance", Name: "X", Params: []string{"id"}, ParseErr: "failed to parse widget"}}}},
		{"optional link to a required relationship", &Family{Relationships: yokeRels, Operations: []*OpBind{{Operation: "yokes_createInstance", Name: "X", Optional: []OptionalLink{{Param: "widgetID", Link: "widget"}}}}}},
		{"optional link that is also linked", &Family{Relationships: yokeRels, Operations: []*OpBind{{Operation: "yokes_createInstance", Name: "X", Params: []string{"widgetID"}, Links: []string{"widget"}, Optional: []OptionalLink{{Param: "widgetID2", Link: "widget"}}}}}},
		{"optional link the schema does not define", &Family{Relationships: yokeRels, Operations: []*OpBind{{Operation: "yokes_updateInstance", Name: "X", Params: []string{"id"}, Optional: []OptionalLink{{Param: "ownerID", Link: "owner"}}}}}},
		{"relationships when set with a required link", &Family{Relationships: yokeRels, Operations: []*OpBind{{Operation: "yokes_createInstance", Name: "X", Params: []string{"widgetID"}, Links: []string{"widget"}, Optional: []OptionalLink{{Param: "gadgetID", Link: "gadget"}}, RelationshipsWhenSet: true}}}},
		{"relationships when set without optional links", &Family{Relationships: yokeRels, Operations: []*OpBind{{Operation: "yokes_updateInstance", Name: "X", Params: []string{"id"}, RelationshipsWhenSet: true}}}},
		{"optional links on a read", &Family{Operations: []*OpBind{{Operation: "widgets_getInstance", Name: "X", Params: []string{"id"}, Optional: []OptionalLink{{Param: "gadgetID", Link: "gadget"}}}}}},
		{"positive check on a string arg", &Family{Operations: []*OpBind{{Operation: "wrenches_createInstance", Name: "X", Args: []ArgBind{{Name: "fileName", Positive: "fileName is required"}}}}}},
		{"positive check on a param arg", &Family{Operations: []*OpBind{{Operation: "wrenches_createInstance", Name: "X", Check: checkRequire, Args: []ArgBind{{Name: "fileName", Param: true, Positive: "fileName is required"}}}}}},
		{"decode error text on a relationship change", &Family{Operations: []*OpBind{{Operation: "widgets_gadgets_replaceToManyRelationship", Name: "X", Params: []string{"id"}, IDs: "ids", ParseErr: "failed to parse gadgets"}}}},
		{"path check without an error context", &Family{Operations: []*OpBind{{Operation: "yarns_getInstance", Name: "X", Params: []string{"id"}, Check: checkPath}}}},
		{"path error context without the path check", &Family{Operations: []*OpBind{{Operation: "yarns_getInstance", Name: "X", Params: []string{"id"}, Check: checkRequire, PathErr: "yarn"}}}},
		{"path check on a write", &Family{Operations: []*OpBind{{Operation: "widgets_updateInstance", Name: "X", Params: []string{"id"}, Check: checkPath, PathErr: "widget"}}}},
		{"path check with messages", &Family{Operations: []*OpBind{{Operation: "yarns_getInstance", Name: "X", Params: []string{"id"}, Check: checkPath, PathErr: "yarn", Messages: []string{"yarn ID is required"}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Generate(spec, tt.fam); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

var yokeRels = []*RelBind{{Data: "YokeCreateData", Type: "*YokeCreateRelationships"}, {Data: "YokeUpdateData", Type: "*YokeUpdateRelationships", OmitEmpty: true}}

// wristband binds the wristband create, which links a widget, with args.
func wristband(args ...ArgBind) *Family {
	return &Family{
		Relationships: []*RelBind{{Data: "WristbandCreateData", Type: "*WristbandCreateRelationships"}},
		Operations:    []*OpBind{{Operation: "wristbands_createInstance", Name: "X", Params: []string{"widgetID"}, Check: checkRequire, Links: []string{"widget"}, Args: args}},
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

func TestGoTypeName(t *testing.T) {
	tests := map[string]string{
		"BundleIdCreateAttributes":                     "BundleIDCreateAttributes",
		"MerchantIdsResponse":                          "MerchantIDsResponse",
		"BundleIdBundleIdCapabilitiesLinkagesResponse": "BundleIDBundleIDCapabilitiesLinkagesResponse",
		"PassTypeId":                                   "PassTypeID",
		"AndroidToIosAppMappingDetailAttributes":       "AndroidToIosAppMappingDetailAttributes",
		"AppStoreVersionV2Response":                    "AppStoreVersionV2Response",
		"IdentityResponse":                             "IdentityResponse",
		"BetaAppIdfaAttributes":                        "BetaAppIdfaAttributes",
	}
	for in, want := range tests {
		if got := goTypeName(in); got != want {
			t.Errorf("goTypeName(%q) = %q, want %q", in, got, want)
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
