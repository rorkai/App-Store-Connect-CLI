package app_events

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestNormalizeAppEventPurchaseRequirement(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "already normalized",
			input: "NO_COST_ASSOCIATED",
			want:  "NO_COST_ASSOCIATED",
		},
		{
			name:  "camel case value",
			input: "noCostAssociated",
			want:  "NO_COST_ASSOCIATED",
		},
		{
			name:  "underscore variant",
			input: "no_iap_required",
			want:  "NO_IAP_REQUIRED",
		},
		{
			name:    "invalid value",
			input:   "free",
			wantErr: "--purchase-requirement currently supports only: NO_COST_ASSOCIATED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeAppEventPurchaseRequirement(test.input)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", test.wantErr)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", test.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeAppEventPurchaseRequirement() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestValidateAppEventPurchaseRequirement(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "supported value",
			input: "NO_COST_ASSOCIATED",
		},
		{
			name:    "known unsupported no iap required",
			input:   "NO_IAP_REQUIRED",
			wantErr: "known 500 UNEXPECTED_ERROR",
		},
		{
			name:    "known unsupported iap required",
			input:   "IAP_REQUIRED",
			wantErr: "known 500 UNEXPECTED_ERROR",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAppEventPurchaseRequirement(test.input)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", test.wantErr)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", test.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

func TestAppEventHasTerritoryScheduleIgnoresTerritoryOrder(t *testing.T) {
	event := &asc.AppEventResponse{
		Data: asc.Resource[asc.AppEventAttributes]{
			Attributes: asc.AppEventAttributes{
				TerritorySchedules: []asc.AppEventTerritorySchedule{
					{
						Territories:  []string{"CAN", "USA"},
						PublishStart: "2026-05-15T00:00:00Z",
						EventStart:   "2026-06-01T00:00:00Z",
						EventEnd:     "2026-06-30T23:59:59Z",
					},
				},
			},
		},
	}

	expected := asc.AppEventTerritorySchedule{
		Territories:  []string{"USA", "CAN"},
		PublishStart: "2026-05-15T00:00:00Z",
		EventStart:   "2026-06-01T00:00:00Z",
		EventEnd:     "2026-06-30T23:59:59Z",
	}

	if !appEventHasTerritorySchedule(event, expected) {
		t.Fatal("expected schedule match when territories differ only by order")
	}
}

func TestAppEventHasTerritoryScheduleIgnoresEquivalentTimestampFormatting(t *testing.T) {
	event := &asc.AppEventResponse{
		Data: asc.Resource[asc.AppEventAttributes]{
			Attributes: asc.AppEventAttributes{
				TerritorySchedules: []asc.AppEventTerritorySchedule{
					{
						Territories:  []string{"USA", "CAN"},
						PublishStart: "2026-05-15T02:00:00+02:00",
						EventStart:   "2026-06-01T02:00:00+02:00",
						EventEnd:     "2026-07-01T01:59:59+02:00",
					},
				},
			},
		},
	}

	expected := asc.AppEventTerritorySchedule{
		Territories:  []string{"USA", "CAN"},
		PublishStart: "2026-05-15T00:00:00Z",
		EventStart:   "2026-06-01T00:00:00Z",
		EventEnd:     "2026-06-30T23:59:59Z",
	}

	if !appEventHasTerritorySchedule(event, expected) {
		t.Fatal("expected schedule match when timestamps differ only by RFC3339 formatting/offset")
	}
}

func TestAppEventHasTerritoryScheduleRejectsDifferentTerritories(t *testing.T) {
	event := &asc.AppEventResponse{
		Data: asc.Resource[asc.AppEventAttributes]{
			Attributes: asc.AppEventAttributes{
				TerritorySchedules: []asc.AppEventTerritorySchedule{
					{
						Territories:  []string{"USA", "CAN"},
						PublishStart: "2026-05-15T00:00:00Z",
						EventStart:   "2026-06-01T00:00:00Z",
						EventEnd:     "2026-06-30T23:59:59Z",
					},
				},
			},
		},
	}

	expected := asc.AppEventTerritorySchedule{
		Territories:  []string{"USA", "GBR"},
		PublishStart: "2026-05-15T00:00:00Z",
		EventStart:   "2026-06-01T00:00:00Z",
		EventEnd:     "2026-06-30T23:59:59Z",
	}

	if appEventHasTerritorySchedule(event, expected) {
		t.Fatal("expected schedule mismatch when territories differ")
	}
}
