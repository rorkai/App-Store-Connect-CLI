package cmdtest

import "testing"

// encryptionDeclarationFields and encryptionDocumentFields mirror the allowed
// sets normalizeEncryptionDeclarationFields and normalizeEncryptionDocumentFields
// render into their diagnostics. They are spelled out so a change to either
// list fails this test instead of silently drifting.
const (
	encryptionDeclarationFields = "appDescription, createdDate, usesEncryption, exempt, " +
		"containsProprietaryCryptography, containsThirdPartyCryptography, availableOnFrenchStore, " +
		"platform, uploadedDate, documentUrl, documentName, documentType, " +
		"appEncryptionDeclarationState, codeValue, app, builds, appEncryptionDeclarationDocument"
	encryptionDocumentFields = "fileSize, fileName, assetToken, downloadUrl, sourceFileChecksum, " +
		"uploadOperations, assetDeliveryState"
	encryptionDeclarationIncludes = "app, builds, appEncryptionDeclarationDocument"
)

// TestEncryptionInputValidationReturnsUsageExitCode locks the usage-error
// contract for encryption flag validation: every pre-request flag check must
// print "Error: <message>" to stderr and exit with code 2, not the generic
// runtime failure code.
func TestEncryptionInputValidationReturnsUsageExitCode(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "declarations list limit above maximum",
			args:    []string{"encryption", "declarations", "list", "--app", "123456789", "--limit", "201"},
			wantErr: "encryption declarations list: --limit must be between 1 and 200",
		},
		{
			name:    "declarations list limit below minimum",
			args:    []string{"encryption", "declarations", "list", "--app", "123456789", "--limit", "-1"},
			wantErr: "encryption declarations list: --limit must be between 1 and 200",
		},
		{
			name:    "declarations list build-limit above maximum",
			args:    []string{"encryption", "declarations", "list", "--app", "123456789", "--build-limit", "51"},
			wantErr: "encryption declarations list: --build-limit must be between 1 and 50",
		},
		{
			name:    "declarations list non-App-Store-Connect next",
			args:    []string{"encryption", "declarations", "list", "--next", "http://api.appstoreconnect.apple.com/v1/appEncryptionDeclarations"},
			wantErr: "encryption declarations list: --next must be an App Store Connect URL",
		},
		{
			name:    "declarations list malformed next",
			args:    []string{"encryption", "declarations", "list", "--next", malformedNextURL},
			wantErr: "encryption declarations list: --next must be a valid URL: " + malformedNextURLParseError,
		},
		{
			name:    "declarations list unknown fields",
			args:    []string{"encryption", "declarations", "list", "--app", "123456789", "--fields", "bogus"},
			wantErr: "encryption declarations list: --fields must be one of: " + encryptionDeclarationFields,
		},
		{
			name:    "declarations list unknown document-fields",
			args:    []string{"encryption", "declarations", "list", "--app", "123456789", "--document-fields", "bogus"},
			wantErr: "encryption declarations list: --document-fields must be one of: " + encryptionDocumentFields,
		},
		{
			name:    "declarations list unknown include",
			args:    []string{"encryption", "declarations", "list", "--app", "123456789", "--include", "bogus"},
			wantErr: "encryption declarations list: --include must be one of: " + encryptionDeclarationIncludes,
		},
		{
			name:    "declarations view build-limit above maximum",
			args:    []string{"encryption", "declarations", "view", "--id", "DECL_ID", "--build-limit", "51"},
			wantErr: "encryption declarations view: --build-limit must be between 1 and 50",
		},
		{
			name:    "declarations view unknown fields",
			args:    []string{"encryption", "declarations", "view", "--id", "DECL_ID", "--fields", "bogus"},
			wantErr: "encryption declarations view: --fields must be one of: " + encryptionDeclarationFields,
		},
		{
			name:    "declarations view unknown include",
			args:    []string{"encryption", "declarations", "view", "--id", "DECL_ID", "--include", "bogus"},
			wantErr: "encryption declarations view: --include must be one of: " + encryptionDeclarationIncludes,
		},
		{
			name:    "documents view unknown fields",
			args:    []string{"encryption", "documents", "view", "--id", "DOC_ID", "--fields", "bogus"},
			wantErr: "encryption documents view: --fields must be one of: " + encryptionDocumentFields,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootWithDeps(t, nil, test.args...)
			assertUsageExitCodeResult(t, stdout, stderr, runErr, test.wantErr)
		})
	}
}
