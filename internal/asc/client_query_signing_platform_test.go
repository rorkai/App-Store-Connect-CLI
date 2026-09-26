package asc

import (
	"net/url"
	"testing"
)

func TestProfilesAndCertificatesListQueriesOmitPlatformFilter(t *testing.T) {
	profiles := buildProfilesQuery(&profilesQuery{
		profileTypes: []string{"IOS_APP_STORE"},
		listQuery:    listQuery{limit: 1},
	})
	assertNoPlatformFilter(t, "profiles", profiles)

	certificates := buildCertificatesQuery(&certificatesQuery{
		certificateTypes: []string{"IOS_DISTRIBUTION"},
		listQuery:        listQuery{limit: 1},
	})
	assertNoPlatformFilter(t, "certificates", certificates)
}

func assertNoPlatformFilter(t *testing.T, name, rawQuery string) {
	t.Helper()
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("parse %s query %q: %v", name, rawQuery, err)
	}
	if got, present := values["filter[platform]"]; present {
		t.Fatalf("%s query sent filter[platform]=%q", name, got)
	}
}
