package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	serviceIDDomainInput    = "APPLE_ID_AUTH_WEB_DOMAIN"
	serviceIDReturnURLInput = "APPLE_ID_AUTH_WEB_RETURN_URL"
)

// DeveloperServiceIDDomainsSetRequest replaces the complete website URL lists
// of an already configured Sign in with Apple Services ID.
type DeveloperServiceIDDomainsSetRequest struct {
	ServiceID  string
	Domains    []string
	ReturnURLs []string
}

// NormalizeDeveloperServiceIDDomainsSetRequest validates before authentication.
func NormalizeDeveloperServiceIDDomainsSetRequest(r DeveloperServiceIDDomainsSetRequest) (DeveloperServiceIDDomainsSetRequest, error) {
	r.ServiceID = strings.TrimSpace(r.ServiceID)
	if r.ServiceID == "" {
		return r, fmt.Errorf("--service-id is required")
	}
	if len(r.Domains) == 0 || (len(r.Domains) == 1 && strings.TrimSpace(r.Domains[0]) == "") {
		return r, fmt.Errorf("--domain is required")
	}
	if len(r.ReturnURLs) == 0 || (len(r.ReturnURLs) == 1 && strings.TrimSpace(r.ReturnURLs[0]) == "") {
		return r, fmt.Errorf("--return-url is required")
	}
	r.Domains = append([]string(nil), r.Domains...)
	r.ReturnURLs = append([]string(nil), r.ReturnURLs...)
	for i, d := range r.Domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || !strings.Contains(d, ".") || net.ParseIP(d) != nil {
			return r, fmt.Errorf("--domain must contain DNS hostnames, without a scheme, port, or path")
		}
		for _, label := range strings.Split(d, ".") {
			if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return r, fmt.Errorf("--domain contains an invalid hostname")
			}
			for _, ch := range label {
				if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
					return r, fmt.Errorf("--domain must contain ASCII DNS hostnames (use punycode for international names)")
				}
			}
		}
		r.Domains[i] = d
	}
	for i, raw := range r.ReturnURLs {
		raw = strings.TrimSpace(raw)
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || strings.Contains(raw, "#") {
			return r, fmt.Errorf("--return-url must contain absolute HTTPS URLs without credentials or fragments")
		}
		found := false
		for _, d := range r.Domains {
			if strings.EqualFold(u.Hostname(), d) {
				found = true
			}
		}
		if !found {
			return r, fmt.Errorf("--return-url hosts must appear in --domain")
		}
		r.ReturnURLs[i] = raw
	}
	r.Domains = sortedServiceIDValues(r.Domains)
	r.ReturnURLs = sortedServiceIDValues(r.ReturnURLs)
	return r, nil
}

func sortedServiceIDValues(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	for i := 1; i < len(result); {
		if result[i] == result[i-1] {
			result = append(result[:i], result[i+1:]...)
		} else {
			i++
		}
	}
	return result
}

type serviceIDInput struct {
	Key    string                `json:"key"`
	Values []serviceIDInputValue `json:"values"`
}
type serviceIDInputValue struct {
	Value string `json:"value"`
}
type serviceIDDomainSnapshot struct {
	developerServiceIDCapabilitySnapshot
	Inputs map[string][]string
}

