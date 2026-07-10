package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAgeRatingSetAllNoneUsesSafeDefaultsAndPreservesOverrides(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var seenRequest asc.AgeRatingDeclarationUpdateRequest
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ageRatingDeclarations/age-1" {
			t.Fatalf("expected age rating update path, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if err := json.Unmarshal(body, &seenRequest); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"data":{
					"type":"ageRatingDeclarations",
					"id":"age-1",
					"attributes":{
						"advertising":false,
						"gambling":true,
						"violenceRealistic":"FREQUENT_OR_INTENSE",
						"contests":"NONE"
					}
				}
			}`)),
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"age-rating", "edit",
			"--id", "age-1",
			"--all-none",
			"--gambling", "true",
			"--violence-realistic", "FREQUENT_OR_INTENSE",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}

	if seenRequest.Data.ID != "age-1" {
		t.Fatalf("expected declaration id age-1, got %q", seenRequest.Data.ID)
	}
	if seenRequest.Data.Attributes.Advertising == nil || *seenRequest.Data.Attributes.Advertising {
		t.Fatalf("expected advertising=false default, got %#v", seenRequest.Data.Attributes.Advertising)
	}
	if seenRequest.Data.Attributes.Gambling == nil || !*seenRequest.Data.Attributes.Gambling {
		t.Fatalf("expected gambling=true override, got %#v", seenRequest.Data.Attributes.Gambling)
	}
	if seenRequest.Data.Attributes.Contests == nil || *seenRequest.Data.Attributes.Contests != "NONE" {
		t.Fatalf("expected contests=NONE default, got %#v", seenRequest.Data.Attributes.Contests)
	}
	if seenRequest.Data.Attributes.ViolenceRealistic == nil || *seenRequest.Data.Attributes.ViolenceRealistic != "FREQUENT_OR_INTENSE" {
		t.Fatalf("expected violenceRealistic override, got %#v", seenRequest.Data.Attributes.ViolenceRealistic)
	}
	if seenRequest.Data.Attributes.KidsAgeBand != nil {
		t.Fatalf("expected kidsAgeBand to remain unset, got %#v", seenRequest.Data.Attributes.KidsAgeBand)
	}
	if seenRequest.Data.Attributes.DeveloperAgeRatingInfoURL != nil {
		t.Fatalf("expected developerAgeRatingInfoURL to remain unset, got %#v", seenRequest.Data.Attributes.DeveloperAgeRatingInfoURL)
	}
}
