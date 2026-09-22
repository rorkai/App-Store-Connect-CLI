//go:build windows

package screenshots

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"golang.org/x/sys/windows"
)

var createRestrictedTokenForMatrixTest = windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateRestrictedToken")

func TestCreateMatrixReviewSnapshotWindowsOwnerOnlyAtCreation(t *testing.T) {
	directory, err := createMatrixReviewSnapshotDir()
	if err != nil {
		t.Fatalf("createMatrixReviewSnapshotDir() error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })

	file, err := os.Open(directory)
	if err != nil {
		t.Fatalf("open owner-only snapshot directory: %v", err)
	}
	assertMatrixReviewOwnerOnlyDACL(t, file)
	if err := file.Close(); err != nil {
		t.Fatalf("close snapshot directory: %v", err)
	}

	path := directory + "\\index.html"
	created, err := createMatrixOwnerOnlyFile(path)
	if err != nil {
		t.Fatalf("createMatrixOwnerOnlyFile() error: %v", err)
	}
	assertMatrixReviewOwnerOnlyDACL(t, created)
	if err := created.Close(); err != nil {
		t.Fatalf("close snapshot file: %v", err)
	}
}

func TestCreateMatrixPrivateScratchWindowsOwnerOnlyAtCreation(t *testing.T) {
	parentPath, err := createMatrixPrivateAttemptParent()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptParent() error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(parentPath) })

	grandparent, err := os.OpenRoot(filepath.Dir(parentPath))
	if err != nil {
		t.Fatalf("open scratch grandparent: %v", err)
	}
	defer grandparent.Close()
	parent, err := grandparent.OpenRoot(filepath.Base(parentPath))
	if err != nil {
		t.Fatalf("open scratch parent: %v", err)
	}
	defer parent.Close()
	childName := ".asc-matrix-test-child"
	if err := createMatrixPrivateAttemptChild(parent, parentPath, childName); err != nil {
		t.Fatalf("createMatrixPrivateAttemptChild() error: %v", err)
	}
	childPath := filepath.Join(parentPath, childName)
	childRoot, err := parent.OpenRoot(childName)
	if err != nil {
		t.Fatalf("open scratch child: %v", err)
	}
	defer childRoot.Close()
	childFile := mustOpenWindowsDirectory(t, childPath)
	assertMatrixReviewOwnerOnlyDACL(t, childFile)
	if err := childFile.Close(); err != nil {
		t.Fatalf("close scratch child: %v", err)
	}

	parentFile := mustOpenWindowsDirectory(t, parentPath)
	assertMatrixReviewOwnerOnlyDACL(t, parentFile)
	if err := parentFile.Close(); err != nil {
		t.Fatalf("close scratch parent: %v", err)
	}
	parentDACL, err := lockMatrixPrivateAttemptParentRetained(parent)
	if err != nil {
		t.Fatalf("lockMatrixPrivateAttemptParent() error: %v", err)
	}
	t.Cleanup(func() {
		if err := unlockMatrixPrivateAttemptParentRetained(parentDACL, parent); err != nil {
			t.Errorf("restore scratch parent DACL: %v", err)
		}
	})
	lockedParent := mustOpenWindowsDirectory(t, parentPath)
	assertMatrixReviewOwnerOnlyDACL(t, lockedParent)
	if err := lockedParent.Close(); err != nil {
		t.Fatalf("close locked scratch parent: %v", err)
	}
	assertWindowsRenameDenied(t, childPath, childPath+"-replacement")
	if err := unlockMatrixPrivateAttemptParentRetained(parentDACL, parent); err != nil {
		t.Fatalf("unlockMatrixPrivateAttemptParent() error: %v", err)
	}

	if err := createMatrixPrivateAttemptOutputDir(childPath); err != nil {
		t.Fatalf("createMatrixPrivateAttemptOutputDir() error: %v", err)
	}
	outputFile := mustOpenWindowsDirectory(t, filepath.Join(childPath, "output"))
	assertMatrixReviewOwnerOnlyDACL(t, outputFile)
	if err := outputFile.Close(); err != nil {
		t.Fatalf("close scratch output: %v", err)
	}
	childDACL, err := lockMatrixPrivateAttemptDirectoryRetained(childRoot)
	if err != nil {
		t.Fatalf("lockMatrixPrivateAttemptDirectory() error: %v", err)
	}
	t.Cleanup(func() {
		if err := restoreMatrixPrivateAttemptDirectory(childDACL, childRoot); err != nil {
			t.Errorf("restore scratch child DACL: %v", err)
		}
	})
	assertWindowsRenameDenied(t, filepath.Join(childPath, "output"), filepath.Join(childPath, "output-replacement"))
	if err := restoreMatrixPrivateAttemptDirectory(childDACL, childRoot); err != nil {
		t.Fatalf("unlockMatrixPrivateAttemptDirectory() error: %v", err)
	}

	configPath := filepath.Join(childPath, "config.yaml")
	configFile, err := createMatrixPrivateAttemptFile(configPath)
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptFile() error: %v", err)
	}
	assertMatrixReviewOwnerOnlyDACL(t, configFile)
	if err := configFile.Close(); err != nil {
		t.Fatalf("close scratch config: %v", err)
	}
}

