package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWithConcurrencyLimitsParallelism(t *testing.T) {
	var active atomic.Int32
	var maxActive atomic.Int32

	runWithConcurrency(2, 8, func(int) {
		current := active.Add(1)
		for {
			previous := maxActive.Load()
			if current <= previous || maxActive.CompareAndSwap(previous, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
	})

	if got := maxActive.Load(); got > 2 {
		t.Fatalf("max concurrent workers = %d, want <= 2", got)
	}
}

func TestBoundedStudioConcurrencyCapsLargeFanOut(t *testing.T) {
	if got := boundedStudioConcurrency(100); got != studioSubprocessConcurrency {
		t.Fatalf("boundedStudioConcurrency(100) = %d, want %d", got, studioSubprocessConcurrency)
	}
	if got := boundedStudioConcurrency(3); got != 3 {
		t.Fatalf("boundedStudioConcurrency(3) = %d, want 3", got)
	}
}
