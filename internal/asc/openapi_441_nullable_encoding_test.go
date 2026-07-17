package asc

import (
	"encoding/json"
	"testing"
)

func nullableEncodingTestPtr[T any](value T) *T {
	return &value
}

func assertNullableAttributeEncoding(t *testing.T, request any, key, want string, present bool) {
	t.Helper()

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var payload struct {
		Data struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	got, ok := payload.Data.Attributes[key]
	if ok != present {
		t.Fatalf("%s presence = %t in %s, want %t", key, ok, encoded, present)
	}
	if present && string(got) != want {
		t.Fatalf("%s JSON = %s, want %s", key, got, want)
	}
}

func TestAgeRatingDeclarationNewFieldsNullableEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		attrs AgeRatingDeclarationAttributes
		want  string
		set   bool
	}{
		{name: "social media omitted", key: "socialMedia", attrs: AgeRatingDeclarationAttributes{}},
		{name: "social media value", key: "socialMedia", attrs: AgeRatingDeclarationAttributes{SocialMedia: &NullableBool{Value: nullableEncodingTestPtr(true)}}, want: "true", set: true},
		{name: "social media null", key: "socialMedia", attrs: AgeRatingDeclarationAttributes{SocialMedia: &NullableBool{}}, want: "null", set: true},
		{name: "restricted omitted", key: "socialMediaAgeRestricted", attrs: AgeRatingDeclarationAttributes{}},
		{name: "restricted value", key: "socialMediaAgeRestricted", attrs: AgeRatingDeclarationAttributes{SocialMediaAgeRestricted: &NullableBool{Value: nullableEncodingTestPtr(false)}}, want: "false", set: true},
		{name: "restricted null", key: "socialMediaAgeRestricted", attrs: AgeRatingDeclarationAttributes{SocialMediaAgeRestricted: &NullableBool{}}, want: "null", set: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertNullableAttributeEncoding(t, AgeRatingDeclarationUpdateRequest{Data: AgeRatingDeclarationUpdateData{
				Type: ResourceTypeAgeRatingDeclarations, ID: "age-1", Attributes: test.attrs,
			}}, test.key, test.want, test.set)
		})
	}
}

func TestVersionScopedCreateFieldsNullableEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request any
		key     string
		want    string
		set     bool
	}{
		{
			name: "iap description omitted",
			request: InAppPurchaseLocalizationV2CreateRequest{Data: InAppPurchaseLocalizationV2CreateData{
				Type: ResourceTypeInAppPurchaseLocalizations, Attributes: InAppPurchaseLocalizationV2CreateAttributes{Name: "Name", Locale: "en-US"},
			}},
			key: "description",
		},
		{
			name: "iap description value",
			request: InAppPurchaseLocalizationV2CreateRequest{Data: InAppPurchaseLocalizationV2CreateData{
				Type: ResourceTypeInAppPurchaseLocalizations, Attributes: InAppPurchaseLocalizationV2CreateAttributes{Name: "Name", Locale: "en-US", Description: &NullableString{Value: nullableEncodingTestPtr("Details")}},
			}},
			key: "description", want: `"Details"`, set: true,
		},
		{
			name: "iap description null",
			request: InAppPurchaseLocalizationV2CreateRequest{Data: InAppPurchaseLocalizationV2CreateData{
				Type: ResourceTypeInAppPurchaseLocalizations, Attributes: InAppPurchaseLocalizationV2CreateAttributes{Name: "Name", Locale: "en-US", Description: &NullableString{}},
			}},
			key: "description", want: "null", set: true,
		},
		{
			name: "subscription description omitted",
			request: SubscriptionLocalizationV2CreateRequest{Data: SubscriptionLocalizationV2CreateData{
				Type: ResourceTypeSubscriptionLocalizations, Attributes: SubscriptionLocalizationV2CreateAttributes{Name: "Pro", Locale: "en-US"},
			}},
			key: "description",
		},
		{
			name: "subscription description value",
			request: SubscriptionLocalizationV2CreateRequest{Data: SubscriptionLocalizationV2CreateData{
				Type: ResourceTypeSubscriptionLocalizations, Attributes: SubscriptionLocalizationV2CreateAttributes{Name: "Pro", Locale: "en-US", Description: &NullableString{Value: nullableEncodingTestPtr("Details")}},
			}},
			key: "description", want: `"Details"`, set: true,
		},
		{
			name: "subscription description null",
			request: SubscriptionLocalizationV2CreateRequest{Data: SubscriptionLocalizationV2CreateData{
				Type: ResourceTypeSubscriptionLocalizations, Attributes: SubscriptionLocalizationV2CreateAttributes{Name: "Pro", Locale: "en-US", Description: &NullableString{}},
			}},
			key: "description", want: "null", set: true,
		},
		{
			name: "group custom app name omitted",
			request: SubscriptionGroupLocalizationV2CreateRequest{Data: SubscriptionGroupLocalizationV2CreateData{
				Type: ResourceTypeSubscriptionGroupLocalizations, Attributes: SubscriptionGroupLocalizationV2CreateAttributes{Name: "Premium", Locale: "en-US"},
			}},
			key: "customAppName",
		},
		{
			name: "group custom app name value",
			request: SubscriptionGroupLocalizationV2CreateRequest{Data: SubscriptionGroupLocalizationV2CreateData{
				Type: ResourceTypeSubscriptionGroupLocalizations, Attributes: SubscriptionGroupLocalizationV2CreateAttributes{Name: "Premium", Locale: "en-US", CustomAppName: &NullableString{Value: nullableEncodingTestPtr("Example")}},
			}},
			key: "customAppName", want: `"Example"`, set: true,
		},
		{
			name: "group custom app name null",
			request: SubscriptionGroupLocalizationV2CreateRequest{Data: SubscriptionGroupLocalizationV2CreateData{
				Type: ResourceTypeSubscriptionGroupLocalizations, Attributes: SubscriptionGroupLocalizationV2CreateAttributes{Name: "Premium", Locale: "en-US", CustomAppName: &NullableString{}},
			}},
			key: "customAppName", want: "null", set: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertNullableAttributeEncoding(t, test.request, test.key, test.want, test.set)
		})
	}
}

func TestVersionScopedImageUploadedNullableEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request any
		want    string
		set     bool
	}{
		{name: "iap omitted", request: InAppPurchaseImageV2UpdateRequest{Data: InAppPurchaseImageV2UpdateData{Type: ResourceTypeInAppPurchaseImages, ID: "image-1"}}},
		{name: "iap value", request: InAppPurchaseImageV2UpdateRequest{Data: InAppPurchaseImageV2UpdateData{Type: ResourceTypeInAppPurchaseImages, ID: "image-1", Attributes: &InAppPurchaseImageV2UpdateAttributes{Uploaded: &NullableBool{Value: nullableEncodingTestPtr(true)}}}}, want: "true", set: true},
		{name: "iap null", request: InAppPurchaseImageV2UpdateRequest{Data: InAppPurchaseImageV2UpdateData{Type: ResourceTypeInAppPurchaseImages, ID: "image-1", Attributes: &InAppPurchaseImageV2UpdateAttributes{Uploaded: &NullableBool{}}}}, want: "null", set: true},
		{name: "subscription omitted", request: SubscriptionImageV2UpdateRequest{Data: SubscriptionImageV2UpdateData{Type: ResourceTypeSubscriptionImages, ID: "image-1"}}},
		{name: "subscription value", request: SubscriptionImageV2UpdateRequest{Data: SubscriptionImageV2UpdateData{Type: ResourceTypeSubscriptionImages, ID: "image-1", Attributes: SubscriptionImageV2UpdateAttributes{Uploaded: &NullableBool{Value: nullableEncodingTestPtr(false)}}}}, want: "false", set: true},
		{name: "subscription null", request: SubscriptionImageV2UpdateRequest{Data: SubscriptionImageV2UpdateData{Type: ResourceTypeSubscriptionImages, ID: "image-1", Attributes: SubscriptionImageV2UpdateAttributes{Uploaded: &NullableBool{}}}}, want: "null", set: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertNullableAttributeEncoding(t, test.request, "uploaded", test.want, test.set)
		})
	}
}
