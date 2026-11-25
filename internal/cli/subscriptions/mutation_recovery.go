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
		return false, write(attemptCtx)
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
	var matched bool
	status, err := runReconciledMutation(ctx, d, matches, func(requestCtx context.Context) error {
		var err error
		matched, err = writeSubscriptionPrice(requestCtx, replay, matches, eligible, write)
		replay = true
		return err
	})
	if matched && err == nil {
		return reconciledMutationReconciled, nil
	}
	return status, err
}
