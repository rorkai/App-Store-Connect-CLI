package agerating

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var ageRatingLevelValues = []string{
	"NONE",
	"INFREQUENT_OR_MILD",
	"FREQUENT_OR_INTENSE",
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
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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

			return shared.PrintOutput(resp, *output, *pretty)
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

	gambling := fs.String("gambling", "", "Real gambling content (true/false)")
	gamblingSimulated := fs.String("gambling-simulated", "", "Simulated gambling: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	alcoholTobaccoDrug := fs.String("alcohol-tobacco-drug-use", "", "Alcohol/tobacco/drug references: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	contests := fs.String("contests", "", "Contests: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	medicalTreatment := fs.String("medical-treatment", "", "Medical/treatment information: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	profanityHumor := fs.String("profanity-humor", "", "Profanity or crude humor: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	sexualContentNudity := fs.String("sexual-content-nudity", "", "Sexual content or nudity: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	sexualContentGraphicNudity := fs.String("sexual-content-graphic-nudity", "", "Graphic sexual content or nudity: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	horrorFear := fs.String("horror-fear", "", "Horror or fear themes: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	matureSuggestive := fs.String("mature-suggestive", "", "Mature or suggestive themes: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	violenceCartoon := fs.String("violence-cartoon", "", "Cartoon/fantasy violence: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	violenceRealistic := fs.String("violence-realistic", "", "Realistic violence: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	violenceRealisticGraphic := fs.String("violence-realistic-graphic", "", "Prolonged graphic/sadistic violence: NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE")
	seventeenPlus := fs.String("seventeen-plus", "", "17+ content (true/false, not supported by API)")
	unrestrictedWebAccess := fs.String("unrestricted-web-access", "", "Unrestricted web access (true/false)")
	kidsAgeBand := fs.String("kids-age-band", "", "Kids age band: FIVE_AND_UNDER, SIX_TO_EIGHT, NINE_TO_ELEVEN")

	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				"gambling":                      *gambling,
				"gambling-simulated":            *gamblingSimulated,
				"alcohol-tobacco-drug-use":      *alcoholTobaccoDrug,
				"contests":                      *contests,
				"medical-treatment":             *medicalTreatment,
				"profanity-humor":               *profanityHumor,
				"sexual-content-nudity":         *sexualContentNudity,
				"sexual-content-graphic-nudity": *sexualContentGraphicNudity,
				"horror-fear":                   *horrorFear,
				"mature-suggestive":             *matureSuggestive,
				"violence-cartoon":              *violenceCartoon,
				"violence-realistic":            *violenceRealistic,
				"violence-realistic-graphic":    *violenceRealisticGraphic,
				"seventeen-plus":                *seventeenPlus,
				"unrestricted-web-access":       *unrestrictedWebAccess,
				"kids-age-band":                 *kidsAgeBand,
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

			return shared.PrintOutput(resp, *output, *pretty)
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

	gambling, err := shared.ParseOptionalBoolFlag("--gambling", values["gambling"])
	if err != nil {
		return attrs, err
	}
	if strings.TrimSpace(values["seventeen-plus"]) != "" {
		return attrs, fmt.Errorf("--seventeen-plus is not supported by the App Store Connect API")
	}
	unrestrictedWebAccess, err := shared.ParseOptionalBoolFlag("--unrestricted-web-access", values["unrestricted-web-access"])
	if err != nil {
		return attrs, err
	}

	gamblingSimulated, err := parseOptionalEnumFlag("--gambling-simulated", values["gambling-simulated"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	alcoholTobaccoDrug, err := parseOptionalEnumFlag("--alcohol-tobacco-drug-use", values["alcohol-tobacco-drug-use"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	contests, err := parseOptionalEnumFlag("--contests", values["contests"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	medicalTreatment, err := parseOptionalEnumFlag("--medical-treatment", values["medical-treatment"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	profanityHumor, err := parseOptionalEnumFlag("--profanity-humor", values["profanity-humor"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	sexualContentNudity, err := parseOptionalEnumFlag("--sexual-content-nudity", values["sexual-content-nudity"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	sexualContentGraphicNudity, err := parseOptionalEnumFlag("--sexual-content-graphic-nudity", values["sexual-content-graphic-nudity"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	horrorFear, err := parseOptionalEnumFlag("--horror-fear", values["horror-fear"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	matureSuggestive, err := parseOptionalEnumFlag("--mature-suggestive", values["mature-suggestive"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	violenceCartoon, err := parseOptionalEnumFlag("--violence-cartoon", values["violence-cartoon"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	violenceRealistic, err := parseOptionalEnumFlag("--violence-realistic", values["violence-realistic"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	violenceRealisticGraphic, err := parseOptionalEnumFlag("--violence-realistic-graphic", values["violence-realistic-graphic"], ageRatingLevelValues)
	if err != nil {
		return attrs, err
	}
	kidsAgeBand, err := parseOptionalEnumFlag("--kids-age-band", values["kids-age-band"], kidsAgeBandValues)
	if err != nil {
		return attrs, err
	}

	attrs.Gambling = gambling
	attrs.UnrestrictedWebAccess = unrestrictedWebAccess
	attrs.GamblingSimulated = gamblingSimulated
	attrs.AlcoholTobaccoOrDrugUseOrReferences = alcoholTobaccoDrug
	attrs.Contests = contests
	attrs.MedicalOrTreatmentInformation = medicalTreatment
	attrs.ProfanityOrCrudeHumor = profanityHumor
	attrs.SexualContentOrNudity = sexualContentNudity
	attrs.SexualContentGraphicAndNudity = sexualContentGraphicNudity
	attrs.HorrorOrFearThemes = horrorFear
	attrs.MatureOrSuggestiveThemes = matureSuggestive
	attrs.ViolenceCartoonOrFantasy = violenceCartoon
	attrs.ViolenceRealistic = violenceRealistic
	attrs.ViolenceRealisticProlongedGraphicOrSadistic = violenceRealisticGraphic
	attrs.KidsAgeBand = kidsAgeBand

	return attrs, nil
}

func hasAgeRatingUpdates(attrs asc.AgeRatingDeclarationAttributes) bool {
	return attrs.Gambling != nil ||
		attrs.UnrestrictedWebAccess != nil ||
		attrs.GamblingSimulated != nil ||
		attrs.AlcoholTobaccoOrDrugUseOrReferences != nil ||
		attrs.Contests != nil ||
		attrs.MedicalOrTreatmentInformation != nil ||
		attrs.ProfanityOrCrudeHumor != nil ||
		attrs.SexualContentOrNudity != nil ||
		attrs.SexualContentGraphicAndNudity != nil ||
		attrs.HorrorOrFearThemes != nil ||
		attrs.MatureOrSuggestiveThemes != nil ||
		attrs.ViolenceCartoonOrFantasy != nil ||
		attrs.ViolenceRealistic != nil ||
		attrs.ViolenceRealisticProlongedGraphicOrSadistic != nil ||
		attrs.KidsAgeBand != nil
}

func parseOptionalEnumFlag(name, raw string, allowed []string) (*string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	normalized := strings.ToUpper(raw)
	for _, value := range allowed {
		if normalized == value {
			return &normalized, nil
		}
	}
	return nil, fmt.Errorf("%s must be one of: %s", name, strings.Join(allowed, ", "))
}
