package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const installStubBinaryContents = "fake-binary\n"

// installScriptAsset mirrors the OS/architecture mapping in install.sh for the
// platform the test runs on.
func installScriptAsset(t *testing.T, version, goos string) string {
	t.Helper()

	var osLabel string
	switch goos {
	case "darwin":
		osLabel = "macOS"
	case "linux":
		osLabel = "linux"
	default:
		t.Skipf("install.sh does not support GOOS %q", goos)
	}

	var archLabel string
	switch runtime.GOARCH {
	case "amd64":
		archLabel = "amd64"
	case "arm64":
		archLabel = "arm64"
	default:
		t.Skipf("install.sh does not support GOARCH %q", runtime.GOARCH)
	}

	return fmt.Sprintf("asc_%s_%s_%s", version, osLabel, archLabel)
}

// runInstallScript executes install.sh with a stubbed curl so no network
// access happens. checksumsMode controls what the stub serves for
// checksums.txt: valid, wrong, unlisted, or missing.
func runInstallScript(t *testing.T, checksumsMode string, extraEnv ...string) (output, installedBinary string, err error) {
	t.Helper()
	output, installedBinary, _, err = runInstallScriptPlatform(t, checksumsMode, runtime.GOOS, "13.0", "go 1.27.1", extraEnv...)
	return output, installedBinary, err
}

func runInstallScriptPlatform(t *testing.T, checksumsMode, goos, macOSVersion, goMod string, extraEnv ...string) (output, installedBinary, stderr string, err error) {
	t.Helper()

	repoRoot, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("getwd: %v", wdErr)
	}

	version := "0.0.1"
	asset := installScriptAsset(t, version, goos)
	sum := sha256.Sum256([]byte(installStubBinaryContents))
	checksum := hex.EncodeToString(sum[:])

	workDir := t.TempDir()
	stubDir := filepath.Join(workDir, "stub-bin")
	installDir := filepath.Join(workDir, "install-bin")
	if mkErr := os.MkdirAll(stubDir, 0o755); mkErr != nil {
		t.Fatalf("mkdir stub dir: %v", mkErr)
	}

	curlStub := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${STUB_STATE_DIR}/curl-requests"
out=""
url=""
prev=""
for arg in "$@"; do
  if [ "${prev}" = "-o" ]; then
    out="${arg}"
  fi
  case "${arg}" in
    http://*|https://*) url="${arg}" ;;
  esac
  prev="${arg}"
done

retry_target=""
case "${url}" in
  *_checksums.txt) retry_target="checksums" ;;
  */releases/download/*) retry_target="binary" ;;
esac
if [ -n "${retry_target}" ] && [ "${STUB_FAIL_ONCE:-}" = "${retry_target}" ]; then
  attempt_file="${STUB_STATE_DIR}/${retry_target}-attempted"
  if [ ! -e "${attempt_file}" ]; then
    touch "${attempt_file}"
    exit 22
  fi
fi

case "${url}" in
  */go.mod)
    if [ "${STUB_GO_MOD}" = "missing" ]; then exit 22; fi
    printf '%s\n' "${STUB_GO_MOD}"
    ;;
  */releases/latest)
    printf '%s' "https://github.com/rorkai/App-Store-Connect-CLI/releases/tag/${STUB_VERSION}"
    ;;
  *_checksums.txt)
    case "${STUB_CHECKSUMS_MODE}" in
      missing) exit 22 ;;
      valid) printf '%s  %s\n' "${STUB_SHA256}" "${STUB_ASSET}" > "${out}" ;;
      unlisted) printf '%s  %s\n' "${STUB_SHA256}" "${STUB_ASSET//./x}" > "${out}" ;;
      wrong) printf '%s  %s\n' "0000000000000000000000000000000000000000000000000000000000000000" "${STUB_ASSET}" > "${out}" ;;
      *) echo "unknown STUB_CHECKSUMS_MODE" >&2; exit 1 ;;
    esac
    ;;
  *)
    printf 'fake-binary\n' > "${out}"
    ;;
