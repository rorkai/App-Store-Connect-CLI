package web

import (
	"context"
	"fmt"
	"os"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

func SetUnassignDeveloperAppGroup(fn func(context.Context, *webcore.Client, webcore.DeveloperAppGroupUnassignRequest) (*asc.WebAppGroupUnassignResult, error)) func() {
	prev := unassignDeveloperAppGroupFn
	unassignDeveloperAppGroupFn = fn
	return func() {
		unassignDeveloperAppGroupFn = prev
	}
}

func SetSetDeveloperAppGroups(fn func(context.Context, *webcore.Client, webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error)) func() {
	prev := setDeveloperAppGroupsFn
	setDeveloperAppGroupsFn = fn
	return func() {
		setDeveloperAppGroupsFn = prev
	}
}

func SetDeleteDeveloperAppGroup(fn func(context.Context, *webcore.Client, webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error)) func() {
	prev := deleteDeveloperAppGroupFn
	deleteDeveloperAppGroupFn = fn
	return func() {
		deleteDeveloperAppGroupFn = prev
	}
}

func SetEnableDeveloperBundleIDCapability(fn func(context.Context, *webcore.Client, webcore.DeveloperBundleIDCapabilityEnableRequest) (*webcore.DeveloperBundleIDCapabilityEnableResult, error)) func() {
	prev := enableDeveloperBundleIDCapabilityFn
	enableDeveloperBundleIDCapabilityFn = fn
	return func() {
		enableDeveloperBundleIDCapabilityFn = prev
	}
}

func SetDisableDeveloperBundleIDCapability(fn func(context.Context, *webcore.Client, webcore.DeveloperBundleIDCapabilityDisableRequest) (*asc.DeveloperBundleIDCapabilityDisableResult, error)) func() {
	prev := disableDeveloperBundleIDCapabilityFn
	disableDeveloperBundleIDCapabilityFn = fn
	return func() {
		disableDeveloperBundleIDCapabilityFn = prev
	}
}

func SetListDeveloperBundleIDs(fn func(context.Context, *webcore.Client) (*webcore.DeveloperBundleIDsListResult, error)) func() {
	prev := listDeveloperBundleIDsFn
	listDeveloperBundleIDsFn = fn
	return func() {
		listDeveloperBundleIDsFn = prev
	}
}

func SetGetDeveloperBundleID(fn func(context.Context, *webcore.Client, string) (*webcore.DeveloperBundleIDGetResult, error)) func() {
	prev := getDeveloperBundleIDFn
	getDeveloperBundleIDFn = fn
	return func() {
		getDeveloperBundleIDFn = prev
	}
}

func SetPersistWebSession(fn func(*webcore.AuthSession) error) func() {
	prev := persistWebSessionFn
	persistWebSessionFn = fn
	return func() {
		persistWebSessionFn = prev
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

func SetSetDeveloperServiceIDDomains(fn func(context.Context, *webcore.Client, webcore.DeveloperServiceIDDomainsSetRequest) (*asc.WebServiceIDMutationResult, error)) func() {
	previous := setDeveloperServiceIDDomainsFn
	setDeveloperServiceIDDomainsFn = fn
	return func() { setDeveloperServiceIDDomainsFn = previous }
}
