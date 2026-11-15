package subscriptions

import (
	"context"

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
