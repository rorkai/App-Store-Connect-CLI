package ads

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestPlatformAssetsUploadIsOneCustomCommand(t *testing.T) {
	assets := findCommand(AdsCommand(), "assets")
	if assets == nil {
		t.Fatal("missing assets group")
	}
	count := 0
	for _, command := range assets.Subcommands {
		if command.Name == "upload" {
			count++
			if command.ShortUsage != "asc ads assets upload --file IMAGE --brand ID --ad-account ID" {
				t.Fatalf("assets upload usage = %q", command.ShortUsage)
			}
			if !strings.Contains(command.LongHelp, `Poll "asc ads assets view"`) {
				t.Fatalf("assets upload help does not point to direct assets view: %q", command.LongHelp)
			}
			for _, status := range []string{"ELIGIBLE", "LIMITED", "PENDING", "INELIGIBLE"} {
				if !strings.Contains(command.LongHelp, status) {
					t.Fatalf("assets upload help = %q, want eligibility status %q", command.LongHelp, status)
				}
			}
			for _, want := range []string{"Schema: form-data:", "Shape: multipart/form-data", "Required: yes"} {
				if !strings.Contains(command.LongHelp, want) {
					t.Fatalf("assets upload help = %q, want %q", command.LongHelp, want)
				}
			}
			for _, flagName := range []string{"file", "brand", "ad-account", "ads-profile", "output"} {
				if command.FlagSet.Lookup(flagName) == nil {
					t.Fatalf("assets upload missing --%s", flagName)
				}
			}
			if command.FlagSet.Lookup("confirm") != nil {
				t.Fatal("assets upload must not expose --confirm")
			}
		}
	}
	if count != 1 {
		t.Fatalf("assets upload command count = %d, want 1", count)
	}
}

