package asc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func rawResponse(body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/a-gzip"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestBuildSalesReportQuery(t *testing.T) {
	query := buildSalesReportQuery(SalesReportParams{
		VendorNumber:  "12345678",
		ReportType:    SalesReportTypeSales,
		ReportSubType: SalesReportSubTypeSummary,
		Frequency:     SalesReportFrequencyDaily,
		ReportDate:    "2024-01-20",
		Version:       SalesReportVersion1_0,
	})

	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[vendorNumber]"); got != "12345678" {
		t.Fatalf("expected vendorNumber filter, got %q", got)
	}
	if got := values.Get("filter[reportType]"); got != "SALES" {
		t.Fatalf("expected reportType filter, got %q", got)
	}
	if got := values.Get("filter[reportSubType]"); got != "SUMMARY" {
		t.Fatalf("expected reportSubType filter, got %q", got)
	}
	if got := values.Get("filter[frequency]"); got != "DAILY" {
		t.Fatalf("expected frequency filter, got %q", got)
	}
	if got := values.Get("filter[reportDate]"); got != "2024-01-20" {
		t.Fatalf("expected reportDate filter, got %q", got)
	}
	if got := values.Get("filter[version]"); got != "1_0" {
		t.Fatalf("expected version filter, got %q", got)
	}
}

func TestBuildSalesReportQuerySupportsDocumentedInstallsSubtype(t *testing.T) {
	query := buildSalesReportQuery(SalesReportParams{
		VendorNumber:  "12345678",
		ReportType:    SalesReportTypeInstalls,
		ReportSubType: SalesReportSubTypeSummaryChannel,
		Frequency:     SalesReportFrequencyYearly,
		ReportDate:    "2024-12-31",
		Version:       SalesReportVersion1_0,
	})
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[reportType]"); got != "INSTALLS" {
		t.Fatalf("expected reportType INSTALLS, got %q", got)
	}
	if got := values.Get("filter[reportSubType]"); got != "SUMMARY_CHANNEL" {
		t.Fatalf("expected reportSubType SUMMARY_CHANNEL, got %q", got)
	}
}

func TestGetSalesReport_SendsRequest(t *testing.T) {
	response := rawResponse("gzdata")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/salesReports" {
			t.Fatalf("expected path /v1/salesReports, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[vendorNumber]") != "12345678" {
			t.Fatalf("expected vendorNumber filter, got %q", values.Get("filter[vendorNumber]"))
		}
		if req.Header.Get("Accept") != "application/a-gzip" {
			t.Fatalf("expected gzip Accept header, got %q", req.Header.Get("Accept"))
		}
		assertAuthorized(t, req)
	}, response)

	download, err := client.GetSalesReport(context.Background(), SalesReportParams{
		VendorNumber:  "12345678",
		ReportType:    SalesReportTypeSales,
		ReportSubType: SalesReportSubTypeSummary,
		Frequency:     SalesReportFrequencyDaily,
		ReportDate:    "2024-01-20",
		Version:       SalesReportVersion1_0,
	})
	if err != nil {
		t.Fatalf("GetSalesReport() error: %v", err)
	}
	_ = download.Body.Close()
}

