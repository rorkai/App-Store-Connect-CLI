package shared

// AppInfoSparseFields is the fields[appInfos] enum, which every operation that
// accepts the parameter shares. kidsAgeBand stays last: API 4.5 removed it, but
// it remains accepted as a deprecated selector.
var AppInfoSparseFields = []string{
	"appStoreState", "state", "appStoreAgeRating", "australiaAgeRating", "brazilAgeRating",
	"brazilAgeRatingV2", "franceAgeRating", "koreaAgeRating", "app", "ageRatingDeclaration",
	"appInfoLocalizations", "primaryCategory", "primarySubcategoryOne", "primarySubcategoryTwo",
	"secondaryCategory", "secondarySubcategoryOne", "secondarySubcategoryTwo", "territoryAgeRatings",
	"kidsAgeBand",
}

// AgeRatingDeclarationSparseFields is the fields[ageRatingDeclarations] enum,
// which every operation that accepts the parameter shares.
var AgeRatingDeclarationSparseFields = []string{
	"advertising", "alcoholTobaccoOrDrugUseOrReferences", "contests", "gambling", "gamblingSimulated",
	"gunsOrOtherWeapons", "healthOrWellnessTopics", "kidsAgeBand", "lootBox",
	"medicalOrTreatmentInformation", "messagingAndChat", "parentalControls", "profanityOrCrudeHumor",
	"ageAssurance", "sexualContentGraphicAndNudity", "sexualContentOrNudity", "socialMedia",
	"socialMediaAgeRestricted", "horrorOrFearThemes", "matureOrSuggestiveThemes", "unrestrictedWebAccess",
	"userGeneratedContent", "violenceCartoonOrFantasy", "violenceRealisticProlongedGraphicOrSadistic",
	"violenceRealistic", "ageRatingOverride", "ageRatingOverrideV2", "koreaAgeRatingOverride",
	"gracRatingClassificationNumber", "developerAgeRatingInfoUrl",
}

// AppInAppPurchaseSparseFields is the fields[inAppPurchases] enum for operations
// that include IAPs from an app: GET /v1/apps, /v1/apps/{id}, and
// /v1/ciProducts/{id}/app. Other operations use narrower enums.
var AppInAppPurchaseSparseFields = []string{
	"referenceName", "productId", "inAppPurchaseType", "state", "apps", "name", "reviewNote",
	"familySharable", "contentHosting", "inAppPurchaseLocalizations", "pricePoints", "content",
	"appStoreReviewScreenshot", "promotedPurchase", "iapPriceSchedule", "inAppPurchaseAvailability",
	"images", "offerCodes", "versions",
}

// SubscriptionGroupSparseFields is the fields[subscriptionGroups] enum, which
// every operation that accepts the parameter shares.
var SubscriptionGroupSparseFields = []string{
	"referenceName", "subscriptions", "subscriptionGroupLocalizations", "versions",
}
