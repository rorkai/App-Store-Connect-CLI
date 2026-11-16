//go:build windows

package web

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWriteWebSessionBundleAppliesOwnerOnlyDACL(t *testing.T) {
	d, _, _ := newWebTestDeps()
	path := filepath.Join(t.TempDir(), "session.json")
	if _, err := writeWebSessionBundle(d, path, []byte(`{"kind":"asc-web-session"}`), false); err != nil {
		t.Fatalf("writeWebSessionBundle() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		t.Fatalf("GetTokenUser() = (%v, %v), want current user SID", user, err)
	}
	descriptor, err := windows.GetSecurityInfo(
		windows.Handle(file.Fd()),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("GetSecurityInfo() error = %v", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		t.Fatalf("Owner() error = %v", err)
	}
	if owner == nil || !owner.Equals(user.User.Sid) {
		t.Fatalf("bundle owner = %v, want current user", owner)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("Control() error = %v", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("bundle DACL is inheritable")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatalf("DACL() error = %v", err)
	}
	if dacl == nil || dacl.AceCount != 1 {
		t.Fatalf("bundle DACL = %#v, want exactly one owner ACE", dacl)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatalf("GetAce() error = %v", err)
	}
	if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 {
		t.Fatalf("bundle ACE = %#v, want non-inherited allow ACE", ace)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if aceSID == nil || !aceSID.Equals(user.User.Sid) {
		t.Fatal("bundle DACL grants access to a different account")
	}
	required := uint32(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE)
	if uint32(ace.Mask)&required != required {
		t.Fatalf("bundle owner ACE mask = %#x, want read/write rights %#x", ace.Mask, required)
	}
}
