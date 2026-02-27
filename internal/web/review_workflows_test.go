package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testWebClient(server *httptest.Server) *Client {
	return &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
}

func TestListReviewSubmissionsParsesIncludedContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/reviewSubmissions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("include"); got == "" || !strings.Contains(got, "appStoreVersionForReview") {
			t.Fatalf("expected include query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "sub-1",
				"type": "reviewSubmissions",
				"attributes": {
					"state": "UNRESOLVED_ISSUES",
					"submittedDate": "2026-02-25T00:00:00Z"
				},
				"relationships": {
					"appStoreVersionForReview": {"data": {"type": "appStoreVersions", "id": "v1"}},
					"submittedByActor": {"data": {"type": "actors", "id": "actor-submit"}},
					"lastUpdatedByActor": {"data": {"type": "actors", "id": "actor-update"}}
				}
			}],
			"included": [
				{"id":"v1","type":"appStoreVersions","attributes":{"versionString":"1.2.3","platform":"IOS"}},
				{"id":"actor-submit","type":"actors","attributes":{"actorType":"USER","name":"Submit User"}},
				{"id":"actor-update","type":"actors","attributes":{"actorType":"USER","name":"Update User"}}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListReviewSubmissions(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListReviewSubmissions() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one submission, got %d", len(got))
	}
	if got[0].ID != "sub-1" || got[0].State != "UNRESOLVED_ISSUES" {
		t.Fatalf("unexpected submission: %#v", got[0])
	}
	if got[0].AppStoreVersionForReview == nil || got[0].AppStoreVersionForReview.Version != "1.2.3" {
		t.Fatalf("expected app store version context, got %#v", got[0].AppStoreVersionForReview)
	}
	if got[0].SubmittedByActor == nil || got[0].SubmittedByActor.Name != "Submit User" {
		t.Fatalf("expected submitted actor context, got %#v", got[0].SubmittedByActor)
	}
}

func TestListReviewSubmissionItemsFlattensRelationships(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reviewSubmissions/sub-1/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "item-1",
				"type": "reviewSubmissionItems",
				"relationships": {
					"appStoreVersion": {"data": {"type":"appStoreVersions","id":"v1"}},
					"appEvent": {"data": [{"type":"appEvents","id":"e1"},{"type":"appEvents","id":"e2"}]}
				}
			}]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	items, err := client.ListReviewSubmissionItems(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("ListReviewSubmissionItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if len(items[0].Related) != 3 {
		t.Fatalf("expected 3 related resources, got %#v", items[0].Related)
	}
}

func TestListResolutionCenterMessagesSupportsPlainText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resolutionCenterThreads/thread-1/resolutionCenterMessages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "m1",
				"type": "resolutionCenterMessages",
				"attributes": {"createdDate":"2026-02-25T10:00:00Z","messageBody":"<p>Hello <strong>World</strong></p>"},
				"relationships": {
					"fromActor": {"data": {"type":"actors","id":"actor-1"}},
					"rejections": {"data": [{"type":"reviewRejections","id":"rej-1"}]},
					"resolutionCenterMessageAttachments": {"data": [{"type":"resolutionCenterMessageAttachments","id":"att-1"}]}
				}
			}],
			"included": [
				{"id":"actor-1","type":"actors","attributes":{"actorType":"USER","name":"Reviewer"}}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	messages, err := client.ListResolutionCenterMessages(context.Background(), "thread-1", true)
	if err != nil {
		t.Fatalf("ListResolutionCenterMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	if messages[0].MessageBodyPlain != "Hello World" {
		t.Fatalf("expected plain text body, got %q", messages[0].MessageBodyPlain)
	}
	if messages[0].FromActor == nil || messages[0].FromActor.ActorType != "USER" {
		t.Fatalf("expected actor context, got %#v", messages[0].FromActor)
	}
}

func TestListReviewRejectionsParsesReasons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reviewRejections" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("filter[resolutionCenterMessage.resolutionCenterThread]"); got != "thread-1" {
			t.Fatalf("unexpected thread filter: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "rej-1",
				"type": "reviewRejections",
				"attributes": {
					"reasons": [{
						"reasonSection": "2.1",
						"reasonDescription": "App crashed during review",
						"reasonCode": "2.1.0"
					}]
				},
				"relationships": {
					"rejectionAttachments": {"data": [{"type":"rejectionAttachments","id":"ratt-1"}]}
				}
			}]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	rejections, err := client.ListReviewRejections(context.Background(), "thread-1")
	if err != nil {
		t.Fatalf("ListReviewRejections() error = %v", err)
	}
	if len(rejections) != 1 {
		t.Fatalf("expected one rejection, got %d", len(rejections))
	}
	if len(rejections[0].Reasons) != 1 || rejections[0].Reasons[0].ReasonCode != "2.1.0" {
		t.Fatalf("unexpected reasons: %#v", rejections[0].Reasons)
	}
	if len(rejections[0].AttachmentIDs) != 1 || rejections[0].AttachmentIDs[0] != "ratt-1" {
		t.Fatalf("unexpected attachment ids: %#v", rejections[0].AttachmentIDs)
	}
}