func TestCaptureWithRootWindowsRejectsProviderScratchReplacement(t *testing.T) {
	directory := t.TempDir()
	destinationPath := filepath.Join(directory, "destination")
	if err := os.Mkdir(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	destination, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer destination.Close()
	var renameErr error
	provider := ProviderFunc(func(_ context.Context, request CaptureRequest) (string, error) {
		replacement := request.OutputDir + "-replacement"
		renameErr = os.Rename(request.OutputDir, replacement)
		if renameErr == nil {
			_ = os.Rename(replacement, request.OutputDir)
			if windowsProcessTokenBypassesDACLs(t) {
				t.Skip("current Windows token bypasses directory DACLs; restricted-token tests still cover the lock")
			}
			t.Fatalf("provider renamed private scratch directory despite its protected parent")
		}
		path := filepath.Join(request.OutputDir, request.Name+".png")
		writeMinimalPNG(t, path, 100, 200)
		return path, nil
	})
	result, err := captureWithRootProvider(context.Background(), CaptureRequest{Name: "home", OutputDir: destinationPath}, destination, provider)
	if err != nil {
		t.Fatalf("captureWithRootProvider() error = %v, want protected scratch to remain writable", err)
	}
	if result == nil || result.Path == "" {
		t.Fatalf("capture result = %+v, want published image", result)
	}
	if renameErr == nil {
		t.Fatal("provider scratch rename unexpectedly succeeded")
	}
}

func TestMatrixPrivateWindowsLocksPathBasedProviderInputs(t *testing.T) {
	attempt, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptRoot() error: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = cleanupMatrixPrivateAttemptForExecution(attempt)
			_ = closeMatrixPrivateAttemptForExecution(attempt)
		}
	})

	outputPath := filepath.Join(attempt.path, "output")
	if err := createMatrixPrivateAttemptOutputDirInRoot(attempt.pinned); err != nil {
		t.Fatalf("createMatrixPrivateAttemptOutputDirInRoot() error: %v", err)
	}
	configPath := filepath.Join(attempt.path, "frame.yaml")
	configFile, err := createMatrixPrivateAttemptFileInRoot(attempt.pinned, "frame.yaml", configPath)
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptFileInRoot() error: %v", err)
	}
	configDACL, err := lockMatrixPrivateAttemptFileRetained(configFile)
	if err != nil {
		t.Fatalf("lockMatrixPrivateAttemptFile() error: %v", err)
	}
	defer func() {
		if err := finalizeMatrixPrivateAttemptFile(configDACL); err != nil {
			t.Errorf("restore locked provider config: %v", err)
		}
	}()
	if err := lockMatrixPrivateAttemptChild(&attempt); err != nil {
		t.Fatalf("lockMatrixPrivateAttemptChild() error: %v", err)
	}

	assertWindowsRenameDenied(t, outputPath, outputPath+"-replacement")
	assertWindowsRenameDenied(t, configPath, configPath+"-replacement")
	if file, openErr := os.OpenFile(configPath, os.O_WRONLY, 0); openErr == nil {
		_ = file.Close()
		t.Fatal("opened locked provider config for writing")
	}
	// The nested output remains writable for the provider, while its parent
	// entry and the read-only config cannot be redirected or modified.
	if err := os.WriteFile(filepath.Join(outputPath, "provider-output"), []byte("provider"), 0o600); err != nil {
		t.Fatalf("write nested provider output: %v", err)
	}

	if err := cleanupMatrixPrivateAttemptForExecution(attempt); err != nil {
		t.Fatalf("cleanupMatrixPrivateAttemptForExecution() error: %v", err)
	}
	if err := closeMatrixPrivateAttemptForExecution(attempt); err != nil {
		t.Fatalf("closeMatrixPrivateAttemptForExecution() error: %v", err)
	}
	closed = true
}

func TestMatrixPrivateWindowsOwnerOnlyRejectsRestrictedToken(t *testing.T) {
	attempt, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptRoot() error: %v", err)
	}
	t.Cleanup(func() {
		_ = cleanupMatrixPrivateAttemptForExecution(attempt)
		_ = closeMatrixPrivateAttemptForExecution(attempt)
	})

	path := filepath.Join(attempt.path, "owner-only.txt")
	file, err := createMatrixPrivateAttemptFile(path)
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptFile() error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close owner-only file: %v", err)
	}

	var processToken windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &processToken); err != nil {
		t.Fatalf("OpenProcessToken() error: %v", err)
	}
	defer processToken.Close()
	user, err := processToken.GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error: %v", err)
	}
	var restricted windows.Token
	disabledSID := windows.SIDAndAttributes{Sid: user.User.Sid}
	const disableMaxPrivilege = 0x1
	callResult, _, callErr := createRestrictedTokenForMatrixTest.Call(
		uintptr(processToken),
		disableMaxPrivilege,
		1,
		uintptr(unsafe.Pointer(&disabledSID)),
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&restricted)),
	)
	if callResult == 0 {
		t.Fatalf("CreateRestrictedToken() error: %v", callErr)
	}
	defer restricted.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(restricted, windows.TOKEN_IMPERSONATE|windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
		t.Fatalf("DuplicateTokenEx() error: %v", err)
	}
	defer impersonation.Close()
	if err := windows.SetThreadToken(nil, impersonation); err != nil {
		t.Fatalf("SetThreadToken() error: %v", err)
	}
	defer windows.RevertToSelf()

	if restrictedFile, openErr := os.Open(path); openErr == nil {
		_ = restrictedFile.Close()
		t.Fatal("restricted token opened owner-only file")
	}
}

