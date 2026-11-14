package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const handWrittenClient = `package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetWidget fetches one widget.
func (c *Client) GetWidget(ctx context.Context, widgetID string) (*WidgetResponse, error) {
	widgetID = strings.TrimSpace(widgetID)
	if widgetID == "" {
		return nil, fmt.Errorf("widget ID is required")
	}

	return getJSON[WidgetResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/widgets/%s", widgetID), ParseErr: "failed to parse widget"})
}

// UpdateWidgetGadgets replaces the gadgets of a widget.
func (c *Client) UpdateWidgetGadgets(ctx context.Context, widgetID string, gadgetIDs []string) error {
	payload := RelationshipRequest{Data: buildRelationshipData(ResourceTypeGadgets, gadgetIDs)}
	return sendNoContent(ctx, c, "PATCH", fmt.Sprintf("/v1/widgets/%s/relationships/gadgets", strings.TrimSpace(widgetID)), payload)
}

// DeleteWidget logs before it deletes, so it stays hand-written.
func (c *Client) DeleteWidget(ctx context.Context, widgetID string) error {
	fmt.Println("deleting", widgetID)
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/widgets/%s", widgetID), nil)
}
`

const handWrittenTypes = `package asc

// WidgetAttributes describes a widget.
type WidgetAttributes struct {
	Name     string   ` + "`json:\"name,omitempty\"`" + `
	Count    int      ` + "`json:\"count,omitempty\"`" + `
	Enabled  bool     ` + "`json:\"enabled,omitempty\"`" + `
	APIKeyID string   ` + "`json:\"apiKeyId,omitempty\"`" + `
	Tags     []string ` + "`json:\"tags,omitempty\"`" + `
}

// WidgetsResponse keeps a pointer on purpose.
type WidgetsResponse = Response[*WidgetAttributes]
`

const handWrittenResponse = `package asc

import "fmt"

// WidgetResponse is a single widget.
type WidgetResponse = SingleResponse[WidgetAttributes]
`

func writeHandWritten(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runAdopt(dir string, dryRun bool) error {
	return run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, false, "Widgets", dryRun, io.Discard)
}

func TestAdoptReplacesOnlyCodeTheGeneratorReproduces(t *testing.T) {
	otherFamily := `{"tag": "Other", "file": "other_gen.go"}`
	dir := writeHandWritten(t, map[string]string{"client.go": handWrittenClient, "types.go": handWrittenTypes, "response.go": handWrittenResponse, "bindings/Other.json": otherFamily})
	if err := runAdopt(dir, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "bindings", "Other.json")); got != otherFamily {
		t.Errorf("adopt rewrote the bindings of a family it did not adopt:\n%s", got)
	}

	bindings, err := LoadBindings(filepath.Join(dir, "bindings"))
	if err != nil {
		t.Fatal(err)
	}
	fam := bindings.Families[1]
	if fam.Tag != "Widgets" || fam.File != "widgets_gen.go" || strings.Join(fam.Types, ",") != "WidgetAttributes,WidgetResponse" {
		t.Errorf("family = %s with types %v", fam.File, fam.Types)
	}
	adopted := map[string]*OpBind{}
	for _, op := range fam.Operations {
		adopted[op.Name] = op
	}
	get := adopted["GetWidget"]
	if len(adopted) != 2 || get == nil || adopted["UpdateWidgetGadgets"] == nil {
		t.Fatalf("adopted methods = %v", adopted)
	}
	if get.Check != checkRequire || get.ParseErr != "failed to parse widget" || strings.Join(get.Messages, ",") != "widget ID is required" {
		t.Errorf("GetWidget binding = %+v", get)
	}

	generated := readFile(t, filepath.Join(dir, "widgets_gen.go"))
	if !strings.Contains(generated, "func (c *Client) GetWidget(") || !strings.Contains(generated, "type WidgetAttributes struct") {
		t.Errorf("widgets_gen.go:\n%s", generated)
	}
	client := readFile(t, filepath.Join(dir, "client.go"))
	if strings.Contains(client, "GetWidget") || strings.Contains(client, "UpdateWidgetGadgets") || !strings.Contains(client, "func (c *Client) DeleteWidget(") {
		t.Errorf("client.go after adopt:\n%s", client)
	}
	if strings.Contains(client, `"strings"`) || !strings.Contains(client, `"fmt"`) {
		t.Errorf("client.go imports not pruned:\n%s", client)
	}
	types := readFile(t, filepath.Join(dir, "types.go"))
	if strings.Contains(types, "WidgetAttributes struct") || !strings.Contains(types, "WidgetsResponse") {
		t.Errorf("types.go after adopt:\n%s", types)
	}
	if _, err := os.Stat(filepath.Join(dir, "response.go")); !os.IsNotExist(err) {
		t.Errorf("response.go should be deleted once empty, stat err = %v", err)
	}
}

func TestAdoptIntoAnExistingFamilyLeavesItUpToDate(t *testing.T) {
	existing := `{"tag": "Widgets", "file": "widgets_gen.go", "operations": [{"operation": "widgets_updateInstance", "name": "UpdateWidget", "params": ["widgetID"]}]}`
	dir := writeHandWritten(t, map[string]string{"client.go": handWrittenClient, "bindings/Widgets.json": existing})
	if err := runAdopt(dir, false); err != nil {
		t.Fatal(err)
	}
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, true, "", false, io.Discard); err != nil {
		t.Fatalf("check right after adopt: %v", err)
	}
}

func TestAdoptWritesNothingOnDryRunOrFailure(t *testing.T) {
	brokenBindings := `{"tag": "Other", "file": "other_gen.go", "operations": [{"operation": "widgets_missing", "name": "Missing"}]}`
	tests := []struct {
		name     string
		bindings string
		dryRun   bool
	}{
		{"dry run", "", true},
		{"an existing binding no longer renders", brokenBindings, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{"client.go": handWrittenClient}
			if tt.bindings != "" {
				files["bindings/Other.json"] = tt.bindings
			}
			dir := writeHandWritten(t, files)
			err := runAdopt(dir, tt.dryRun)
			if tt.dryRun != (err == nil) {
				t.Fatalf("err = %v", err)
			}
			if readFile(t, filepath.Join(dir, "client.go")) != handWrittenClient {
				t.Error("client.go was modified")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != len(files) {
				t.Errorf("files after adopt: %v", entries)
			}
		})
	}
}

func TestWriteAllCheckFailsOnOutdatedOrStaleFiles(t *testing.T) {
	spec, bindings := loadTestdata(t)
	dir := t.TempDir()
	if err := writeAll(spec, bindings, dir, true); err == nil {
		t.Fatal("check passed with the generated file missing")
	}
	if err := writeAll(spec, bindings, dir, false); err != nil {
		t.Fatal(err)
	}
	if err := writeAll(spec, bindings, dir, true); err != nil {
		t.Fatalf("check failed right after writing: %v", err)
	}

	stale := filepath.Join(dir, "gone_gen.go")
	if err := os.WriteFile(stale, []byte(header+"\n\npackage asc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAll(spec, bindings, dir, true); err == nil {
		t.Fatal("check passed with a stale generated file")
	}
	if err := writeAll(spec, bindings, dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale generated file not removed, stat err = %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
