package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestOptimizeKeywordsRankUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"optimize", "keywords", "rank", "--keywords", "focus timer"},
			wantErr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:    "missing keywords",
			args:    []string{"optimize", "keywords", "rank", "--app", "1234567890"},
			wantErr: "--keywords is required",
		},
		{
			name:    "keyword word budget",
			args:    []string{"optimize", "keywords", "rank", "--app", "1234567890", "--keywords", "one two three four five"},
			wantErr: "must contain at most 4 space-separated words",
		},
		{
			name:    "keyword count budget",
			args:    []string{"optimize", "keywords", "rank", "--app", "1234567890", "--keywords", strings.Join(rankKeywordFixtures(101), ",")},
			wantErr: "--keywords accepts at most 100 keywords per invocation",
		},
		{
			name:    "positional argument",
			args:    []string{"optimize", "keywords", "rank", "--app", "1234567890", "--keywords", "focus timer", "extra"},
			wantErr: "optimize keywords rank does not accept positional arguments",
		},
		{
			name:    "unsupported platform",
			args:    []string{"optimize", "keywords", "rank", "--app", "1234567890", "--keywords", "focus timer", "--platform", "MAC_OS"},
			wantErr: "--platform must be one of: IOS, TV_OS",
		},
		{
			name:    "tv storefront without numeric id",
			args:    []string{"optimize", "keywords", "rank", "--app", "1234567890", "--keywords", "focus timer", "--platform", "TV_OS", "--country", "kz"},
			wantErr: "TV_OS ranking is unavailable for storefront KZ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_APP_ID", "")
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Errorf("unexpected request before input validation: %s", req.URL.String())
				return nil, errors.New("unexpected request")
			})

			stdout, stderr, runErr := runCommand(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestOptimizeKeywordsRankJSONComposesPublicSearchWindow(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "itunes.apple.com" || req.URL.Path != "/search" {
			t.Errorf("request URL = %s, want the public iTunes search endpoint", req.URL.String())
			return nil, errors.New("unexpected request URL")
		}
		if got := req.URL.Query().Get("country"); got != "de" {
			t.Errorf("country = %q, want de", got)
			return nil, errors.New("unexpected country")
		}
		switch req.URL.Query().Get("term") {
		case "focus timer":
			return jsonResponse(http.StatusOK, `{"resultCount":2,"results":[{"trackId":111},{"trackId":1234567890}]}`)
		case "habit tracker":
			return jsonResponse(http.StatusOK, `{"resultCount":1,"results":[{"trackId":111}]}`)
		case "broken keyword":
			return jsonResponse(http.StatusServiceUnavailable, `{}`)
		default:
			t.Errorf("unexpected term %q", req.URL.Query().Get("term"))
			return nil, errors.New("unexpected term")
		}
	})

	stdout, stderr, runErr := runCommand(t, []string{
		"optimize", "keywords", "rank",
		"--app", "1234567890",
		"--keywords", " Focus Timer , habit tracker ,broken keyword,focus timer",
		"--country", "DE",
		"--workers", "3",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload asc.KeywordRankReport
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if payload.SchemaVersion != "1" || payload.AppID != "1234567890" || payload.Country != "DE" || payload.Platform != "IOS" || payload.Workers != 3 {
		t.Fatalf("unexpected report identity: %+v", payload)
	}
	if len(payload.Rows) != 3 {
		t.Fatalf("rows = %d, want 3 deduplicated keywords", len(payload.Rows))
	}
	if payload.Rows[0].Keyword != "focus timer" || payload.Rows[0].Status != "available" ||
		payload.Rows[0].Rank == nil || *payload.Rows[0].Rank != 2 {
		t.Fatalf("ranked row = %+v", payload.Rows[0])
	}
	if payload.Rows[1].Status != "empty" || payload.Rows[1].Rank != nil {
		t.Fatalf("absent row = %+v", payload.Rows[1])
	}
	if payload.Rows[2].Status != "unavailable" || !strings.Contains(payload.Rows[2].Error, "503") {
		t.Fatalf("unavailable row = %+v", payload.Rows[2])
	}
	if payload.Summary.Keywords != 3 || payload.Summary.Ranked != 1 || payload.Summary.Absent != 1 || payload.Summary.Unavailable != 1 {
		t.Fatalf("summary = %+v", payload.Summary)
	}
}

func TestOptimizeKeywordsRankFailsWhenEveryKeywordLookupFails(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `{}`)
	})

	stdout, _, runErr := runCommand(t, []string{
		"optimize", "keywords", "rank",
		"--app", "1234567890",
		"--keywords", "focus timer,habit tracker",
		"--output", "json",
	})
	if runErr == nil {
		t.Fatal("expected an error when every keyword lookup fails")
	}
	if !strings.Contains(runErr.Error(), "optimize keywords rank") || !strings.Contains(runErr.Error(), "500") {
		t.Fatalf("error = %v, want the representative failure", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func rankKeywordFixtures(count int) []string {
	keywords := make([]string, 0, count)
	for index := range count {
		keywords = append(keywords, fmt.Sprintf("keyword%03d", index))
	}
	return keywords
}
