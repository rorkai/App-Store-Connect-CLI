package backgroundassets

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBackgroundAssetsTestDeps returns deps whose client fails unless client is
// given, with streams that discard output.
func newBackgroundAssetsTestDeps(client func() (*asc.Client, error)) *shared.Deps {
	if client == nil {
		client = func() (*asc.Client, error) { return nil, errors.New("unexpected client request") }
	}
	return &shared.Deps{
		Client:        client,
		IO:            &shared.IO{Out: io.Discard, ErrOut: io.Discard},
		Timeout:       shared.ContextWithTimeout,
		UploadTimeout: shared.ContextWithUploadTimeout,
	}
}

func TestBackgroundAssetsListCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := newBackgroundAssetsListCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestBackgroundAssetsGetCommand_MissingID(t *testing.T) {
	cmd := newBackgroundAssetsGetCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestBackgroundAssetsCreateCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := newBackgroundAssetsCreateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--asset-pack-identifier", "com.example.assetpack"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestBackgroundAssetsCreateCommand_MissingAssetPackIdentifier(t *testing.T) {
	cmd := newBackgroundAssetsCreateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--app", "APP_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --asset-pack-identifier is missing, got %v", err)
	}
}

func TestBackgroundAssetsUpdateCommand_MissingID(t *testing.T) {
	cmd := newBackgroundAssetsUpdateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--archived", "true"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUpdateCommand_MissingArchived(t *testing.T) {
	cmd := newBackgroundAssetsUpdateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--id", "ASSET_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --archived is missing, got %v", err)
	}
}

func TestBackgroundAssetsVersionsListCommand_MissingAssetID(t *testing.T) {
	cmd := newBackgroundAssetsVersionsListCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --background-asset-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsVersionsGetCommand_MissingID(t *testing.T) {
	cmd := newBackgroundAssetsVersionsGetCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsVersionsCreateCommand_MissingAssetID(t *testing.T) {
	cmd := newBackgroundAssetsVersionsCreateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --background-asset-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesListCommand_MissingVersionID(t *testing.T) {
	cmd := newBackgroundAssetsUploadFilesListCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesGetCommand_MissingID(t *testing.T) {
	cmd := newBackgroundAssetsUploadFilesGetCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --upload-file-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesCreateCommand_MissingVersionID(t *testing.T) {
	cmd := newBackgroundAssetsUploadFilesCreateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--file", "./asset.zip", "--asset-type", "ASSET"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesCreateCommand_MissingFile(t *testing.T) {
	cmd := newBackgroundAssetsUploadFilesCreateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--version-id", "VERSION_ID", "--asset-type", "ASSET"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --file is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesCreateCommand_MissingAssetType(t *testing.T) {
	cmd := newBackgroundAssetsUploadFilesCreateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--version-id", "VERSION_ID", "--file", "./asset.zip"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --asset-type is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesUpdateCommand_MissingID(t *testing.T) {
	cmd := newBackgroundAssetsUploadFilesUpdateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--uploaded", "true"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --upload-file-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesUpdateCommand_MissingUploaded(t *testing.T) {
	cmd := newBackgroundAssetsUploadFilesUpdateCommand(newBackgroundAssetsTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--upload-file-id", "UPLOAD_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --uploaded is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesUpdateCommand_FileRequiresChecksumBeforeFileOrClientAccess(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "missing.zip")
	clientFactoryCalled := false
	cmd := newBackgroundAssetsUploadFilesUpdateCommand(newBackgroundAssetsTestDeps(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not run during flag validation")
	}))
	if err := cmd.FlagSet.Parse([]string{
		"--upload-file-id", "UPLOAD_ID",
		"--uploaded", "true",
		"--file", filePath,
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --file is used without --checksum, got %v", err)
	}
	if got, want := err.Error(), "--file requires --checksum"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if clientFactoryCalled {
		t.Fatal("client factory ran before --file/--checksum validation")
	}
}
