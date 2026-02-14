package agerating

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var ageRatingLevelValues = []string{
	"NONE",
	"INFREQUENT_OR_MILD",
	"FREQUENT_OR_INTENSE",
	"INFREQUENT",
	"FREQUENT",
}

var ageRatingOverrideValues = []string{
	"NONE",
	"NINE_PLUS",
	"THIRTEEN_PLUS",
	"SIXTEEN_PLUS",
	"SEVENTEEN_PLUS",
	"UNRATED",
}

var ageRatingOverrideV2Values = []string{
	"NONE",
	"NINE_PLUS",
	"THIRTEEN_PLUS",
	"SIXTEEN_PLUS",
	"EIGHTEEN_PLUS",
	"UNRATED",
}

var koreaAgeRatingOverrideValues = []string{
	"NONE",
	"FIFTEEN_PLUS",
	"NINETEEN_PLUS",
}

var kidsAgeBandValues = []string{
	"FIVE_AND_UNDER",
	"SIX_TO_EIGHT",
	"NINE_TO_ELEVEN",
}

// AgeRatingCommand returns the age rating command with subcommands.
func AgeRatingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("age-rating", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "age-rating",
		ShortUsage: "asc age-rating <subcommand> [flags]",
		ShortHelp:  "Manage App Store age rating declarations.",
		LongHelp: `Manage App Store age rating declarations for an app, app info, or version.

Examples:
  asc age-rating get --app APP_ID
  asc age-rating get --app-info-id APP_INFO_ID
  asc age-rating set --app APP_ID --kids-age-band FIVE_AND_UNDER --gambling false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AgeRatingGetCommand(),
			AgeRatingSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AgeRatingGetCommand returns the age-rating get subcommand.
func AgeRatingGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("age-rating get", flag.ExitOnError)

	appID := fs.String("app", os.Getenv("ASC_APP_ID"), "App ID (required unless --app-info-id or --version-id is provided)")
	appInfoID := fs.String("app-info-id", "", "App info ID (optional)")
	versionID := fs.String("version-id", "", "App Store version ID (optional)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc age-rating get --app APP_ID [flags]",
		ShortHelp:  "Get an age rating declaration.",
		LongHelp: `Get the current age rating declaration.

Examples:
  asc age-rating get --app APP_ID
  asc age-rating get --app-info-id APP_INFO_ID
  asc age-rating get --version-id VERSION_ID`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appInfoValue := strings.TrimSpace(*appInfoID)
			versionValue := strings.TrimSpace(*versionID)
			appValue := strings.TrimSpace(shared.ResolveAppID(strings.TrimSpace(*appID)))

			if appInfoValue != "" && versionValue != "" {
				return fmt.Errorf("age-rating get: only one of --app-info-id or --version-id is allowed")
			}
			if appInfoValue == "" && versionValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("age-rating get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := fetchAgeRatingDeclaration(requestCtx, client, appValue, appInfoValue, versionValue)
			if err != nil {
				return fmt.Errorf("age-rating get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AgeRatingSetCommand returns the age-rating set subcommand.
func AgeRatingSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("age-rating set", flag.ExitOnError)

	id := fs.String("id", "", "Age rating declaration ID (optional)")
	appID := fs.String("app", os.Getenv("ASC_APP_ID"), "App ID (required unless --id, --app-info-id, or --version-id is provided)")
	appInfoID := fs.String("app-info-id", "", "App info ID (optional)")
	versionID := fs.String("version-id", "", "App Store version ID (optional)")

	// Boolean content descriptors
	advertising := fs.String("advertising", "", "Contains advertising (true/false)")
	gambling := fs.String("gambling", "", "Real gambling content (true/false)")
	healthOrWellnessTopics := fs.String("health-or-wellness-topics", "", "Health or wellness topics (true/false)")
	lootBox := fs.String("loot-box", "", "Loot box mechanics (true/false)")
	messagingAndChat := fs.String("messaging-and-chat", "", "Messaging and chat (true/false)")
	parentalControls := fs.String("parental-controls", "", "Parental controls (true/false)")
	ageAssurance := fs.String("age-assurance", "", "Age assurance (true/false)")
	unrestrictedWebAccess := fs.String("unrestricted-web-access", "", "Unrestricted web access (true/false)")
	userGeneratedContent := fs.String("user-generated-content", "", "User-generated content (true/false)")

	// Enum content descriptors (NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE)
	alcoholTobaccoDrug := fs.String("alcohol-tobacco-drug-use", "", "Alcohol/tobacco/drug references: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	contests := fs.String("contests", "", "Contests: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	gamblingSimulated := fs.String("gambling-simulated", "", "Simulated gambling: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	gunsOrOtherWeapons := fs.String("guns-or-other-weapons", "", "Guns or other weapons: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	medicalTreatment := fs.String("medical-treatment", "", "Medical/treatment information: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	profanityHumor := fs.String("profanity-humor", "", "Profanity or crude humor: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	sexualContentNudity := fs.String("sexual-content-nudity", "", "Sexual content or nudity: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	sexualContentGraphicNudity := fs.String("sexual-content-graphic-nudity", "", "Graphic sexual content or nudity: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	horrorFear := fs.String("horror-fear", "", "Horror or fear themes: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	matureSuggestive := fs.String("mature-suggestive", "", "Mature or suggestive themes: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	violenceCartoon := fs.String("violence-cartoon", "", "Cartoon/fantasy violence: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	violenceRealistic := fs.String("violence-realistic", "", "Realistic violence: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	violenceRealisticGraphic := fs.String("violence-realistic-graphic", "", "Prolonged graphic/sadistic violence: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")

	// Other
	kidsAgeBand := fs.String("kids-age-band", "", "Kids age band: FIVE_AND_UNDER, SIX_TO_EIGHT, NINE_TO_ELEVEN")
	ageRatingOverride := fs.String("age-rating-override", "", "Deprecated age rating override: NONE, NINE_PLUS, THIRTEEN_PLUS, SIXTEEN_PLUS, SEVENTEEN_PLUS, UNRATED")
	ageRatingOverrideV2 := fs.String("age-rating-override-v2", "", "Age rating override v2: NONE, NINE_PLUS, THIRTEEN_PLUS, SIXTEEN_PLUS, EIGHTEEN_PLUS, UNRATED")
	koreaAgeRatingOverride := fs.String("korea-age-rating-override", "", "Korea age rating override: NONE, FIFTEEN_PLUS, NINETEEN_PLUS")
	developerAgeRatingInfoURL := fs.String("developer-age-rating-info-url", "", "Developer age rating information URL")

	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc age-rating set --id DECLARATION_ID [flags]",
		ShortHelp:  "Update an age rating declaration.",
		LongHelp: `Update an age rating declaration.

Examples:
  asc age-rating set --id DECLARATION_ID --gambling false --kids-age-band FIVE_AND_UNDER
  asc age-rating set --app APP_ID --violence-realistic FREQUENT_OR_INTENSE --unrestricted-web-access true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appInfoValue := strings.TrimSpace(*appInfoID)
			versionValue := strings.TrimSpace(*versionID)
			appValue := strings.TrimSpace(shared.ResolveAppID(strings.TrimSpace(*appID)))

			if idValue == "" {
				if appInfoValue != "" && versionValue != "" {
					return fmt.Errorf("age-rating set: only one of --app-info-id or --version-id is allowed")
				}
				if appInfoValue == "" && versionValue == "" && appValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id or --app is required (or set ASC_APP_ID)")
					return flag.ErrHelp
				}
			}

			attributes, err := buildAgeRatingAttributes(map[string]string{
				// Boolean content descriptors
				"advertising":               *advertising,
				"gambling":                  *gambling,
				"health-or-wellness-topics": *healthOrWellnessTopics,
				"loot-box":                  *lootBox,
				"messaging-and-chat":        *messagingAndChat,
				"parental-controls":         *parentalControls,
				"age-assurance":             *ageAssurance,
				"unrestricted-web-access":   *unrestrictedWebAccess,
				"user-generated-content":    *userGeneratedContent,
				// Enum content descriptors
				"alcohol-tobacco-drug-use":      *alcoholTobaccoDrug,
				"contests":                      *contests,
				"gambling-simulated":            *gamblingSimulated,
				"guns-or-other-weapons":         *gunsOrOtherWeapons,
				"medical-treatment":             *medicalTreatment,
				"profanity-humor":               *profanityHumor,
				"sexual-content-nudity":         *sexualContentNudity,
				"sexual-content-graphic-nudity": *sexualContentGraphicNudity,
				"horror-fear":                   *horrorFear,
				"mature-suggestive":             *matureSuggestive,
				"violence-cartoon":              *violenceCartoon,
				"violence-realistic":            *violenceRealistic,
				"violence-realistic-graphic":    *violenceRealisticGraphic,
				// Other
				"kids-age-band":                 *kidsAgeBand,
				"age-rating-override":           *ageRatingOverride,
				"age-rating-override-v2":        *ageRatingOverrideV2,
				"korea-age-rating-override":     *koreaAgeRatingOverride,
				"developer-age-rating-info-url": *developerAgeRatingInfoURL,
			})
			if err != nil {
				return err
			}

			if !hasAgeRatingUpdates(attributes) {
				return fmt.Errorf("age-rating set: at least one update flag is required")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("age-rating set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if idValue == "" {
				idValue, err = resolveAgeRatingDeclarationID(requestCtx, client, appValue, appInfoValue, versionValue)
				if err != nil {
					return fmt.Errorf("age-rating set: %w", err)
				}
			}

			resp, err := client.UpdateAgeRatingDeclaration(requestCtx, idValue, attributes)
			if err != nil {
				return fmt.Errorf("age-rating set: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func fetchAgeRatingDeclaration(ctx context.Context, client *asc.Client, appID, appInfoID, versionID string) (*asc.AgeRatingDeclarationResponse, error) {
	switch {
	case appInfoID != "":
		return client.GetAgeRatingDeclarationForAppInfo(ctx, appInfoID)
	case versionID != "":
		return client.GetAgeRatingDeclarationForAppStoreVersion(ctx, versionID)
	default:
		appInfos, err := client.GetAppInfos(ctx, appID)
		if err != nil {
			return nil, fmt.Errorf("failed to get app info: %w", err)
		}
		if len(appInfos.Data) == 0 {
			return nil, fmt.Errorf("no app info found for app %s", appID)
		}
		appInfoID := appInfos.Data[0].ID
		if strings.TrimSpace(appInfoID) == "" {
			return nil, fmt.Errorf("app info id is empty for app %s", appID)
		}
		return client.GetAgeRatingDeclarationForAppInfo(ctx, appInfoID)
	}
}

func resolveAgeRatingDeclarationID(ctx context.Context, client *asc.Client, appID, appInfoID, versionID string) (string, error) {
	resp, err := fetchAgeRatingDeclaration(ctx, client, appID, appInfoID, versionID)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(resp.Data.ID)
	if id == "" {
		return "", fmt.Errorf("age rating declaration id is empty")
	}
	return id, nil
}

func buildAgeRatingAttributes(values map[string]string) (asc.AgeRatingDeclarationAttributes, error) {
	var attrs asc.AgeRatingDeclarationAttributes

	// Boolean content descriptors
	boolFields := []struct {
		flag string
		dest **bool
	}{
		{"advertising", &attrs.Advertising},
		{"gambling", &attrs.Gambling},
		{"health-or-wellness-topics", &attrs.HealthOrWellnessTopics},
		{"loot-box", &attrs.LootBox},
		{"messaging-and-chat", &attrs.MessagingAndChat},
		{"parental-controls", &attrs.ParentalControls},
		{"age-assurance", &attrs.AgeAssurance},
		{"unrestricted-web-access", &attrs.UnrestrictedWebAccess},
		{"user-generated-content", &attrs.UserGeneratedContent},
	}
	for _, f := range boolFields {
		val, err := shared.ParseOptionalBoolFlag("--"+f.flag, values[f.flag])
		if err != nil {
			return attrs, err
		}
		*f.dest = val
	}

	// Enum content descriptors (NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE)
	enumFields := []struct {
		flag    string
		dest    **string
		allowed []string
	}{
		{"alcohol-tobacco-drug-use", &attrs.AlcoholTobaccoOrDrugUseOrReferences, ageRatingLevelValues},
		{"contests", &attrs.Contests, ageRatingLevelValues},
		{"gambling-simulated", &attrs.GamblingSimulated, ageRatingLevelValues},
		{"guns-or-other-weapons", &attrs.GunsOrOtherWeapons, ageRatingLevelValues},
		{"medical-treatment", &attrs.MedicalOrTreatmentInformation, ageRatingLevelValues},
		{"profanity-humor", &attrs.ProfanityOrCrudeHumor, ageRatingLevelValues},
		{"sexual-content-nudity", &attrs.SexualContentOrNudity, ageRatingLevelValues},
		{"sexual-content-graphic-nudity", &attrs.SexualContentGraphicAndNudity, ageRatingLevelValues},
		{"horror-fear", &attrs.HorrorOrFearThemes, ageRatingLevelValues},
		{"mature-suggestive", &attrs.MatureOrSuggestiveThemes, ageRatingLevelValues},
		{"violence-cartoon", &attrs.ViolenceCartoonOrFantasy, ageRatingLevelValues},
		{"violence-realistic", &attrs.ViolenceRealistic, ageRatingLevelValues},
		{"violence-realistic-graphic", &attrs.ViolenceRealisticProlongedGraphicOrSadistic, ageRatingLevelValues},
		{"kids-age-band", &attrs.KidsAgeBand, kidsAgeBandValues},
		{"age-rating-override", &attrs.AgeRatingOverride, ageRatingOverrideValues},
		{"age-rating-override-v2", &attrs.AgeRatingOverrideV2, ageRatingOverrideV2Values},
		{"korea-age-rating-override", &attrs.KoreaAgeRatingOverride, koreaAgeRatingOverrideValues},
	}
	for _, f := range enumFields {
		val, err := parseOptionalEnumFlag("--"+f.flag, values[f.flag], f.allowed)
		if err != nil {
			return attrs, err
		}
		*f.dest = val
	}

	if raw := strings.TrimSpace(values["developer-age-rating-info-url"]); raw != "" {
		if _, err := url.ParseRequestURI(raw); err != nil {
			return attrs, fmt.Errorf("--developer-age-rating-info-url must be a valid URL")
		}
		attrs.DeveloperAgeRatingInfoURL = &raw
	}

	return attrs, nil
}

func hasAgeRatingUpdates(attrs asc.AgeRatingDeclarationAttributes) bool {
	return attrs.Advertising != nil ||
		attrs.Gambling != nil ||
		attrs.HealthOrWellnessTopics != nil ||
		attrs.LootBox != nil ||
		attrs.MessagingAndChat != nil ||
		attrs.ParentalControls != nil ||
		attrs.AgeAssurance != nil ||
		attrs.UnrestrictedWebAccess != nil ||
		attrs.UserGeneratedContent != nil ||
		attrs.AlcoholTobaccoOrDrugUseOrReferences != nil ||
		attrs.Contests != nil ||
		attrs.GamblingSimulated != nil ||
		attrs.GunsOrOtherWeapons != nil ||
		attrs.MedicalOrTreatmentInformation != nil ||
		attrs.ProfanityOrCrudeHumor != nil ||
		attrs.SexualContentOrNudity != nil ||
		attrs.SexualContentGraphicAndNudity != nil ||
		attrs.HorrorOrFearThemes != nil ||
		attrs.MatureOrSuggestiveThemes != nil ||
		attrs.ViolenceCartoonOrFantasy != nil ||
		attrs.ViolenceRealistic != nil ||
		attrs.ViolenceRealisticProlongedGraphicOrSadistic != nil ||
		attrs.KidsAgeBand != nil ||
		attrs.AgeRatingOverride != nil ||
		attrs.AgeRatingOverrideV2 != nil ||
		attrs.KoreaAgeRatingOverride != nil ||
		attrs.DeveloperAgeRatingInfoURL != nil
}

func parseOptionalEnumFlag(name, raw string, allowed []string) (*string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	normalized := strings.ToUpper(raw)
	if slices.Contains(allowed, normalized) {
		return &normalized, nil
	}
	return nil, fmt.Errorf("%s must be one of: %s", name, strings.Join(allowed, ", "))
}