func TestListReviewAttachmentsByThreadCollectsMessageAndRejectionAttachments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/resolutionCenterThreads/thread-1/resolutionCenterMessages":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "m1",
					"type": "resolutionCenterMessages",
					"relationships": {
						"resolutionCenterMessageAttachments": {"data": [{"type":"resolutionCenterMessageAttachments","id":"att-1"}]}
					}
				}],
				"included": [{
					"id":"att-1",
					"type":"resolutionCenterMessageAttachments",
					"attributes":{
						"fileName":"Screenshot-1.png",
						"fileSize":1024,
						"assetDeliveryState":"AVAILABLE",
						"downloadUrl":"https://example.invalid/download/att-1"
					}
				}]
			}`))
		case "/reviewRejections":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [{
					"id":"rej-1",
					"type":"reviewRejections",
					"relationships":{"rejectionAttachments":{"data":[{"type":"rejectionAttachments","id":"ratt-1"}]}}
				}],
				"included": [{
					"id":"ratt-1",
					"type":"rejectionAttachments",
					"attributes":{
						"fileName":"Crash.png",
						"fileSize":2048,
						"assetDeliveryState":"AVAILABLE",
						"downloadUrl":"https://example.invalid/download/ratt-1"
					}
				}]
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	attachments, err := client.ListReviewAttachmentsByThread(context.Background(), "thread-1", false)
	if err != nil {
		t.Fatalf("ListReviewAttachmentsByThread() error = %v", err)
	}
	if len(attachments) != 2 {
		t.Fatalf("expected two attachments, got %#v", attachments)
	}
	for _, attachment := range attachments {
		if attachment.DownloadURL != "" {
			t.Fatalf("expected signed URL to be redacted by default, got %#v", attachment)
		}
	}

	attachmentsWithURL, err := client.ListReviewAttachmentsByThread(context.Background(), "thread-1", true)
	if err != nil {
		t.Fatalf("ListReviewAttachmentsByThread(include-url) error = %v", err)
	}
	hasURL := false
	for _, attachment := range attachmentsWithURL {
		if strings.TrimSpace(attachment.DownloadURL) != "" {
			hasURL = true
		}
	}
	if !hasURL {
		t.Fatalf("expected at least one attachment to include signed URL, got %#v", attachmentsWithURL)
	}
}

func TestListReviewThreadDetailsUsesSinglePassThreadCalls(t *testing.T) {
	messageCalls := 0
	rejectionCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/resolutionCenterThreads/thread-1/resolutionCenterMessages":
			messageCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "m1",
					"type": "resolutionCenterMessages",
					"attributes": {"createdDate":"2026-02-25T10:00:00Z","messageBody":"<p>Hello <strong>World</strong></p>"},
					"relationships": {
						"rejections": {"data": [{"type":"reviewRejections","id":"rej-1"}]},
						"resolutionCenterMessageAttachments": {"data": [{"type":"resolutionCenterMessageAttachments","id":"att-1"}]}
					}
				}],
				"included": [{
					"id":"att-1",
					"type":"resolutionCenterMessageAttachments",
					"attributes":{
						"fileName":"Screenshot-1.png",
						"fileSize":1024,
						"assetDeliveryState":"AVAILABLE",
						"downloadUrl":"https://example.invalid/download/att-1"
					}
				}]
			}`))
		case "/reviewRejections":
			rejectionCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [{
					"id":"rej-1",
					"type":"reviewRejections",
					"attributes":{
						"reasons":[{"reasonSection":"2.1","reasonDescription":"Crash","reasonCode":"2.1.0"}]
					},
					"relationships":{"rejectionAttachments":{"data":[{"type":"rejectionAttachments","id":"ratt-1"}]}}
				}],
				"included": [{
					"id":"ratt-1",
					"type":"rejectionAttachments",
					"attributes":{
						"fileName":"Crash.png",
						"fileSize":2048,
						"assetDeliveryState":"AVAILABLE",
						"downloadUrl":"https://example.invalid/download/ratt-1"
					}
				}]
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	details, err := client.ListReviewThreadDetails(context.Background(), "thread-1", true, true)
	if err != nil {
		t.Fatalf("ListReviewThreadDetails() error = %v", err)
	}
	if messageCalls != 1 || rejectionCalls != 1 {
		t.Fatalf("expected one messages and one rejections call, got messages=%d rejections=%d", messageCalls, rejectionCalls)
	}
	if len(details.Messages) != 1 || details.Messages[0].MessageBodyPlain != "Hello World" {
		t.Fatalf("unexpected messages: %#v", details.Messages)
	}
	if len(details.Rejections) != 1 || len(details.Rejections[0].Reasons) != 1 {
		t.Fatalf("unexpected rejections: %#v", details.Rejections)
	}
	if len(details.Attachments) != 2 {
		t.Fatalf("expected two attachments, got %#v", details.Attachments)
	}
	for _, attachment := range details.Attachments {
		if strings.TrimSpace(attachment.DownloadURL) == "" {
			t.Fatalf("expected attachment download URL, got %#v", attachment)
		}
	}
}

