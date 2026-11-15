package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestValidateWebAppDeleteAvailabilityNamesTerritories(t *testing.T) {
	err := validateWebAppDeleteAvailability("1234567890", &webcore.AppAvailability{
		AvailableTerritories: []string{"USA", "GBR"},
	})
	if err == nil {
		t.Fatal("expected territory blocker")
	}
	if !strings.Contains(err.Error(), "still available in territories USA, GBR") {
		t.Fatalf("expected named territories, got %v", err)
	}
}

func TestValidateWebAppDeleteAvailabilityNamesNewTerritories(t *testing.T) {
	err := validateWebAppDeleteAvailability("1234567890", &webcore.AppAvailability{
		AvailableInNewTerritories:      true,
		AvailableTerritoriesLoaded:     true,
		AvailableInNewTerritoriesKnown: true,
	})
	if err == nil {
		t.Fatal("expected new-territory blocker")
	}
	if !strings.Contains(err.Error(), "still available in new territories") {
		t.Fatalf("expected new-territory message, got %v", err)
	}
}

func TestValidateWebAppDeleteAvailabilityFailsWhenTerritoriesUnloaded(t *testing.T) {
	err := validateWebAppDeleteAvailability("1234567890", &webcore.AppAvailability{
		ID: "avail-1",
	})
	if err == nil {
		t.Fatal("expected missing territory linkage blocker")
	}
	if !strings.Contains(err.Error(), "could not confirm") || !strings.Contains(err.Error(), "availableTerritories") {
		t.Fatalf("expected stderr to name missing territory linkage, got %v", err)
	}
}

func TestValidateWebAppDeleteAvailabilityFailsWhenNewTerritoriesUnknown(t *testing.T) {
	err := validateWebAppDeleteAvailability("1234567890", &webcore.AppAvailability{
		ID:                         "avail-1",
		AvailableTerritoriesLoaded: true,
	})
	if err == nil {
		t.Fatal("expected unknown new-territory blocker")
	}
	if !strings.Contains(err.Error(), "could not confirm") || !strings.Contains(err.Error(), "availableInNewTerritories") {
		t.Fatalf("expected stderr to name missing new-territory setting, got %v", err)
	}
}

func TestValidateWebAppDeleteRemovalStateFailsWhenReviewStatusUnknown(t *testing.T) {
	err := validateWebAppDeleteRemovalState(&webcore.AppRemovalState{
		ID:                        "1234567890",
		Marketplace:               "APP_STORE",
		DisplayableVersionsLoaded: true,
	})
	if err == nil {
		t.Fatal("expected unknown review-status blocker")
	}
	if !strings.Contains(err.Error(), "could not confirm") || !strings.Contains(err.Error(), "appStoreLegacyStatus") {
		t.Fatalf("expected stderr to name missing app-level review status, got %v", err)
	}
}

func TestValidateWebAppDeleteRemovalStateFailsWhenVersionsUnloaded(t *testing.T) {
	err := validateWebAppDeleteRemovalState(&webcore.AppRemovalState{
		ID:                   "1234567890",
		AppStoreLegacyStatus: "PREPARE_FOR_SUBMISSION",
		Marketplace:          "APP_STORE",
	})
	if err == nil {
		t.Fatal("expected missing displayableVersions blocker")
	}
	if !strings.Contains(err.Error(), "could not confirm") || !strings.Contains(err.Error(), "displayableVersions") {
		t.Fatalf("expected stderr to name missing version linkage, got %v", err)
	}
}

func TestValidateWebAppDeleteRemovalStateBlocksReview(t *testing.T) {
	err := validateWebAppDeleteRemovalState(&webcore.AppRemovalState{
		ID:                   "1234567890",
		AppStoreLegacyStatus: "WAITING_FOR_REVIEW",
	})
	if err == nil {
		t.Fatal("expected review-state blocker")
	}
	if !strings.Contains(err.Error(), "WAITING_FOR_REVIEW") {
		t.Fatalf("expected review state named, got %v", err)
	}
}

func TestValidateWebAppDeleteRemovalStateBlocksMarketplace(t *testing.T) {
	err := validateWebAppDeleteRemovalState(&webcore.AppRemovalState{
		ID:                        "1234567890",
		AppStoreLegacyStatus:      "PREPARE_FOR_SUBMISSION",
		Marketplace:               "ALT_MARKETPLACE",
		DisplayableVersionsLoaded: true,
	})
	if err == nil {
		t.Fatal("expected marketplace blocker")
	}
	if !strings.Contains(err.Error(), "ALT_MARKETPLACE") {
		t.Fatalf("expected marketplace named, got %v", err)
	}
}

