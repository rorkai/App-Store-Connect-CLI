package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestCertificatesResponsePreservesSparseAttributeFields(t *testing.T) {
	input := []byte(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"displayName":"Example","serialNumber":""}}],"links":{}}`)
	var response CertificatesResponse
	if err := json.Unmarshal(input, &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	output, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var envelope struct {
		Data []struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("data count = %d, want 1", len(envelope.Data))
	}
	attributes := envelope.Data[0].Attributes
	if len(attributes) != 2 {
		t.Fatalf("attributes = %s, want only displayName and serialNumber", output)
	}
	if string(attributes["displayName"]) != `"Example"` || string(attributes["serialNumber"]) != `""` {
		t.Fatalf("attributes = %s", output)
	}
	if _, present := attributes["name"]; present {
		t.Fatalf("output invented name: %s", output)
	}
	if _, present := attributes["certificateType"]; present {
		t.Fatalf("output invented certificateType: %s", output)
	}
}

func TestCertificatesResponseUsesMutatedSparseAttributeValues(t *testing.T) {
	input := []byte(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"displayName":"Before","activated":false}}],"links":{}}`)
	var response CertificatesResponse
	if err := json.Unmarshal(input, &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	response.Data[0].Attributes.DisplayName = "After"
	activated := true
	response.Data[0].Attributes.Activated = &activated

	output, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(output), `"attributes":{"displayName":"After","activated":true}`) {
		t.Fatalf("output = %s, want mutated sparse attributes", output)
	}
}

