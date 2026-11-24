package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppClipsAdvancedExperiencesCreateValidatesAppScope(t *testing.T) {
	tests := []struct {
		name                 string
		args                 []string
		appEnv               string
		appClipsJSON         string
		nextPageJSON         string
		nextPageURL          string
		appSelector          string
		appLookupKind        string
		wantQuery            string
		wantError            string
		redirectFirst        bool
		redirectContinuation bool
		wantAppGets          int
		wantGets             int
		wantCreates          int
	}{
		{
			name:        "direct id without app preserves direct lookup",
			args:        []string{"--app-clip-id", "clip-1"},
			appEnv:      "",
			wantCreates: 1,
		},
		{
			name:         "direct id with configured app from environment",
			args:         []string{"--app-clip-id", "clip-1"},
			appEnv:       "123456789",
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}]`,
			wantGets:     1,
			wantCreates:  1,
		},
		{
			name:         "direct id with matching app",
			args:         []string{"--app", "123456789", "--app-clip-id", "clip-1"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}]`,
			wantGets:     1,
			wantCreates:  1,
		},
		{
			name:          "first-page redirect refuses before create",
			args:          []string{"--app", "123456789", "--app-clip-id", "clip-1"},
			appSelector:   "123456789",
			nextPageJSON:  "[{\"id\":\"clip-1\"}]",
			redirectFirst: true,
			wantError:     "API request failed",
			wantGets:      1,
		},
		{
			name:                 "continuation redirect refuses before create",
			args:                 []string{"--app", "123456789", "--app-clip-id", "clip-1"},
			appSelector:          "123456789",
			appClipsJSON:         "[]",
			nextPageJSON:         "[{\"id\":\"clip-1\"}]",
			nextPageURL:          "/v1/apps/123456789/appClips?cursor=next",
			redirectContinuation: true,
			wantError:            "API request failed",
			wantGets:             2,
		},
		{
			name:         "direct id with matching app on later page",
			args:         []string{"--app", "123456789", "--app-clip-id", "clip-1"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}}]`,
			nextPageJSON: `[ {"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}]`,
			nextPageURL:  "/v1/apps/123456789/appClips?cursor=next",
			wantGets:     2,
			wantCreates:  1,
		},
		{
			name:         "direct id with matching app on absolute HTTPS later page",
			args:         []string{"--app", "123456789", "--app-clip-id", "clip-1"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}}]`,
			nextPageJSON: `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}]`,
			nextPageURL:  "HTTPS://api.appstoreconnect.apple.com/v1/apps/123456789/appClips?cursor=next",
			wantGets:     2,
			wantCreates:  1,
		},
		{
			name:         "direct id with different app refuses before create",
			args:         []string{"--app", "123456789", "--app-clip-id", "clip-foreign"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}]`,
			wantError:    `app clip "clip-foreign" does not belong to selected app "123456789"`,
			wantGets:     1,
		},
		{
			name:         "bundle filter rejects nonexact returned identity",
			args:         []string{"--app", "123456789", "--bundle-id", "com.example.clip"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}}]`,
			wantQuery:    "com.example.clip",
			wantError:    `no App Clip found for exact bundle ID`,
			wantGets:     1,
		},
		{
			name:         "bundle filter selects exact among prefix candidates",
			args:         []string{"--app", "123456789", "--bundle-id", "com.example.clip"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}},{"type":"appClips","id":"clip-1","attributes":{"bundleId":"COM.EXAMPLE.CLIP"}}]`,
			wantQuery:    "com.example.clip",
			wantGets:     1,
			wantCreates:  1,
		},
		{
			name:         "bundle filter paginates to exact identity",
			args:         []string{"--app", "123456789", "--bundle-id", "com.example.clip"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}}]`,
			nextPageJSON: `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"COM.EXAMPLE.CLIP"}}]`,
			nextPageURL:  "/v1/apps/123456789/appClips?cursor=next",
			wantQuery:    "com.example.clip",
			wantGets:     2,
			wantCreates:  1,
		},
		{
			name:         "bundle filter paginates to exact identity on absolute HTTPS page",
			args:         []string{"--app", "123456789", "--bundle-id", "com.example.clip"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}}]`,
			nextPageJSON: `[ {"type":"appClips","id":"clip-1","attributes":{"bundleId":"COM.EXAMPLE.CLIP"}}]`,
			nextPageURL:  "https://api.appstoreconnect.apple.com/v1/apps/123456789/appClips?cursor=next",
			wantQuery:    "com.example.clip",
			wantGets:     2,
			wantCreates:  1,
		},
		{
			name:         "bundle continuation drift refuses before create",
			args:         []string{"--app", "123456789", "--bundle-id", "com.example.clip"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}}]`,
			nextPageJSON: `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"COM.EXAMPLE.CLIP"}}]`,
			nextPageURL:  "https://api.appstoreconnect.apple.com/v1/apps/foreign/appClips?cursor=next",
			wantQuery:    "com.example.clip",
			wantError:    "selected app's App Clips",
			wantGets:     1,
		},
		{
			name:         "direct id continuation drift refuses before create",
			args:         []string{"--app", "123456789", "--app-clip-id", "clip-1"},
			appSelector:  "123456789",
			appClipsJSON: `[{"type":"appClips","id":"clip-sibling","attributes":{"bundleId":"com.example.clip.sibling"}}]`,
			nextPageJSON: `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}]`,
			nextPageURL:  "https://api.appstoreconnect.apple.com/v1/apps/foreign/appClips?cursor=next",
			wantError:    "provider continuation URL",
			wantGets:     1,
		},
		{
			name:          "explicit app name resolves selected app",
			args:          []string{"--app", "Example App", "--app-clip-id", "clip-1"},
			appSelector:   "Example App",
			appLookupKind: "name",
			appClipsJSON:  `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"COM.EXAMPLE.CLIP"}}]`,
			wantAppGets:   2,
			wantGets:      1,
			wantCreates:   1,
		},
		{
			name:          "configured bundle resolves selected app first",
			args:          []string{"--bundle-id", "com.example.clip"},
			appEnv:        "com.example.app",
			appSelector:   "com.example.app",
			appLookupKind: "bundle",
			appClipsJSON:  `[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"COM.EXAMPLE.CLIP"}}]`,
			wantQuery:     "com.example.clip",
			wantAppGets:   1,
			wantGets:      1,
			wantCreates:   1,
		},
		{
			name:          "configured bundle name collision refuses before clip reads",
			args:          []string{"--app-clip-id", "clip-1"},
			appEnv:        "com.example.app",
			appSelector:   "com.example.app",
			appLookupKind: "collision",
			wantError:     "bundle ID",
			wantAppGets:   1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", test.appEnv)
			var gets, followups, creates int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v1/apps":
					query := r.URL.Query()
					switch test.appLookupKind {
					case "name":
						if query.Get("filter[bundleId]") != "" {
							if test.wantAppGets == 0 {
								t.Fatalf("unexpected app lookup for selector %q", test.appSelector)
							}
							test.wantAppGets--
							w.Header().Set("Content-Type", "application/json")
							_, _ = fmt.Fprint(w, `{"data":[],"links":{}}`)
							return
						}
						if got := query.Get("filter[name]"); got != test.appSelector {
							t.Fatalf("app filter[name] = %q, want %q", got, test.appSelector)
						}
					case "collision", "bundle":
						if got := query.Get("filter[bundleId]"); got != test.appSelector {
							t.Fatalf("app filter[bundleId] = %q, want %q", got, test.appSelector)
						}
					default:
						t.Fatalf("unexpected app lookup for selector %q", test.appSelector)
					}
					if test.wantAppGets == 0 {
						t.Fatalf("unexpected app lookup for selector %q", test.appSelector)
					}
					test.wantAppGets--
					w.Header().Set("Content-Type", "application/json")
					if test.appLookupKind == "collision" {
						_, _ = fmt.Fprint(w, `{"data":[{"type":"apps","id":"app-name-collision","attributes":{"bundleId":"com.example.other","name":"com.example.app"}}],"links":{}}`)
					} else if test.appLookupKind == "name" && query.Get("filter[name]") == test.appSelector {
						_, _ = fmt.Fprint(w, `{"data":[{"type":"apps","id":"123456789","attributes":{"bundleId":"com.example.app","name":"Example App"}}],"links":{}}`)
					} else {
						_, _ = fmt.Fprint(w, `{"data":[{"type":"apps","id":"123456789","attributes":{"bundleId":"com.example.app"}}],"links":{}}`)
					}
				case r.Method == http.MethodGet && r.URL.Path == "/fixture/app-clip-redirect-target":
					followups++
					w.WriteHeader(http.StatusNotFound)
				case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/123456789/appClips":
					gets++
					cursor := r.URL.Query().Get("cursor")
					if test.redirectFirst && cursor == "" {
						w.Header().Set("Location", "/fixture/app-clip-redirect-target")
						w.WriteHeader(http.StatusFound)
						return
					}
					if test.redirectContinuation && cursor == "next" {
						w.Header().Set("Location", "/fixture/app-clip-redirect-target")
						w.WriteHeader(http.StatusFound)
						return
					}
					if r.URL.Query().Get("cursor") == "" {
						if got := r.URL.Query().Get("filter[bundleId]"); got != test.wantQuery {
							t.Fatalf("filter[bundleId] = %q, want %q", got, test.wantQuery)
						}
					} else {
						if got := r.URL.Query().Get("cursor"); got != "next" {
							t.Fatalf("page-two cursor = %q, want next", got)
						}
						if got := r.URL.Query().Get("filter[bundleId]"); got != "" {
							t.Fatalf("page-two filter[bundleId] = %q, want empty", got)
						}
					}
					data := test.appClipsJSON
					links := `{}`
					if r.URL.Query().Get("cursor") != "" {
						data = test.nextPageJSON
					} else if test.nextPageJSON != "" {
						nextPageURL := test.nextPageURL
						if nextPageURL == "" {
							nextPageURL = "https://api.appstoreconnect.apple.com/v1/apps/123456789/appClips?cursor=next"
						}
						links = fmt.Sprintf(`{"next":%q}`, nextPageURL)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintf(w, `{"data":%s,"links":%s}`, data, links)
				case r.Method == http.MethodPost && r.URL.Path == "/v1/appClipAdvancedExperiences":
					var payload struct {
						Data struct {
							Relationships struct {
								AppClip struct {
									Data struct {
										ID string `json:"id"`
									} `json:"data"`
								} `json:"appClip"`
							} `json:"relationships"`
						} `json:"data"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatalf("decode create payload: %v", err)
					}
					if got := payload.Data.Relationships.AppClip.Data.ID; got != "clip-1" {
						t.Fatalf("created App Clip ID = %q, want clip-1", got)
					}
					creates++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"data":{"type":"appClipAdvancedExperiences","id":"experience-1","attributes":{}},"links":{}}`))
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				}
			}))
			t.Cleanup(server.Close)

			args := append([]string{
				"app-clips", "advanced-experiences", "create",
				"--link", "https://example.com/clip",
				"--default-language", "EN",
				"--is-powered-by",
				"--header-image-id", "image-1",
				"--language", "EN",
				"--title", "Order ahead",
			}, test.args...)
			stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server), args...)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want substring %q", err, test.wantError)
				}
				if stdout != "" || stderr != "" {
					t.Fatalf("refusal wrote stdout=%q stderr=%q", stdout, stderr)
				}
			} else if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if gets != test.wantGets {
				t.Fatalf("app clip membership requests = %d, want %d", gets, test.wantGets)
			}
			if test.wantAppGets != 0 {
				t.Fatalf("app lookup requests remaining = %d", test.wantAppGets)
			}
			if creates != test.wantCreates {
				t.Fatalf("advanced experience creates = %d, want %d", creates, test.wantCreates)
			}
			if followups != 0 {
				t.Fatalf("redirect follow-up requests = %d, want 0", followups)
			}
		})
	}
}
