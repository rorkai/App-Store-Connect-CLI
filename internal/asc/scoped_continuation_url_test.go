package asc

import "testing"

func TestScopedContinuationURLBindsCanonicalCollection(t *testing.T) {
	expectedPath := "/v1/appInfos/info-1/appInfoLocalizations"
	tests := []struct {
		name    string
		nextURL string
		wantURL string
		wantErr bool
	}{
		{name: "absolute same collection", nextURL: "https://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next"},
		{name: "uppercase scheme canonicalized", nextURL: "HTTPS://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next", wantURL: "https://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next"},
		{name: "relative same collection", nextURL: expectedPath + "?cursor=next"},
		{name: "different resource", nextURL: "https://api.appstoreconnect.apple.com/v1/appInfos/info-2/appInfoLocalizations?cursor=next", wantErr: true},
		{name: "different collection", nextURL: "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/territoryAgeRatings?cursor=next", wantErr: true},
		{name: "other host", nextURL: "https://example.com" + expectedPath + "?cursor=next", wantErr: true},
		{name: "protocol relative", nextURL: "//api.appstoreconnect.apple.com" + expectedPath + "?cursor=next", wantErr: true},
		{name: "userinfo", nextURL: "https://user:pass@api.appstoreconnect.apple.com" + expectedPath + "?cursor=next", wantErr: true},
		{name: "fragment", nextURL: "https://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next#fragment", wantErr: true},
		{name: "insecure scheme", nextURL: "http://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next", wantErr: true},
		{name: "opaque URL", nextURL: "https:" + expectedPath + "?cursor=next", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScopedContinuationURL(tt.nextURL, expectedPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected continuation URL rejection, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ScopedContinuationURL() error: %v", err)
			}
			want := tt.wantURL
			if want == "" {
				want = tt.nextURL
			}
			if got != want {
				t.Fatalf("ScopedContinuationURL() = %q, want %q", got, want)
			}
		})
	}
}
