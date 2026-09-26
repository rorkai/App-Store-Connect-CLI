package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestClassifyAttemptedChild(t *testing.T) {
	tests := []struct {
		token string
		want  string
	}{
		{token: "List", want: "list"},
		{token: " push ", want: "push"},
		{token: "my-secret-app", want: "other"},
		{token: "6759231657", want: "other"},
		{token: "--help", want: "other"},
		{token: "path/to/file", want: "other"},
		{token: "", want: "other"},
	}
	for _, test := range tests {
		if got := ClassifyAttemptedChild(test.token); got != test.want {
			t.Fatalf("ClassifyAttemptedChild(%q) = %q, want %q", test.token, got, test.want)
		}
	}
}

func TestBuildEventAttemptedChildIsNullUnlessUnknownChild(t *testing.T) {
	leaf, ok := BuildEventWithContext("asc builds", "1.0.0", time.Millisecond, 2, EventContext{
		InvocationShape: InvocationShapeLeaf,
		AttemptedChild:  "list",
	})
	if !ok {
		t.Fatal("expected a leaf event")
	}
	encoded, err := json.Marshal(leaf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"attempted_child":null`) {
		t.Fatalf("leaf event = %s, want null attempted_child", encoded)
	}

	unknown, ok := BuildEventWithContext("asc builds", "1.0.0", time.Millisecond, 2, EventContext{
		InvocationShape: InvocationShapeUnknownChild,
		AttemptedChild:  "com.secret.app",
	})
	if !ok {
		t.Fatal("expected an unknown-child event")
	}
	encoded, err = json.Marshal(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"attempted_child":"other"`) {
		t.Fatalf("unknown event = %s, want other", encoded)
	}
	if strings.Contains(string(encoded), "com.secret.app") {
		t.Fatalf("raw token leaked: %s", encoded)
	}
}

func TestSchemaV4SpoolRecordOmitsAttemptedChild(t *testing.T) {
	event := Event{
		EventID:       "event-1",
		SchemaVersion: 4,
		CommandPath:   "asc builds",
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "attempted_child") {
		t.Fatalf("v4 record included attempted_child: %s", encoded)
	}
	var decoded Event
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 4 || decoded.AttemptedChild != nil {
		t.Fatalf("decoded v4 = %+v", decoded)
	}
}