func TestWebAppsDeleteDryRunSucceedsWhenRemovedFromSaleEverywhere(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origGetRemovalState := getWebAppRemovalStateFn
	origGetAvailability := getWebAppAvailabilityFn
	origDeleteWebApp := deleteWebAppFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		getWebAppRemovalStateFn = origGetRemovalState
		getWebAppAvailabilityFn = origGetAvailability
		deleteWebAppFn = origDeleteWebApp
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	getWebAppRemovalStateFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppRemovalState, error) {
		return &webcore.AppRemovalState{
			ID:                        appID,
			Name:                      "Throwaway",
			BundleID:                  "com.example.throwaway",
			RemovedKnown:              true,
			AppStoreLegacyStatus:      "PREPARE_FOR_SUBMISSION",
			Marketplace:               "APP_STORE",
			DisplayableVersionsLoaded: true,
		}, nil
	}
	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return &webcore.AppAvailability{
			ID:                             appID,
			AvailableTerritories:           nil,
			AvailableTerritoriesLoaded:     true,
			AvailableInNewTerritories:      false,
			AvailableInNewTerritoriesKnown: true,
		}, nil
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		t.Fatal("did not expect delete during dry-run")
		return nil, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "1234567890", "--dry-run", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr != nil {
		t.Fatalf("expected dry-run success, got %v", execErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.WebAppDeleteResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON dry-run receipt, got %q: %v", stdout, err)
	}
	if result.AppID != "1234567890" || result.Name != "Throwaway" || result.BundleID != "com.example.throwaway" {
		t.Fatalf("unexpected dry-run receipt: %#v", result)
	}
	if result.Removed {
		t.Fatal("expected removed=false on a dry-run of an unremoved app")
	}
	if !result.DryRun {
		t.Fatal("expected dryRun=true on the receipt")
	}
}

func TestWebAppsDeleteDryRunFailsWhenTerritoryStillAvailable(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origGetRemovalState := getWebAppRemovalStateFn
	origGetAvailability := getWebAppAvailabilityFn
	origDeleteWebApp := deleteWebAppFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		getWebAppRemovalStateFn = origGetRemovalState
		getWebAppAvailabilityFn = origGetAvailability
		deleteWebAppFn = origDeleteWebApp
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	getWebAppRemovalStateFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppRemovalState, error) {
		return &webcore.AppRemovalState{
			ID:                        appID,
			Name:                      "Throwaway",
			BundleID:                  "com.example.throwaway",
			RemovedKnown:              true,
			AppStoreLegacyStatus:      "PREPARE_FOR_SUBMISSION",
			Marketplace:               "APP_STORE",
			DisplayableVersionsLoaded: true,
		}, nil
	}
	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return &webcore.AppAvailability{
			ID:                             appID,
			AvailableTerritories:           []string{"USA"},
			AvailableTerritoriesLoaded:     true,
			AvailableInNewTerritoriesKnown: true,
		}, nil
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		t.Fatal("did not expect delete after territory preflight failure")
		return nil, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "1234567890", "--dry-run"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if execErr == nil {
		t.Fatal("expected territory availability error")
	}
	if !strings.Contains(execErr.Error(), "still available in territories USA") {
		t.Fatalf("expected named territory blocker, got %v", execErr)
	}
}

func TestWebAppsDeleteDryRunFailsWhenTerritoryAvailabilitiesUnreadable(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origGetRemovalState := getWebAppRemovalStateFn
	origGetAvailability := getWebAppAvailabilityFn
	origDeleteWebApp := deleteWebAppFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		getWebAppRemovalStateFn = origGetRemovalState
		getWebAppAvailabilityFn = origGetAvailability
		deleteWebAppFn = origDeleteWebApp
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	getWebAppRemovalStateFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppRemovalState, error) {
		return &webcore.AppRemovalState{
			ID:                        appID,
			Name:                      "Throwaway",
			BundleID:                  "com.example.throwaway",
			RemovedKnown:              true,
			AppStoreLegacyStatus:      "PREPARE_FOR_SUBMISSION",
			Marketplace:               "APP_STORE",
			DisplayableVersionsLoaded: true,
		}, nil
	}
	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return nil, fmt.Errorf("could not read territoryAvailabilities for app availability %q: territory availability %q omitted or mistyped the available attribute", appID, "ta-usa")
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		t.Fatal("did not expect delete after unreadable territoryAvailabilities")
		return nil, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "1234567890", "--dry-run"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if execErr == nil {
		t.Fatal("expected unreadable territoryAvailabilities to fail closed")
	}
	if !strings.Contains(execErr.Error(), "could not read availability") || !strings.Contains(execErr.Error(), "territoryAvailabilities") {
		t.Fatalf("expected availability read to name territoryAvailabilities, got %v", execErr)
	}
}

func TestWebAppsDeleteReviewStateStopsBeforeDelete(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origGetRemovalState := getWebAppRemovalStateFn
	origDeleteWebApp := deleteWebAppFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		getWebAppRemovalStateFn = origGetRemovalState
		deleteWebAppFn = origDeleteWebApp
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	getWebAppRemovalStateFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppRemovalState, error) {
		return &webcore.AppRemovalState{
			ID:                   appID,
			Name:                 "Throwaway",
			BundleID:             "com.example.throwaway",
			RemovedKnown:         true,
			AppStoreLegacyStatus: "IN_REVIEW",
			Marketplace:          "APP_STORE",
		}, nil
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		t.Fatal("did not expect delete after review-state preflight failure")
		return nil, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "1234567890", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err = cmd.Exec(context.Background(), nil)
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err == nil {
		t.Fatal("expected review-state error")
	}
	if !strings.Contains(err.Error(), "IN_REVIEW") {
		t.Fatalf("expected IN_REVIEW named, got %v", err)
	}
}
