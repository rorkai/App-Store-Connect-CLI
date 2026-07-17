package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAgeRatingEditSendsSocialMediaFields(t *testing.T) {
	setupAuth(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/ageRatingDeclarations/age-441" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		var payload asc.AgeRatingDeclarationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.SocialMedia == nil || !*payload.Data.Attributes.SocialMedia {
			t.Fatalf("expected socialMedia=true, got %#v", payload.Data.Attributes.SocialMedia)
		}
		if payload.Data.Attributes.SocialMediaAgeRestricted == nil || *payload.Data.Attributes.SocialMediaAgeRestricted {
			t.Fatalf("expected socialMediaAgeRestricted=false, got %#v", payload.Data.Attributes.SocialMediaAgeRestricted)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"ageRatingDeclarations","id":"age-441","attributes":{"socialMedia":true,"socialMediaAgeRestricted":false}}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"age-rating", "edit",
			"--id", "age-441",
			"--social-media", "true",
			"--social-media-age-restricted", "false",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var response asc.AgeRatingDeclarationResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
	}
	if response.Data.Attributes.SocialMedia == nil || !*response.Data.Attributes.SocialMedia {
		t.Fatalf("expected socialMedia=true output, got %#v", response.Data.Attributes.SocialMedia)
	}
	if response.Data.Attributes.SocialMediaAgeRestricted == nil || *response.Data.Attributes.SocialMediaAgeRestricted {
		t.Fatalf("expected socialMediaAgeRestricted=false output, got %#v", response.Data.Attributes.SocialMediaAgeRestricted)
	}
}

func TestAgeRatingEditInvalidSocialMediaReturnsUsageExit(t *testing.T) {
	assertUsageExit(t, []string{
		"age-rating", "edit", "--id", "age-441", "--social-media", "sometimes",
	}, "--social-media must be true or false")
}

func TestAgeRatingEditRejectsExplicitDependencyContradictions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "social media with user generated content disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media", "true",
				"--user-generated-content", "false",
			},
			wantErr: "--social-media true cannot be combined with --user-generated-content false",
		},
		{
			name: "age restricted social media with age assurance disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media-age-restricted", "true",
				"--age-assurance", "false",
			},
			wantErr: "--social-media-age-restricted true cannot be combined with --age-assurance false",
		},
		{
			name: "age restricted social media with social media disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media-age-restricted", "true",
				"--social-media", "false",
			},
			wantErr: "--social-media-age-restricted true cannot be combined with --social-media false",
		},
		{
			name: "age restricted social media with user generated content disabled",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--social-media-age-restricted", "true",
				"--user-generated-content", "false",
			},
			wantErr: "--social-media-age-restricted true cannot be combined with --user-generated-content false",
		},
		{
			name: "all none materializes conflicting false prerequisites",
			args: []string{
				"age-rating", "edit", "--id", "age-441",
				"--all-none",
				"--social-media", "true",
			},
			wantErr: "--social-media true cannot be combined with --user-generated-content false",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}
