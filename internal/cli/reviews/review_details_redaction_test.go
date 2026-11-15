package reviews

import (
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const reviewDetailRedactionSentinel = "asc-red-sentinel-reviews-unit-pw-b3f8e1"

func TestPresentableReviewDetailRedactsUnlessRequested(t *testing.T) {
	resp := &asc.AppStoreReviewDetailResponse{}
	resp.Data.ID = "detail-1"
	resp.Data.Attributes.DemoAccountPassword = reviewDetailRedactionSentinel

	safe := presentableReviewDetail(resp, false)
	if safe.Data.Attributes.DemoAccountPassword != asc.RedactedValuePlaceholder {
		t.Fatalf("rendered password = %q, want placeholder", safe.Data.Attributes.DemoAccountPassword)
	}
	if resp.Data.Attributes.DemoAccountPassword != reviewDetailRedactionSentinel {
		t.Fatalf("the fetched response was mutated to %q", resp.Data.Attributes.DemoAccountPassword)
	}

	if got := presentableReviewDetail(resp, true); got != resp {
		t.Fatal("--include-sensitive must render the original response")
	}
}

func TestReviewDetailCommandsExposeIncludeSensitiveFlag(t *testing.T) {
	d, _, _ := newReviewTestDeps(nil)
	commands := map[string]*ffcli.Command{
		"details-get":         newReviewDetailsGetCommand(d),
		"details-for-version": newReviewDetailsForVersionCommand(d),
		"details-create":      newReviewDetailsCreateCommand(d),
		"details-update":      newReviewDetailsUpdateCommand(d),
	}

	for name, cmd := range commands {
		flag := cmd.FlagSet.Lookup(shared.IncludeSensitiveFlagName)
		if flag == nil {
			t.Fatalf("%s is missing --%s", name, shared.IncludeSensitiveFlagName)
		}
		if flag.DefValue != "false" {
			t.Fatalf("%s --%s defaults to %q, want false", name, shared.IncludeSensitiveFlagName, flag.DefValue)
		}
		if !strings.Contains(flag.Usage, "only to this invocation") {
			t.Fatalf("%s --%s usage = %q, want per-invocation scope", name, shared.IncludeSensitiveFlagName, flag.Usage)
		}
	}
}
