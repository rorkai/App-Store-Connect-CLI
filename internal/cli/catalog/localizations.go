package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type localization struct {
	Locale        string  `json:"locale"`
	Name          string  `json:"name"`
	Description   *string `json:"description,omitempty"`
	CustomAppName *string `json:"customAppName,omitempty"`
}

type catalogVersion struct {
	Version int    `json:"version"`
	State   string `json:"state"`
}

func validateLocalizations(kind string, values []localization) error {
	seen := map[string]bool{}
	for i := range values {
		value := &values[i]
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(value.Locale)
		if err != nil {
			return err
		}
		value.Locale = locale
		if seen[locale] {
			return fmt.Errorf("duplicate localization %s", locale)
		}
		seen[locale] = true
		if strings.TrimSpace(value.Name) == "" {
			return fmt.Errorf("localization %s name is required", locale)
		}
		if kind != "group" && (utf8.RuneCountInString(value.Name) < 2 || utf8.RuneCountInString(value.Name) > 30) {
			return fmt.Errorf("localization %s name must contain 2 to 30 characters", locale)
		}
		if kind == "group" && value.Description != nil {
			return fmt.Errorf("group localizations do not support description")
		}
		if kind != "group" && value.CustomAppName != nil {
			return fmt.Errorf("product localizations do not support customAppName")
		}
		descriptionLimit := 55
		if kind == "inAppPurchase" {
			descriptionLimit = 45
		}
		if value.Description != nil && utf8.RuneCountInString(*value.Description) > descriptionLimit {
			return fmt.Errorf("localization %s description exceeds %d characters", locale, descriptionLimit)
		}
	}
	return nil
}

func readResources[T any](ctx context.Context, c *asc.Client, path, kind string) ([]asc.Resource[T], error) {
	data, err := c.RawPaginatedGET(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var response asc.Response[T]
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return nil, fmt.Errorf("%s data must be an array", kind)
	}
	for _, r := range response.Data {
		if r.ID == "" || string(r.Type) != kind {
			return nil, fmt.Errorf("invalid %s resource identity", kind)
		}
	}
	return response.Data, nil
}

func localizationPaths(kind string) (parent, version, localized string) {
	switch kind {
	case "group":
		return "subscriptionGroups", "subscriptionGroupVersions", "subscriptionGroupLocalizations"
	case "subscription":
		return "subscriptions", "subscriptionVersions", "subscriptionLocalizations"
	default:
		return "inAppPurchases", "inAppPurchaseVersions", "inAppPurchaseLocalizations"
	}
}

func localizedMatch(want localization, got localization) bool {
	return want.Name == got.Name && (want.Description == nil || (got.Description != nil && *want.Description == *got.Description) || (got.Description == nil && *want.Description == "")) && (want.CustomAppName == nil || (got.CustomAppName != nil && *want.CustomAppName == *got.CustomAppName) || (got.CustomAppName == nil && *want.CustomAppName == ""))
}

func localizationIndex(values []asc.Resource[localization]) (map[string]asc.Resource[localization], error) {
	result := map[string]asc.Resource[localization]{}
	for _, r := range values {
		if r.Attributes.Locale == "" {
			return nil, fmt.Errorf("localization is missing locale")
		}
		if _, ok := result[r.Attributes.Locale]; ok {
			return nil, fmt.Errorf("ambiguous locale %s", r.Attributes.Locale)
		}
		result[r.Attributes.Locale] = r
	}
	return result, nil
}

