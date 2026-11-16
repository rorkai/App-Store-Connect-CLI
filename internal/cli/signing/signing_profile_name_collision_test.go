package signing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// accountProfiles models the live rule captured in docs/API_NOTES.md: a profile
// name is unique across the account, and an INVALID profile keeps holding it.
type accountProfiles struct {
	t        *testing.T
	profiles []accountProfile
}

type accountProfile struct {
	id, bundle, name, state string
}

func (a *accountProfiles) roundTrip(req *http.Request) *http.Response {
	switch {
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/bundleIds/"):
		bundle := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/bundleIds/"), "/profiles")
		data := []string{}
		for _, profile := range a.profiles {
			if profile.bundle == bundle {
				data = append(data, fmt.Sprintf(`{"type":"profiles","id":%q,"attributes":{"name":%q,"profileType":"IOS_APP_STORE","profileState":%q}}`, profile.id, profile.name, profile.state))
			}
		}
		return signingFetchJSONResponse(http.StatusOK, `{"data":[`+strings.Join(data, ",")+`]}`)
	case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
		return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`)
	case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
		var payload struct {
			Data struct {
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
				Relationships struct {
					BundleID struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"bundleId"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			a.t.Fatalf("decode profile create: %v", err)
		}
		name := payload.Data.Attributes.Name
		for _, profile := range a.profiles {
			if profile.name == name {
				return signingFetchJSONResponse(http.StatusConflict, fmt.Sprintf(`{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"There is a problem with the request entity","detail":"Multiple profiles found with the name '%s'."}]}`, name))
			}
		}
		id := fmt.Sprintf("profile-%d", len(a.profiles)+1)
		a.profiles = append(a.profiles, accountProfile{id: id, bundle: payload.Data.Relationships.BundleID.Data.ID, name: name, state: string(asc.ProfileStateActive)})
		return signingFetchJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"profiles","id":%q,"attributes":{"name":%q,"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}}`, id, name))
	default:
		a.t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil
	}
}

func TestResolveSigningAssetsAvoidsProfileNamesHeldInTheAccount(t *testing.T) {
	longBundle := strings.Repeat("com.company.product.suite.feature.extension.", 4) + strings.Repeat("l", 24)

	resolve := func(t *testing.T, client *asc.Client, resourceID, identifier string) {
		t.Helper()
		profile, _, created, err := resolveSigningAssets(context.Background(), client, signingAssetsOptions{
			Now:                time.Now,
			BundleIDResourceID: resourceID,
			BundleIdentifier:   identifier,
			ProfileType:        "IOS_APP_STORE",
			CreateMissing:      true,
		})
		if err != nil {
			t.Fatalf("resolveSigningAssets(%s) error: %v", identifier, err)
		}
		if !created {
			t.Fatalf("resolveSigningAssets(%s) reused profile %s, want a new one", identifier, profile.Data.ID)
		}
		name := profile.Data.Attributes.Name
		if got := utf8.RuneCountInString(name); got > maxProfileNameLength {
			t.Fatalf("profile name has %d characters, want <= %d: %q", got, maxProfileNameLength, name)
		}
	}

	bundles := []struct{ label, identifier string }{
		{label: "short bundle ID", identifier: "com.example.app"},
		{label: "long bundle ID", identifier: longBundle},
	}
	for _, bundle := range bundles {
		identifier := bundle.identifier

		t.Run(bundle.label+"/another bundle ID the same day", func(t *testing.T) {
			account := &accountProfiles{t: t}
			client := newSigningFetchTestClient(t, account.roundTrip)

			resolve(t, client, "bundle-a", identifier)
			resolve(t, client, "bundle-b", identifier+".widget")
		})

		t.Run(bundle.label+"/rotation while an invalid profile holds the name", func(t *testing.T) {
			account := &accountProfiles{t: t}
			client := newSigningFetchTestClient(t, account.roundTrip)

			for rotation := 0; rotation < 3; rotation++ {
				resolve(t, client, "bundle-a", identifier)
				account.profiles[rotation].state = string(asc.ProfileStateInvalid)
			}
		})

		t.Run(bundle.label+"/name held on another bundle ID", func(t *testing.T) {
			account := &accountProfiles{t: t, profiles: []accountProfile{{
				id:     "profile-elsewhere",
				bundle: "bundle-elsewhere",
				name:   profileCreateNameForTarget("IOS_APP_STORE", identifier, time.Now()),
				state:  string(asc.ProfileStateActive),
			}}}
			client := newSigningFetchTestClient(t, account.roundTrip)

			resolve(t, client, "bundle-a", identifier)
		})
	}
}
