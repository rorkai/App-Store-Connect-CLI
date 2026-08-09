package web

import (
	"context"
	"fmt"
	"os"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func SetResolveWebAuthCredentials(fn func(string) (shared.ResolvedAuthCredentials, error)) func() {
	prev := resolveWebAuthCredentialsFn
	resolveWebAuthCredentialsFn = fn
	return func() {
		resolveWebAuthCredentialsFn = prev
	}
}

func SetResolveWebSession(fn any) func() {
	switch fn.(type) {
	case func(context.Context, string, string, string) (*webcore.AuthSession, string, error):
	case func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error):
	case func(context.Context, string, string, string, ...string) (*webcore.AuthSession, string, error):
	default:
		panic(fmt.Sprintf("unsupported resolve session hook type %T", fn))
	}
	prev := resolveSessionFn
	prevNoPersist := resolveSessionWithoutPersistFn
	resolveSessionFn = fn
	resolveSessionWithoutPersistFn = fn
	return func() {
		resolveSessionFn = prev
		resolveSessionWithoutPersistFn = prevNoPersist
	}
}

func SetNewWebAuthClient(fn func(*webcore.AuthSession) *webcore.Client) func() {
	prev := newWebAuthClientFn
	newWebAuthClientFn = fn
	return func() {
		newWebAuthClientFn = prev
	}
}

func SetLookupWebAuthKey(fn func(context.Context, *webcore.Client, string) (*webcore.APIKeyRoleLookup, error)) func() {
	prev := lookupWebAuthKeyFn
	lookupWebAuthKeyFn = fn
	return func() {
		lookupWebAuthKeyFn = prev
	}
}

func SetSyncAppClipBundleIDCapability(fn func(context.Context, *webcore.Client, webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error)) func() {
	prev := syncAppClipBundleIDCapabilityFn
	syncAppClipBundleIDCapabilityFn = fn
	return func() {
		syncAppClipBundleIDCapabilityFn = prev
	}
}

func SetEnableDeveloperBundleIDCapability(fn func(context.Context, *webcore.Client, webcore.DeveloperBundleIDCapabilityEnableRequest) (*webcore.DeveloperBundleIDCapabilityEnableResult, error)) func() {
	prev := enableDeveloperBundleIDCapabilityFn
	enableDeveloperBundleIDCapabilityFn = fn
	return func() {
		enableDeveloperBundleIDCapabilityFn = prev
	}
}

// DisableControllingTTYForTesting prevents tests from opening the process's
// controlling terminal. The returned function restores the previous behavior.
func DisableControllingTTYForTesting() func() {
	previous := openTTYFn
	openTTYFn = func() (*os.File, error) {
		return nil, os.ErrNotExist
	}
	return func() {
		openTTYFn = previous
	}
}