func serviceIDCapabilityInputs(raw json.RawMessage) ([]serviceIDInput, error) {
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return nil, err
	}
	value, exists := attrs["inputs"]
	if !exists || string(value) == "null" {
		return nil, nil
	}
	var rawInputs []map[string]json.RawMessage
	if err := json.Unmarshal(value, &rawInputs); err != nil {
		return nil, fmt.Errorf("invalid capability inputs: %w", err)
	}
	for _, input := range rawInputs {
		// Input presentation metadata is read-only; value objects are the writable
		// contract. Reject unknown shapes rather than silently dropping their data.
		for key := range input {
			switch key {
			case "key", "values", "name", "description", "allowedInstances", "minInstances", "displayOrder", "maxLimit":
			default:
				return nil, fmt.Errorf("unsupported capability input member %q", key)
			}
		}
		rawValues, ok := input["values"]
		if !ok || string(rawValues) == "null" {
			return nil, fmt.Errorf("capability input values are unresolved")
		}
		var values []map[string]json.RawMessage
		if err := json.Unmarshal(rawValues, &values); err != nil {
			return nil, err
		}
		for _, value := range values {
			raw, ok := value["value"]
			if !ok || len(value) != 1 || string(raw) == "null" {
				return nil, fmt.Errorf("unsupported capability input value")
			}
			var text string
			if err := json.Unmarshal(raw, &text); err != nil {
				return nil, fmt.Errorf("capability input value must be a string")
			}
		}
	}
	var inputs []serviceIDInput
	if err := json.Unmarshal(value, &inputs); err != nil {
		return nil, fmt.Errorf("invalid capability inputs: %w", err)
	}
	seen := map[string]bool{}
	for _, input := range inputs {
		if strings.TrimSpace(input.Key) == "" || seen[input.Key] {
			return nil, fmt.Errorf("capability inputs contain an empty or duplicate key")
		}
		seen[input.Key] = true
	}
	return inputs, nil
}

// Services ID detail responses use a zero total with the maximum include limit
// even for populated linkage (captured 2026-09-26). Relax only that placeholder;
// all pagination, positive count discrepancies, and unresolved linkage fail.
func serviceIDDomainCapabilities(current developerBundleIDResponse) ([]developerResource, error) {
	raw := current.Data.Relationships["bundleIdCapabilities"]
	refs, err := decodeStrictDeveloperRelationship(raw)
	if err != nil {
		return nil, err
	}
	var relationship map[string]json.RawMessage
	if err = json.Unmarshal(raw, &relationship); err != nil {
		return nil, err
	}
	var meta struct {
		Paging struct {
			Total *int `json:"total"`
			Limit *int `json:"limit"`
		} `json:"paging"`
	}
	if v, ok := relationship["meta"]; ok {
		if err = json.Unmarshal(v, &meta); err != nil {
			return nil, err
		}
	}
	if meta.Paging.Total != nil && *meta.Paging.Total == 0 && meta.Paging.Limit != nil && *meta.Paging.Limit == 2147483647 && len(refs) > 0 {
		var m map[string]json.RawMessage
		if err = json.Unmarshal(relationship["meta"], &m); err != nil {
			return nil, err
		}
		var paging map[string]json.RawMessage
		if err = json.Unmarshal(m["paging"], &paging); err != nil {
			return nil, err
		}
		paging["total"], _ = json.Marshal(len(refs))
		m["paging"], _ = json.Marshal(paging)
		relationship["meta"], _ = json.Marshal(m)
		raw, _ = json.Marshal(relationship)
	}
	if err = validateDeveloperRelationshipCompleteness(raw, len(refs), developerBundleIDCapabilitiesIncludeLimit, "Services ID capability graph"); err != nil {
		return nil, err
	}
	if _, err = developerServiceIDCapabilityGraph(current); err != nil {
		return nil, err
	}
	referenced := map[string]bool{}
	for _, ref := range refs {
		referenced[ref.ID] = true
	}
	included := map[string]developerResource{}
	for _, resource := range current.Included {
		if resource.Type != "bundleIdCapabilities" {
			continue
		}
		if !referenced[resource.ID] {
			return nil, fmt.Errorf("unreferenced included capability %q", resource.ID)
		}
		included[resource.ID] = resource
	}
	result := make([]developerResource, 0, len(refs))
	for _, ref := range refs {
		result = append(result, included[ref.ID])
	}
	return result, nil
}

