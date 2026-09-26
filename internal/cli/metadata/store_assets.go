package metadata

import (
	"context"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/storeassets"
)

func includesScope(includes []string, scope string) bool {
	for _, value := range includes {
		if value == scope {
			return true
		}
	}
	return false
}

func loadStoreAssetInputs(ctx context.Context, dir string, includes []string) (*storeassets.AppClipLayout, []storeassets.PreviewLayout, func(), error) {
	var clip *storeassets.AppClipLayout
	var previews []storeassets.PreviewLayout
	if includesScope(includes, "app-clip") {
		layout, present, err := storeassets.ReadAppClip(dir)
		if err != nil {
			return nil, nil, nil, err
		}
		if present {
			clip = &layout
		}
	}
	if includesScope(includes, "previews") {
		var err error
		previews, err = storeassets.ReadPreviews(dir)
		if err != nil {
			clip.Close()
			return nil, nil, nil, err
		}
	}
	cleanup, err := storeassets.Validate(ctx, clip, previews)
	if err != nil {
		clip.Close()
		storeassets.ClosePreviews(previews)
		return nil, nil, nil, err
	}
	return clip, previews, func() { cleanup(); clip.Close(); storeassets.ClosePreviews(previews) }, nil
}

func addStoreAssetChanges(result *PushPlanResult, plan *storeassets.ImportPlan) {
	for _, change := range plan.Changes() {
		item := PlanItem{Key: "store-assets/" + change.Kind + "/" + change.Locale + "/" + change.Path, Scope: "store-assets", Locale: change.Locale, Version: result.Version, Field: change.Path, Reason: change.Action, From: change.From, To: change.To}
		switch change.Action {
		case "create", "upload":
			result.Adds = append(result.Adds, item)
		case "replace":
			// Header replacement deletes the existing asset before reserving
			// its replacement, so the normal deletion authorization applies.
			result.Deletes = append(result.Deletes, item)
		default:
			result.Updates = append(result.Updates, item)
		}
	}
	sortPlanItems(result.Adds)
	sortPlanItems(result.Updates)
	sortPlanItems(result.Deletes)
}

func appendStoreAssetActions(result *PushPlanResult, receipts []asc.StoreAssetResult) {
	result.AssetResults = receipts
	for _, receipt := range receipts {
		if receipt.Status == "skipped" {
			continue
		}
		action := ApplyAction{Scope: receipt.Kind, Locale: receipt.Locale, Version: result.Version, Action: receipt.Action, Status: receipt.Status, LocalizationID: receipt.ID, Error: receipt.Error}
		if receipt.PreviousDeleted {
			action.DesiredFields = map[string]string{"deletedHeaderId": receipt.PreviousID}
		}
		if action.Status == "applied" {
			action.Status = "succeeded"
		}
		result.Actions = append(result.Actions, action)
	}
}

func requireConfirmedStoreAssets(opts PushExecutionOptions, clip *storeassets.AppClipLayout, previews []storeassets.PreviewLayout) error {
	if !opts.DryRun && !opts.Confirm && (clip != nil || len(previews) > 0) {
		return shared.UsageError("--confirm is required when applying App Clip or preview assets")
	}
	return nil
}