func TestGetCertificates_SendsQuerySurface(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/certificates" {
			t.Fatalf("request = %s %s, want GET /v1/certificates", req.Method, req.URL.Path)
		}
		values := req.URL.Query()
		wantQuery := map[string]string{
			"filter[displayName]":     "Alpha,Beta",
			"filter[certificateType]": "IOS_DISTRIBUTION,PASS_TYPE_ID",
			"filter[serialNumber]":    "SN1,SN2",
			"filter[id]":              "cert-1,cert-2",
			"sort":                    "-displayName",
			"fields[certificates]":    "displayName,serialNumber",
			"fields[passTypeIds]":     "name,identifier",
			"include":                 "passTypeId",
			"limit":                   "5",
		}
		for key, want := range wantQuery {
			if got := values.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		if len(values) != len(wantQuery) {
			t.Errorf("query = %s, want exactly %d parameters", values.Encode(), len(wantQuery))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificates(
		context.Background(),
		WithCertificatesFilterDisplayNames([]string{"Alpha", "Beta"}),
		WithCertificatesTypes([]string{"IOS_DISTRIBUTION", "PASS_TYPE_ID"}),
		WithCertificatesFilterSerialNumbers([]string{"SN1", "SN2"}),
		WithCertificatesFilterIDs([]string{"cert-1", "cert-2"}),
		WithCertificatesSort("-displayName"),
		WithCertificatesFields([]string{"displayName", "serialNumber"}),
		WithCertificatesPassTypeIDFields([]string{"name", "identifier"}),
		WithCertificatesInclude([]string{"passTypeId"}),
		WithCertificatesLimit(5),
	); err != nil {
		t.Fatalf("GetCertificates() error: %v", err)
	}
}

func TestCertificateSerialFiltersIncludeFormWithoutLeadingZeros(t *testing.T) {
	const response = `{"data":[]}`
	lists := []struct {
		name string
		list func(*Client, []string) error
	}{
		{name: "certificates", list: func(client *Client, serials []string) error {
			_, err := client.GetCertificates(context.Background(), WithCertificatesFilterSerialNumbers(serials))
			return err
		}},
		{name: "pass type ID certificates", list: func(client *Client, serials []string) error {
			_, err := client.GetPassTypeIDCertificates(context.Background(), "p1", WithPassTypeIDCertificatesFilterSerialNumbers(serials))
			return err
		}},
		{name: "merchant ID certificates", list: func(client *Client, serials []string) error {
			_, err := client.GetMerchantIDCertificates(context.Background(), "m1", WithMerchantIDCertificatesFilterSerialNumbers(strings.Join(serials, ",")))
			return err
		}},
	}
	tests := []struct {
		name    string
		serials []string
		want    string
	}{
		{name: "no leading zero", serials: []string{"E41"}, want: "E41"},
		{name: "repeated value without leading zero", serials: []string{"E41", "E41"}, want: "E41,E41"},
		{name: "leading zero", serials: []string{"0E41"}, want: "0E41,E41"},
		{name: "several leading zeros", serials: []string{"000e41"}, want: "000e41,e41"},
		{name: "both forms given", serials: []string{"E41", "0E41", "00E41"}, want: "E41,0E41,00E41"},
		{name: "mixed values", serials: []string{"0E41", "A7"}, want: "0E41,E41,A7"},
		{name: "only zeros", serials: []string{"00"}, want: "00"},
	}
	for _, list := range lists {
		for _, test := range tests {
			t.Run(list.name+"/"+test.name, func(t *testing.T) {
				client := newTestClient(t, func(req *http.Request) {
					if got := req.URL.Query().Get("filter[serialNumber]"); got != test.want {
						t.Errorf("filter[serialNumber] = %q, want %q", got, test.want)
					}
				}, jsonResponse(http.StatusOK, response))
				if err := list.list(client, test.serials); err != nil {
					t.Fatalf("list error: %v", err)
				}
			})
		}
	}
}

func TestGetCertificate_RejectsCollectionQueryOptionsBeforeRequest(t *testing.T) {
	tests := []struct {
		name   string
		option CertificatesOption
		want   string
	}{
		{name: "limit", option: WithCertificatesLimit(5), want: "limit option cannot be used with GetCertificate"},
		{name: "next URL", option: WithCertificatesNextURL("https://api.appstoreconnect.apple.com/v1/certificates?cursor=next"), want: "next URL option cannot be used with GetCertificate"},
		{name: "certificate type", option: WithCertificatesTypes([]string{"IOS_DISTRIBUTION"}), want: "certificate type option cannot be used with GetCertificate"},
		{name: "display name", option: WithCertificatesFilterDisplayNames([]string{"Alpha"}), want: "display name option cannot be used with GetCertificate"},
		{name: "serial number", option: WithCertificatesFilterSerialNumbers([]string{"SN1"}), want: "serial number option cannot be used with GetCertificate"},
		{name: "ID", option: WithCertificatesFilterIDs([]string{"cert-1"}), want: "ID option cannot be used with GetCertificate"},
		{name: "sort", option: WithCertificatesSort("-displayName"), want: "sort option cannot be used with GetCertificate"},
		{name: "certificate type CSV", option: WithCertificatesFilterType("IOS_DISTRIBUTION"), want: "certificate type option cannot be used with GetCertificate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := newTestClient(t, func(*http.Request) {
				requests++
			}, jsonResponse(http.StatusOK, `{"data":null}`))

			if _, err := client.GetCertificate(context.Background(), "cert-1", test.option); err == nil {
				t.Fatal("GetCertificate() error = nil, want unsupported option error")
			} else if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("GetCertificate() error = %q, want substring %q", err, test.want)
			}
			if requests != 0 {
				t.Fatalf("request count = %d, want 0", requests)
			}
		})
	}
}

func TestGetCertificate_DetailQueryOnlyAllowsSparseFieldsAndInclude(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"certificates","id":"cert-1","attributes":{"name":"Certificate","certificateType":"IOS_DISTRIBUTION"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/certificates/cert-1" {
			t.Fatalf("request = %s %s, want GET /v1/certificates/cert-1", req.Method, req.URL.Path)
		}
		values := req.URL.Query()
		wantQuery := map[string]string{
			"fields[certificates]": "displayName,serialNumber",
			"fields[passTypeIds]":  "name,identifier",
			"include":              "passTypeId",
		}
		for key, want := range wantQuery {
			if got := values.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		if len(values) != len(wantQuery) {
			t.Errorf("query = %s, want exactly %d parameters", values.Encode(), len(wantQuery))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificate(
		context.Background(),
		"cert-1",
		WithCertificatesFields([]string{"displayName", "serialNumber"}),
		WithCertificatesPassTypeIDFields([]string{"name", "identifier"}),
		WithCertificatesInclude([]string{"passTypeId"}),
	); err != nil {
		t.Fatalf("GetCertificate() error: %v", err)
	}
}