func serviceIDDomainState(caps []developerResource) (map[string]serviceIDDomainSnapshot, error) {
	state := map[string]serviceIDDomainSnapshot{}
	for _, capability := range caps {
		snapshot, err := developerServiceIDCapabilitySnapshotFor(capability)
		if err != nil {
			return nil, err
		}
		inputs, err := serviceIDCapabilityInputs(capability.Attributes)
		if err != nil {
			return nil, err
		}
		values := map[string][]string{}
		for _, input := range inputs {
			list := []string{}
			for _, v := range input.Values {
				list = append(list, v.Value)
			}
			values[input.Key] = sortedServiceIDValues(list)
		}
		state[capability.ID] = serviceIDDomainSnapshot{snapshot, values}
	}
	return state, nil
}

// SetDeveloperServiceIDDomains preserves the configured primary app and enabled
// state. The captured PATCH is followed by a separate authoritative detail read.
func (c *Client) SetDeveloperServiceIDDomains(ctx context.Context, r DeveloperServiceIDDomainsSetRequest) (*asc.WebServiceIDMutationResult, error) {
	r, err := NormalizeDeveloperServiceIDDomainsSetRequest(r)
	if err != nil {
		return nil, err
	}
	if err = c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	current, err := c.loadDeveloperServiceIDResourceAfterSession(ctx, r.ServiceID)
	if err != nil {
		return nil, err
	}
	identifier, err := developerServiceIDRequiredRawAttribute(current.Data.Attributes, r.ServiceID, "identifier")
	if err != nil {
		return nil, err
	}
	name, err := developerServiceIDRequiredRawAttribute(current.Data.Attributes, r.ServiceID, "name")
	if err != nil {
		return nil, err
	}
	caps, err := serviceIDDomainCapabilities(current)
	if err != nil {
		return nil, fmt.Errorf("cannot safely update Services ID domains: %w", err)
	}
	before, err := serviceIDDomainState(caps)
	if err != nil {
		return nil, err
	}
	target := -1
	for i, capability := range caps {
		id, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return nil, err
		}
		if id == "APPLE_ID_AUTH" {
			if target != -1 {
				return nil, fmt.Errorf("services ID has duplicate Sign in with Apple capabilities")
			}
			target = i
		}
	}
	if target < 0 {
		return nil, fmt.Errorf("configure Sign in with Apple and its primary App ID in Developer Portal before setting domains")
	}
	enabled, present, err := developerBundleIDCapabilityEnabledValue(caps[target])
	if err != nil || !present || !enabled {
		return nil, fmt.Errorf("sign in with Apple must already be enabled before setting domains")
	}
	var consent struct {
		Data *struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(caps[target].Relationships["appConsentBundleId"], &consent); err != nil || consent.Data == nil || consent.Data.Type != "bundleIds" || strings.TrimSpace(consent.Data.ID) == "" {
		return nil, fmt.Errorf("services ID must already have a primary App ID configured")
	}
	inputs, err := serviceIDCapabilityInputs(caps[target].Attributes)
	if err != nil {
		return nil, err
	}
	for _, update := range []struct {
		key    string
		values []string
	}{{serviceIDReturnURLInput, r.ReturnURLs}, {serviceIDDomainInput, r.Domains}} {
		next := serviceIDInput{Key: update.key, Values: []serviceIDInputValue{}}
		for _, value := range update.values {
			next.Values = append(next.Values, serviceIDInputValue{Value: value})
		}
		found := false
		for i := range inputs {
			if inputs[i].Key == update.key {
				inputs[i] = next
				found = true
			}
		}
		if !found {
			inputs = append(inputs, next)
		}
	}
	var attributes map[string]json.RawMessage
	if err = json.Unmarshal(caps[target].Attributes, &attributes); err != nil {
		return nil, err
	}
	attributes["inputs"], err = json.Marshal(inputs)
	if err != nil {
		return nil, err
	}
	caps[target].Attributes, err = json.Marshal(attributes)
	if err != nil {
		return nil, err
	}
	expected, err := serviceIDDomainState(caps)
	if err != nil {
		return nil, err
	}
	receipt := &asc.WebServiceIDMutationResult{Operation: "domains-set", ServiceID: r.ServiceID, Identifier: identifier, Name: name, Verified: true, Status: "unchanged"}
	if reflect.DeepEqual(before, expected) {
		return receipt, nil
	}
	var payload developerBundleIDPatchRequest
	payload.Data.ID = current.Data.ID
	payload.Data.Type = current.Data.Type
	payload.Data.Attributes = append(json.RawMessage(nil), current.Data.Attributes...)
	payload.Data.Relationships = cloneRawMessageMap(current.Data.Relationships)
	payload, err = addDeveloperPortalTeamID(payload, c.developerPortalTeamID())
	if err != nil {
		return nil, err
	}
	// Emit only writable capability attributes and relationship linkage. In
	// particular, inputs must not pass through the enabled/settings-only helper.
	writable := make([]developerResource, 0, len(caps))
	for _, capability := range caps {
		var attrs map[string]json.RawMessage
		if err = json.Unmarshal(capability.Attributes, &attrs); err != nil {
			return nil, err
		}
		writeAttrs := map[string]json.RawMessage{}
		for _, key := range []string{"enabled", "settings", "inputs"} {
			if v, ok := attrs[key]; ok && (key != "settings" || string(v) != "null") {
				writeAttrs[key] = v
			}
		}
		if v, ok := writeAttrs["inputs"]; ok && string(v) != "null" {
			parsed, e := serviceIDCapabilityInputs(capability.Attributes)
			if e != nil {
				return nil, e
			}
			writeAttrs["inputs"], err = json.Marshal(parsed)
			if err != nil {
				return nil, err
			}
		}
		rawAttrs, e := json.Marshal(writeAttrs)
		if e != nil {
			return nil, e
		}
		links := map[string]json.RawMessage{}
		for key, raw := range capability.Relationships {
			var relation map[string]json.RawMessage
			if err = json.Unmarshal(raw, &relation); err != nil {
				return nil, err
			}
			data, ok := relation["data"]
			if !ok {
				return nil, fmt.Errorf("capability relationship %s is unresolved", key)
			}
			links[key], err = json.Marshal(map[string]json.RawMessage{"data": data})
			if err != nil {
				return nil, err
			}
		}
		writable = append(writable, developerResource{Type: capability.Type, Attributes: rawAttrs, Relationships: links})
	}
	payload.Data.Relationships["bundleIdCapabilities"], err = json.Marshal(developerResourceRelationship{Data: writable})
	if err != nil {
		return nil, err
	}
	if _, err = c.doDeveloperPortalRequest(ctx, http.MethodPatch, "/bundleIds/"+url.PathEscape(r.ServiceID), payload, developerPortalHeaders(r.ServiceID), true); err != nil {
		return nil, developerServiceIDWriteError("domains-set", err)
	}
	view, err := c.getDeveloperServiceIDAfterSession(ctx, r.ServiceID)
	if err == nil {
		err = verifyDeveloperServiceIDIdentity(view.Data, r.ServiceID, identifier, name)
	}
	if err == nil {
		var post developerBundleIDResponse
		err = json.Unmarshal(view.Raw, &post)
		if err == nil {
			var gotCaps []developerResource
			gotCaps, err = serviceIDDomainCapabilities(post)
			if err == nil {
				var got map[string]serviceIDDomainSnapshot
				got, err = serviceIDDomainState(gotCaps)
				if err == nil && !reflect.DeepEqual(expected, got) {
					err = fmt.Errorf("capability settings, domains, or primary App ID disagreed with the requested state")
				}
			}
		}
	}
	if err != nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID domain update but verification failed; inspect it before retrying: %w", err)}
	}
	receipt.Changed = true
	receipt.Status = "updated"
	return receipt, nil
}
