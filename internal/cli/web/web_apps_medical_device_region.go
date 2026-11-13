package web

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const maxMedicalDeviceRegionInputSize = 1 << 20

type webMedicalDeviceRegionSupportInput struct {
	Locale      string `json:"locale"`
	Instruction string `json:"instruction"`
	Statement   string `json:"statement"`
	SafetyInfo  string `json:"safetyInfo"`
}

type webMedicalDeviceRegionInput struct {
	Declaration        *bool                                 `json:"declaration"`
	RegistrationNumber *string                               `json:"registrationNumber"`
	SupportInfo        *[]webMedicalDeviceRegionSupportInput `json:"supportInfo"`
}

type webMedicalDeviceRegionInputMetadata struct {
	RegistrationNumberProvided bool
	SupportInfoProvided        bool
}

var setWebMedicalDeviceRegionFn = func(ctx context.Context, client *webcore.Client, accountID, appID, region string, options webcore.MedicalDeviceRegionOptions) (*webcore.MedicalDeviceRegionResult, error) {
	return client.SetMedicalDeviceRegion(ctx, accountID, appID, region, options)
}

// WebAppsMedicalDeviceRegionCommand returns the detailed regional command group.
func WebAppsMedicalDeviceRegionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps medical-device region", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "region",
		ShortUsage: "asc web apps medical-device region <subcommand> [flags]",
		ShortHelp:  "Manage one detailed medical-device region answer.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage one detailed region in Apple's regulated medical-device compliance form.
The app-level declaration must already be "yes". This command preserves the
form, existing contacts, and unrelated region rows, then verifies the form
after a single Apple web-session PUT.