func TestDownloadAttachmentReturnsStatusAndBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("PNGDATA"))
		case "/expired":
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte("expired"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	t.Setenv(attachmentHostsEnv, parsedURL.Hostname())

	client := testWebClient(server)
	body, status, err := client.DownloadAttachment(context.Background(), server.URL+"/ok")
	if err != nil {
		t.Fatalf("DownloadAttachment(ok) error = %v", err)
	}
	if status != http.StatusOK || string(body) != "PNGDATA" {
		t.Fatalf("unexpected success response: status=%d body=%q", status, string(body))
	}

	_, status, err = client.DownloadAttachment(context.Background(), server.URL+"/expired")
	if err == nil {
		t.Fatalf("expected error for expired download URL")
	}
	if status != http.StatusGone {
		t.Fatalf("expected gone status, got %d", status)
	}
}

func TestDownloadAttachmentErrorDoesNotLeakSignedURLTokens(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	t.Setenv(attachmentHostsEnv, parsedURL.Hostname())

	client := testWebClient(server)
	signedURL := server.URL + "/download?token=very-secret&X-Amz-Signature=abc123"
	server.Close()

	_, _, err = client.DownloadAttachment(context.Background(), signedURL)
	if err == nil {
		t.Fatal("expected download error after server close")
	}
	if strings.Contains(err.Error(), "very-secret") || strings.Contains(err.Error(), "X-Amz-Signature") || strings.Contains(err.Error(), "?token=") {
		t.Fatalf("expected redacted error message, got %q", err.Error())
	}
}

func TestDownloadAttachmentRejectsNonHTTPS(t *testing.T) {
	client := &Client{httpClient: &http.Client{}}

	_, _, err := client.DownloadAttachment(context.Background(), "http://appstoreconnect.apple.com/download")
	if err == nil {
		t.Fatal("expected non-https URL to fail")
	}
	if !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadAttachmentRejectsUntrustedHost(t *testing.T) {
	client := &Client{httpClient: &http.Client{}}

	_, _, err := client.DownloadAttachment(context.Background(), "https://example.invalid/download")
	if err == nil {
		t.Fatal("expected untrusted host URL to fail")
	}
	if !strings.Contains(err.Error(), "host is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
