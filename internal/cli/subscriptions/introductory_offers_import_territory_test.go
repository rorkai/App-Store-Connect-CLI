package subscriptions

import "testing"

func TestNormalizeSubscriptionIntroductoryOfferImportTerritoryID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "alpha three", input: "USA", want: "USA"},
		{name: "alpha two", input: "US", want: "USA"},
		{name: "english name", input: "Afghanistan", want: "AFG"},
		{name: "alias kosovo", input: "Kosovo", want: "XKS"},
		{name: "ascii curacao alias", input: "Curacao", want: "CUW"},
		{name: "unknown", input: "Atlantis", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeSubscriptionIntroductoryOfferImportTerritoryID(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}
