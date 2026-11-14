package cmdtest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSigningFetchAllDevicesCreatesProfileFromEnabledDevices(t *testing.T) {
	const (
		phone      = `{"type":"devices","id":"device-phone","attributes":{"udid":"PHONE","deviceClass":"IPHONE","status":"ENABLED"}}`
		pad        = `{"type":"devices","id":"device-pad","attributes":{"udid":"PAD","deviceClass":"IPAD","status":"ENABLED"}}`
		tv         = `{"type":"devices","id":"device-tv","attributes":{"udid":"TV","deviceClass":"APPLE_TV","status":"ENABLED"}}`
		mac        = `{"type":"devices","id":"device-mac","attributes":{"udid":"00008103-001A2B3C4D5E6F70","deviceClass":"MAC","status":"ENABLED"}}`
		disabled   = `{"type":"devices","id":"device-old","attributes":{"udid":"OLD","deviceClass":"IPHONE","status":"DISABLED"}}`
		activeProf = `{"type":"profiles","id":"profile-existing","attributes":{"name":"Existing","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z","profileContent":"cHJvZmlsZQ=="}}`
	)
	for _, test := range []struct {
		name            string
		profiles        string
		devices         string
		wantCode        int
		wantDeviceLists int
		wantCreated     []string
		wantStderr      string
	}{
		{
			name:            "no profile creates one with every enabled device the type accepts",
			devices:         strings.Join([]string{phone, tv, mac, disabled, pad}, ","),
			wantCode:        rootcmd.ExitSuccess,
			wantDeviceLists: 1,
			wantCreated:     []string{"device-pad", "device-phone"},
		},
		{
			name:     "existing valid profile is fetched without listing devices",
			profiles: activeProf,
			devices:  phone,
			wantCode: rootcmd.ExitSuccess,
		},
		{
			name:            "no enabled device fails before creating a profile",
			devices:         strings.Join([]string{tv, disabled}, ","),
			wantCode:        rootcmd.ExitError,
			wantDeviceLists: 1,
			wantStderr:      "no enabled devices match IOS_APP_ADHOC profiles",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			certificate := fmt.Sprintf(`{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"CERT1","certificateContent":%q,"activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}`, base64.StdEncoding.EncodeToString([]byte("certificate")))
			deviceLists := 0
			var created [][]string
			setBundleIDPlatformTestServer(t, func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
					writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-main","attributes":{"identifier":"com.example.app"}}]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
					writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+test.profiles+`]}`)
				case req.Method == http.MethodGet && (req.URL.Path == "/v1/certificates" || req.URL.Path == "/v1/profiles/profile-existing/certificates"):
					writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+certificate+`]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/devices":
					deviceLists++
					if got := req.URL.Query().Get("filter[status]"); got != "ENABLED" {
						t.Errorf("devices filter[status] = %q, want ENABLED", got)
					}
					writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+test.devices+`],"links":{}}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
					var body struct {
						Data struct {
							Relationships struct {
								Devices struct {
									Data []struct {
										ID string `json:"id"`
									} `json:"data"`
								} `json:"devices"`
							} `json:"relationships"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Errorf("decode profile create body: %v", err)
					}
					ids := []string{}
					for _, device := range body.Data.Relationships.Devices.Data {
						ids = append(ids, device.ID)
					}
					created = append(created, ids)
					writeSigningFetchOutputJSON(t, w, http.StatusCreated, `{"data":{"type":"profiles","id":"profile-created","attributes":{"name":"Created","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","profileContent":"cHJvZmlsZQ=="}}}`)
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			})

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run([]string{
					"signing", "fetch",
					"--bundle-id", "com.example.app",
					"--profile-type", "IOS_APP_ADHOC",
					"--create-missing", "--all-devices",
					"--output", t.TempDir(),
					"--format", "json",
				}, "test")
			})
			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d (stdout %q, stderr %q)", code, test.wantCode, stdout, stderr)
			}
			if deviceLists != test.wantDeviceLists {
				t.Fatalf("device list requests = %d, want %d", deviceLists, test.wantDeviceLists)
			}
			if test.wantCreated == nil {
				if len(created) != 0 {
					t.Fatalf("profile create requests = %v, want none", created)
				}
			} else if len(created) != 1 || !reflect.DeepEqual(created[0], test.wantCreated) {
				t.Fatalf("profile create devices = %v, want one request with %v", created, test.wantCreated)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}
