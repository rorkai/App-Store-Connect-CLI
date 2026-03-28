package main

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseWorkflowExportsHomebrewChecksumsBeforeFormulaGeneration(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	exportAMD64 := "export SHA256_AMD64="
	exportArm64 := "export SHA256="
	pythonStep := "python3 - <<'PY'"

	exportAMD64Index := strings.Index(workflow, exportAMD64)
	if exportAMD64Index == -1 {
		t.Fatalf("release workflow missing %q", exportAMD64)
	}

	exportArm64Index := strings.Index(workflow, exportArm64)
	if exportArm64Index == -1 {
		t.Fatalf("release workflow missing %q", exportArm64)
	}

	pythonIndex := strings.Index(workflow, pythonStep)
	if pythonIndex == -1 {
		t.Fatalf("release workflow missing %q", pythonStep)
	}

	if exportAMD64Index > pythonIndex {
		t.Fatalf("%q must appear before %q", exportAMD64, pythonStep)
	}
	if exportArm64Index > pythonIndex {
		t.Fatalf("%q must appear before %q", exportArm64, pythonStep)
	}
}

func TestReleaseWorkflowPreservesRubyBinInterpolationInFormulaTest(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	want := `shell_output("#{{bin}}/asc --help")`
	if !strings.Contains(workflow, want) {
		t.Fatalf("release workflow missing escaped Ruby interpolation %q", want)
	}

	unwanted := `shell_output("#{bin}/asc --help")`
	if strings.Contains(workflow, unwanted) {
		t.Fatalf("release workflow still contains unescaped Ruby interpolation %q", unwanted)
	}
}

func TestReleaseWorkflowKeepsDocsGuardrails(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	for _, want := range []string{
		`python3 scripts/test_check_docs.py`,
		`make check-release-docs VERSION="${VERSION}"`,
		`make check-docs`,
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow missing docs guardrail %q", want)
		}
	}
}
