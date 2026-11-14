package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_EndUserLicenseAgreement(t *testing.T) {
	resp := &EndUserLicenseAgreementResponse{
		Data: EndUserLicenseAgreementResource{
			ID: "eula-1",
			Attributes: EndUserLicenseAgreementAttributes{
				AgreementText: strings.Repeat("Terms and conditions ", 8192) + "FINAL_CLAUSE",
			},
			Relationships: &EndUserLicenseAgreementRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   "app-1",
					},
				},
				Territories: &RelationshipList{
					Data: []ResourceData{
						{Type: ResourceTypeTerritories, ID: "USA"},
						{Type: ResourceTypeTerritories, ID: "CAN"},
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Agreement Text") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "eula-1") {
		t.Fatalf("expected eula id in output, got: %s", output)
	}
	if !strings.Contains(output, "USA,CAN") {
		t.Fatalf("expected territories in output, got: %s", output)
	}
	if !strings.Contains(output, "FINAL_CLAUSE") {
		t.Fatal("expected the end of the agreement in output")
	}
}

func TestPrintMarkdown_EndUserLicenseAgreement(t *testing.T) {
	resp := &EndUserLicenseAgreementResponse{
		Data: EndUserLicenseAgreementResource{
			ID: "eula-2",
			Attributes: EndUserLicenseAgreementAttributes{
				AgreementText: "More terms",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Agreement Text") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "eula-2") {
		t.Fatalf("expected eula id in output, got: %s", output)
	}
}
