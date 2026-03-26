package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

func TestReviewsRatingsSuccessOutputs(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/lookup":
			if got := req.URL.Query().Get("country"); got != "us" {
				t.Fatalf("expected lookup country=us, got %q", got)
			}
			if got := req.URL.Query().Get("entity"); got != "software" {
				t.Fatalf("expected lookup entity=software, got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"resultCount": 1,
					"results": [{
						"trackId": 123,
						"trackName": "Alpha",
						"averageUserRating": 4.5,
						"userRatingCount": 12,
						"averageUserRatingForCurrentVersion": 4.4,
						"userRatingCountForCurrentVersion": 11
					}]
				}`)),
				Header: http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/us/customer-reviews/id123":
			if got := req.Header.Get("X-Apple-Store-Front"); got != itunes.Storefronts["us"]+",12" {
				t.Fatalf("expected storefront header %q, got %q", itunes.Storefronts["us"]+",12", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`<span class="total">10</span><span class="total">1</span><span class="total">0</span><span class="total">0</span><span class="total">0</span>`)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			}, nil
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})

	tests := []struct {
		name        string
		args        []string
		wantStrings []string
		unmarshal   func(t *testing.T, stdout string)
	}{
		{
			name: "json",
			args: []string{"reviews", "ratings", "--app", "123", "--output", "json"},
			unmarshal: func(t *testing.T, stdout string) {
				t.Helper()
				var payload itunes.AppRatings
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal ratings: %v", err)
				}
				if payload.AppID != 123 || payload.Country != "US" || payload.RatingCount != 12 {
					t.Fatalf("unexpected ratings payload: %+v", payload)
				}
			},
		},
		{
			name:        "table",
			args:        []string{"reviews", "ratings", "--app", "123", "--output", "table"},
			wantStrings: []string{"Alpha", "App ID: 123 | Country: US", "Histogram", "5★"},
		},
		{
			name:        "markdown",
			args:        []string{"reviews", "ratings", "--app", "123", "--output", "markdown"},
			wantStrings: []string{"## Alpha", "**App ID:** 123 | **Country:** US", "### Histogram"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if test.unmarshal != nil {
				test.unmarshal(t, stdout)
			}
			for _, want := range test.wantStrings {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
				}
			}
		})
	}
}

func TestReviewsRatingsAcceptsCountryOutsideHistogramStorefrontMap(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("country"); got != "kz" {
			t.Fatalf("expected lookup country=kz, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"resultCount": 1,
				"results": [{
					"trackId": 123,
					"trackName": "Alpha",
					"averageUserRating": 4.5,
					"userRatingCount": 12
				}]
			}`)),
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "ratings", "--app", "123", "--country", "kz", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload itunes.AppRatings
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal ratings: %v", err)
	}
	if payload.Country != "KZ" {
		t.Fatalf("Country = %q, want KZ", payload.Country)
	}
	if len(payload.Histogram) != 0 {
		t.Fatalf("expected empty histogram, got %+v", payload.Histogram)
	}
}

func TestReviewsRatingsAllSuccess(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/lookup":
			country := req.URL.Query().Get("country")
			if country == "" {
				t.Fatal("expected country query parameter")
			}
			if country == "us" || country == "gb" {
				rating := "4.0"
				count := "100"
				if country == "gb" {
					rating = "5.0"
					count = "50"
				}
				body := `{"resultCount":1,"results":[{"trackId":123,"trackName":"Alpha","averageUserRating":` + rating + `,"userRatingCount":` + count + `}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"resultCount":0,"results":[]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/us/customer-reviews/id123", "/gb/customer-reviews/id123":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`<html></html>`)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			}, nil
		default:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"resultCount":0,"results":[]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "ratings", "--app", "123", "--all", "--workers", "4", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload itunes.GlobalRatings
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal global ratings: %v", err)
	}
	if payload.TotalCount != 150 || payload.CountryCount < 2 {
		t.Fatalf("unexpected global ratings payload: %+v", payload)
	}
	if len(payload.ByCountry) < 2 {
		t.Fatalf("expected at least 2 countries, got %+v", payload.ByCountry)
	}
	if payload.ByCountry[0].Country != "US" || payload.ByCountry[1].Country != "GB" {
		t.Fatalf("expected US then GB ordering, got %+v", payload.ByCountry[:2])
	}
}
