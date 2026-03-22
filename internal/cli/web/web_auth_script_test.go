package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadAppleTwoFactorScript(t *testing.T) string {
	t.Helper()

	scriptPath := filepath.Join("..", "..", "..", "scripts", "get-apple-2fa-code.scpt")
	contents, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	return string(contents)
}

func TestAppleTwoFactorScriptScansForCodeBeforeTrustClick(t *testing.T) {
	script := loadAppleTwoFactorScript(t)

	scanIndex := strings.Index(script, "set code to my scanWindowForCode(currentWindow)")
	clickIndex := strings.Index(script, "set didAdvanceTrustPrompt to my clickTrustButtonIfPresent(currentWindow)")
	if scanIndex == -1 || clickIndex == -1 {
		t.Fatalf("expected scan and trust-click flow in script")
	}
	if scanIndex > clickIndex {
		t.Fatalf("expected script to scan for a code before attempting to advance the trust dialog")
	}
}

func TestAppleTwoFactorScriptAllowsSingleButtonTrustDialogs(t *testing.T) {
	script := loadAppleTwoFactorScript(t)

	if !strings.Contains(script, "if not (my looksLikeTrustDialog(theWindow)) then") {
		t.Fatalf("expected trust-dialog guard before single-button fallback")
	}
	if strings.Contains(script, "return my clickRightmostButton(theWindow)") {
		guardIndex := strings.Index(script, "if not (my looksLikeTrustDialog(theWindow)) then")
		clickIndex := strings.Index(script, "return my clickRightmostButton(theWindow)")
		if guardIndex == -1 || clickIndex == -1 || guardIndex > clickIndex {
			t.Fatalf("expected trust-dialog guard before rightmost-button fallback")
		}
	}
}

func TestAppleTwoFactorScriptUsesTwoStepDeadlineAssignment(t *testing.T) {
	script := loadAppleTwoFactorScript(t)

	if strings.Contains(script, "set deadlineAt to (current date) + timeoutSeconds") {
		t.Fatalf("expected conservative two-step deadline assignment for AppleScript compatibility")
	}
	if !strings.Contains(script, "set deadlineAt to (current date)") {
		t.Fatalf("expected deadline initialization in script")
	}
	if !strings.Contains(script, "set deadlineAt to deadlineAt + timeoutSeconds") {
		t.Fatalf("expected deadline extension in script")
	}
}

func TestAppleTwoFactorScriptRestrictsFallbackToRecognizedTrustPrompts(t *testing.T) {
	script := loadAppleTwoFactorScript(t)

	if !strings.Contains(script, "property trustDialogTextHints :") {
		t.Fatalf("expected trust dialog text hints in script")
	}
	if strings.Contains(script, "\"trust\"") || strings.Contains(script, "\"trusted\"") {
		t.Fatalf("expected trust dialog hints to avoid generic trust/trusted substrings")
	}
	if !strings.Contains(script, "return my windowContainsAnyTextHint(theWindow, trustDialogTextHints)") {
		t.Fatalf("expected trust dialog detection helper in script")
	}
	if !strings.Contains(script, "if not (my looksLikeTrustDialog(theWindow)) then") || !strings.Contains(script, "\t\treturn false") {
		t.Fatalf("expected script to refuse unknown FollowUpUI dialogs before fallback clicking")
	}
}

func TestAppleTwoFactorScriptDoesNotTreatTrustedDevicesPromptAsTrustDialog(t *testing.T) {
	script := loadAppleTwoFactorScript(t)

	if !strings.Contains(script, "property codeEntryDialogTextHints :") {
		t.Fatalf("expected code-entry dialog text hints in script")
	}
	if !strings.Contains(script, "if my windowContainsAnyTextHint(theWindow, codeEntryDialogTextHints) then") {
		t.Fatalf("expected trust dialog detection to reject ordinary code-entry prompts first")
	}
	if !strings.Contains(script, "\"trusted devices\"") || !strings.Contains(script, "\"verification code\"") {
		t.Fatalf("expected code-entry dialog hints for ordinary 2FA challenge text")
	}
}

func TestAppleTwoFactorScriptExtractsStandaloneSixDigitCodes(t *testing.T) {
	script := loadAppleTwoFactorScript(t)

	if strings.Contains(script, "/usr/bin/tr -cd '0-9' | /usr/bin/grep -Eo '[0-9]{6}'") {
		t.Fatalf("expected script to avoid collapsing unrelated digits before matching 2FA codes")
	}
	if !strings.Contains(script, "/usr/bin/grep -Eo '(^|[^0-9])[0-9]{6}([^0-9]|$)'") {
		t.Fatalf("expected standalone 6-digit extraction pattern in script")
	}
}

func TestAppleTwoFactorScriptExtractsSpacedSixDigitCodes(t *testing.T) {
	script := loadAppleTwoFactorScript(t)

	if !strings.Contains(script, "set extractedCode to do shell script") {
		t.Fatalf("expected primary shell-based extraction in script")
	}
	if !strings.Contains(script, "if extractedCode is not \"\" then") {
		t.Fatalf("expected script to fall back only after checking for an empty shell extraction result")
	}
	if !strings.Contains(script, "set extractedCode to my extractSpacedCodeText(sourceText)") {
		t.Fatalf("expected labeled spaced-digit extraction before the digit-only fallback")
	}
	if !strings.Contains(script, "on extractSpacedCodeText(sourceText)") {
		t.Fatalf("expected dedicated spaced-digit extraction helper in script")
	}
	if !strings.Contains(script, "/usr/bin/grep -Eo '[0-9]([[:space:][:punct:]]+[0-9]){5}'") {
		t.Fatalf("expected spaced-digit helper to extract six-digit sequences without concatenating unrelated label digits")
	}
	if !strings.Contains(script, "set digitsOnly to my digitsOnlyText(sourceText)") {
		t.Fatalf("expected spaced-digit fallback extraction in script")
	}
	if !strings.Contains(script, "if (length of digitsOnly) is 6 then") {
		t.Fatalf("expected plain digit-normalization fallback for already-isolated six-digit text")
	}
	if !strings.Contains(script, "on digitsOnlyText(sourceText)") {
		t.Fatalf("expected digit-normalization helper in script")
	}
	if strings.Contains(script, "containsLatinLetters") {
		t.Fatalf("did not expect letter guard to reject labeled FollowUpUI verification code strings")
	}
}
