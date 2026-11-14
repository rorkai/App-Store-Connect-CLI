package main

import (
	"bytes"
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

func TestReportListsOnlyWhatTheSpecDoesNotBack(t *testing.T) {
	spec, _ := loadTestdata(t)
	dir := writeHandWritten(t, map[string]string{"client.go": reportClient})
	var out bytes.Buffer
	if err := Report(spec, dir, &out); err != nil {
		t.Fatal(err)
	}
	want := "method\tGetWidgetOrphans\tGET /v1/widgets/%s/orphans\tno spec operation\n" +
		"method\tUpdateWidgetGizmos\tPATCH /v1/widgets/%s/relationships/%s (through updateWidgetRelationship)\tno spec operation\n" +
		"type\tWidgetAttributes\tWidget\tlacks enabled, apiKeyId, tags\n" +
		"3 hand-written methods or types without spec backing\n"
	if out.String() != want {
		t.Errorf("report:\n%s\nwant:\n%s", out.String(), want)
	}
}
