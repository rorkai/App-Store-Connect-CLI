package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateResolutionCenterDraftMessageBuildsCapturedRequest(t *testing.T) {
	messageBody := "\n  Reply body \t\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/resolutionCenterDraftMessages" {
			t.Fatalf("unexpected draft create request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		decodeReviewWriteBody(t, r, &body)
		data := reviewWriteMap(t, body, "data")
		if got := data["type"]; got != "resolutionCenterDraftMessages" {
			t.Fatalf("draft type = %#v", got)
		}
		attributes := reviewWriteMap(t, data, "attributes")
		if got := attributes["messageBody"]; got != messageBody {
			t.Fatalf("messageBody = %#v, want %q", got, messageBody)
		}
		relationships := reviewWriteMap(t, data, "relationships")
		thread := reviewWriteMap(t, relationships, "resolutionCenterThread")
		threadData := reviewWriteMap(t, thread, "data")
		if got := threadData["type"]; got != "resolutionCenterThreads" {
			t.Fatalf("thread type = %#v", got)
		}
		if got := threadData["id"]; got != "thread-1" {
			t.Fatalf("thread id = %#v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"id":"draft-1","type":"resolutionCenterDraftMessages","attributes":{"messageBody":"Reply body"}}}`)
	}))
	defer server.Close()

	draft, err := testWebClient(server).CreateResolutionCenterDraftMessage(context.Background(), "thread-1", messageBody)
	if err != nil {
		t.Fatalf("CreateResolutionCenterDraftMessage() error = %v", err)
	}
	if draft == nil || draft.ID != "draft-1" || draft.ThreadID != "thread-1" {
		t.Fatalf("unexpected draft: %#v", draft)
	}
}

func TestUpdateResolutionCenterDraftMessageBuildsCapturedRequest(t *testing.T) {
	messageBody := "\nUpdated body  \n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/resolutionCenterDraftMessages/draft-1" {
			t.Fatalf("unexpected draft update request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		decodeReviewWriteBody(t, r, &body)
		data := reviewWriteMap(t, body, "data")
		if got := data["id"]; got != "draft-1" {
			t.Fatalf("draft id = %#v", got)
		}
		if got := data["type"]; got != "resolutionCenterDraftMessages" {
			t.Fatalf("draft type = %#v", got)
		}
		attributes := reviewWriteMap(t, data, "attributes")
		if got := attributes["messageBody"]; got != messageBody {
			t.Fatalf("messageBody = %#v, want %q", got, messageBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"id":"draft-1","type":"resolutionCenterDraftMessages","attributes":{"messageBody":"Updated body"}}}`)
	}))
	defer server.Close()

	draft, err := testWebClient(server).UpdateResolutionCenterDraftMessage(context.Background(), "draft-1", messageBody)
	if err != nil {
		t.Fatalf("UpdateResolutionCenterDraftMessage() error = %v", err)
	}
	if draft == nil || draft.ID != "draft-1" || draft.MessageBody != "Updated body" {
		t.Fatalf("unexpected draft: %#v", draft)
	}
}

func TestSendResolutionCenterDraftMessageBuildsCapturedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/resolutionCenterMessages" {
			t.Fatalf("unexpected send request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		decodeReviewWriteBody(t, r, &body)
		data := reviewWriteMap(t, body, "data")
		if got := data["type"]; got != "resolutionCenterMessages" {
			t.Fatalf("message type = %#v", got)
		}
		relationships := reviewWriteMap(t, data, "relationships")
		draft := reviewWriteMap(t, relationships, "createFromDraftMessage")
		draftData := reviewWriteMap(t, draft, "data")
		if got := draftData["type"]; got != "resolutionCenterDraftMessages" {
			t.Fatalf("draft type = %#v", got)
		}
		if got := draftData["id"]; got != "draft-1" {
			t.Fatalf("draft id = %#v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}}`)
	}))
	defer server.Close()

	message, err := testWebClient(server).SendResolutionCenterDraftMessage(context.Background(), "draft-1")
	if err != nil {
		t.Fatalf("SendResolutionCenterDraftMessage() error = %v", err)
	}
	if message == nil || message.ID != "message-1" || message.MessageBody != "Reply body" {
		t.Fatalf("unexpected message: %#v", message)
	}
}

func TestSendResolutionCenterDraftMessageDoesNotRetryAfterServerError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/resolutionCenterMessages" {
			t.Fatalf("unexpected send request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"errors":[{"code":"MESSAGE_CREATE_FAILED"}]}`)
	}))
	defer server.Close()

	if _, err := testWebClient(server).SendResolutionCenterDraftMessage(context.Background(), "draft-1"); err == nil {
		t.Fatal("expected send error")
	}
	if requests != 1 {
		t.Fatalf("send requests = %d, want 1 after a server error", requests)
	}
}

func decodeReviewWriteBody(t *testing.T, r *http.Request, target any) {
	t.Helper()
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
}

func reviewWriteMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", key, parent[key])
	}
	return value
}
