package ads

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type platformCampaignStatusWorkflowFlags struct {
	common   commonFlags
	output   shared.OutputFlags
	campaign *string
	confirm  *bool
}

func platformWorkflowSubcommands(d *shared.Deps, path []string) []*ffcli.Command {
	if len(path) != 1 || path[0] != "campaigns" {
		return nil
	}
	return []*ffcli.Command{
		platformCampaignStatusWorkflowCommand(d, "pause", "PAUSED", "Pause a Platform API campaign."),
		platformCampaignStatusWorkflowCommand(d, "resume", "ENABLED", "Resume a Platform API campaign."),
	}
}

func platformCampaignStatusWorkflowCommand(d *shared.Deps, name, status, shortHelp string) *ffcli.Command {
	fs := d.NewFlagSet("campaigns "+name, flag.ExitOnError)
	flags := platformCampaignStatusWorkflowFlags{
		common: commonFlags{
			AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
			AdAccount:  fs.String("ad-account", "", "Apple Ads ad account ID (or ASC_ADS_AD_ACCOUNT_ID env)"),
		},
		output:   bindAdsRawOutputFlags(fs),
		campaign: fs.String("campaign", "", "Apple Ads Platform campaign ID (required)"),
		confirm:  fs.Bool("confirm", false, "Confirm this Apple Ads Platform campaign status change"),
	}
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc ads campaigns " + name + " [flags]",
		ShortHelp:  shortHelp,
		LongHelp:   platformCampaignStatusWorkflowHelp(name, status, shortHelp),
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return executePlatformCampaignStatusWorkflow(ctx, d, name, status, flags)
		},
	}
}

func executePlatformCampaignStatusWorkflow(ctx context.Context, d *shared.Deps, commandName, status string, flags platformCampaignStatusWorkflowFlags) error {
	campaignID := value(flags.campaign)
	if campaignID == "" {
		return d.UsageError("--campaign is required")
	}
	if status != "PAUSED" {
		if err := d.RequireConfirm(flags.confirm != nil && *flags.confirm); err != nil {
			return err
		}
	}

	spec, ok := appleads.PlatformEndpointByCommandPath("campaigns", "update")
	if !ok {
		return fmt.Errorf("ads campaigns status workflow: missing campaigns update endpoint")
	}
	outputFormat, err := validateAdsRawOutput(flags.output)
	if err != nil {
		return d.UsageError(err.Error())
	}
	client, _, err := resolvePlatformClientAndAdAccountID(ctx, d, flags.common, spec.Context)
	if err != nil {
		return fmt.Errorf("ads campaigns %s: %w", commandName, err)
	}

	body, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return err
	}
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()
	result, err := client.Do(requestCtx, spec, map[string]string{"id": campaignID}, nil, body)
	if err != nil {
		return fmt.Errorf("ads campaigns %s: %w", commandName, err)
	}
	return d.PrintOutput(result, outputFormat, *flags.output.Pretty)
}

func platformCampaignStatusWorkflowHelp(name, status, shortHelp string) string {
	confirm := ""
	if status != "PAUSED" {
		confirm = " --confirm"
	}
	return fmt.Sprintf(`%s

Endpoint: PUT v1/campaigns/{id}
Payload: {"status":"%s"}

Pausing is a spend-reducing safety operation and does not require --confirm;
resuming requires --confirm.

Examples:
  asc ads campaigns %s --campaign CAMPAIGN_ID%s --ad-account AD_ACCOUNT_ID`, shortHelp, status, name, confirm)
}
