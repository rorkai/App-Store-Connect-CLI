package asc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
)

// apiAttributes remembers the attributes object Apple returned so read output
// keeps null, false, empty, and unmodeled values that omitempty struct fields
// would drop. Values built in code have no snapshot and encode as before.
type apiAttributes struct {
	original map[string]json.RawMessage
	decoded  map[string]json.RawMessage
}

// unmarshalAPIAttributes decodes data into dst, a method-free alias of an
// attributes type, and snapshots Apple's original object into api. Neither is
// changed when decoding fails.
func unmarshalAPIAttributes[T any](data []byte, dst *T, api *apiAttributes) error {
	var decoded T
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	baseline, err := json.Marshal(decoded)
	if err != nil {
		return err
	}
	captured, err := captureAPIAttributes(data, baseline, reflect.TypeOf(decoded))
	if err != nil {
		return err
	}
	*dst = decoded
	*api = captured
	return nil
}

// captureAPIAttributes snapshots Apple's object next to baseline, the
// encoding of what the modeled struct decoded from it. Keys that encoding/json
// matched to a modeled field under different casing are dropped, so a value
// such as a redacted secret cannot reappear under another spelling.
func captureAPIAttributes(data, baseline []byte, modeled reflect.Type, extraNames ...string) (apiAttributes, error) {
	var original map[string]json.RawMessage
	if err := json.Unmarshal(data, &original); err != nil || original == nil {
		return apiAttributes{}, err
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(baseline, &decoded); err != nil {
		return apiAttributes{}, err
	}
	names := extraNames
	for i := range modeled.NumField() {
		field := modeled.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if !field.IsExported() || name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		names = append(names, name)
	}
	for key, value := range original {
		modeledKey := false
		for _, name := range names {
			if strings.EqualFold(key, name) {
				modeledKey = true
				if key != name {
					delete(original, key)
				}
			}
		}
		if !modeledKey && isSecretAttributeName(key) && string(value) != "null" && string(value) != `""` {
			original[key] = json.RawMessage(`"` + RedactedValuePlaceholder + `"`)
		}
	}
	return apiAttributes{original: original, decoded: decoded}, nil
}

// isSecretAttributeName reports unmodeled attribute names that may carry a
// credential. Replaying Apple's object must not print one that no modeled
// redaction knows about.
func isSecretAttributeName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"password", "secret", "privatekey", "apikey", "accesstoken", "refreshtoken", "authtoken"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// marshal encodes value, a method-free alias of an attributes type, over
// Apple's original object.
func (a apiAttributes) marshal(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return a.apply(encoded)
}

// apply lays encoded, the modeled struct's current encoding, over Apple's
// original object, keeping only the fields changed in code.
func (a apiAttributes) apply(encoded []byte) ([]byte, error) {
	if a.decoded == nil {
		return encoded, nil
	}
	var current map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &current); err != nil {
		return nil, err
	}
	result := make(map[string]json.RawMessage, len(a.original))
	for name, value := range a.original {
		result[name] = value
	}
	for name, value := range current {
		if !bytes.Equal(value, a.decoded[name]) {
			result[name] = value
		}
	}
	for name := range a.decoded {
		if _, present := current[name]; !present {
			delete(result, name)
		}
	}
	return json.Marshal(result)
}

func (a *AppStoreReviewDetailAttributes) UnmarshalJSON(data []byte) error {
	type alias AppStoreReviewDetailAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a AppStoreReviewDetailAttributes) MarshalJSON() ([]byte, error) {
	type alias AppStoreReviewDetailAttributes
	return a.api.marshal(alias(a))
}

func (a *AppStoreVersionLocalizationAttributes) UnmarshalJSON(data []byte) error {
	type alias AppStoreVersionLocalizationAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a AppStoreVersionLocalizationAttributes) MarshalJSON() ([]byte, error) {
	type alias AppStoreVersionLocalizationAttributes
	return a.api.marshal(alias(a))
}

func (a *AppInfoLocalizationAttributes) UnmarshalJSON(data []byte) error {
	type alias AppInfoLocalizationAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a AppInfoLocalizationAttributes) MarshalJSON() ([]byte, error) {
	type alias AppInfoLocalizationAttributes
	return a.api.marshal(alias(a))
}

