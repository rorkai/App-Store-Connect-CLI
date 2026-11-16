package signing

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// lifecycleAPI is an in-memory App Store Connect fake for profile lifecycle
// tests. It records every request so tests can assert exact mutation order.
type lifecycleAPI struct {
	t               *testing.T
	events          []string
	bundleProfiles  string
	profileDevices  map[string]string
	profileCerts    string
	certificates    string
	devices         string
	createdProfile  string
	createBodies    []asc.ProfileCreateRequest
	deleteStatus    int
	createStatus    int
	unexpectedCalls int
}

func newLifecycleAPI(t *testing.T) *lifecycleAPI {
	return &lifecycleAPI{
		t:              t,
		profileDevices: map[string]string{},
		profileCerts:   `{"data":[{"type":"certificates","id":"cert-1","attributes":{"serialNumber":"serial-1","certificateType":"IOS_DEVELOPMENT","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`,
		certificates:   `{"data":[{"type":"certificates","id":"cert-1","attributes":{"serialNumber":"serial-1","certificateType":"IOS_DEVELOPMENT","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`,
		createdProfile: `{"data":{"type":"profiles","id":"profile-new","attributes":{"name":"Created","profileType":"IOS_APP_DEVELOPMENT","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"}}}`,
		deleteStatus:   http.StatusNoContent,
		createStatus:   http.StatusCreated,
	}
}

