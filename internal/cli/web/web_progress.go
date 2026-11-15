package web

import (
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const webSpinnerDelay = 200 * time.Millisecond

var webWithSpinnerDelayedFn = shared.WithSpinnerDelayed

func withWebSpinner(label string, fn func() error) error {
	return webWithSpinnerDelayedFn(label, webSpinnerDelay, fn)
}

func withWebSpinnerValue[T any](label string, fn func() (T, error)) (T, error) {
	var result T
	err := withWebSpinner(label, func() error {
		var innerErr error
		result, innerErr = fn()
		return innerErr
	})
	return result, err
}

func withWebSpinnerWith(d *shared.Deps, label string, fn func() error) error {
	return d.WithSpinnerDelayed(label, webSpinnerDelay, fn)
}

func withWebSpinnerValueWith[T any](d *shared.Deps, label string, fn func() (T, error)) (T, error) {
	var result T
	err := withWebSpinnerWith(d, label, func() error {
		var innerErr error
		result, innerErr = fn()
		return innerErr
	})
	return result, err
}
