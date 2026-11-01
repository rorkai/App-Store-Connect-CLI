//go:build darwin && cgo && fido2

package securitykey

/*
#cgo pkg-config: libfido2
#include <fido.h>
#include <stdlib.h>
#include <string.h>

static void asc_fido_log(const char *message) { (void)message; }
static void asc_fido_init(void) {
	fido_init(0);
	fido_set_log_handler(asc_fido_log);
}
static void asc_free_pin(char *pin) {
	if (pin == NULL) return;
	volatile unsigned char *p = (volatile unsigned char *)pin;
	size_t n = strlen(pin);
	while (n--) *p++ = 0;
	free(pin);
}
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var initialize sync.Once

// Available reports whether this build includes the native signer.
func Available() bool {
	initialize.Do(func() { C.asc_fido_init() })
	return true
}

// Sign uses one attached FIDO2 key. The native operation owns the device until
// cancellation has finished, so no background call can access a freed handle.
func Sign(ctx context.Context, request Request, pin string) (*Assertion, error) {
	Available()
	if strings.IndexByte(pin, 0) >= 0 {
		return nil, fmt.Errorf("security-key PIN must not contain a NUL byte")
	}
	if request.RelyingParty == "" || strings.IndexByte(request.RelyingParty, 0) >= 0 || len(request.CredentialIDs) == 0 {
		return nil, fmt.Errorf("security-key relying party and credential allow-list are required")
	}
	ctx, cancel := context.WithTimeout(ctx, AssertionTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	const maxDevices = 64
	devices := C.fido_dev_info_new(maxDevices)
	if devices == nil {
		return nil, fmt.Errorf("allocate security-key discovery")
	}
	defer C.fido_dev_info_free(&devices, maxDevices)
	var count C.size_t
	if code := C.fido_dev_info_manifest(devices, maxDevices, &count); code != C.FIDO_OK {
		return nil, nativeError("discover security keys", code)
	}
	if count == 0 {
		return nil, fmt.Errorf("no security key found; connect your USB security key and retry")
	}
	if count != 1 {
		return nil, fmt.Errorf("multiple security keys found; leave only the key for this Apple Account connected and retry")
	}
	device := C.fido_dev_new()
	if device == nil {
		return nil, fmt.Errorf("allocate security-key device")
	}
	defer C.fido_dev_free(&device)
	if err := setNativeTimeout(ctx, device); err != nil {
		return nil, err
	}
	path := C.fido_dev_info_path(C.fido_dev_info_ptr(devices, 0))
	if path == nil {
		return nil, fmt.Errorf("security key has no device path")
	}
	if code := C.fido_dev_open(device, path); code != C.FIDO_OK {
		return nil, nativeError("open security key", code)
	}
	defer C.fido_dev_close(device)
	if !bool(C.fido_dev_is_fido2(device)) {
		return nil, fmt.Errorf("the connected security key does not support FIDO2")
	}
	assertion := C.fido_assert_new()
	if assertion == nil {
		return nil, fmt.Errorf("allocate security-key assertion")
	}
	defer C.fido_assert_free(&assertion)
	rp := C.CString(request.RelyingParty)
	defer C.free(unsafe.Pointer(rp))
	if code := C.fido_assert_set_rp(assertion, rp); code != C.FIDO_OK {
		return nil, nativeError("set security-key relying party", code)
	}
	if code := C.fido_assert_set_clientdata_hash(assertion, (*C.uchar)(unsafe.Pointer(&request.ClientDataHash[0])), 32); code != C.FIDO_OK {
		return nil, nativeError("set security-key client data", code)
	}
	for _, credential := range request.CredentialIDs {
		if len(credential) == 0 {
			return nil, fmt.Errorf("security-key credential ID must not be empty")
		}
		if code := C.fido_assert_allow_cred(assertion, (*C.uchar)(unsafe.Pointer(&credential[0])), C.size_t(len(credential))); code != C.FIDO_OK {
			return nil, nativeError("set security-key credential allow-list", code)
		}
	}
	if code := C.fido_assert_set_up(assertion, C.FIDO_OPT_TRUE); code != C.FIDO_OK {
		return nil, nativeError("require security-key touch", code)
	}
	var nativePIN *C.char
	if pin != "" {
		nativePIN = C.CString(pin)
		defer C.asc_free_pin(nativePIN)
	}
	if err := setNativeTimeout(ctx, device); err != nil {
		return nil, err
	}
	cancelled := make(chan struct{})
	stopCancel := context.AfterFunc(ctx, func() {
		C.fido_dev_cancel(device)
		close(cancelled)
	})
	defer func() {
		if !stopCancel() {
			<-cancelled
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	code := C.fido_dev_get_assert(device, assertion, nativePIN)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("security-key authentication interrupted: %w", err)
	}
	if code != C.FIDO_OK {
		return nil, nativeError("sign Apple challenge", code)
	}
	if C.fido_assert_count(assertion) == 0 {
		return nil, fmt.Errorf("security key returned no assertion")
	}
	return &Assertion{
		CredentialID:      C.GoBytes(unsafe.Pointer(C.fido_assert_id_ptr(assertion, 0)), C.int(C.fido_assert_id_len(assertion, 0))),
		AuthenticatorData: C.GoBytes(unsafe.Pointer(C.fido_assert_authdata_raw_ptr(assertion, 0)), C.int(C.fido_assert_authdata_raw_len(assertion, 0))),
		Signature:         C.GoBytes(unsafe.Pointer(C.fido_assert_sig_ptr(assertion, 0)), C.int(C.fido_assert_sig_len(assertion, 0))),
		UserID:            C.GoBytes(unsafe.Pointer(C.fido_assert_user_id_ptr(assertion, 0)), C.int(C.fido_assert_user_id_len(assertion, 0))),
	}, nil
}

func setNativeTimeout(ctx context.Context, device *C.fido_dev_t) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remaining := AssertionTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining = min(remaining, time.Until(deadline))
	}
	if remaining <= 0 {
		return context.DeadlineExceeded
	}
	if code := C.fido_dev_set_timeout(device, C.int(max(1, remaining.Milliseconds()))); code != C.FIDO_OK {
		return nativeError("set security-key timeout", code)
	}
	return nil
}

func nativeError(stage string, code C.int) error {
	switch code {
	case C.FIDO_ERR_PIN_REQUIRED:
		return fmt.Errorf("security key requires a PIN; set ASC_WEB_SECURITY_KEY_PIN and retry")
	case C.FIDO_ERR_PIN_INVALID:
		return fmt.Errorf("security-key PIN is incorrect; no automatic retry was attempted")
	case C.FIDO_ERR_PIN_BLOCKED, C.FIDO_ERR_PIN_AUTH_BLOCKED:
		return fmt.Errorf("security-key PIN is blocked; unlock the key using its manufacturer's tools before retrying")
	case C.FIDO_ERR_NO_CREDENTIALS:
		return fmt.Errorf("this security key has no matching credential for the Apple Account")
	case C.FIDO_ERR_KEEPALIVE_CANCEL:
		return fmt.Errorf("security-key authentication was cancelled")
	case C.FIDO_ERR_USER_ACTION_TIMEOUT, C.FIDO_ERR_ACTION_TIMEOUT:
		return fmt.Errorf("security-key authentication timed out; touch the key when retrying")
	default:
		return fmt.Errorf("%s: %s", stage, C.GoString(C.fido_strerr(code)))
	}
}
