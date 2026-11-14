package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const webTestSpec = `{
  "paths": {
    "/iris/v1/widgets/{id}": {
      "delete": {"tags": ["Widgets"], "operationId": "widgets_deleteInstance", "responses": {"2XX": {"description": "ok"}}, "x-asc-source": ["Client.DeleteWidget"]},
      "patch": {"tags": ["Widgets"], "operationId": "widgets_updateInstance", "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/WidgetUpdateRequest"}}}}, "responses": {"2XX": {"description": "ok"}}, "x-asc-source": ["Client.UpdateWidget"]}
    },
    "/iris/v1/gadgets/{id}": {
      "delete": {"tags": ["Widgets"], "operationId": "gadgets_deleteInstance", "responses": {"2XX": {"description": "ok"}}, "x-asc-source": ["Client.DeleteGadget"]}
    },
    "/iris/v1/sessions/{id}": {
      "delete": {"tags": ["Widgets"], "operationId": "sessions_deleteInstance", "responses": {"2XX": {"description": "ok"}}, "x-asc-source": ["Client.DeleteSession"], "x-asc-hand-written": true}
    },
    "/iris/v1/parts/{id}": {
      "delete": {"tags": ["Widgets"], "operationId": "parts_deleteInstance", "responses": {"2XX": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/PartResponse"}}}}}}
    },
    "/ci/api/widgets/{id}": {
      "delete": {"tags": ["Widgets"], "operationId": "ciWidgets_deleteInstance", "responses": {"2XX": {"description": "ok"}}, "x-asc-not-implemented": true}
    }
  },
  "components": {"schemas": {"Flag": {"type": "boolean", "enum": [true]}}}
}`

const webHandWritten = `package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// DeleteWidget removes a widget.
func (c *Client) DeleteWidget(ctx context.Context, widgetID string) error {
	widgetID = strings.TrimSpace(widgetID)
	if widgetID == "" {
		return fmt.Errorf("widget id is required")
	}
	_, err := c.doRequest(ctx, http.MethodDelete, "/widgets/"+url.PathEscape(widgetID), nil)
	return err
}

// DeleteGadget builds its path in a variable, so it stays hand-written.
func (c *Client) DeleteGadget(ctx context.Context, gadgetID string) error {
	gadgetID = strings.TrimSpace(gadgetID)
	if gadgetID == "" {
		return fmt.Errorf("gadget id is required")
	}
	path := "/gadgets/" + url.PathEscape(gadgetID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

// DeleteSession has the generated shape, but its operation is hand-written.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	_, err := c.doRequest(ctx, http.MethodDelete, "/sessions/"+url.PathEscape(sessionID), nil)
	return err
}
`

func writeWebSpec(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "web.json")
	if err := os.WriteFile(path, []byte(webTestSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAdoptWebReplacesOnlyCodeTheGeneratorReproduces(t *testing.T) {
	specPath := writeWebSpec(t)
	dir := writeHandWritten(t, map[string]string{"client.go": webHandWritten})
	bindingsDir := filepath.Join(dir, "openapi_bindings")
	var report bytes.Buffer
	if err := runWeb(specPath, bindingsDir, dir, false, "Widgets", false, &report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.String(), "sessions_deleteInstance\tnot generated: x-asc-hand-written") {
		t.Errorf("report does not name the hand-written operation:\n%s", report.String())
	}

	bindings, err := LoadBindings(bindingsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings.Families) != 1 || len(bindings.Families[0].Operations) != 1 {
		t.Fatalf("bindings = %+v", bindings.Families)
	}
	got := bindings.Families[0].Operations[0]
	if got.Name != "DeleteWidget" || got.Check != checkRequire || strings.Join(got.Messages, ",") != "widget id is required" {
		t.Errorf("DeleteWidget binding = %+v", got)
	}

	generated := readFile(t, filepath.Join(dir, "widgets_gen.go"))
	if !strings.HasPrefix(generated, webHeader+"\n\npackage web\n") || !strings.Contains(generated, "func (c *Client) DeleteWidget(") {
		t.Errorf("widgets_gen.go:\n%s", generated)
	}
	client := readFile(t, filepath.Join(dir, "client.go"))
	if strings.Contains(client, "DeleteWidget") || !strings.Contains(client, "func (c *Client) DeleteGadget(") || !strings.Contains(client, "func (c *Client) DeleteSession(") {
		t.Errorf("client.go after adopt:\n%s", client)
	}
	if err := runWeb(specPath, bindingsDir, dir, true, "", false, &report); err != nil {
		t.Fatalf("check right after adopt: %v", err)
	}
}

func TestGenerateWebRejectsWhatItCannotModel(t *testing.T) {
	spec, err := loadWebSpec(writeWebSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	deleteWidget := func(change func(*OpBind)) *Family {
		b := &OpBind{Operation: "widgets_deleteInstance", Name: "DeleteWidget", Params: []string{"widgetID"}, Check: checkRequire}
		change(b)
		return &Family{Tag: "Widgets", Operations: []*OpBind{b}}
	}
	tests := []struct {
		name string
		fam  *Family
	}{
		{"an operation marked x-asc-hand-written", deleteWidget(func(b *OpBind) { b.Operation = "sessions_deleteInstance" })},
		{"an operation marked x-asc-not-implemented", deleteWidget(func(b *OpBind) { b.Operation = "ciWidgets_deleteInstance" })},
		{"a request body", deleteWidget(func(b *OpBind) { b.Operation = "widgets_updateInstance" })},
		{"a response body", deleteWidget(func(b *OpBind) { b.Operation = "parts_deleteInstance" })},
		{"another check style", deleteWidget(func(b *OpBind) { b.Check = checkTrim })},
		{"a type", &Family{Tag: "Widgets", Types: []string{"WidgetAttributes"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := GenerateWeb(spec, tt.fam); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// TestCheckedInWebClientIsUpToDate fails when a generated file in
// internal/web differs from what the web interface schema and the bindings
// produce, or no binding produces it any more.
func TestCheckedInWebClientIsUpToDate(t *testing.T) {
	err := runWeb(filepath.Join("..", "..", "docs", "openapi", "web", "latest.json"), filepath.Join("..", "web", "openapi_bindings"), filepath.Join("..", "web"), true, "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
}
