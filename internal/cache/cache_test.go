package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrSet_SingleFlight(t *testing.T) {
	c := New()
	var calls atomic.Int32

	fn := func() (any, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "value", nil
	}

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			val, err := c.GetOrSet("key", time.Minute, fn)
			require.NoError(t, err)
			assert.Equal(t, "value", val)
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Only one goroutine should have executed the factory despite 10 concurrent
	// callers for the same key.
	assert.Equal(t, int32(1), calls.Load())

	// Subsequent call hits the cache without re-running the factory.
	val, found := c.Get("key")
	assert.True(t, found)
	assert.Equal(t, "value", val)
}

func TestGetOrSet_ErrorNotCached(t *testing.T) {
	c := New()
	var calls atomic.Int32

	fn := func() (any, error) {
		calls.Add(1)
		return nil, assert.AnError
	}

	val, err := c.GetOrSet("key", time.Minute, fn)
	require.Error(t, err)
	assert.Nil(t, val)

	// Error results are not cached and not retained in the inflight map.
	val, err = c.GetOrSet("key", time.Minute, fn)
	require.Error(t, err)
	assert.Nil(t, val)
	assert.Equal(t, int32(2), calls.Load())
}

func TestGetOrSet_FactoryPanicReleasesWaiters(t *testing.T) {
	c := New()
	var active, maxActive atomic.Int32

	fn := func() (any, error) {
		cur := active.Add(1)
		for {
			m := maxActive.Load()
			if cur <= m || maxActive.CompareAndSwap(m, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		panic("boom")
	}

	// A panicking factory must not strand concurrent or future callers, and
	// must never run more than one factory at a time for the same key.
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = c.GetOrSet("key", time.Minute, fn)
		}(i)
	}
	wg.Wait()

	// All callers observed an error (recovered panic), none hung.
	for i, err := range errs {
		require.Error(t, err, "caller %d should receive an error, not hang", i)
	}

	// Single-flight: never more than one concurrent factory execution.
	assert.LessOrEqual(t, maxActive.Load(), int32(1))

	// A subsequent call runs the factory again (inflight entry was cleaned up).
	_, err := c.GetOrSet("key", time.Minute, fn)
	require.Error(t, err)
	assert.LessOrEqual(t, maxActive.Load(), int32(1))
}
