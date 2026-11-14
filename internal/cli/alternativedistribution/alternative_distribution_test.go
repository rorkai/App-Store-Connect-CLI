package alternativedistribution

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newTestDeps() *shared.Deps {
	return &shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}}
}

func TestAlternativeDistributionDomainsGetCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionDomainsGetCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --domain-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionDomainsCreateCommand_MissingDomain(t *testing.T) {
	cmd := newAlternativeDistributionDomainsCreateCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--reference-name", "Example"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --domain is missing, got %v", err)
	}
}

func TestAlternativeDistributionDomainsCreateCommand_MissingReferenceName(t *testing.T) {
	cmd := newAlternativeDistributionDomainsCreateCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--domain", "example.com"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --reference-name is missing, got %v", err)
	}
}

func TestAlternativeDistributionDomainsDeleteCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionDomainsDeleteCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --domain-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionDomainsDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := newAlternativeDistributionDomainsDeleteCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--domain-id", "DOMAIN_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestAlternativeDistributionKeysGetCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionKeysGetCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --key-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionKeysCreateCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := newAlternativeDistributionKeysCreateCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--public-key", "KEY"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestAlternativeDistributionKeysCreateCommand_MissingKey(t *testing.T) {
	cmd := newAlternativeDistributionKeysCreateCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--app", "APP_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when key input is missing, got %v", err)
	}
}

func TestAlternativeDistributionKeysDeleteCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionKeysDeleteCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --key-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionKeysDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := newAlternativeDistributionKeysDeleteCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestAlternativeDistributionKeysAppCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := newAlternativeDistributionKeysAppCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackagesGetCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackagesGetCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --package-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackagesCreateCommand_MissingAppStoreVersionID(t *testing.T) {
	cmd := newAlternativeDistributionPackagesCreateCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app-store-version-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackagesAppStoreVersionCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackagesAppStoreVersionCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app-store-version-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackageVariantsCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackageVariantsCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --variant-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackageDeltasCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackageDeltasCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --delta-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackageVersionsGetCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackageVersionsGetCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackageVersionsListCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackageVersionsListCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --package-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackageVersionsDeltasCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackageVersionsDeltasCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestAlternativeDistributionPackageVersionsVariantsCommand_MissingID(t *testing.T) {
	cmd := newAlternativeDistributionPackageVersionsVariantsCommand(newTestDeps())
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}