func (api *lifecycleAPI) client() *asc.Client {
	return newSigningFetchTestClient(api.t, func(req *http.Request) *http.Response {
		api.events = append(api.events, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/bundleIds/") && strings.HasSuffix(req.URL.Path, "/profiles"):
			return signingFetchJSONResponse(http.StatusOK, api.bundleProfiles)
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/profiles/") && strings.HasSuffix(req.URL.Path, "/certificates"):
			return signingFetchJSONResponse(http.StatusOK, api.profileCerts)
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/profiles/") && strings.HasSuffix(req.URL.Path, "/devices"):
			id := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/profiles/"), "/devices")
			body, ok := api.profileDevices[id]
			if !ok {
				body = `{"data":[]}`
			}
			return signingFetchJSONResponse(http.StatusOK, body)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, api.certificates)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/devices":
			if got := req.URL.Query().Get("filter[status]"); got != "ENABLED" {
				api.t.Errorf("device list filter[status] = %q, want ENABLED", got)
			}
			return signingFetchJSONResponse(http.StatusOK, api.devices)
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/profiles/"):
			if api.deleteStatus >= 300 {
				return signingFetchJSONResponse(api.deleteStatus, `{"errors":[{"status":"500","code":"UNEXPECTED","title":"delete failed"}]}`)
			}
			return &http.Response{StatusCode: api.deleteStatus, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			var payload asc.ProfileCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				api.t.Fatalf("decode profile create: %v", err)
			}
			api.createBodies = append(api.createBodies, payload)
			if api.createStatus >= 300 {
				return signingFetchJSONResponse(api.createStatus, `{"errors":[{"status":"409","code":"CONFLICT","title":"create failed"}]}`)
			}
			return signingFetchJSONResponse(api.createStatus, api.createdProfile)
		default:
			api.unexpectedCalls++
			api.t.Errorf("unexpected request %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
}

func (api *lifecycleAPI) mutations() []string {
	mutations := []string{}
	for _, event := range api.events {
		if !strings.HasPrefix(event, http.MethodGet+" ") {
			mutations = append(mutations, event)
		}
	}
	return mutations
}

func lifecycleDevicesJSON(devices ...[4]string) string {
	items := make([]string, 0, len(devices))
	for _, device := range devices {
		items = append(items, fmt.Sprintf(`{"type":"devices","id":%q,"attributes":{"udid":%q,"deviceClass":%q,"status":%q}}`, device[0], device[1], device[2], device[3]))
	}
	return `{"data":[` + strings.Join(items, ",") + `]}`
}

func createDeviceIDs(t *testing.T, payload asc.ProfileCreateRequest) []string {
	t.Helper()
	ids := []string{}
	if payload.Data.Relationships != nil && payload.Data.Relationships.Devices != nil {
		for _, device := range payload.Data.Relationships.Devices.Data {
			ids = append(ids, device.ID)
		}
	}
	return ids
}

const activeDevelopmentProfile = `{"data":[{"type":"profiles","id":"profile-old","attributes":{"name":"Team Dev","profileType":"IOS_APP_DEVELOPMENT","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"}}]}`

func TestResolveSigningAssetsForceForNewDevicesIsNoOpWhenDeviceSetUnchanged(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.profileDevices["profile-old"] = lifecycleDevicesJSON(
		[4]string{"device-2", "UDID2", "IPAD", "ENABLED"},
		[4]string{"device-1", "UDID1", "IPHONE", "ENABLED"},
	)
	api.devices = lifecycleDevicesJSON(
		[4]string{"device-1", "UDID1", "IPHONE", "ENABLED"},
		[4]string{"device-2", "UDID2", "IPAD", "ENABLED"},
		[4]string{"device-tv", "UDIDTV", "APPLE_TV", "ENABLED"},
		[4]string{"device-mac", "00008103-001A2B3C4D5E6F70", "MAC", "ENABLED"},
	)
	progress := &signingAssetsProgress{}
	beforeCreateCalls := 0

	profile, _, created, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		ForceForNewDevices: true,
		Progress:           progress,
		BeforeCreate: func(profileCreatePlan) error {
			beforeCreateCalls++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if created || profile.Data.ID != "profile-old" {
		t.Fatalf("resolved profile = %s created=%t, want reused profile-old", profile.Data.ID, created)
	}
	if got := api.mutations(); len(got) != 0 {
		t.Fatalf("mutations = %v, want none when the device set is unchanged", got)
	}
	if beforeCreateCalls != 0 {
		t.Fatalf("BeforeCreate calls = %d, want 0", beforeCreateCalls)
	}
	want := &asc.SigningSyncProfileLifecycle{Action: asc.SigningSyncLifecycleUnchanged, ProfileID: "profile-old"}
	if !reflect.DeepEqual(progress.Lifecycle, want) {
		t.Fatalf("lifecycle = %+v, want %+v", progress.Lifecycle, want)
	}
}

func TestResolveSigningAssetsForceForNewDevicesRecreatesProfileWithSameName(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.profileDevices["profile-old"] = lifecycleDevicesJSON(
		[4]string{"device-1", "UDID1", "IPHONE", "ENABLED"},
		[4]string{"device-3", "UDID3", "IPHONE", "DISABLED"},
	)
	api.devices = lifecycleDevicesJSON(
		[4]string{"device-2", "UDID2", "IPAD", "ENABLED"},
		[4]string{"device-1", "UDID1", "IPHONE", "ENABLED"},
		[4]string{"device-watch", "UDIDW", "APPLE_WATCH", "ENABLED"},
		[4]string{"device-intel-mac", "12345678-1234-1234-1234-123456789ABC", "MAC", "ENABLED"},
	)
	progress := &signingAssetsProgress{}
	var events []string
	options := signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		ForceForNewDevices: true,
		Progress:           progress,
		BeforeCreate: func(plan profileCreatePlan) error {
			events = append(events, "preflight "+plan.ProfileName+" "+strings.Join(extractIDs(plan.Certificates), ","))
			return nil
		},
	}
	client := api.client()

	profile, certificates, created, err := resolveSigningAssets(context.Background(), client, options)
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if !created || profile.Data.ID != "profile-new" {
		t.Fatalf("resolved profile = %s created=%t, want created profile-new", profile.Data.ID, created)
	}
	if got := extractIDs(certificates.Data); !reflect.DeepEqual(got, []string{"cert-1"}) {
		t.Fatalf("certificates = %v, want the replaced profile's cert-1", got)
	}
	if want := []string{"preflight Team Dev cert-1"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("preflight events = %v, want %v", events, want)
	}
	if want := []string{"DELETE /v1/profiles/profile-old", "POST /v1/profiles"}; !reflect.DeepEqual(api.mutations(), want) {
		t.Fatalf("mutations = %v, want %v", api.mutations(), want)
	}
	if len(api.createBodies) != 1 {
		t.Fatalf("create bodies = %d, want 1", len(api.createBodies))
	}
	body := api.createBodies[0]
	if body.Data.Attributes.Name != "Team Dev" || body.Data.Attributes.ProfileType != "IOS_APP_DEVELOPMENT" {
		t.Fatalf("create attributes = %+v, want the replaced name and type", body.Data.Attributes)
	}
	if got := createDeviceIDs(t, body); !reflect.DeepEqual(got, []string{"device-1", "device-2", "device-watch"}) {
		t.Fatalf("create devices = %v, want enabled iOS-family devices without Macs or TVs", got)
	}
	want := &asc.SigningSyncProfileLifecycle{
		Action:               asc.SigningSyncLifecycleDevicesRefreshed,
		ProfileID:            "profile-new",
		ReplacedProfileID:    "profile-old",
		ReplacedProfileName:  "Team Dev",
		ReplacedProfileState: asc.SigningSyncReplacedProfileDeleted,
		DevicesAdded:         []string{"device-2", "device-watch"},
		DevicesRemoved:       []string{"device-3"},
	}
	if !reflect.DeepEqual(progress.Lifecycle, want) {
		t.Fatalf("lifecycle = %+v, want %+v", progress.Lifecycle, want)
	}
}

func TestResolveSigningAssetsForceForNewDevicesUsesExplicitDevices(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.profileDevices["profile-old"] = lifecycleDevicesJSON([4]string{"device-1", "UDID1", "IPHONE", "ENABLED"})

	progress := &signingAssetsProgress{}
	_, _, created, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		DeviceIDs:          []string{"device-1"},
		ForceForNewDevices: true,
		Progress:           progress,
	})
	if err != nil || created {
		t.Fatalf("resolveSigningAssets() created=%t err=%v, want unchanged reuse", created, err)
	}
	for _, event := range api.events {
		if event == "GET /v1/devices" {
			t.Fatal("explicit --device must not list the enabled device set")
		}
	}
	if progress.Lifecycle == nil || progress.Lifecycle.Action != asc.SigningSyncLifecycleUnchanged {
		t.Fatalf("lifecycle = %+v, want unchanged", progress.Lifecycle)
	}
}

func TestResolveSigningAssetsIncludeMacAddsOnlyAppleSiliconMacs(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.profileDevices["profile-old"] = lifecycleDevicesJSON([4]string{"device-1", "UDID1", "IPHONE", "ENABLED"})
	api.devices = lifecycleDevicesJSON(
		[4]string{"device-1", "UDID1", "IPHONE", "ENABLED"},
		[4]string{"device-silicon", "00008103-001A2B3C4D5E6F70", "MAC", "ENABLED"},
		[4]string{"device-intel", "12345678-1234-1234-1234-123456789ABC", "MAC", "ENABLED"},
	)
	progress := &signingAssetsProgress{}
	_, _, created, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		ForceForNewDevices: true,
		IncludeMacDevices:  true,
		Progress:           progress,
	})
	if err != nil || !created {
		t.Fatalf("resolveSigningAssets() created=%t err=%v, want replacement", created, err)
	}
	if got := createDeviceIDs(t, api.createBodies[0]); !reflect.DeepEqual(got, []string{"device-1", "device-silicon"}) {
		t.Fatalf("create devices = %v, want the iPhone and the Apple silicon Mac only", got)
	}
	if !reflect.DeepEqual(progress.Lifecycle.DevicesAdded, []string{"device-silicon"}) {
		t.Fatalf("devicesAdded = %v", progress.Lifecycle.DevicesAdded)
	}
}

