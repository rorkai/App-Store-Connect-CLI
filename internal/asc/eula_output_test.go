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
				AgreementText: "Terms and conditions",
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

func TestPrintTable_EndUserLicenseAgreementDeleteResult(t *testing.T) {
	result := &EndUserLicenseAgreementDeleteResult{
		ID:      "eula-3",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "eula-3") {
		t.Fatalf("expected eula id in output, got: %s", output)
	}
}
