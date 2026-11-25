package subscriptions

import (
	"context"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type reconciledMutationStatus string

const (
	reconciledMutationCreated    reconciledMutationStatus = "created"
	reconciledMutationSkipped    reconciledMutationStatus = "skipped"
	reconciledMutationReconciled reconciledMutationStatus = "reconciled"
)

func runReconciledMutation(
	ctx context.Context,
	d *shared.Deps,
	readback func(context.Context) (bool, error),
	mutate func(context.Context) error,
) (reconciledMutationStatus, error) {
	_, status, err := shared.RunReconciledMutationWith(
		ctx,
		d,
		func(requestCtx context.Context) (struct{}, error) {
			return struct{}{}, mutate(requestCtx)
		},
		func(readbackCtx context.Context) (struct{}, bool, error) {
			exists, readErr := readback(readbackCtx)
			return struct{}{}, exists, readErr
		},
	)
	if err != nil {
		return "", err
	}
	if status == shared.ReconciledMutationRecovered {
		return reconciledMutationReconciled, nil
	}
	return reconciledMutationCreated, nil
}

// A 429 is retried after backoff in the client; an ambiguous failure may be
// retried by the caller after reconciliation. Both need fresh price eligibility.
func writeSubscriptionPrice(
	ctx context.Context,
	d *shared.Deps,
	replay bool,
	matches func(context.Context) (bool, error),
	eligible func(context.Context) error,
	write func(context.Context) error,
) (bool, error) {
	return asc.RetryGuardedMutation(ctx, func(attemptCtx context.Context, rateLimited bool) (bool, error) {
		if rateLimited && matches != nil {
			matched, err := matches(attemptCtx)
			if err != nil || matched {
				return matched, err
			}
		}
		if replay || rateLimited {
			if err := eligible(attemptCtx); err != nil {
				return false, err
			}
		}
		writeCtx, cancel := d.Timeout(attemptCtx)
		defer cancel()
		return false, write(writeCtx)
	})
}

func runReconciledPriceMutation(
	ctx context.Context,
	d *shared.Deps,
	matches func(context.Context) (bool, error),
	eligible func(context.Context) error,
	write func(context.Context) error,
) (reconciledMutationStatus, error) {
	replay := false
	matched, _, err := shared.RunReconciledMultiRequestMutationWith(ctx, d,
		func(attemptCtx context.Context) (bool, error) {
			matched, err := writeSubscriptionPrice(attemptCtx, d, replay, matches, eligible, write)
			replay = true
			return matched, err
		},
		func(readCtx context.Context) (bool, bool, error) {
			matched, err := matches(readCtx)
			return matched, matched, err
		}, true)
	if err != nil {
		return "", err
	}
	if matched {
		return reconciledMutationReconciled, nil
	}
	return reconciledMutationCreated, nil
}