func TestResolveSigningAssetsForceForNewDevicesFailsBeforeMutationWithoutEnabledDevices(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.devices = lifecycleDevicesJSON([4]string{"device-tv", "UDIDTV", "APPLE_TV", "ENABLED"})
	_, _, _, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		ForceForNewDevices: true,
		Progress:           &signingAssetsProgress{},
	})
	if err == nil || !strings.Contains(err.Error(), "no enabled devices") {
		t.Fatalf("error = %v, want no enabled devices", err)
	}
	if got := api.mutations(); len(got) != 0 {
		t.Fatalf("mutations = %v, want none", got)
	}
}

func TestResolveSigningAssetsForceForNewDevicesDeleteFailureStopsBeforeCreate(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.profileDevices["profile-old"] = lifecycleDevicesJSON([4]string{"device-1", "UDID1", "IPHONE", "ENABLED"})
	api.devices = lifecycleDevicesJSON([4]string{"device-2", "UDID2", "IPHONE", "ENABLED"})
	api.deleteStatus = http.StatusInternalServerError
	progress := &signingAssetsProgress{}
	_, _, _, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		ForceForNewDevices: true,
		Progress:           progress,
	})
	if err == nil || !strings.Contains(err.Error(), "delete replaced profile profile-old") {
		t.Fatalf("error = %v, want replaced-profile deletion failure", err)
	}
	if want := []string{"DELETE /v1/profiles/profile-old"}; !reflect.DeepEqual(api.mutations(), want) {
		t.Fatalf("mutations = %v, want only the failed delete", api.mutations())
	}
	if progress.ProfileCreateAttempted {
		t.Fatal("profile create must not be attempted after a failed replacement delete")
	}
	if progress.Lifecycle == nil || progress.Lifecycle.ReplacedProfileState != asc.SigningSyncReplacedProfileUnknown || !progress.ReplacementAttempted {
		t.Fatalf("lifecycle = %+v attempted=%t, want unknown replaced state", progress.Lifecycle, progress.ReplacementAttempted)
	}
}