esac
`
	if writeErr := os.WriteFile(filepath.Join(stubDir, "curl"), []byte(curlStub), 0o755); writeErr != nil {
		t.Fatalf("write curl stub: %v", writeErr)
	}
	if writeErr := os.WriteFile(filepath.Join(stubDir, "sleep"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); writeErr != nil {
		t.Fatalf("write sleep stub: %v", writeErr)
	}

	unameOS := "Linux"
	if goos == "darwin" {
		unameOS = "Darwin"
	}
	unameStub := fmt.Sprintf("#!/usr/bin/env bash\ncase \"$1\" in\n-s) echo %q ;;\n-m) echo %q ;;\nesac\n", unameOS, runtime.GOARCH)
	if writeErr := os.WriteFile(filepath.Join(stubDir, "uname"), []byte(unameStub), 0o755); writeErr != nil {
		t.Fatalf("write uname stub: %v", writeErr)
	}
	swVersStub := "#!/usr/bin/env bash\nprintf '%s\\n' \"$STUB_MACOS_VERSION\"\n"
	if writeErr := os.WriteFile(filepath.Join(stubDir, "sw_vers"), []byte(swVersStub), 0o755); writeErr != nil {
		t.Fatalf("write sw_vers stub: %v", writeErr)
	}

	cmd := exec.Command("bash", filepath.Join(repoRoot, "install.sh"))
	cmd.Dir = workDir
	cmd.Env = append(
		os.Environ(),
		"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"INSTALL_DIR="+installDir,
		"STUB_VERSION="+version,
		"STUB_ASSET="+asset,
		"STUB_SHA256="+checksum,
		"STUB_CHECKSUMS_MODE="+checksumsMode,
		"STUB_STATE_DIR="+workDir,
		"STUB_MACOS_VERSION="+macOSVersion,
		"STUB_GO_MOD="+goMod,
	)
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderrBuffer bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderrBuffer
	runErr := cmd.Run()
	return stdout.String() + stderrBuffer.String(), filepath.Join(installDir, "asc"), stderrBuffer.String(), runErr
}

func TestInstallScriptChecksumVerification(t *testing.T) {
	for _, tt := range []struct {
		name          string
		checksumsMode string
		env           []string
		wantFailure   bool
		wantOutput    []string
	}{
		{name: "valid checksum", checksumsMode: "valid", wantOutput: []string{"Checksum verified."}},
		{name: "transient binary download failure", checksumsMode: "valid", env: []string{"STUB_FAIL_ONCE=binary"}, wantOutput: []string{"Download failed; retrying (2/3)"}},
		{name: "transient checksum download failure", checksumsMode: "valid", env: []string{"STUB_FAIL_ONCE=checksums"}, wantOutput: []string{"Download failed; retrying (2/3)"}},
		{name: "missing checksums", checksumsMode: "missing", wantFailure: true, wantOutput: []string{"Refusing to install without SHA-256 checksum verification.", "ASC_INSTALL_INSECURE=1"}},
		{name: "asset unlisted", checksumsMode: "unlisted", wantFailure: true, wantOutput: []string{"Refusing to install without SHA-256 checksum verification."}},
		{name: "insecure override allows missing checksums", checksumsMode: "missing", env: []string{"ASC_INSTALL_INSECURE=1"}, wantOutput: []string{"installing WITHOUT checksum verification"}},
		{name: "mismatch fails despite insecure override", checksumsMode: "wrong", env: []string{"ASC_INSTALL_INSECURE=1"}, wantFailure: true, wantOutput: []string{"Checksum verification failed"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output, installedBinary, err := runInstallScript(t, tt.checksumsMode, tt.env...)
			if tt.wantFailure != (err != nil) {
				t.Fatalf("install.sh error = %v, want failure %v\n%s", err, tt.wantFailure, output)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Fatalf("expected output to contain %q, got:\n%s", want, output)
				}
			}
			contents, readErr := os.ReadFile(installedBinary)
			if tt.wantFailure {
				if !os.IsNotExist(readErr) {
					t.Fatalf("expected no binary installed, read err=%v\n%s", readErr, output)
				}
				return
			}
			if readErr != nil || string(contents) != installStubBinaryContents {
				t.Fatalf("expected installed binary, contents=%q err=%v\n%s", contents, readErr, output)
			}
		})
	}
}

func TestInstallScriptMacOSReleaseCompatibility(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh targets unix shells")
	}
	for _, tt := range []struct {
		name, goos, macOSVersion, goMod, wantError string
		wantMetadata                               bool
	}{
		{"monterey-compatible-release", "darwin", "12.7.6", "module example.com/asc\n\ngo 1.26.8\n", "", true},
		{"monterey-new-release", "darwin", "12.7.6", "go 1.27.1", "requires macOS 13", true},
		{"monterey-new-toolchain", "darwin", "12.7.6", "go 1.26.8\ntoolchain go1.27.1", "requires macOS 13", true},
		{"monterey-future-go", "darwin", "12.7.6", "go 1.28.0", "requires macOS 13", true},
		{"monterey-invalid-toolchain", "darwin", "12.7.6", "go 1.26.8\ntoolchain go1.banana", "Could not determine macOS compatibility", true},
		{"monterey-missing-metadata", "darwin", "12.7.6", "missing", "Could not determine macOS compatibility", true},
		{"monterey-invalid-metadata", "darwin", "12.7.6", "not a go.mod file", "Could not determine macOS compatibility", true},
		{"monterey-default-go", "darwin", "12.7.6", "go default", "Could not determine macOS compatibility", true},
		{"monterey-default-toolchain", "darwin", "12.7.6", "go 1.26.8\ntoolchain default", "", true},
		{"monterey-invalid-version", "darwin", "12.7.6", "go banana", "Could not determine macOS compatibility", true},
		{"ventura", "darwin", "13.0.1", "go 1.27.1", "", false},
		{"newer-macos", "darwin", "26.0", "go 1.27.1", "", false},
		{"linux", "linux", "", "go 1.27.1", "", false},
		{"invalid-macos-version", "darwin", "unknown", "go 1.27.1", "Could not determine macOS version", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output, installedBinary, stderr, err := runInstallScriptPlatform(t, "valid", tt.goos, tt.macOSVersion, tt.goMod)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("install failed: %v\n%s", err, output)
				}
				if contents, readErr := os.ReadFile(installedBinary); readErr != nil || string(contents) != installStubBinaryContents {
					t.Fatalf("expected installed binary, contents=%q err=%v\n%s", contents, readErr, output)
				}
			} else {
				if err == nil || !strings.Contains(stderr, tt.wantError) {
					t.Fatalf("expected %q, err=%v\n%s", tt.wantError, err, output)
				}
				if _, statErr := os.Stat(filepath.Dir(installedBinary)); !os.IsNotExist(statErr) {
					t.Fatalf("unsupported release created install directory: %v", statErr)
				}
			}
			requests, readErr := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(installedBinary)), "curl-requests"))
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			metadataURL := "https://raw.githubusercontent.com/rorkai/App-Store-Connect-CLI/0.0.1/go.mod"
			if got := strings.Contains(string(requests), metadataURL); got != tt.wantMetadata {
				t.Fatalf("release metadata fetched=%v, want %v: %s", got, tt.wantMetadata, requests)
			}
			if tt.wantError != "" && strings.Contains(string(requests), "/releases/download/") {
				t.Fatalf("downloaded unsupported release: %s", requests)
			}
		})
	}
}
