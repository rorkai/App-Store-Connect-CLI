//go:build darwin && cgo

package signing

/*
#cgo CFLAGS: -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

typedef struct {
    OSStatus operation_status;
    OSStatus cleanup_status;
    Boolean created;
} ASCSigningKeychainCreateResult;

typedef ASCSigningKeychainCreateResult ASCSigningKeychainImportResult;

static ASCSigningKeychainCreateResult asc_signing_keychain_create(
    const char *path,
    const unsigned char *password,
    size_t password_length,
    Boolean delete_on_unlock_failure
) {
    ASCSigningKeychainCreateResult result = { errSecSuccess, errSecSuccess };
    SecKeychainRef keychain = NULL;
    OSStatus status = SecKeychainCreate(
        path,
        (UInt32)password_length,
        password,
        false,
        NULL,
        &keychain
    );
    if (status == errSecSuccess) {
        result.created = true;
        status = SecKeychainUnlock(
            keychain,
            (UInt32)password_length,
            password,
            true
        );
        if (status != errSecSuccess && delete_on_unlock_failure) {
            result.cleanup_status = SecKeychainDelete(keychain);
        }
    }
    result.operation_status = status;
    if (keychain != NULL) {
        CFRelease(keychain);
    }
    return result;
}

static ASCSigningKeychainImportResult asc_signing_keychain_import_pkcs12(
    const char *keychain_path,
    const unsigned char *pkcs12_data,
    size_t pkcs12_length,
    const unsigned char *pkcs12_password,
    size_t pkcs12_password_length,
    const char *trusted_application_path,
    Boolean use_legacy_decoder
) {
    ASCSigningKeychainImportResult result = { errSecSuccess, errSecSuccess };
    OSStatus status = errSecSuccess;
    SecKeychainRef keychain = NULL;
    CFDataRef data = NULL;
    CFStringRef passphrase = NULL;
    SecTrustedApplicationRef application = NULL;
    CFArrayRef applications = NULL;
    SecAccessRef access = NULL;
    CFDictionaryRef options = NULL;
    CFArrayRef items = NULL;
    Boolean previous_user_interaction_allowed = true;
    Boolean user_interaction_state_captured = false;

    status = SecKeychainGetUserInteractionAllowed(&previous_user_interaction_allowed);
    if (status != errSecSuccess) goto cleanup;
    user_interaction_state_captured = true;
    status = SecKeychainSetUserInteractionAllowed(false);
    if (status != errSecSuccess) goto cleanup;
    status = SecKeychainOpen(keychain_path, &keychain);
    if (status != errSecSuccess) goto cleanup;

    data = CFDataCreate(NULL, pkcs12_data, (CFIndex)pkcs12_length);
    if (data == NULL) { status = errSecAllocate; goto cleanup; }

    passphrase = CFStringCreateWithBytes(
        NULL,
        pkcs12_password,
        (CFIndex)pkcs12_password_length,
        kCFStringEncodingUTF8,
        false
    );
    if (passphrase == NULL) { status = errSecAllocate; goto cleanup; }

    status = SecTrustedApplicationCreateFromPath(trusted_application_path, &application);
    if (status != errSecSuccess) goto cleanup;
    const void *values[] = { application };
    applications = CFArrayCreate(NULL, values, 1, &kCFTypeArrayCallBacks);
    if (applications == NULL) { status = errSecAllocate; goto cleanup; }
    status = SecAccessCreate(CFSTR("ASC signing identity"), applications, &access);
    if (status != errSecSuccess) goto cleanup;

    if (use_legacy_decoder) {
        SecItemImportExportKeyParameters parameters = {
            SEC_KEY_IMPORT_EXPORT_PARAMS_VERSION, 0, passphrase, NULL, NULL, access
        };
        SecExternalFormat format = kSecFormatPKCS12;
        SecExternalItemType type = kSecItemTypeAggregate;
        status = SecItemImport(data, NULL, &format, &type, 0, &parameters, keychain, &items);
    } else {
        const void *option_keys[] = {
            kSecImportExportPassphrase, kSecImportExportKeychain, kSecImportExportAccess
        };
        const void *option_values[] = { passphrase, keychain, access };
        options = CFDictionaryCreate(
            NULL, option_keys, option_values, 3,
            &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks
        );
        if (options == NULL) { status = errSecAllocate; goto cleanup; }
        // SecItemImport's legacy PKCS#12 decoder rejects modern PBES2/SHA-256 exports.
        status = SecPKCS12Import(data, options, &items);
    }

cleanup:
    if (user_interaction_state_captured) {
        OSStatus restore_status = SecKeychainSetUserInteractionAllowed(previous_user_interaction_allowed);
        if (restore_status != errSecSuccess) {
            result.cleanup_status = restore_status;
        }
    }
    if (items != NULL) CFRelease(items);
    if (options != NULL) CFRelease(options);
    if (access != NULL) CFRelease(access);
    if (applications != NULL) CFRelease(applications);
    if (application != NULL) CFRelease(application);
    if (passphrase != NULL) CFRelease(passphrase);
    if (data != NULL) CFRelease(data);
    if (keychain != NULL) CFRelease(keychain);
    result.operation_status = status;
    return result;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

func signingRunSecurityAvailable() bool { return true }

func createKeychainWithSecurityFramework(path string, password []byte) error {
	_, err := createKeychainWithSecurityFrameworkMode(path, password, false)
	return err
}

func createPersistentKeychainWithSecurityFramework(path string, password []byte) (bool, error) {
	return createKeychainWithSecurityFrameworkMode(path, password, false)
}

func createKeychainWithSecurityFrameworkMode(path string, password []byte, deleteOnUnlockFailure bool) (bool, error) {
	if len(password) == 0 {
		return false, fmt.Errorf("keychain password is empty")
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	deleteOnUnlockFailureValue := C.Boolean(0)
	if deleteOnUnlockFailure {
		deleteOnUnlockFailureValue = C.Boolean(1)
	}
	result := C.asc_signing_keychain_create(
		cPath,
		(*C.uchar)(unsafe.Pointer(&password[0])),
		C.size_t(len(password)),
		deleteOnUnlockFailureValue,
	)
	return result.created != 0, securityFrameworkKeychainCreationError(int32(result.operation_status), int32(result.cleanup_status))
}

func securityFrameworkKeychainCreationError(operationStatus, cleanupStatus int32) error {
	var operationErr error
	if operationStatus != 0 {
		operationErr = fmt.Errorf("security framework status %d", operationStatus)
	}
	var cleanupErr error
	if cleanupStatus != 0 {
		cleanupErr = fmt.Errorf("security framework keychain cleanup status %d", cleanupStatus)
	}
	return errors.Join(operationErr, cleanupErr)
}

func importPKCS12WithSecurityFramework(keychainPath string, data, password []byte) error {
	return importPKCS12WithSecurityFrameworkMode(keychainPath, data, password, false)
}

// SecItemImport preserves signing run's normalized, leaf-only keychain.
func importLegacyPKCS12WithSecurityFramework(keychainPath string, data, password []byte) error {
	return importPKCS12WithSecurityFrameworkMode(keychainPath, data, password, true)
}

func importPKCS12WithSecurityFrameworkMode(keychainPath string, data, password []byte, legacyDecoder bool) error {
	if len(data) == 0 || len(password) == 0 {
		return fmt.Errorf("PKCS#12 data or password is empty")
	}
	cKeychainPath := C.CString(keychainPath)
	cCodesignPath := C.CString("/usr/bin/codesign")
	defer C.free(unsafe.Pointer(cKeychainPath))
	defer C.free(unsafe.Pointer(cCodesignPath))
	legacyDecoderValue := C.Boolean(0)
	if legacyDecoder {
		legacyDecoderValue = C.Boolean(1)
	}
	result := C.asc_signing_keychain_import_pkcs12(
		cKeychainPath,
		(*C.uchar)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.uchar)(unsafe.Pointer(&password[0])),
		C.size_t(len(password)),
		cCodesignPath,
		legacyDecoderValue,
	)
	return securityFrameworkKeychainImportError(int32(result.operation_status), int32(result.cleanup_status))
}

func securityFrameworkKeychainImportError(operationStatus, cleanupStatus int32) error {
	var operationErr error
	if operationStatus != 0 {
		operationErr = fmt.Errorf("security framework status %d", operationStatus)
	}
	var cleanupErr error
	if cleanupStatus != 0 {
		cleanupErr = fmt.Errorf("security framework keychain import cleanup status %d", cleanupStatus)
	}
	return errors.Join(operationErr, cleanupErr)
}
