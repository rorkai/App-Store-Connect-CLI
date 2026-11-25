package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type manifest struct {
	Groups         []group        `json:"groups"`
	Subscriptions  []subscription `json:"subscriptions"`
	InAppPurchases []purchase     `json:"inAppPurchases"`
}

type group struct {
	ID            string         `json:"id,omitempty"`
	ReferenceName string         `json:"referenceName"`
	Localizations []localization `json:"localizations,omitempty"`
}

type purchase struct {
	ProductID     string         `json:"productId"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	ReviewNote    *string        `json:"reviewNote,omitempty"`
	Availability  *availability  `json:"availability,omitempty"`
	Pricing       *pricing       `json:"pricing,omitempty"`
	Localizations []localization `json:"localizations,omitempty"`
}

type operation struct {
	item        asc.CatalogItem
	write       func(context.Context) (string, error)
	destructive bool
}

func NewCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name: "catalog", ShortUsage: "asc catalog <subcommand> [flags]", ShortHelp: "Plan and reconcile a product catalog from a JSON file.",
		FlagSet: d.NewFlagSet("catalog", flag.ExitOnError), UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newCommand(d, "plan"), newCommand(d, "apply")},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newCommand(d *shared.Deps, mode string) *ffcli.Command {
	fs := d.NewFlagSet(mode, flag.ExitOnError)
	app := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	file := fs.String("file", "", "Catalog JSON file")
	confirm := new(bool)
	if mode == "apply" {
		confirm = fs.Bool("confirm", false, "Confirm removal of existing availability or replacement of offers")
	}
	out := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: mode, ShortUsage: "asc catalog " + mode + " --app APP_ID --file catalog.json [flags]",
		ShortHelp: map[string]string{"plan": "Show catalog changes without writing to Apple.", "apply": "Apply catalog changes with per-item receipts."}[mode],
		LongHelp: `Validate the complete JSON manifest before writing. Compare existing resources
with Apple and print each change. Price points and version localizations for new
products require discovery after creation; plan marks those items deferred.

Items omitted from the manifest are untouched. Products match by productId.
Groups match by explicit id or unique referenceName; use id to rename a group.
Writes run in order: groups, products, localizations, prices, offers, availability.
Re-running apply reads Apple again and skips matching items. On failure, apply
prints completed, failed, and pending receipts and stops. It does not roll back.

Example catalog.json:
  {
    "groups": [{"referenceName":"Pro","localizations":[{"locale":"en-US","name":"Pro"}]}],
    "subscriptions": [{
      "group":"Pro","productId":"com.example.monthly","name":"Monthly",
      "subscriptionPeriod":"ONE_MONTH",
      "localizations":[{"locale":"en-US","name":"Monthly","description":"Premium features"}],
      "pricing":{"baseTerritory":"USA","customerPrice":"9.99","equalize":true},
      "introductoryOffers":[{"territory":"USA","offerMode":"FREE_TRIAL","duration":"ONE_WEEK","numberOfPeriods":1}],
      "availability":{"territories":["USA","CAN"],"availableInNewTerritories":true}
    }],
    "inAppPurchases": [{"productId":"com.example.lifetime","name":"Lifetime","type":"NON_CONSUMABLE"}]
  }

IAPs also support localizations, pricing, availability, and reviewNote.
Subscriptions additionally support reviewNote and groupLevel. Group localizations
support customAppName; product localizations support description.

Pricing accepts startDate (YYYY-MM-DD). Use equalize:true for Apple's equivalents,
or territories:[{"territory":"CAN","customerPrice":"12.99"}] for overrides.
IAP territories without an override remain automatically equalized by Apple.
Existing schedules with future, bounded, outside-territory, preserved or
alternate-plan prices are not replaced when their scope cannot be preserved.
Use the low-level pricing commands for those schedule edits.
Paid offers require customerPrice. Offers accept startDate and endDate.
Replacing an offer or removing availability requires apply --confirm. An explicit
empty introductoryOffers array removes all existing introductory offers.

Unknown fields, duplicate keys, null values, and unsupported fields are errors.
No product or group is deleted.`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return d.UsageError("catalog " + mode + " does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*out.Output, *out.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			appID := d.ResolveAppID(*app)
			if appID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if strings.TrimSpace(*file) == "" {
				return d.UsageError("--file is required")
			}
			today := shared.PricingDate(d.Now()).Format(time.DateOnly)
			m, err := readManifest(*file, today)
			if err != nil {
				return d.UsageError("catalog: " + err.Error())
			}
			client, err := d.Client()
			if err != nil {
				return err
			}
			ctx, cancel := d.Timeout(ctx)
			defer cancel()
			ctx = asc.WithNoRedirectReads(ctx)
			ops, err := plan(ctx, client, appID, m, today)
			if err != nil {
				return fmt.Errorf("catalog preflight: %w", err)
			}
			if mode == "apply" && !*confirm {
				for _, op := range ops {
					if op.destructive {
						return d.UsageError("--confirm is required for " + op.item.Kind + " " + op.item.Key)
					}
				}
			}
			result := &asc.CatalogResult{AppID: appID, Status: "planned", Items: make([]asc.CatalogItem, len(ops))}
			for i := range ops {
				result.Items[i] = ops[i].item
			}
			if mode == "apply" {
				result.Status = "applied"
				for i, op := range ops {
					item := &result.Items[i]
					if item.Action == "noop" {
						item.Status = "unchanged"
						continue
					}
					id, writeErr := op.write(ctx)
					if writeErr == nil && strings.TrimSpace(id) == "" {
						writeErr = fmt.Errorf("response from Apple did not contain a resource ID; rerun to reconcile")
					}
					if writeErr != nil {
						item.Status = "failed"
						item.Error = writeErr.Error()
						result.Status = "partial"
						if err := d.PrintOutput(result, *out.Output, *out.Pretty); err != nil {
							return errors.Join(writeErr, err)
						}
						return shared.NewReportedError(fmt.Errorf("catalog %s %s: %w", item.Kind, item.Key, writeErr))
					}
					item.ID = id
					item.Status = "applied"
				}
			}
			return d.PrintOutput(result, *out.Output, *out.Pretty)
		},
	}
}

func readManifest(path, today string) (manifest, error) {
	var m manifest
	f, err := rootfs.OpenFile(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if err != nil {
		return m, err
	}
	if len(data) > 4<<20 {
		return m, fmt.Errorf("manifest exceeds 4 MiB")
	}
	if err := validateJSON(json.NewDecoder(bytes.NewReader(data)), reflect.TypeFor[manifest]()); err != nil {
		return m, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return m, fmt.Errorf("manifest must contain one JSON object")
	}
	if len(m.Groups)+len(m.InAppPurchases)+len(m.Subscriptions) == 0 {
		return m, fmt.Errorf("manifest must contain groups or inAppPurchases")
	}
	groups, ids, products := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, g := range m.Groups {
		if err := validateLocalizations("group", g.Localizations); err != nil {
			return m, err
		}
		if strings.TrimSpace(g.ReferenceName) == "" || utf8.RuneCountInString(g.ReferenceName) > 64 {
			return m, fmt.Errorf("groups[%d].referenceName must contain 1 to 64 characters", i)
		}
		if groups[g.ReferenceName] || (g.ID != "" && ids[g.ID]) {
			return m, fmt.Errorf("duplicate group %q", g.ReferenceName)
		}
		groups[g.ReferenceName] = true
		ids[g.ID] = true
	}
	for i, p := range m.InAppPurchases {
		if err := validateLocalizations("inAppPurchase", p.Localizations); err != nil {
			return m, err
		}
		if err := validatePricing(p.Pricing); err != nil {
			return m, fmt.Errorf("inAppPurchases[%d]: %w", i, err)
		}
		if err := validateAvailability(p.Availability); err != nil {
			return m, fmt.Errorf("inAppPurchases[%d]: %w", i, err)
		}
		if !validProductID(p.ProductID) {
			return m, fmt.Errorf("inAppPurchases[%d].productId is invalid", i)
		}
		if products[p.ProductID] {
			return m, fmt.Errorf("duplicate productId %q", p.ProductID)
		}
		products[p.ProductID] = true
		if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 64 {
			return m, fmt.Errorf("inAppPurchases[%d].name must contain 1 to 64 characters", i)
		}
		if !slices.Contains(asc.ValidIAPTypes, p.Type) {
			return m, fmt.Errorf("inAppPurchases[%d].type must be one of %s", i, strings.Join(asc.ValidIAPTypes, ", "))
		}
		if p.ReviewNote != nil && utf8.RuneCountInString(*p.ReviewNote) > 4000 {
			return m, fmt.Errorf("inAppPurchases[%d].reviewNote exceeds 4000 characters", i)
		}
	}
	if err := validateSubscriptions(m, groups, products, today); err != nil {
		return m, err
	}
	return m, nil
}

func validProductID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	for _, char := range id {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '.' && char != '_' {
			return false
		}
	}
	return true
}

func plan(ctx context.Context, c *asc.Client, appID string, m manifest, today string) ([]operation, error) {
	ops := []operation{}
	groupRefs := map[string]*string{}
	resolvedGroups := map[string]bool{}
	productRefs := map[string]*string{}
	var remoteGroups []asc.Resource[asc.SubscriptionGroupAttributes]
	if len(m.Groups)+len(m.Subscriptions)+len(m.InAppPurchases) > 0 {
		var err error
		remoteGroups, err = collect(ctx, "/v1/apps/"+appID+"/subscriptionGroups", func(ctx context.Context, next string) (*asc.SubscriptionGroupsResponse, error) {
			return c.GetSubscriptionGroups(ctx, appID, asc.Limit[asc.SubscriptionGroupsOption](200), asc.NextURL[asc.SubscriptionGroupsOption](next))
		})
		if err != nil {
			return nil, err
		}
		for _, r := range remoteGroups {
			if r.Type != asc.ResourceTypeSubscriptionGroups || r.ID == "" || r.Attributes.ReferenceName == "" {
				return nil, fmt.Errorf("subscription group response is missing identity")
			}
		}
		for _, g := range m.Groups {
			var matches []asc.Resource[asc.SubscriptionGroupAttributes]
			for _, r := range remoteGroups {
				if g.ID != "" && r.ID != g.ID && r.Attributes.ReferenceName == g.ReferenceName {
					return nil, fmt.Errorf("group name %q already belongs to %s", g.ReferenceName, r.ID)
				}
				if (g.ID != "" && r.ID == g.ID) || (g.ID == "" && r.Attributes.ReferenceName == g.ReferenceName) {
					matches = append(matches, r)
				}
			}
			if len(matches) > 1 {
				return nil, fmt.Errorf("ambiguous group %q; specify id", g.ReferenceName)
			}
			if g.ID != "" && len(matches) == 0 {
				return nil, fmt.Errorf("group %s does not belong to app %s", g.ID, appID)
			}
			id := new(string)
			groupRefs[g.ReferenceName] = id
			op := operation{item: asc.CatalogItem{Kind: "group", Key: g.ReferenceName, Action: "create", Status: "pending", Changes: map[string]any{"referenceName": g.ReferenceName}}}
			op.write = func(ctx context.Context) (string, error) {
				r, e := c.CreateSubscriptionGroup(ctx, appID, asc.SubscriptionGroupCreateAttributes{ReferenceName: g.ReferenceName})
				if e != nil {
					return "", e
				}
				*id = r.Data.ID
				return r.Data.ID, nil
			}
			if len(matches) == 1 {
				r := matches[0]
				if resolvedGroups[r.ID] {
					return nil, fmt.Errorf("group %s is targeted more than once", r.ID)
				}
				resolvedGroups[r.ID] = true
				*id = r.ID
				op.item.ID = r.ID
				op.item.Action = "update"
				op.write = func(ctx context.Context) (string, error) {
					r, e := c.UpdateSubscriptionGroup(ctx, r.ID, asc.SubscriptionGroupUpdateAttributes{ReferenceName: &g.ReferenceName})
					if e != nil {
						return "", e
					}
					return r.Data.ID, nil
				}
				if r.Attributes.ReferenceName == g.ReferenceName {
					op.item.Action = "noop"
					op.item.Changes = nil
				}
			}
			ops = append(ops, op)
		}
	}
	if len(m.InAppPurchases)+len(m.Subscriptions) > 0 {
		all, err := collect(ctx, "/v1/apps/"+appID+"/inAppPurchasesV2", func(ctx context.Context, next string) (*asc.InAppPurchasesV2Response, error) {
			return c.GetInAppPurchasesV2Strict(ctx, appID, asc.Limit[asc.IAPOption](200), asc.NextURL[asc.IAPOption](next))
		})
		if err != nil {
			return nil, err
		}
		for _, r := range all {
			if r.ID == "" || r.Attributes.ProductID == "" {
				return nil, fmt.Errorf("in-app purchase response is missing identity")
			}
			for _, s := range m.Subscriptions {
				if r.Attributes.ProductID == s.ProductID {
					return nil, fmt.Errorf("productId %q already belongs to an in-app purchase", s.ProductID)
				}
			}
		}
		for _, p := range m.InAppPurchases {
			var matches []asc.Resource[asc.InAppPurchaseV2Attributes]
			for _, r := range all {
				if r.Attributes.ProductID == p.ProductID {
					matches = append(matches, r)
				}
			}
			if len(matches) > 1 {
				return nil, fmt.Errorf("ambiguous productId %q", p.ProductID)
			}
			id := new(string)
			productRefs[p.ProductID] = id
			op := operation{item: asc.CatalogItem{Kind: "inAppPurchase", Key: p.ProductID, Action: "create", Status: "pending", Changes: map[string]any{"name": p.Name, "type": p.Type}}}
			attrs := asc.InAppPurchaseV2CreateAttributes{Name: p.Name, ProductID: p.ProductID, InAppPurchaseType: p.Type}
			if p.ReviewNote != nil {
				attrs.ReviewNote = *p.ReviewNote
				op.item.Changes["reviewNote"] = *p.ReviewNote
			}
			op.write = func(ctx context.Context) (string, error) {
				r, e := c.CreateInAppPurchaseV2(ctx, appID, attrs)
				if e != nil {
					return "", e
				}
				*id = r.Data.ID
				return r.Data.ID, nil
			}
			if len(matches) == 1 {
				r := matches[0]
				*id = r.ID
				if r.Attributes.InAppPurchaseType != p.Type {
					return nil, fmt.Errorf("productId %q has immutable type %s, requested %s", p.ProductID, r.Attributes.InAppPurchaseType, p.Type)
				}
				op.item.ID = r.ID
				op.item.Action = "update"
				op.item.Changes = map[string]any{}
				update := asc.InAppPurchaseV2UpdateAttributes{}
				if r.Attributes.Name != p.Name {
					update.Name = &p.Name
					op.item.Changes["name"] = p.Name
				}
				if p.ReviewNote != nil && r.Attributes.ReviewNote != *p.ReviewNote {
					update.ReviewNote = p.ReviewNote
					op.item.Changes["reviewNote"] = *p.ReviewNote
				}
				op.write = func(ctx context.Context) (string, error) {
					r, e := c.UpdateInAppPurchaseV2(ctx, r.ID, update)
					if e != nil {
						return "", e
					}
					return r.Data.ID, nil
				}
				if len(op.item.Changes) == 0 {
					op.item.Action = "noop"
				}
			}
			ops = append(ops, op)
		}
	}
	subOps, err := planSubscriptions(ctx, c, m, groupRefs, remoteGroups, productRefs)
	if err != nil {
		return nil, err
	}
	ops = append(ops, subOps...)
	localizationOps, err := planLocalizationItems(ctx, c, m, groupRefs, productRefs)
	if err != nil {
		return nil, err
	}
	ops = append(ops, localizationOps...)
	pricingOps, err := planPricingItems(ctx, c, m, productRefs, today)
	if err != nil {
		return nil, err
	}
	ops = append(ops, pricingOps...)
	for _, s := range m.Subscriptions {
		offerOps, err := planOffers(ctx, c, s, productRefs[s.ProductID], today)
		if err != nil {
			return nil, err
		}
		ops = append(ops, offerOps...)
	}
	availabilityOps, err := planAvailabilityItems(ctx, c, m, productRefs)
	if err != nil {
		return nil, err
	}
	ops = append(ops, availabilityOps...)
	return ops, nil
}

func validateJSON(d *json.Decoder, typ reflect.Type) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return fmt.Errorf("manifest fields must not be null; omit optional fields")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		if typ.Kind() != reflect.Struct {
			return fmt.Errorf("unexpected JSON object")
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			fields[strings.Split(field.Tag.Get("json"), ",")[0]] = field.Type
		}
		seen := map[string]bool{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("invalid JSON field")
			}
			folded := strings.ToLower(key)
			if seen[folded] {
				return fmt.Errorf("duplicate JSON field %q", key)
			}
			seen[folded] = true
			child, ok := fields[key]
			if !ok {
				return fmt.Errorf("unknown field %q", key)
			}
			if err := validateJSON(d, child); err != nil {
				return err
			}
		}
	case '[':
		if typ.Kind() != reflect.Slice {
			return fmt.Errorf("unexpected JSON array")
		}
		for d.More() {
			if err := validateJSON(d, typ.Elem()); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