Use ` + "`set`" + ` with a rootfs-anchored JSON input file. Apple does not expose
these fields on the public App Store Connect API.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsMedicalDeviceRegionSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppsMedicalDeviceRegionSetCommand sets one detailed regional answer.
func WebAppsMedicalDeviceRegionSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps medical-device region set", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	region := fs.String("region", "", "Medical-device region: EEA, GBR, USA (EU aliases EEA)")
	inputPath := fs.String("input", "", "Rootfs-anchored JSON input file path (required)")
	confirm := fs.Bool("confirm", false, "Confirm saving the detailed regulated medical-device answer")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web apps medical-device region set --app APP_ID --region REGION --input PATH --confirm [flags]",
		ShortHelp:  "Set and verify one detailed medical-device region answer.",
		LongHelp: `WEB SESSION WORKFLOWS

Set one detailed regional answer through Apple's compliance-form web endpoint.
The app-level declaration must already be "yes". Input JSON must contain a
boolean ` + "`declaration`" + ` field. For ` + "`true`" + `, USA and EEA require
` + "`registrationNumber`" + ` and every support locale requires
` + "`instruction`" + `, ` + "`statement`" + `, and ` + "`safetyInfo`" + `. For ` + "`false`" + `,
the existing regional details are preserved. Contact records are read from
the current form and are never supplied or printed by this command.

Examples:
  asc web apps medical-device region set --app "6759231657" --region "GBR" --input "./medical-gbr.json" --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			regionValues, err := webcore.NormalizeMedicalDeviceDeclarationRegions([]string{*region})
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(regionValues) != 1 {
				return shared.UsageError("--region is required")
			}
			resolvedRegion := regionValues[0]

			pathValue := strings.TrimSpace(*inputPath)
			if pathValue == "" {
				return shared.UsageError("--input is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}

			options, metadata, err := readWebMedicalDeviceRegionInput(pathValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if metadata.RegistrationNumberProvided && !options.Declaration {
				return shared.UsageError("registrationNumber is only supported when declaration is true")
			}
			if metadata.SupportInfoProvided && !options.Declaration {
				return shared.UsageError("supportInfo is only supported when declaration is true")
			}
			if err := webcore.ValidateMedicalDeviceRegionOptions(resolvedRegion, options); err != nil {
				return shared.UsageError(err.Error())
			}

			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, authFlags, "web apps medical-device region set")
			defer cancel()
			if err != nil {
				return err
			}

			var result *webcore.MedicalDeviceRegionResult
			err = withWebSpinner("Saving medical device region", func() error {
				var err error
				result, err = setWebMedicalDeviceRegionFn(requestCtx, client, accountID, resolvedAppID, resolvedRegion, options)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps medical-device region set")
			}
			if result == nil {
				return fmt.Errorf("web apps medical-device region set failed: missing region result")
			}

			return shared.PrintOutput(webMedicalDeviceRegionResultOutput(result), *output.Output, *output.Pretty)
		},
	}
}

func readWebMedicalDeviceRegionInput(path string) (webcore.MedicalDeviceRegionOptions, webMedicalDeviceRegionInputMetadata, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("open --input: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxMedicalDeviceRegionInputSize+1))
	if err != nil {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("read --input: %w", err)
	}
	if len(data) > maxMedicalDeviceRegionInputSize {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("--input exceeds %d-byte limit", maxMedicalDeviceRegionInputSize)
	}
	if strings.TrimSpace(string(data)) == "" {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("--input is empty")
	}

	var fields map[string]json.RawMessage
	if err := decodeStrictMedicalDeviceRegionJSON(data, &fields); err != nil {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("invalid --input JSON: %w", err)
	}
	allowed := map[string]struct{}{
		"declaration": {}, "registrationNumber": {}, "supportInfo": {},
	}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("unsupported --input field %q", key)
		}
	}

	var input webMedicalDeviceRegionInput
	if err := decodeStrictMedicalDeviceRegionJSON(data, &input); err != nil {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("invalid --input JSON: %w", err)
	}
	if input.Declaration == nil {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("--input declaration must be a boolean")
	}
	if raw, ok := fields["registrationNumber"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || input.RegistrationNumber == nil {
			return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("--input registrationNumber must be a string")
		}
	}
	if raw, ok := fields["supportInfo"]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return webcore.MedicalDeviceRegionOptions{}, webMedicalDeviceRegionInputMetadata{}, fmt.Errorf("--input supportInfo must be an array")
	}

	options := webcore.MedicalDeviceRegionOptions{
		Declaration: *input.Declaration,
	}
	if input.RegistrationNumber != nil {
		options.RegistrationNumber = *input.RegistrationNumber
	}
	if input.SupportInfo != nil {
		options.SupportInfo = make([]webcore.MedicalDeviceRegionSupportInfo, 0, len(*input.SupportInfo))
		for _, support := range *input.SupportInfo {
			options.SupportInfo = append(options.SupportInfo, webcore.MedicalDeviceRegionSupportInfo{
				Locale:      support.Locale,
				Instruction: support.Instruction,
				Statement:   support.Statement,
				SafetyInfo:  support.SafetyInfo,
			})
		}
	}
	_, registrationNumberProvided := fields["registrationNumber"]
	_, supportInfoProvided := fields["supportInfo"]
	return options, webMedicalDeviceRegionInputMetadata{
		RegistrationNumberProvided: registrationNumberProvided,
		SupportInfoProvided:        supportInfoProvided,
	}, nil
}

func decodeStrictMedicalDeviceRegionJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values found")
		}
		return err
	}
	return nil
}

func webMedicalDeviceRegionResultOutput(result *webcore.MedicalDeviceRegionResult) *asc.WebMedicalDeviceRegionResult {
	if result == nil {
		return nil
	}
	return &asc.WebMedicalDeviceRegionResult{
		AppID:           result.AppID,
		RequirementID:   result.RequirementID,
		RequirementName: result.RequirementName,
		Status:          result.Status,
		FormID:          result.FormID,
		Region:          result.Region,
		Declared:        result.Declared,
		Changed:         result.Changed,
	}
}