func planLocalizations(ctx context.Context, c *asc.Client, kind, key string, id *string, values []localization) ([]operation, error) {
	if len(values) == 0 {
		return nil, nil
	}
	parent, versionKind, localizedKind := localizationPaths(kind)
	versionID := ""
	var existing []asc.Resource[localization]
	if *id != "" {
		prefix := "/v1/"
		if kind == "inAppPurchase" {
			prefix = "/v2/"
		}
		versions, err := readResources[catalogVersion](ctx, c, prefix+parent+"/"+url.PathEscape(*id)+"/versions?limit=200", versionKind)
		if err != nil {
			return nil, err
		}
		var approved *asc.Resource[catalogVersion]
		for _, v := range versions {
			if slices.Contains([]string{"PREPARE_FOR_SUBMISSION", "REJECTED", "DEVELOPER_REJECTED"}, v.Attributes.State) {
				if versionID != "" {
					return nil, fmt.Errorf("%s %s has multiple editable versions", kind, key)
				}
				versionID = v.ID
			} else if v.Attributes.State == "APPROVED" {
				if approved == nil || approved.Attributes.Version < v.Attributes.Version {
					copy := v
					approved = &copy
				}
			} else if !slices.Contains([]string{"REPLACED_WITH_NEW_VERSION"}, v.Attributes.State) {
				return nil, fmt.Errorf("%s %s version %s is %s; wait for an editable version", kind, key, v.ID, v.Attributes.State)
			}
		}
		readID := versionID
		if readID == "" && approved != nil {
			readID = approved.ID
		}
		if readID != "" {
			existing, err = readResources[localization](ctx, c, "/v1/"+versionKind+"/"+url.PathEscape(readID)+"/localizations?limit=200", localizedKind)
			if err != nil {
				return nil, err
			}
			if versionID == "" {
				index, err := localizationIndex(existing)
				if err != nil {
					return nil, err
				}
				matched := true
				for _, v := range values {
					r, ok := index[v.Locale]
					if !ok || !localizedMatch(v, r.Attributes) {
						matched = false
					}
				}
				if matched {
					versionID = readID
				} else {
					existing = nil
				}
			}
		}
	}
	ops := []operation{}
	newVersion := versionID == ""
	if newVersion {
		op := operation{item: asc.CatalogItem{Kind: kind + "Version", Key: key, Action: "create", Status: "pending"}}
		op.write = func(ctx context.Context) (string, error) {
			switch kind {
			case "group":
				r, err := c.CreateSubscriptionGroupVersion(ctx, *id)
				if err != nil {
					return "", err
				}
				versionID = r.Data.ID
			case "subscription":
				r, err := c.CreateSubscriptionVersion(ctx, *id)
				if err != nil {
					return "", err
				}
				versionID = r.Data.ID
			default:
				r, err := c.CreateInAppPurchaseVersion(ctx, *id)
				if err != nil {
					return "", err
				}
				versionID = r.Data.ID
			}
			return versionID, nil
		}
		ops = append(ops, op)
	}
	index, err := localizationIndex(existing)
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		op := operation{item: asc.CatalogItem{Kind: kind + "Localization", Key: key + "/" + v.Locale, Action: "create", Status: "pending", Changes: map[string]any{"localization": v}}}
		if newVersion {
			op.item.Action = "reconcile"
			op.item.Deferred = true
		}
		old, ok := index[v.Locale]
		if ok {
			op.item.ID = old.ID
			op.item.Action = "update"
			if localizedMatch(v, old.Attributes) {
				op.item.Action = "noop"
				op.item.Changes = nil
			}
		}
		op.write = func(ctx context.Context) (string, error) {
			localID := old.ID
			if newVersion {
				rows, err := readResources[localization](ctx, c, "/v1/"+versionKind+"/"+url.PathEscape(versionID)+"/localizations?limit=200", localizedKind)
				if err != nil {
					return "", err
				}
				index, err := localizationIndex(rows)
				if err != nil {
					return "", err
				}
				if found, ok := index[v.Locale]; ok {
					localID = found.ID
					if localizedMatch(v, found.Attributes) {
						return localID, nil
					}
				}
			}
			return writeLocalization(ctx, c, kind, versionID, localID, v)
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func nullable(value *string) *asc.NullableString {
	if value == nil {
		return nil
	}
	return &asc.NullableString{Value: value}
}

func writeLocalization(ctx context.Context, c *asc.Client, kind, versionID, id string, v localization) (string, error) {
	switch kind {
	case "group":
		if id == "" {
			r, e := c.CreateSubscriptionGroupLocalizationV2(ctx, versionID, asc.SubscriptionGroupLocalizationV2CreateAttributes{Name: v.Name, Locale: v.Locale, CustomAppName: nullable(v.CustomAppName)})
			if e != nil {
				return "", e
			}
			return r.Data.ID, nil
		}
		r, e := c.UpdateSubscriptionGroupLocalizationV2(ctx, id, asc.SubscriptionGroupLocalizationV2UpdateAttributes{Name: nullable(&v.Name), CustomAppName: nullable(v.CustomAppName)})
		if e != nil {
			return "", e
		}
		return r.Data.ID, nil
	case "subscription":
		if id == "" {
			r, e := c.CreateSubscriptionLocalizationV2(ctx, versionID, asc.SubscriptionLocalizationV2CreateAttributes{Name: v.Name, Locale: v.Locale, Description: nullable(v.Description)})
			if e != nil {
				return "", e
			}
			return r.Data.ID, nil
		}
		r, e := c.UpdateSubscriptionLocalizationV2(ctx, id, asc.SubscriptionLocalizationV2UpdateAttributes{Name: nullable(&v.Name), Description: nullable(v.Description)})
		if e != nil {
			return "", e
		}
		return r.Data.ID, nil
	default:
		if id == "" {
			r, e := c.CreateInAppPurchaseLocalizationV2(ctx, versionID, asc.InAppPurchaseLocalizationV2CreateAttributes{Name: v.Name, Locale: v.Locale, Description: nullable(v.Description)})
			if e != nil {
				return "", e
			}
			return r.Data.ID, nil
		}
		r, e := c.UpdateInAppPurchaseLocalizationV2(ctx, id, asc.InAppPurchaseLocalizationUpdateAttributes{Name: nullable(&v.Name), Description: nullable(v.Description)})
		if e != nil {
			return "", e
		}
		return r.Data.ID, nil
	}
}

func planLocalizationItems(ctx context.Context, c *asc.Client, m manifest, groups, products map[string]*string) ([]operation, error) {
	ops := []operation{}
	for _, g := range m.Groups {
		rows, err := planLocalizations(ctx, c, "group", g.ReferenceName, groups[g.ReferenceName], g.Localizations)
		if err != nil {
			return nil, err
		}
		ops = append(ops, rows...)
	}
	for _, p := range m.InAppPurchases {
		rows, err := planLocalizations(ctx, c, "inAppPurchase", p.ProductID, products[p.ProductID], p.Localizations)
		if err != nil {
			return nil, err
		}
		ops = append(ops, rows...)
	}
	for _, s := range m.Subscriptions {
		rows, err := planLocalizations(ctx, c, "subscription", s.ProductID, products[s.ProductID], s.Localizations)
		if err != nil {
			return nil, err
		}
		ops = append(ops, rows...)
	}
	return ops, nil
}
