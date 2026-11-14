package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

const reportClient = `package asc

func (c *Client) GetWidget(ctx context.Context, id string) error {
	return getJSON[X](ctx, c, readRequest{Path: fmt.Sprintf("/v1/widgets/%s", id)})
}

func (c *Client) DeleteWidgetByConcat(ctx context.Context, id string) error {
	return sendNoContent(ctx, c, http.MethodDelete, "/v1/widgets/"+id, nil)
}

func (c *Client) GetWidgetOrphans(ctx context.Context, id string) error {
	return getJSON[X](ctx, c, readRequest{Path: fmt.Sprintf("/v1/widgets/%s/orphans?x=1", id)})
}

func (c *Client) updateWidgetRelationship(ctx context.Context, id, relationship string) error {
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/widgets/%s/relationships/%s", id, relationship), nil)
}

func (c *Client) UpdateWidgetGadgets(ctx context.Context, id string) error {
	return c.updateWidgetRelationship(ctx, id, "gadgets")
}

func (c *Client) UpdateWidgetGizmos(ctx context.Context, id string) error {
	return c.updateWidgetRelationship(ctx, id, "gizmos")
}

func sendNoContent(ctx context.Context, c *Client, method, path string, payload any) error {
	return checkMethod(method)
}

func checkMethod(method string) error {
	switch method {
	case "GET", "POST", "PATCH", "DELETE":
		return nil
	}
	return fmt.Errorf("method %s", method)
}

func (c *Client) changeWidgetGadgets(ctx context.Context, method, id string) error {
	return sendNoContent(ctx, c, method, fmt.Sprintf("/v1/widgets/%s/relationships/gadgets", id), nil)
}

func (c *Client) ReplaceWidgetGadgets(ctx context.Context, id string) error {
	return c.changeWidgetGadgets(ctx, http.MethodPatch, id)
}

func (c *Client) AddWidgetGadgets(ctx context.Context, id string) error {
	return c.changeWidgetGadgets(ctx, http.MethodPost, id)
}

func (c *Client) GetWidgetsPage(ctx context.Context) error {
	return getJSON[X](ctx, c, readRequest{Path: widgetsPath()})
}

func (c *Client) GetSprocketsPage(ctx context.Context) error {
	return getJSON[X](ctx, c, readRequest{Path: sprocketsPath()})
}

func widgetIDFromNext(next string) string {
	id, _ := strings.CutPrefix(next, "/v1/gizmos/")
	return id
}

func (c *Client) GetWidgetFromNext(ctx context.Context, next string) error {
	return c.GetWidget(ctx, widgetIDFromNext(next))
}

func widgetsPath() string {
	return "/v1/widgets"
}

func sprocketsPath() string {
	return "/v1/sprockets"
}

type WidgetAttributes struct {
	Name  string ` + "`json:\"name\"`" + `
	Count int    ` + "`json:\"count\"`" + `
}

type WidgetPassThroughAttributes struct {
	Name string ` + "`json:\"name\"`" + `
	api  apiAttributes
}

type WidgetSelfEncodedAttributes struct {
	Name string ` + "`json:\"name\"`" + `
}

func (a *WidgetSelfEncodedAttributes) UnmarshalJSON(data []byte) error { return nil }

type WidgetResponse = SingleResponse[WidgetPassThroughAttributes]

type WidgetsResponse = Response[WidgetSelfEncodedAttributes]
`

func runReport(dir string, check bool, adopt string, dryRun bool, out *bytes.Buffer) error {
	return run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, check, adopt, dryRun, true, out)
}

func TestReportListsOnlyWhatTheSpecDoesNotBack(t *testing.T) {
	dir := writeHandWritten(t, map[string]string{"client.go": reportClient})
	var out bytes.Buffer
	if err := runReport(dir, false, "", false, &out); err != nil {
		t.Fatal(err)
	}
	want := "method\tAddWidgetGadgets\tPOST /v1/widgets/%s/relationships/gadgets (through changeWidgetGadgets)\tno spec operation\n" +
		"method\tGetSprocketsPage\tGET /v1/sprockets (through sprocketsPath)\tno spec operation\n" +
		"method\tGetWidgetOrphans\tGET /v1/widgets/%s/orphans\tno spec operation\n" +
		"method\tUpdateWidgetGizmos\tPATCH /v1/widgets/%s/relationships/%s (through updateWidgetRelationship)\tno spec operation\n" +
		"type\tWidgetAttributes\tWidget\tlacks enabled, apiKeyId, tags\n" +
		"5 hand-written methods or types without spec backing\n"
	if out.String() != want {
		t.Errorf("report:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestReportRejectsActionFlags(t *testing.T) {
	tests := []struct {
		name   string
		check  bool
		adopt  string
		dryRun bool
	}{
		{"check", true, "", false},
		{"adopt", false, "Widgets", false},
		{"dry run", false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeHandWritten(t, map[string]string{"client.go": handWrittenClient})
			var out bytes.Buffer
			err := runReport(dir, tt.check, tt.adopt, tt.dryRun, &out)
			if err == nil || err.Error() != "-report cannot be combined with -check, -adopt or -dry-run" {
				t.Fatalf("err = %v", err)
			}
			if out.Len() > 0 || readFile(t, filepath.Join(dir, "client.go")) != handWrittenClient {
				t.Errorf("report ran or the code changed; output:\n%s", out.String())
			}
		})
	}
}