func withSigningFetchNow(t *testing.T, now time.Time) {
	t.Helper()
	previous := signingFetchNowFn
	signingFetchNowFn = func() time.Time { return now }
	t.Cleanup(func() { signingFetchNowFn = previous })
}

func TestResolveSigningAssetsRenewExpiredRecreatesLatestExpiredProfile(t *testing.T) {
	withSigningFetchNow(t, time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	api := newLifecycleAPI(t)
	api.certificates = `{"data":[{"type":"certificates","id":"cert-dist","attributes":{"serialNumber":"serial-dist","certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`
	api.bundleProfiles = `{"data":[
		{"type":"profiles","id":"profile-older","attributes":{"name":"Store Old","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2025-01-01T00:00:00Z"}},
		{"type":"profiles","id":"profile-expired","attributes":{"name":"Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2026-09-01T00:00:00Z"}},
		{"type":"profiles","id":"profile-dev","attributes":{"name":"Dev","profileType":"IOS_APP_DEVELOPMENT","profileState":"ACTIVE","expirationDate":"2026-09-02T00:00:00Z"}}
	]}`
	api.createdProfile = `{"data":{"type":"profiles","id":"profile-renewed","attributes":{"name":"Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2027-09-27T00:00:00Z"}}}`
	progress := &signingAssetsProgress{}

	profile, certificates, created, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_STORE",
		RenewExpired:       true,
		Progress:           progress,
	})
	if err != nil {
		t.Fatalf("resolveSigningAssets() error: %v", err)
	}
	if !created || profile.Data.ID != "profile-renewed" {
		t.Fatalf("profile = %s created=%t, want renewed profile", profile.Data.ID, created)
	}
	if got := extractIDs(certificates.Data); !reflect.DeepEqual(got, []string{"cert-dist"}) {
		t.Fatalf("certificates = %v, want the existing valid certificate", got)
	}
	if want := []string{"DELETE /v1/profiles/profile-expired", "POST /v1/profiles"}; !reflect.DeepEqual(api.mutations(), want) {
		t.Fatalf("mutations = %v, want %v", api.mutations(), want)
	}
	body := api.createBodies[0]
	if body.Data.Attributes.Name != "Store" {
		t.Fatalf("renewed profile name = %q, want Store", body.Data.Attributes.Name)
	}
	if got := createDeviceIDs(t, body); len(got) != 0 {
		t.Fatalf("App Store renewal devices = %v, want none", got)
	}
	want := &asc.SigningSyncProfileLifecycle{
		Action:                 asc.SigningSyncLifecycleRenewed,
		ProfileID:              "profile-renewed",
		ReplacedProfileID:      "profile-expired",
		ReplacedProfileName:    "Store",
		ReplacedExpirationDate: "2026-09-01T00:00:00Z",
		ReplacedProfileState:   asc.SigningSyncReplacedProfileDeleted,
	}
	if !reflect.DeepEqual(progress.Lifecycle, want) {
		t.Fatalf("lifecycle = %+v, want %+v", progress.Lifecycle, want)
	}
}