func TestGetSalesReport_ErrorResponse(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"code":"FORBIDDEN","title":"Forbidden","detail":"nope"}]}`)
	client := newTestClient(t, nil, response)
	_, err := client.GetSalesReport(context.Background(), SalesReportParams{
		VendorNumber:  "12345678",
		ReportType:    SalesReportTypeSales,
		ReportSubType: SalesReportSubTypeSummary,
		Frequency:     SalesReportFrequencyDaily,
		ReportDate:    "2024-01-20",
		Version:       SalesReportVersion1_0,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestCreateAnalyticsReportRequest_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"analyticsReportRequests","id":"req-1","attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReportRequests" {
			t.Fatalf("expected path /v1/analyticsReportRequests, got %s", req.URL.Path)
		}
		var payload AnalyticsReportRequestCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAnalyticsReportRequests {
			t.Fatalf("expected type analyticsReportRequests, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.AccessType != AnalyticsAccessTypeOngoing {
			t.Fatalf("expected accessType ONGOING, got %q", payload.Data.Attributes.AccessType)
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAnalyticsReportRequest(context.Background(), "app-1", AnalyticsAccessTypeOngoing); err != nil {
		t.Fatalf("CreateAnalyticsReportRequest() error: %v", err)
	}
}

func TestGetAnalyticsReportRequests_WithAccessTypeFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/analyticsReportRequests" {
			t.Fatalf("expected path /v1/apps/app-1/analyticsReportRequests, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[accessType]") != "ONGOING" {
			t.Fatalf("expected filter[accessType]=ONGOING, got %q", values.Get("filter[accessType]"))
		}
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAnalyticsReportRequests(
		context.Background(),
		"app-1",
		WithAnalyticsReportRequestsLimit(10),
		WithAnalyticsReportRequestsAccessType(AnalyticsAccessTypeOngoing),
	); err != nil {
		t.Fatalf("GetAnalyticsReportRequests() error: %v", err)
	}
}

func TestGetAnalyticsReportRequest_ByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"analyticsReportRequests","id":"req-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReportRequests/req-1" {
			t.Fatalf("expected path /v1/analyticsReportRequests/req-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAnalyticsReportRequest(context.Background(), "req-1"); err != nil {
		t.Fatalf("GetAnalyticsReportRequest() error: %v", err)
	}
}

func TestDeleteAnalyticsReportRequest_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReportRequests/req-1" {
			t.Fatalf("expected path /v1/analyticsReportRequests/req-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAnalyticsReportRequest(context.Background(), "req-1"); err != nil {
		t.Fatalf("DeleteAnalyticsReportRequest() error: %v", err)
	}
}

func TestAnalyticsRelationshipEndpoints_WithLinkagesLimit(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		path     string
		response string
		call     func(*testing.T, *Client)
	}{
		{
			name:     "GetAnalyticsReportInstancesRelationships",
			path:     "/v1/analyticsReports/report-1/relationships/instances",
			response: `{"data":[{"type":"analyticsReportInstances","id":"inst-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetAnalyticsReportInstancesRelationships(ctx, "report-1", WithLinkagesLimit(10))
				if err != nil {
					t.Fatalf("GetAnalyticsReportInstancesRelationships() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "inst-1" {
					t.Fatalf("expected decoded analytics instance relationship, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAnalyticsReportInstanceSegmentsRelationships",
			path:     "/v1/analyticsReportInstances/inst-1/relationships/segments",
			response: `{"data":[{"type":"analyticsReportSegments","id":"seg-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetAnalyticsReportInstanceSegmentsRelationships(ctx, "inst-1", WithLinkagesLimit(10))
				if err != nil {
					t.Fatalf("GetAnalyticsReportInstanceSegmentsRelationships() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "seg-1" {
					t.Fatalf("expected decoded analytics segment relationship, got %+v", resp.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != tt.path {
					t.Fatalf("expected path %s, got %s", tt.path, req.URL.Path)
				}
				if got := req.URL.Query().Get("limit"); got != "10" {
					t.Fatalf("expected limit=10, got %q", got)
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.response))

			tt.call(t, client)
		})
	}
}

func TestAnalyticsListEndpoints_UseNextURL(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		next     string
		response string
		call     func(*testing.T, *Client, string)
	}{
		{
			name:     "GetAnalyticsReports",
			next:     "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/req-1/reports?cursor=abc",
			response: `{"data":[{"type":"analyticsReports","id":"report-1"}]}`,
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetAnalyticsReports(ctx, "req-1", WithAnalyticsReportsNextURL(next))
				if err != nil {
					t.Fatalf("GetAnalyticsReports() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "report-1" {
					t.Fatalf("expected decoded analytics report, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAnalyticsReportInstances",
			next:     "https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/instances?cursor=abc",
			response: `{"data":[{"type":"analyticsReportInstances","id":"inst-1"}]}`,
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetAnalyticsReportInstances(ctx, "report-1", WithAnalyticsReportInstancesNextURL(next))
				if err != nil {
					t.Fatalf("GetAnalyticsReportInstances() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "inst-1" {
					t.Fatalf("expected decoded analytics report instance, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAnalyticsReportSegments",
			next:     "https://api.appstoreconnect.apple.com/v1/analyticsReportInstances/inst-1/segments?cursor=abc",
			response: `{"data":[{"type":"analyticsReportSegments","id":"seg-1"}]}`,
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetAnalyticsReportSegments(ctx, "inst-1", WithAnalyticsReportSegmentsNextURL(next))
				if err != nil {
					t.Fatalf("GetAnalyticsReportSegments() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "seg-1" {
					t.Fatalf("expected decoded analytics report segment, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAnalyticsReportInstancesRelationships",
			next:     "https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/relationships/instances?cursor=abc",
			response: `{"data":[{"type":"analyticsReportInstances","id":"inst-1"}]}`,
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetAnalyticsReportInstancesRelationships(ctx, "report-1", WithLinkagesNextURL(next))
				if err != nil {
					t.Fatalf("GetAnalyticsReportInstancesRelationships() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "inst-1" {
					t.Fatalf("expected decoded analytics report instance relationship, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAnalyticsReportInstanceSegmentsRelationships",
			next:     "https://api.appstoreconnect.apple.com/v1/analyticsReportInstances/inst-1/relationships/segments?cursor=abc",
			response: `{"data":[{"type":"analyticsReportSegments","id":"seg-1"}]}`,
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetAnalyticsReportInstanceSegmentsRelationships(ctx, "inst-1", WithLinkagesNextURL(next))
				if err != nil {
					t.Fatalf("GetAnalyticsReportInstanceSegmentsRelationships() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "seg-1" {
					t.Fatalf("expected decoded analytics report segment relationship, got %+v", resp.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.URL.String() != tt.next {
					t.Fatalf("expected next URL %q, got %q", tt.next, req.URL.String())
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.response))

			tt.call(t, client, tt.next)
		})
	}
}

func TestGetAnalyticsReport_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"analyticsReports","id":"report-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReports/report-1" {
			t.Fatalf("expected path /v1/analyticsReports/report-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAnalyticsReport(context.Background(), "report-1"); err != nil {
		t.Fatalf("GetAnalyticsReport() error: %v", err)
	}
}

func TestGetAnalyticsReportInstance_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"analyticsReportInstances","id":"inst-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReportInstances/inst-1" {
			t.Fatalf("expected path /v1/analyticsReportInstances/inst-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAnalyticsReportInstance(context.Background(), "inst-1"); err != nil {
		t.Fatalf("GetAnalyticsReportInstance() error: %v", err)
	}
}

func TestAnalyticsResourceEndpointsRejectIDsThatEscapePathSegment(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "GetAnalyticsReportRequest",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReportRequest(context.Background(), "req-1/reports")
				return err
			},
		},
		{
			name: "DeleteAnalyticsReportRequest",
			call: func(client *Client) error {
				return client.DeleteAnalyticsReportRequest(context.Background(), "req-1/reports")
			},
		},
		{
			name: "GetAnalyticsReport",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReport(context.Background(), "report-1/instances")
				return err
			},
		},
		{
			name: "GetAnalyticsReports",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReports(context.Background(), "req-1/reports")
				return err
			},
		},
		{
			name: "GetAnalyticsReportInstances",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReportInstances(context.Background(), "report-1/instances")
				return err
			},
		},
		{
			name: "GetAnalyticsReportInstance",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReportInstance(context.Background(), "inst-1/relationships/reports")
				return err
			},
		},
		{
			name: "GetAnalyticsReportInstanceSegmentsRelationships",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReportInstanceSegmentsRelationships(context.Background(), "inst-1/relationships/reports")
				return err
			},
		},
		{
			name: "GetAnalyticsReportSegments",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReportSegments(context.Background(), "inst-1/relationships/reports")
				return err
			},
		},
		{
			name: "GetAnalyticsReportSegment",
			call: func(client *Client) error {
				_, err := client.GetAnalyticsReportSegment(context.Background(), "seg-1/relationships/report")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			client := newTestClient(t, func(_ *http.Request) {
				requestCount++
			}, jsonResponse(http.StatusOK, `{}`))

			err := tt.call(client)
			if err == nil || !strings.Contains(err.Error(), "single path segment") {
				t.Fatalf("error = %v, want path-segment rejection", err)
			}
			if requestCount != 0 {
				t.Fatalf("request count = %d, want 0", requestCount)
			}
		})
	}
}

func TestGetAnalyticsReportSegment_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"analyticsReportSegments","id":"seg-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReportSegments/seg-1" {
			t.Fatalf("expected path /v1/analyticsReportSegments/seg-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAnalyticsReportSegment(context.Background(), "seg-1"); err != nil {
		t.Fatalf("GetAnalyticsReportSegment() error: %v", err)
	}
}

func TestDownloadAnalyticsReport_NoAuthHeader(t *testing.T) {
	// Use an allowed host for the download URL
	downloadURL := "https://mzstatic.com/report.gz"
	response := rawResponse("gzdata")
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != downloadURL {
			t.Fatalf("expected URL %q, got %q", downloadURL, req.URL.String())
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Authorization header")
		}
	}, response)

	download, err := client.DownloadAnalyticsReport(context.Background(), downloadURL)
	if err != nil {
		t.Fatalf("DownloadAnalyticsReport() error: %v", err)
	}
	_ = download.Body.Close()
}

func TestDownloadAnalyticsReport_InvalidHost(t *testing.T) {
	// Test that URLs from untrusted hosts are rejected
	downloadURL := "https://example.com/report.gz"
	client := newTestClient(t, nil, nil)

	_, err := client.DownloadAnalyticsReport(context.Background(), downloadURL)
	if err == nil {
		t.Fatal("expected error for untrusted host, got nil")
	}
}

func TestDownloadAnalyticsReport_InsecureScheme(t *testing.T) {
	// Test that HTTP URLs are rejected
	downloadURL := "http://mzstatic.com/report.gz"
	client := newTestClient(t, nil, nil)

	_, err := client.DownloadAnalyticsReport(context.Background(), downloadURL)
	if err == nil {
		t.Fatalf("expected error for insecure scheme, got nil")
	}
}

func TestDownloadAnalyticsReport_CDNHostRequiresSignature(t *testing.T) {
	downloadURL := "https://example.cloudfront.net/report.gz"
	client := newTestClient(t, nil, nil)

	_, err := client.DownloadAnalyticsReport(context.Background(), downloadURL)
	if err == nil {
		t.Fatal("expected error for unsigned CDN host, got nil")
	}
}

func TestDownloadAnalyticsReport_CDNHostWithSignature(t *testing.T) {
	downloadURL := "https://example.cloudfront.net/report.gz?Signature=abc&Key-Pair-Id=key"
	response := rawResponse("gzdata")
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != downloadURL {
			t.Fatalf("expected URL %q, got %q", downloadURL, req.URL.String())
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Authorization header")
		}
	}, response)

	download, err := client.DownloadAnalyticsReport(context.Background(), downloadURL)
	if err != nil {
		t.Fatalf("DownloadAnalyticsReport() error: %v", err)
	}
	_ = download.Body.Close()
}

func TestDownloadURLParseErrorsOmitSignedQuery(t *testing.T) {
	const secret = "secret-signature"
	downloadURL := "https://example.cloudfront.net:bad/report.gz?Signature=" + secret + "&Key-Pair-Id=key"
	client := newTestClient(t, nil, nil)

	_, analyticsErr := client.DownloadAnalyticsReport(context.Background(), downloadURL)
	_, artifactErr := client.DownloadCiArtifact(context.Background(), downloadURL)
	for name, err := range map[string]error{"analytics": analyticsErr, "ci artifact": artifactErr} {
		if err == nil {
			t.Fatalf("%s: expected error for unparsable URL, got nil", name)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("%s: error leaks signed query: %v", name, err)
		}
	}
}