func TestPlatformMapDeleteCommandsRequireConfirmationBeforeOtherWork(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	setAdsResolverTestEnv(t)
	for _, path := range [][]string{{"location-groups", "delete"}, {"creatives", "delete"}, {"assets", "delete"}} {
		t.Run(strings.Join(path, "_"), func(t *testing.T) {
			spec, ok := appleads.PlatformEndpointByCommandPath(path...)
			if !ok {
				t.Fatalf("missing spec %s", strings.Join(path, " "))
			}
			_, flags := bindEndpointFlags(spec, "test")
			err := executeEndpoint(context.Background(), spec, flags)
			if err == nil || !strings.Contains(err.Error(), "--confirm is required") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPlatformLocationGroupUpdateRequiresRiskConfirmationBeforeOtherWork(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	setAdsResolverTestEnv(t)

	spec, ok := appleads.PlatformEndpointByCommandPath("location-groups", "update")
	if !ok {
		t.Fatal("missing location-groups update spec")
	}
	if !spec.RiskConfirm || spec.RiskConfirmBodyField != "" {
		t.Fatalf("location-groups update risk metadata = %+v, want unconditional confirmation", spec)
	}
	_, flags := bindEndpointFlags(spec, "location-groups update")
	*flags.pathStrings["id"] = "GROUP"
	err := executeEndpoint(context.Background(), spec, flags)
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--confirm is required") {
		t.Fatalf("location-groups update error = %v, want pre-auth confirmation usage error", err)
	}

	for _, path := range [][]string{
		{"location-groups", "create"},
		{"creatives", "create"},
		{"creatives", "update"},
		{"assets", "upload"},
	} {
		other, ok := appleads.PlatformEndpointByCommandPath(path...)
		if !ok {
			t.Fatalf("missing %s spec", strings.Join(path, " "))
		}
		if other.RiskConfirm || other.RequiresConfirm {
			t.Fatalf("%s confirmation metadata = %+v, want confirmation-free", strings.Join(path, " "), other)
		}
		otherCommand := findCommand(AdsCommand(), path...)
		if otherCommand == nil || otherCommand.FlagSet.Lookup("confirm") != nil {
			t.Fatalf("%s must remain confirmation-free", strings.Join(path, " "))
		}
	}
}

func TestOpenPlatformAssetUploadFileValidation(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	unsupported := filepath.Join(dir, "asset.gif")
	if err := os.WriteFile(unsupported, []byte("gif"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(dir, "asset.HEIC")
	if err := os.WriteFile(valid, []byte("heic"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "link.png")
	if err := os.Symlink(valid, symlink); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		path      string
		wantError string
		wantIs    error
	}{
		{name: "missing", path: filepath.Join(dir, "missing.png"), wantError: "could not be opened", wantIs: os.ErrNotExist},
		{name: "empty", path: empty, wantError: "must not be empty"},
		{name: "directory", path: dir, wantError: "must be a regular file"},
		{name: "symlink", path: symlink, wantError: "must not be a symlink", wantIs: rootfs.ErrSymlink},
		{name: "unsupported", path: unsupported, wantError: "must be PNG, JPEG, or HEIC"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, _, _, _, err := openPlatformAssetUploadFile(test.path)
			if file != nil {
				_ = file.Close()
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
			if test.wantIs != nil && !errors.Is(err, test.wantIs) {
				t.Fatalf("error = %v, want errors.Is(%v)", err, test.wantIs)
			}
			if strings.Contains(err.Error(), test.path) {
				t.Fatalf("error leaks asset path: %v", err)
			}
		})
	}

	file, size, name, contentType, err := openPlatformAssetUploadFile(valid)
	if err != nil {
		t.Fatalf("valid file error: %v", err)
	}
	defer file.Close()
	if size != 4 || name != "asset.HEIC" || contentType != "image/heic" {
		t.Fatalf("valid file = size %d name %q MIME %q", size, name, contentType)
	}

	for extension, wantMIME := range map[string]string{
		".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".heic": "image/heic",
	} {
		path := filepath.Join(dir, "supported"+extension)
		if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
			t.Fatal(err)
		}
		opened, _, _, gotMIME, err := openPlatformAssetUploadFile(path)
		if err != nil {
			t.Fatalf("%s error: %v", extension, err)
		}
		_ = opened.Close()
		if gotMIME != wantMIME {
			t.Fatalf("%s MIME = %q, want %q", extension, gotMIME, wantMIME)
		}
	}
}

func TestPlatformAssetUploadMissingFlagsFailBeforeAuth(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	setAdsResolverTestEnv(t)
	command := PlatformAssetUploadCommand()
	if err := command.FlagSet.Parse([]string{"--brand", "BRAND", "--ad-account", "ACCOUNT"}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), command.FlagSet.Args())
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", err)
	}
}

func TestPlatformAssetUploadValidatesOutputBeforeMutation(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	setAdsResolverTestEnv(t)
	asset := filepath.Join(t.TempDir(), "asset.png")
	if err := os.WriteFile(asset, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unsupported format",
			args: []string{"--file", asset, "--brand", "BRAND", "--ad-account", "ACCOUNT", "--output", "yaml"},
			want: `(got "yaml")`,
		},
		{
			name: "pretty table",
			args: []string{"--file", asset, "--brand", "BRAND", "--ad-account", "ACCOUNT", "--output", "table", "--pretty"},
			want: `(got "table")`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := PlatformAssetUploadCommand()
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatal(err)
			}
			err := command.Exec(context.Background(), command.FlagSet.Args())
			if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want usage error containing %q", err, test.want)
			}
		})
	}
}

func TestPlatformAssetUploadRejectsUnsafeFilesBeforeAuth(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	setAdsResolverTestEnv(t)
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	unsupported := filepath.Join(dir, "asset.gif")
	if err := os.WriteFile(unsupported, []byte("gif"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(dir, "asset.png")
	if err := os.WriteFile(valid, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "link.png")
	if err := os.Symlink(valid, symlink); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name, path, want string
	}{
		{name: "missing", path: filepath.Join(dir, "missing.png"), want: "could not be opened"},
		{name: "empty", path: empty, want: "must not be empty"},
		{name: "directory", path: dir, want: "must be a regular file"},
		{name: "symlink", path: symlink, want: "must not be a symlink"},
		{name: "unsupported", path: unsupported, want: "must be PNG, JPEG, or HEIC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := PlatformAssetUploadCommand()
			if err := command.FlagSet.Parse([]string{"--file", test.path, "--brand", "BRAND", "--ad-account", "ACCOUNT"}); err != nil {
				t.Fatal(err)
			}
			err := command.Exec(context.Background(), command.FlagSet.Args())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if strings.Contains(err.Error(), test.path) {
				t.Fatalf("error leaks asset path: %v", err)
			}
		})
	}
}
