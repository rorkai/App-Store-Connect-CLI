package web

import (
	"context"
	"fmt"

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
	resolveSessionFn = fn
	return func() {
		resolveSessionFn = prev
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