func TestMatrixPrivateWindowsRootedObjectsRejectRestrictedToken(t *testing.T) {
	attempt, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptRoot() error: %v", err)
	}
	t.Cleanup(func() {
		_ = cleanupMatrixPrivateAttemptForExecution(attempt)
		_ = closeMatrixPrivateAttemptForExecution(attempt)
	})

	outputPath := filepath.Join(attempt.path, "output")
	if err := createMatrixPrivateAttemptOutputDirInRoot(attempt.pinned); err != nil {
		t.Fatalf("create rooted output directory: %v", err)
	}
	configPath := filepath.Join(attempt.path, "frame.yaml")
	configFile, err := createMatrixPrivateAttemptFileInRoot(attempt.pinned, "frame.yaml", configPath)
	if err != nil {
		t.Fatalf("create rooted config file: %v", err)
	}
	if err := configFile.Close(); err != nil {
		t.Fatalf("close rooted config file: %v", err)
	}

	snapshotPath, err := createMatrixReviewBrowserSnapshotWithContext(context.Background(), []byte("<html></html>"), nil)
	if err != nil {
		t.Fatalf("create actual review snapshot: %v", err)
	}
	t.Cleanup(func() { removeMatrixReviewBrowserSnapshot(snapshotPath) })

	for _, path := range []string{attempt.path, outputPath, configPath, snapshotPath} {
		assertMatrixWindowsRestrictedTokenCannotOpen(t, path)
	}
}

func assertMatrixWindowsRestrictedTokenCannotOpen(t *testing.T, path string) {
	t.Helper()
	var processToken windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &processToken); err != nil {
		t.Fatalf("OpenProcessToken() error: %v", err)
	}
	defer processToken.Close()
	user, err := processToken.GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error: %v", err)
	}
	var restricted windows.Token
	disabledSID := windows.SIDAndAttributes{Sid: user.User.Sid}
	const disableMaxPrivilege = 0x1
	callResult, _, callErr := createRestrictedTokenForMatrixTest.Call(
		uintptr(processToken),
		disableMaxPrivilege,
		1,
		uintptr(unsafe.Pointer(&disabledSID)),
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&restricted)),
	)
	if callResult == 0 {
		t.Fatalf("CreateRestrictedToken() error: %v", callErr)
	}
	defer restricted.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(restricted, windows.TOKEN_IMPERSONATE|windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
		t.Fatalf("DuplicateTokenEx() error: %v", err)
	}
	defer impersonation.Close()
	if err := windows.SetThreadToken(nil, impersonation); err != nil {
		t.Fatalf("SetThreadToken() error: %v", err)
	}
	defer windows.RevertToSelf()

	if file, err := os.Open(path); err == nil {
		_ = file.Close()
		t.Fatalf("restricted token opened owner-only path %q", path)
	}
}

func assertWindowsRenameDenied(t *testing.T, original, replacement string) {
	t.Helper()
	if err := os.Rename(original, replacement); err == nil {
		_ = os.Rename(replacement, original)
		if windowsProcessTokenBypassesDACLs(t) {
			t.Skip("current Windows token bypasses directory DACLs; restricted-token tests still cover the lock")
		}
		t.Fatalf("rename %q succeeded while its parent was protected", original)
	}
}

func windowsProcessTokenBypassesDACLs(t *testing.T) bool {
	t.Helper()
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		t.Fatalf("OpenProcessToken() error: %v", err)
	}
	defer token.Close()
	if token.IsElevated() {
		return true
	}
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatalf("CreateWellKnownSid() error: %v", err)
	}
	member, err := token.IsMember(admin)
	if err != nil {
		t.Fatalf("IsMember(Administrators) error: %v", err)
	}
	return member
}

func mustOpenWindowsDirectory(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %q: %v", path, err)
	}
	return file
}

func assertMatrixReviewOwnerOnlyDACL(t *testing.T, file *os.File) {
	t.Helper()
	descriptor, err := windows.GetSecurityInfo(
		windows.Handle(file.Fd()),
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_FUNCTION) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
			t.Skipf("filesystem does not expose DACLs: %v", err)
		}
		t.Fatalf("GetSecurityInfo() error: %v", err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("security descriptor control error: %v", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("security descriptor control = %#x, want protected DACL", control)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatalf("security descriptor DACL error: %v", err)
	}
	if dacl == nil || dacl.AceCount != 1 {
		t.Fatalf("security descriptor DACL = %#v, want one owner ACE", dacl)
	}
}
