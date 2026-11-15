package release

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// verifyResumedCheckpointBinding re-establishes a checkpoint's app, version,
// build, and submission binding from authenticated API state.
//
// The checkpoint file is unsigned local JSON, so matching user-facing arguments
// proves nothing about the IDs and completed-step flags stored alongside them. A
// stored ID that cannot be tied back to the selected app, version string, and
// platform aborts the run. A completed mutation step that current API state
// contradicts is discarded so the step runs again against the verified target
// instead of being reported as done.
func verifyResumedCheckpointBinding(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	opts runOptions,
	checkpoint *runCheckpoint,
	emit func(string),
) error {
	if checkpoint == nil {
		return nil
	}
	emitMessage := func(format string, args ...any) {
		message := fmt.Sprintf(format, args...)
		if emit != nil {
			emit(message)
			return
		}
		fmt.Fprintln(d.IO.ErrOut, message)
	}

	for name, completed := range checkpoint.Completed {
		if !completed {
			delete(checkpoint.Completed, name)
			continue
		}
		if !isReleasePipelineStep(name, strings.TrimSpace(opts.RoutingCoverageFile) != "") {
			return fmt.Errorf(
				"checkpoint records completed step %q, which `asc release stage` does not run; delete the checkpoint file or pass a different --checkpoint-file to start a new run",
				name,
			)
		}
	}

	versionID := strings.TrimSpace(checkpoint.VersionID)
	if versionID == "" {
		if len(checkpoint.Completed) > 0 {
			return fmt.Errorf("checkpoint reports completed steps without a version ID to verify them against")
		}
		return nil
	}

	version, err := shared.ResolveOwnedAppStoreVersionByID(ctx, client, opts.AppID, versionID, opts.Platform)
	if err != nil {
		return fmt.Errorf("checkpoint version %s could not be verified: %w", versionID, err)
	}
	if resolved := strings.TrimSpace(version.Attributes.VersionString); !strings.EqualFold(resolved, strings.TrimSpace(opts.Version)) {
		return fmt.Errorf("checkpoint version %s is version %q, not %q", versionID, resolved, strings.TrimSpace(opts.Version))
	}

	// These completions cannot be authenticated from current remote state.
	// Local inputs may have changed since the checkpoint was written, and
	// readiness is a point-in-time observation rather than a durable server
	// state. An unsigned checkpoint must never be able to suppress these steps.
	if checkpoint.Completed[stepApplyMetadata] {
		delete(checkpoint.Completed, stepApplyMetadata)
		emitMessage("Rechecking %s: an unsigned checkpoint cannot prove the current metadata input was applied.", stepApplyMetadata)
	}
	if checkpoint.Completed[stepApplyRoutingCoverage] {
		delete(checkpoint.Completed, stepApplyRoutingCoverage)
		emitMessage("Rechecking %s: an unsigned checkpoint cannot prove the current routing coverage file was applied.", stepApplyRoutingCoverage)
	}
	if checkpoint.Completed[stepValidateReadiness] {
		delete(checkpoint.Completed, stepValidateReadiness)
		emitMessage("Rechecking %s: readiness must be evaluated again against current App Store Connect state.", stepValidateReadiness)
	}

	if checkpoint.Completed[stepAttachBuild] {
		attachedBuildID, buildErr := attachedAppStoreVersionBuildID(ctx, client, versionID)
		attachDiscarded := false
		switch {
		case buildErr != nil:
			attachDiscarded = true
			delete(checkpoint.Completed, stepAttachBuild)
			emitMessage("Rechecking %s: could not confirm the build attached to version %s (%v).", stepAttachBuild, versionID, buildErr)
		case attachedBuildID != strings.TrimSpace(opts.BuildID):
			attachDiscarded = true
			delete(checkpoint.Completed, stepAttachBuild)
			emitMessage(
				"Rechecking %s: version %s currently has build %q attached, not %q.",
				stepAttachBuild,
				versionID,
				attachedBuildID,
				strings.TrimSpace(opts.BuildID),
			)
		}
		// Readiness was validated against whatever build was attached at the
		// time, so an unproven attachment invalidates that result as well.
		if attachDiscarded && checkpoint.Completed[stepValidateReadiness] {
			delete(checkpoint.Completed, stepValidateReadiness)
			emitMessage("Rechecking %s: it depends on %s, which could not be confirmed.", stepValidateReadiness, stepAttachBuild)
		}
	}

	// A real run completes the pipeline in order and persists each step before
	// the next one starts, so a completed validate_readiness always follows
	// completed prerequisites. An unsigned checkpoint can claim otherwise: the
	// pipeline would then apply the missing mutation and skip readiness, leaving
	// the version unvalidated against the state that mutation produced.
	if checkpoint.Completed[stepValidateReadiness] {
		prerequisites := []string{stepEnsureVersion, stepApplyMetadata, stepAttachBuild}
		if strings.TrimSpace(opts.RoutingCoverageFile) != "" {
			prerequisites = append(prerequisites, stepApplyRoutingCoverage)
		}
		for _, prerequisite := range prerequisites {
			if checkpoint.Completed[prerequisite] {
				continue
			}
			delete(checkpoint.Completed, stepValidateReadiness)
			emitMessage(
				"Rechecking %s: prerequisite step %s is not complete, so readiness must run again after it does.",
				stepValidateReadiness,
				prerequisite,
			)
			break
		}
	}

	return nil
}

func isReleasePipelineStep(name string, hasRoutingCoverage bool) bool {
	switch name {
	case stepEnsureVersion, stepApplyMetadata, stepAttachBuild, stepValidateReadiness:
		return true
	case stepApplyRoutingCoverage:
		return hasRoutingCoverage
	default:
		return false
	}
}

func attachedAppStoreVersionBuildID(ctx context.Context, client *asc.Client, versionID string) (string, error) {
	resp, err := client.GetAppStoreVersionBuild(ctx, versionID)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty build response for version %s", versionID)
	}
	return strings.TrimSpace(resp.Data.ID), nil
}