func TestResolveSigningAssetsRenewExpiredIsNoOpWhenActiveProfileExists(t *testing.T) {
	withSigningFetchNow(t, time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	api := newLifecycleAPI(t)
	api.profileCerts = `{"data":[{"type":"certificates","id":"cert-dist","attributes":{"certificateType":"IOS_DISTRIBUTION","expirationDate":"2100-01-01T00:00:00Z"}}]}`
	api.bundleProfiles = `{"data":[
		{"type":"profiles","id":"profile-expired","attributes":{"name":"Store Old","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2026-09-01T00:00:00Z"}},
		{"type":"profiles","id":"profile-current","attributes":{"name":"Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2027-01-01T00:00:00Z"}}
	]}`
	progress := &signingAssetsProgress{}
	profile, _, created, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_STORE",
		RenewExpired:       true,
		Progress:           progress,
	})
	if err != nil || created || profile.Data.ID != "profile-current" {
		t.Fatalf("resolveSigningAssets() profile=%v created=%t err=%v, want the current profile", profile, created, err)
	}
	if got := api.mutations(); len(got) != 0 {
		t.Fatalf("mutations = %v, want none", got)
	}
	if want := (&asc.SigningSyncProfileLifecycle{Action: asc.SigningSyncLifecycleUnchanged, ProfileID: "profile-current"}); !reflect.DeepEqual(progress.Lifecycle, want) {
		t.Fatalf("lifecycle = %+v, want %+v", progress.Lifecycle, want)
	}
}

func TestResolveSigningAssetsRenewExpiredWithoutProfilesKeepsMissingProfileError(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = `{"data":[]}`
	_, _, _, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_STORE",
		RenewExpired:       true,
		Progress:           &signingAssetsProgress{},
	})
	if err == nil || !strings.Contains(err.Error(), "no active IOS_APP_STORE profile found") {
		t.Fatalf("error = %v, want the missing-profile error", err)
	}
	if got := api.mutations(); len(got) != 0 {
		t.Fatalf("mutations = %v, want none", got)
	}
}