func (a *BetaBuildLocalizationAttributes) UnmarshalJSON(data []byte) error {
	type alias BetaBuildLocalizationAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a BetaBuildLocalizationAttributes) MarshalJSON() ([]byte, error) {
	type alias BetaBuildLocalizationAttributes
	return a.api.marshal(alias(a))
}

func (a *AgeRatingDeclarationAttributes) UnmarshalJSON(data []byte) error {
	type alias AgeRatingDeclarationAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a AgeRatingDeclarationAttributes) MarshalJSON() ([]byte, error) {
	type alias AgeRatingDeclarationAttributes
	return a.api.marshal(alias(a))
}

func (a *InAppPurchaseAppStoreReviewScreenshotAttributes) UnmarshalJSON(data []byte) error {
	type alias InAppPurchaseAppStoreReviewScreenshotAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a InAppPurchaseAppStoreReviewScreenshotAttributes) MarshalJSON() ([]byte, error) {
	type alias InAppPurchaseAppStoreReviewScreenshotAttributes
	return a.api.marshal(alias(a))
}

func (a *ReviewSubmissionAttributes) UnmarshalJSON(data []byte) error {
	type alias ReviewSubmissionAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a ReviewSubmissionAttributes) MarshalJSON() ([]byte, error) {
	type alias ReviewSubmissionAttributes
	return a.api.marshal(alias(a))
}

func (a *ReviewSubmissionItemAttributes) UnmarshalJSON(data []byte) error {
	type alias ReviewSubmissionItemAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a ReviewSubmissionItemAttributes) MarshalJSON() ([]byte, error) {
	type alias ReviewSubmissionItemAttributes
	return a.api.marshal(alias(a))
}

func (a *BuildBetaDetailAttributes) UnmarshalJSON(data []byte) error {
	type alias BuildBetaDetailAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a BuildBetaDetailAttributes) MarshalJSON() ([]byte, error) {
	type alias BuildBetaDetailAttributes
	return a.api.marshal(alias(a))
}

func (a *SubscriptionAttributes) UnmarshalJSON(data []byte) error {
	type alias SubscriptionAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a SubscriptionAttributes) MarshalJSON() ([]byte, error) {
	type alias SubscriptionAttributes
	return a.api.marshal(alias(a))
}

func (a *InAppPurchaseV2Attributes) UnmarshalJSON(data []byte) error {
	type alias InAppPurchaseV2Attributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a InAppPurchaseV2Attributes) MarshalJSON() ([]byte, error) {
	type alias InAppPurchaseV2Attributes
	return a.api.marshal(alias(a))
}

func (a *BetaAppReviewDetailAttributes) UnmarshalJSON(data []byte) error {
	type alias BetaAppReviewDetailAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a BetaAppReviewDetailAttributes) MarshalJSON() ([]byte, error) {
	type alias BetaAppReviewDetailAttributes
	return a.api.marshal(alias(a))
}

func (a *BetaAppLocalizationAttributes) UnmarshalJSON(data []byte) error {
	type alias BetaAppLocalizationAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a BetaAppLocalizationAttributes) MarshalJSON() ([]byte, error) {
	type alias BetaAppLocalizationAttributes
	return a.api.marshal(alias(a))
}

func (a *AppStoreVersionPhasedReleaseAttributes) UnmarshalJSON(data []byte) error {
	type alias AppStoreVersionPhasedReleaseAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a AppStoreVersionPhasedReleaseAttributes) MarshalJSON() ([]byte, error) {
	type alias AppStoreVersionPhasedReleaseAttributes
	return a.api.marshal(alias(a))
}

func (a *AppStoreVersionAttributes) UnmarshalJSON(data []byte) error {
	type alias AppStoreVersionAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a AppStoreVersionAttributes) MarshalJSON() ([]byte, error) {
	type alias AppStoreVersionAttributes
	return a.api.marshal(alias(a))
}

func (a *ReviewAttributes) UnmarshalJSON(data []byte) error {
	type alias ReviewAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a ReviewAttributes) MarshalJSON() ([]byte, error) {
	type alias ReviewAttributes
	return a.api.marshal(alias(a))
}
