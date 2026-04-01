package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (a *App) ListApps() (ListAppsResponse, error) {
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return ListAppsResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	out, err := a.runASCCombinedOutput(ctx, ascPath, "apps", "list", "--paginate", "--output", "json")
	if err != nil {
		return ListAppsResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	rawApps, err := parseAppsListOutput(out)
	if err != nil {
		return ListAppsResponse{Error: "Failed to parse apps list: " + err.Error()}, nil
	}

	apps := make([]AppInfo, len(rawApps))
	for i, raw := range rawApps {
		apps[i] = AppInfo{
			ID:       raw.ID,
			Name:     raw.Attributes.Name,
			BundleID: raw.Attributes.BundleID,
			SKU:      raw.Attributes.SKU,
		}
	}

	// Fetch subtitles concurrently (best-effort; failures are silently skipped)
	subtitleCtx, subtitleCancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer subtitleCancel()

	runWithConcurrency(boundedStudioConcurrency(len(apps)), len(apps), func(i int) {
		apps[i].Subtitle = a.fetchSubtitle(subtitleCtx, ascPath, apps[i].ID)
	})

	return ListAppsResponse{Apps: apps}, nil
}

func (a *App) GetAppDetail(appID string) (AppDetail, error) {
	if strings.TrimSpace(appID) == "" {
		return AppDetail{Error: "app ID is required"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return AppDetail{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	// Fetch app attrs and versions concurrently
	type attrsResult struct {
		name          string
		bundleID      string
		sku           string
		primaryLocale string
		err           error
	}
	type versionsResult struct {
		versions []AppVersion
		err      error
	}
	type subtitleRes struct {
		subtitle string
	}

	attrsCh := make(chan attrsResult, 1)
	versionsCh := make(chan versionsResult, 1)
	subtitleCh := make(chan subtitleRes, 1)

	go func() {
		out, err := a.runASCCombinedOutput(ctx, ascPath, "apps", "view", "--id", appID, "--output", "json")
		if err != nil {
			attrsCh <- attrsResult{err: err}
			return
		}
		var env struct {
			Data struct {
				Attributes struct {
					Name          string `json:"name"`
					BundleID      string `json:"bundleId"`
					SKU           string `json:"sku"`
					PrimaryLocale string `json:"primaryLocale"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if json.Unmarshal(out, &env) != nil {
			attrsCh <- attrsResult{err: errors.New("failed to parse app view")}
			return
		}
		attrs := env.Data.Attributes
		attrsCh <- attrsResult{name: attrs.Name, bundleID: attrs.BundleID, sku: attrs.SKU, primaryLocale: attrs.PrimaryLocale}
	}()

	go func() {
		out, err := a.runASCCombinedOutput(ctx, ascPath, "versions", "list", "--app", appID, "--paginate", "--output", "json")
		if err != nil {
			trimmed := strings.TrimSpace(string(out))
			if trimmed == "" {
				versionsCh <- versionsResult{err: err}
				return
			}
			versionsCh <- versionsResult{err: errors.New(trimmed)}
			return
		}
		type rawVersion struct {
			ID         string `json:"id"`
			Attributes struct {
				Platform        string `json:"platform"`
				VersionString   string `json:"versionString"`
				AppVersionState string `json:"appVersionState"`
				AppStoreState   string `json:"appStoreState"`
			} `json:"attributes"`
		}
		var env struct {
			Data []rawVersion `json:"data"`
		}
		if json.Unmarshal(out, &env) != nil {
			versionsCh <- versionsResult{err: errors.New("failed to parse versions list")}
			return
		}
		vs := make([]AppVersion, 0, len(env.Data))
		for _, rv := range env.Data {
			state := rv.Attributes.AppVersionState
			if state == "" {
				state = rv.Attributes.AppStoreState
			}
			vs = append(vs, AppVersion{
				ID:       rv.ID,
				Platform: rv.Attributes.Platform,
				Version:  rv.Attributes.VersionString,
				State:    state,
			})
		}
		versionsCh <- versionsResult{versions: vs}
	}()

	go func() {
		subtitleCh <- subtitleRes{subtitle: a.fetchSubtitle(ctx, ascPath, appID)}
	}()

	attrs := <-attrsCh
	vers := <-versionsCh
	sub := <-subtitleCh

	if attrs.err != nil {
		return AppDetail{Error: attrs.err.Error()}, nil
	}
	if vers.err != nil {
		return AppDetail{Error: vers.err.Error()}, nil
	}

	return AppDetail{
		ID:            appID,
		Name:          attrs.name,
		Subtitle:      sub.subtitle,
		BundleID:      attrs.bundleID,
		SKU:           attrs.sku,
		PrimaryLocale: attrs.primaryLocale,
		Versions:      vers.versions,
	}, nil
}

func (a *App) fetchSubtitle(ctx context.Context, ascPath, appID string) string {
	out, err := a.runASCCombinedOutput(ctx, ascPath, "localizations", "list",
		"--app", appID, "--type", "app-info", "--locale", "en-US", "--output", "json")
	if err != nil {
		return ""
	}

	type locAttrs struct {
		Subtitle string `json:"subtitle"`
	}
	type locItem struct {
		Attributes locAttrs `json:"attributes"`
	}
	var envelope struct {
		Data []locItem `json:"data"`
	}
	if json.Unmarshal(out, &envelope) != nil || len(envelope.Data) == 0 {
		return ""
	}
	return envelope.Data[0].Attributes.Subtitle
}
