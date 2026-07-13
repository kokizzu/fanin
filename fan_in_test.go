package fanin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFanInFlushesFullBatches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const batchSize = 64
	const eventsCount = 256

	var flushCount atomic.Uint64
	waiter := NewFanIn[int](batchSize, time.Second, func(v []int) error {
		if len(v) == 0 {
			return errors.New("empty flush")
		}
		if len(v) > batchSize {
			return fmt.Errorf("flush batch too large: %d", len(v))
		}
		flushCount.Add(1)
		return nil
	})

	processDone := runFanIn(ctx, waiter)

	var wg sync.WaitGroup
	wg.Add(eventsCount)
	for z := 0; z < eventsCount; z++ {
		z := z
		go func() {
			defer wg.Done()
			if err := <-waiter.SubmitWaitChan(z); err != nil {
				t.Errorf("SubmitWaitChan error: %v", err)
			}
		}()
	}
	waitFor(t, waitGroupDone(&wg), time.Second)

	cancel()
	waitFor(t, processDone, time.Second)

	if waiter.TotalFlushSucceed != eventsCount {
		t.Fatalf("TotalFlushSucceed want %d got %d", eventsCount, waiter.TotalFlushSucceed)
	}
	if waiter.TotalFlushFailed != 0 {
		t.Fatalf("TotalFlushFailed want 0 got %d", waiter.TotalFlushFailed)
	}
	if flushCount.Load() == 0 {
		t.Fatal("expected at least one flush")
	}
}

func TestFanInFlushesOnTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const eventsCount = 5

	var flushCount atomic.Uint64
	waiter := NewFanIn[int](16, 10*time.Millisecond, func(v []int) error {
		if len(v) == 0 {
			return errors.New("empty flush")
		}
		flushCount.Add(1)
		return nil
	})

	processDone := runFanIn(ctx, waiter)

	for z := 0; z < eventsCount; z++ {
		if err := <-waiter.SubmitWaitChan(z); err != nil {
			t.Fatalf("SubmitWaitChan error: %v", err)
		}
	}

	cancel()
	waitFor(t, processDone, time.Second)

	if waiter.TotalFlushSucceed != eventsCount {
		t.Fatalf("TotalFlushSucceed want %d got %d", eventsCount, waiter.TotalFlushSucceed)
	}
	if flushCount.Load() != eventsCount {
		t.Fatalf("tick flush count want %d got %d", eventsCount, flushCount.Load())
	}
}

func TestFanInPropagatesFlushFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wantErr := errors.New("flush failed")
	waiter := NewFanIn[int](1, time.Second, func(v []int) error {
		return wantErr
	})

	processDone := runFanIn(ctx, waiter)

	if err := <-waiter.SubmitWaitChan(42); !errors.Is(err, wantErr) {
		t.Fatalf("SubmitWaitChan error want %v got %v", wantErr, err)
	}

	cancel()
	waitFor(t, processDone, time.Second)

	if waiter.TotalFlushSucceed != 0 {
		t.Fatalf("TotalFlushSucceed want 0 got %d", waiter.TotalFlushSucceed)
	}
	if waiter.TotalFlushFailed != 1 {
		t.Fatalf("TotalFlushFailed want 1 got %d", waiter.TotalFlushFailed)
	}
}

func runFanIn[T any](ctx context.Context, waiter *FanIn[T]) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		waiter.ProcessLoop(ctx)
	}()
	return done
}

func waitGroupDone(wg *sync.WaitGroup) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	return done
}

func waitFor(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("timed out after %s", timeout)
	}
}
