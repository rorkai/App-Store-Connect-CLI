package shared

import "testing"

func TestPriceFilter_Validate(t *testing.T) {
	tests := []struct {
		name    string
		filter  PriceFilter
		wantErr bool
	}{
		{"empty", PriceFilter{}, false},
		{"exact price", PriceFilter{Price: "4.99"}, false},
		{"range", PriceFilter{MinPrice: "1.00", MaxPrice: "9.99"}, false},
		{"min only", PriceFilter{MinPrice: "1.00"}, false},
		{"max only", PriceFilter{MaxPrice: "9.99"}, false},
		{"price with min", PriceFilter{Price: "4.99", MinPrice: "1.00"}, true},
		{"price with max", PriceFilter{Price: "4.99", MaxPrice: "9.99"}, true},
		{"invalid price", PriceFilter{Price: "abc"}, true},
		{"non-finite price", PriceFilter{Price: "NaN"}, true},
		{"invalid min", PriceFilter{MinPrice: "abc"}, true},
		{"non-finite min", PriceFilter{MinPrice: "+Inf"}, true},
		{"invalid max", PriceFilter{MaxPrice: "abc"}, true},
		{"non-finite max", PriceFilter{MaxPrice: "-Inf"}, true},
		{"min > max", PriceFilter{MinPrice: "10.00", MaxPrice: "5.00"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPriceFilter_MatchesPrice(t *testing.T) {
	tests := []struct {
		name          string
		filter        PriceFilter
		customerPrice string
		want          bool
	}{
		{"no filter", PriceFilter{}, "4.99", true},
		{"exact match", PriceFilter{Price: "4.99"}, "4.99", true},
		{"exact no match", PriceFilter{Price: "4.99"}, "5.99", false},
		{"exact match with space", PriceFilter{Price: "4.99"}, " 4.99 ", true},
		{"range match", PriceFilter{MinPrice: "1.00", MaxPrice: "9.99"}, "4.99", true},
		{"range below min", PriceFilter{MinPrice: "5.00", MaxPrice: "9.99"}, "4.99", false},
		{"range above max", PriceFilter{MinPrice: "1.00", MaxPrice: "4.00"}, "4.99", false},
		{"min only match", PriceFilter{MinPrice: "4.99"}, "4.99", true},
		{"min only below", PriceFilter{MinPrice: "5.00"}, "4.99", false},
		{"max only match", PriceFilter{MaxPrice: "4.99"}, "4.99", true},
		{"max only above", PriceFilter{MaxPrice: "4.00"}, "4.99", false},
		{"invalid price string", PriceFilter{Price: "4.99"}, "free", false},
		{"invalid customer non-finite", PriceFilter{Price: "4.99"}, "NaN", false},
		{"invalid filter non-finite exact", PriceFilter{Price: "NaN"}, "4.99", false},
		{"invalid filter non-finite range", PriceFilter{MinPrice: "1.00", MaxPrice: "+Inf"}, "4.99", false},
		{"zero price", PriceFilter{Price: "0"}, "0.00", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.MatchesPrice(tt.customerPrice)
			if got != tt.want {
				t.Fatalf("MatchesPrice(%q) = %v, want %v", tt.customerPrice, got, tt.want)
			}
		})
	}
}
