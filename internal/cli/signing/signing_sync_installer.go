package signing

import (
	"crypto/x509"
	"fmt"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
)

const installerIdentityKind = "pkcs12-installer-identity"

func isInstallerIdentityCategory(category string) bool {
	return category == "mac_installer_distribution" || category == "developer_id_installer"
}

func validateInstallerIdentityPurpose(certificate *x509.Certificate, category string) error {
	kind, err := artifacts.InstallerCertificateType(certificate)
	if err != nil {
		return fmt.Errorf("installer certificate purpose: %w", err)
	}
	wanted := "Mac Installer Distribution"
	if category == "developer_id_installer" {
		wanted = "Developer ID Installer"
	}
	if kind != wanted {
		return fmt.Errorf("installer certificate purpose is %s, directory requires %s", kind, wanted)
	}
	return nil
}