func TestResolveSigningAssetsRenewExpiredDevelopmentReusesEnabledDevices(t *testing.T) {
	withSigningFetchNow(t, time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	api := newLifecycleAPI(t)
	api.bundleProfiles = `{"data":[{"type":"profiles","id":"profile-expired","attributes":{"name":"Dev","profileType":"IOS_APP_DEVELOPMENT","profileState":"ACTIVE","expirationDate":"2026-09-01T00:00:00Z"}}]}`
	api.profileDevices["profile-expired"] = lifecycleDevicesJSON(
		[4]string{"device-2", "UDID2", "IPHONE", "ENABLED"},
		[4]string{"device-1", "UDID1", "IPHONE", "ENABLED"},
		[4]string{"device-off", "UDIDOFF", "IPHONE", "DISABLED"},
	)
	progress := &signingAssetsProgress{}
	_, _, created, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		RenewExpired:       true,
		Progress:           progress,
	})
	if err != nil || !created {
		t.Fatalf("resolveSigningAssets() created=%t err=%v, want renewal", created, err)
	}
	if got := createDeviceIDs(t, api.createBodies[0]); !reflect.DeepEqual(got, []string{"device-1", "device-2"}) {
		t.Fatalf("renewed devices = %v, want the expired profile's enabled devices", got)
	}
	if !reflect.DeepEqual(progress.Lifecycle.DevicesRemoved, []string{"device-off"}) || len(progress.Lifecycle.DevicesAdded) != 0 {
		t.Fatalf("lifecycle devices = %+v", progress.Lifecycle)
	}
}

func TestSigningSyncPushLifecycleFlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "force for new devices with App Store profile",
			args: []string{"--profile-type", "IOS_APP_STORE", "--force-for-new-devices"},
			want: "--force-for-new-devices requires a development or ad hoc profile type; IOS_APP_STORE profiles have no device list",
		},
		{
			name: "include mac without force",
			args: []string{"--profile-type", "IOS_APP_DEVELOPMENT", "--include-mac-in-profiles"},
			want: "--include-mac-in-profiles requires --force-for-new-devices",
		},
		{
			name: "include mac with mac profile",
			args: []string{"--profile-type", "MAC_APP_DEVELOPMENT", "--force-for-new-devices", "--include-mac-in-profiles"},
			want: "--include-mac-in-profiles supports only IOS_APP_DEVELOPMENT and IOS_APP_ADHOC profiles",
		},
		{
			name: "include mac with explicit devices",
			args: []string{"--profile-type", "IOS_APP_ADHOC", "--force-for-new-devices", "--include-mac-in-profiles", "--device", "DEVICE1"},
			want: "--include-mac-in-profiles cannot be combined with --device; list Mac device IDs in --device instead",
		},
		{
			name: "renew with remote storage",
			args: []string{"--profile-type", "IOS_APP_STORE", "--renew-expired", "--storage", "aws-secrets-manager", "--region", "us-east-1", "--prefix", "asc"},
			want: "--renew-expired and --force-for-new-devices require --storage git so replaced profiles can be removed from the store",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(signingSyncPasswordEnvVar, "repository-password")
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client must not be created")
			}))
			cmd := syncPushCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			args := []string{"--bundle-id", "com.example.app"}
			if !strings.Contains(strings.Join(tt.args, " "), "--storage") {
				args = append(args, "--repo", "git@github.com:team/certs.git")
			}
			args = append(args, tt.args...)
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := cmd.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = cmd.Run(context.Background())
			})
			if runErr == nil || !errors.Is(runErr, flag.ErrHelp) || runErr.Error() != tt.want {
				t.Fatalf("error = %v, want usage error %q", runErr, tt.want)
			}
			if clientCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientCalls)
			}
			if stdout != "" || stderr != "Error: "+tt.want+"\n" {
				t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestSigningSyncPushAcceptsDeviceWithForceForNewDevices(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client reached")
	}))
	cmd := syncPushCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	var runErr error
	_, _ = captureOutput(t, func() {
		if err := cmd.Parse([]string{
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_DEVELOPMENT",
			"--repo", "git@github.com:team/certs.git",
			"--force-for-new-devices",
			"--device", "DEVICE1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})
	if runErr == nil || errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), "client reached") {
		t.Fatalf("error = %v, want validation to pass through to the client", runErr)
	}
	if clientCalls != 1 {
		t.Fatalf("client factory calls = %d, want 1", clientCalls)
	}
}

func TestResolveSigningAssetsRenewExpiredKeepsCertificateMismatchErrorWhenActiveProfileExists(t *testing.T) {
	withSigningFetchNow(t, time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	api := newLifecycleAPI(t)
	api.profileCerts = `{"data":[{"type":"certificates","id":"cert-other","attributes":{"certificateType":"IOS_DISTRIBUTION","expirationDate":"2100-01-01T00:00:00Z"}}]}`
	api.bundleProfiles = `{"data":[
		{"type":"profiles","id":"profile-expired","attributes":{"name":"Store Old","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2026-09-01T00:00:00Z"}},
		{"type":"profiles","id":"profile-current","attributes":{"name":"Store","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2027-01-01T00:00:00Z"}}
	]}`
	_, _, _, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_STORE",
		RenewExpired:       true,
		Progress:           &signingAssetsProgress{},
		CertificateFilter: func(certificate asc.Resource[asc.CertificateAttributes]) bool {
			return certificate.ID == "cert-local"
		},
	})
	if err == nil || !errors.Is(err, errNoMatchingProfileCertificates) {
		t.Fatalf("error = %v, want the active profile's certificate mismatch", err)
	}
	if got := api.mutations(); len(got) != 0 {
		t.Fatalf("mutations = %v, want none while an active profile exists", got)
	}
}

