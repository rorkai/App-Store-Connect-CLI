package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestRunArtifactInfoInputFailuresAreClassified(t *testing.T) {
	dir := t.TempDir()
	missingIPA := filepath.Join(dir, "missing.ipa")
	junkIPA := writeLocalInputFixture(t, filepath.Join(dir, "junk.ipa"), "junk\n")
	directoryIPA := filepath.Join(dir, "directory.ipa")
	if err := os.Mkdir(directoryIPA, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []localInputOutcomeCase{
		{
			name:             "missing path",
			args:             []string{"ipa-info", "--path", missingIPA},
			wantStderrPrefix: "Error: ipa-info: --path file not found: " + strconv.Quote(missingIPA) + "\n",
			wantReceipt:      true,
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
		{
			name:             "undecodable path",
			args:             []string{"ipa-info", "--path", junkIPA},
			wantStderrPrefix: "Error: ipa-info: open IPA: truncated ZIP directory\n",
			wantReceipt:      true,
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--path",
		},
		{
			name:             "directory path",
			args:             []string{"ipa-info", "--path", directoryIPA},
			wantStderrPrefix: "Error: ipa-info: ",
			wantReceipt:      true,
			wantDiagnosis:    shared.DiagnosticInvalidInput,
			wantParameter:    "--path",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runLocalInputOutcomeCase(t, test)
		})
	}
}
