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

func TestAdoptReplacesOnlyCodeTheGeneratorReproduces(t *testing.T) {
	spec, _ := loadTestdata(t)
	dir := t.TempDir()
	for name, src := range map[string]string{"client.go": handWrittenClient, "types.go": handWrittenTypes, "response.go": handWrittenResponse} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bindings := &Bindings{}
	if err := Adopt(spec, bindings, dir, []string{"Widgets"}, false, io.Discard); err != nil {
		t.Fatal(err)
	}

	fam := bindings.Families[0]
	if fam.File != "widgets_gen.go" || strings.Join(fam.Types, ",") != "WidgetAttributes,WidgetResponse" {
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

func TestAdoptDryRunChangesNothing(t *testing.T) {
	spec, _ := loadTestdata(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "client.go")
	if err := os.WriteFile(path, []byte(handWrittenClient), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Adopt(spec, &Bindings{}, dir, []string{"Widgets"}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if readFile(t, path) != handWrittenClient {
		t.Error("dry run modified client.go")
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
