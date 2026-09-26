//go:build darwin && cgo

package signing

/*
#cgo CFLAGS: -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

// asc_signing_keychain_lock_state reads a keychain's lock state without
// unlocking it and with user interaction disabled, so it never prompts.
static OSStatus asc_signing_keychain_lock_state(const char *path, Boolean *unlocked) {
    SecKeychainRef keychain = NULL;
    SecKeychainStatus status = 0;
    Boolean previous_user_interaction_allowed = true;
    OSStatus result = SecKeychainGetUserInteractionAllowed(&previous_user_interaction_allowed);
    if (result != errSecSuccess) return result;
    result = SecKeychainSetUserInteractionAllowed(false);
    if (result != errSecSuccess) return result;
    result = SecKeychainOpen(path, &keychain);
    if (result == errSecSuccess) {
        result = SecKeychainGetStatus(keychain, &status);
    }
    if (keychain != NULL) CFRelease(keychain);
    OSStatus restore = SecKeychainSetUserInteractionAllowed(previous_user_interaction_allowed);
    if (result == errSecSuccess && restore != errSecSuccess) result = restore;
    if (result == errSecSuccess) *unlocked = (status & kSecUnlockStateStatus) != 0;
    return result;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// errSecNoSuchKeychain is returned for a search-list entry whose file is gone.
const errSecNoSuchKeychain = -25294

func defaultKeychainLockState(path string) (exists, locked bool, err error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var unlocked C.Boolean
	status := int32(C.asc_signing_keychain_lock_state(cPath, &unlocked))
	switch status {
	case 0:
		return true, unlocked == 0, nil
	case errSecNoSuchKeychain:
		return false, false, nil
	default:
		return false, false, fmt.Errorf("security framework status %d", status)
	}
}
