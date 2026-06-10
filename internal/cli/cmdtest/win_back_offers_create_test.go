package cmdtest

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestWinBackOffersCreateInvalidPricePointExitUsage(t *testing.T) {
	bin := buildCLIBinary(t)

	cmd := exec.Command(
		bin,
		"subscriptions", "offers", "win-back", "create",
		"--subscription-id", "sub-1",
		"--reference-name", "spring-2026",
		"--offer-id", "OFFER-1",
		"--duration", "ONE_MONTH",
		"--offer-mode", "PAY_AS_YOU_GO",
		"--period-count", "1",
		"--eligibility-paid-months", "6",
		"--eligibility-last-subscribed-min", "3",
		"--start-date", "2026-02-01",
		"--priority", "HIGH",
		"--price", "not-a-price-point!!",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exit error, got %v", err)
	}
	if code := exitErr.ExitCode(); code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "is not a subscription price point ID") {
		t.Fatalf("expected invalid price point error, got %q", stderr.String())
	}
}
