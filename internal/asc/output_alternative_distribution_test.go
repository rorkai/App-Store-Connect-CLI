package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_AlternativeDistributionDomains(t *testing.T) {
	resp := &AlternativeDistributionDomainsResponse{
		Data: []Resource[AlternativeDistributionDomainAttributes]{
			{
				ID: "domain-1",
				Attributes: AlternativeDistributionDomainAttributes{
					Domain:        "example.com",
					ReferenceName: "Example",
					CreatedDate:   "2024-01-01T00:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Domain") || !strings.Contains(output, "Reference Name") {
		t.Fatalf("expected domain headers, got: %s", output)
	}
	if !strings.Contains(output, "domain-1") || !strings.Contains(output, "example.com") || !strings.Contains(output, "Example") {
		t.Fatalf("expected domain values, got: %s", output)
	}
}

func TestPrintMarkdown_AlternativeDistributionDomains(t *testing.T) {
	resp := &AlternativeDistributionDomainsResponse{
		Data: []Resource[AlternativeDistributionDomainAttributes]{
			{
				ID: "domain-1",
				Attributes: AlternativeDistributionDomainAttributes{
					Domain:        "example.com",
					ReferenceName: "Example",
					CreatedDate:   "2024-01-01T00:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Domain") {
		t.Fatalf("expected domain header, got: %s", output)
	}
	if !strings.Contains(output, "domain-1") || !strings.Contains(output, "example.com") || !strings.Contains(output, "Example") {
		t.Fatalf("expected domain values, got: %s", output)
	}
}

func TestPrintTable_AlternativeDistributionKeys(t *testing.T) {
	resp := &AlternativeDistributionKeysResponse{
		Data: []Resource[AlternativeDistributionKeyAttributes]{
			{
				ID: "key-1",
				Attributes: AlternativeDistributionKeyAttributes{
					PublicKey: "KEYDATA",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Public Key") {
		t.Fatalf("expected key headers, got: %s", output)
	}
	if !strings.Contains(output, "key-1") || !strings.Contains(output, "KEYDATA") {
		t.Fatalf("expected key values, got: %s", output)
	}
}

func TestPrintMarkdown_AlternativeDistributionKeys(t *testing.T) {
	resp := &AlternativeDistributionKeysResponse{
		Data: []Resource[AlternativeDistributionKeyAttributes]{
			{
				ID: "key-1",
				Attributes: AlternativeDistributionKeyAttributes{
					PublicKey: "KEYDATA",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Public Key") {
		t.Fatalf("expected key header, got: %s", output)
	}
	if !strings.Contains(output, "key-1") || !strings.Contains(output, "KEYDATA") {
		t.Fatalf("expected key values, got: %s", output)
	}
}

func TestPrintTable_AlternativeDistributionPackage(t *testing.T) {
	resp := &AlternativeDistributionPackageResponse{
		Data: Resource[AlternativeDistributionPackageAttributes]{
			ID: "package-1",
			Attributes: AlternativeDistributionPackageAttributes{
				SourceFileChecksum: &Checksums{
					File: &Checksum{
						Hash:      "file-hash",
						Algorithm: ChecksumAlgorithmSHA256,
					},
					Composite: &Checksum{
						Hash:      "composite-hash",
						Algorithm: ChecksumAlgorithmMD5,
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Source File Checksum") {
		t.Fatalf("expected package headers, got: %s", output)
	}
	if !strings.Contains(output, "package-1") || !strings.Contains(output, "file-hash") || !strings.Contains(output, "composite-hash") {
		t.Fatalf("expected package values, got: %s", output)
	}
}

func TestPrintMarkdown_AlternativeDistributionPackage(t *testing.T) {
	resp := &AlternativeDistributionPackageResponse{
		Data: Resource[AlternativeDistributionPackageAttributes]{
			ID: "package-1",
			Attributes: AlternativeDistributionPackageAttributes{
				SourceFileChecksum: &Checksums{
					File: &Checksum{
						Hash:      "file-hash",
						Algorithm: ChecksumAlgorithmSHA256,
					},
					Composite: &Checksum{
						Hash:      "composite-hash",
						Algorithm: ChecksumAlgorithmMD5,
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Source File Checksum") {
		t.Fatalf("expected package header, got: %s", output)
	}
	if !strings.Contains(output, "package-1") || !strings.Contains(output, "file-hash") || !strings.Contains(output, "composite-hash") {
		t.Fatalf("expected package values, got: %s", output)
	}
}

func TestPrintTable_AlternativeDistributionPackageVariants(t *testing.T) {
	resp := &AlternativeDistributionPackageVariantsResponse{
		Data: []Resource[AlternativeDistributionPackageVariantAttributes]{
			{
				ID: "variant-1",
				Attributes: AlternativeDistributionPackageVariantAttributes{
					URL:                            "https://example.com/variant",
					URLExpirationDate:              "2024-01-02T00:00:00Z",
					AlternativeDistributionKeyBlob: "BLOB",
					FileChecksum:                   "checksum",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "URL") || !strings.Contains(output, "Key Blob") {
		t.Fatalf("expected variant headers, got: %s", output)
	}
	if !strings.Contains(output, "variant-1") || !strings.Contains(output, "https://example.com/variant") {
		t.Fatalf("expected variant values, got: %s", output)
	}
}

func TestPrintMarkdown_AlternativeDistributionPackageVariants(t *testing.T) {
	resp := &AlternativeDistributionPackageVariantsResponse{
		Data: []Resource[AlternativeDistributionPackageVariantAttributes]{
			{
				ID: "variant-1",
				Attributes: AlternativeDistributionPackageVariantAttributes{
					URL:                            "https://example.com/variant",
					URLExpirationDate:              "2024-01-02T00:00:00Z",
					AlternativeDistributionKeyBlob: "BLOB",
					FileChecksum:                   "checksum",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Key Blob") {
		t.Fatalf("expected variant header, got: %s", output)
	}
	if !strings.Contains(output, "variant-1") || !strings.Contains(output, "https://example.com/variant") {
		t.Fatalf("expected variant values, got: %s", output)
	}
}

func TestPrintTable_AlternativeDistributionPackageDeltas(t *testing.T) {
	resp := &AlternativeDistributionPackageDeltasResponse{
		Data: []Resource[AlternativeDistributionPackageDeltaAttributes]{
			{
				ID: "delta-1",
				Attributes: AlternativeDistributionPackageDeltaAttributes{
					URL:                            "https://example.com/delta",
					URLExpirationDate:              "2024-01-03T00:00:00Z",
					AlternativeDistributionKeyBlob: "BLOB",
					FileChecksum:                   "checksum",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "URL") || !strings.Contains(output, "Key Blob") {
		t.Fatalf("expected delta headers, got: %s", output)
	}
	if !strings.Contains(output, "delta-1") || !strings.Contains(output, "https://example.com/delta") {
		t.Fatalf("expected delta values, got: %s", output)
	}
}

func TestPrintMarkdown_AlternativeDistributionPackageDeltas(t *testing.T) {
	resp := &AlternativeDistributionPackageDeltasResponse{
		Data: []Resource[AlternativeDistributionPackageDeltaAttributes]{
			{
				ID: "delta-1",
				Attributes: AlternativeDistributionPackageDeltaAttributes{
					URL:                            "https://example.com/delta",
					URLExpirationDate:              "2024-01-03T00:00:00Z",
					AlternativeDistributionKeyBlob: "BLOB",
					FileChecksum:                   "checksum",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Key Blob") {
		t.Fatalf("expected delta header, got: %s", output)
	}
	if !strings.Contains(output, "delta-1") || !strings.Contains(output, "https://example.com/delta") {
		t.Fatalf("expected delta values, got: %s", output)
	}
}
