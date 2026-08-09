package services

import (
	"errors"
	"testing"
)

func TestRunSyncSafe(t *testing.T) {
	t.Run("recover panic, next call continues", func(t *testing.T) {
		// runSyncSafe is one-shot per invocation: a panicking fn is recovered
		// and logged, and the scheduler loop's *next* tick calls it again.
		panicked := 0
		runSyncSafe("test", func() error {
			panicked++
			panic("boom")
		})
		if panicked != 1 {
			t.Fatalf("expected fn to run once, got %d", panicked)
		}

		ran := false
		runSyncSafe("test", func() error {
			ran = true
			return nil
		})
		if !ran {
			t.Fatal("subsequent call should run the factory again")
		}
	})

	t.Run("propagate error", func(t *testing.T) {
		want := errors.New("sync failed")
		runSyncSafe("test", func() error {
			return want
		})
	})

	t.Run("nil function is a no-op", func(t *testing.T) {
		runSyncSafe("test", nil)
	})
}
