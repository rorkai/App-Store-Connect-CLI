package winbackoffers

import (
	"strings"
	"testing"
)

func TestWinBackOffersCommandConstructors(t *testing.T) {
	top := WinBackOffersCommand()
	if top == nil {
		t.Fatal("expected win-back-offers command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := WinBackOffersCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	if got := WinBackOffersPricesCommand(); got == nil {
		t.Fatal("expected prices command")
	}
	if got := WinBackOffersRelationshipsCommand(); got == nil {
		t.Fatal("expected relationships command")
	}
}

func TestWinBackOffersCreateHelpDescribesPricePointIDs(t *testing.T) {
	cmd := WinBackOffersCreateCommand()
	priceFlag := cmd.FlagSet.Lookup("price")
	if priceFlag == nil {
		t.Fatal("expected --price flag")
	}
	if got := priceFlag.Usage; got != "Subscription price point ID(s), comma-separated" {
		t.Fatalf("unexpected --price usage: %q", got)
	}
	if !strings.Contains(cmd.LongHelp, `--price "SUBSCRIPTION_PRICE_POINT_ID"`) {
		t.Fatalf("expected long help to describe subscription price point IDs, got %q", cmd.LongHelp)
	}
}

func TestTerritoryFromPricePointID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr bool
	}{
		{
			// base64 of {"s":"6755496237","t":"USA","p":"10101"}
			name: "valid price point ID",
			id:   "eyJzIjoiNjc1NTQ5NjIzNyIsInQiOiJVU0EiLCJwIjoiMTAxMDEifQ",
			want: "USA",
		},
		{
			name: "valid price point ID with padding",
			id:   "eyJzIjoiNjc1NTQ5NjIzNyIsInQiOiJVU0EiLCJwIjoiMTAxMDEifQ==",
			want: "USA",
		},
		{
			name:    "not base64",
			id:      "not-a-price-point!!",
			wantErr: true,
		},
		{
			name:    "base64 of non-JSON",
			id:      "aGVsbG8",
			wantErr: true,
		},
		{
			// base64 of {"s":"1"}; decodes but lacks a territory.
			name:    "missing territory field",
			id:      "eyJzIjoiMSJ9",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := territoryFromPricePointID(test.id)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got territory %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected territory %q, got %q", test.want, got)
			}
		})
	}
}