func TestResolveSigningAssetsReplacementCreateFailureNamesRecovery(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.profileDevices["profile-old"] = lifecycleDevicesJSON([4]string{"device-1", "UDID1", "IPHONE", "ENABLED"})
	api.devices = lifecycleDevicesJSON([4]string{"device-2", "UDID2", "IPHONE", "ENABLED"})
	api.createStatus = http.StatusConflict
	progress := &signingAssetsProgress{}
	_, _, _, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		ForceForNewDevices: true,
		Progress:           progress,
	})
	if err == nil || !strings.Contains(err.Error(), `profile profile-old ("Team Dev") was deleted but its replacement was not created`) ||
		!strings.Contains(err.Error(), "rerun with --create-missing") {
		t.Fatalf("error = %v, want the deleted profile and --create-missing recovery", err)
	}
	if progress.Lifecycle.ReplacedProfileState != asc.SigningSyncReplacedProfileDeleted || progress.Lifecycle.ProfileID != "" || !progress.ProfileCreateAttempted {
		t.Fatalf("lifecycle = %+v attempted=%t", progress.Lifecycle, progress.ProfileCreateAttempted)
	}
}

func TestResolveSigningAssetsReplacementUsesSeparateRequestContexts(t *testing.T) {
	api := newLifecycleAPI(t)
	api.bundleProfiles = activeDevelopmentProfile
	api.profileDevices["profile-old"] = lifecycleDevicesJSON([4]string{"device-1", "UDID1", "IPHONE", "ENABLED"})
	contexts := 0
	_, _, created, err := resolveSigningAssets(context.Background(), api.client(), signingAssetsOptions{
		Now:                signingFetchNowFn,
		BundleIDResourceID: "bundle-1",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_DEVELOPMENT",
		DeviceIDs:          []string{"device-2"},
		ForceForNewDevices: true,
		Progress:           &signingAssetsProgress{},
		CreateContext: func() (context.Context, context.CancelFunc) {
			contexts++
			return context.WithCancel(context.Background())
		},
	})
	if err != nil || !created {
		t.Fatalf("resolveSigningAssets() created=%t error=%v", created, err)
	}
	if want := []string{"DELETE /v1/profiles/profile-old", "POST /v1/profiles"}; !reflect.DeepEqual(api.mutations(), want) {
		t.Fatalf("mutations = %v, want %v", api.mutations(), want)
	}
	if contexts != 2 {
		t.Fatalf("request contexts = %d, want one for the deletion and one for the creation", contexts)
	}
}
