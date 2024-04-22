package fanin

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/alitto/pond"
)

func TestFanIn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// capturing ctrl+C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)
	// goroutine to capture signal
	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		cancel()
		done <- true
	}()

	// main logic
	const workerCount = 1000
	waiter := NewFanIn[int](workerCount, 500*time.Millisecond, func(v []int) error {
		fmt.Printf("FanIn[%T].flush: %v\n", v[0], len(v))
		return nil
	})
	go waiter.ProcessLoop(ctx)

	// max queue/worker will filled first before tick reached
	const EventsCount = 10_203

	workerPool := pond.New(workerCount, EventsCount)
	go func() {
		for z := EventsCount; z > 0; z-- { // assuming there's at least 10K items to be written
			z := z
			workerPool.Submit(func() { // assuming this is goroutine of queue/pubsub/msgbroker client
				// preprocessing here
				err := <-waiter.SubmitWaitChan(z)
				if err != nil {
					t.Errorf("SubmitWaitChan error: %v", err)
				}
				// ack the event here
			})
		}
		workerPool.StopAndWaitFor(5 * time.Second)
		done <- true
	}()

	fmt.Println("awaiting signal")
	<-done
	fmt.Println("exiting")
	fmt.Println("total flushed: ", waiter.TotalFlushSucceed)

	if waiter.TotalFlushSucceed != EventsCount {
		t.Errorf("TotalFlushSucceed want %d got %d", EventsCount, waiter.TotalFlushSucceed)
	}
	if waiter.TotalFlushFailed != 0 {
		t.Errorf("TotalFlushFailed want %d got %d", 0, waiter.TotalFlushFailed)
	}
}

func TestWaitForTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// capturing ctrl+C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)
	// goroutine to capture signal
	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		cancel()
		done <- true
	}()

	// main logic
	const workerCount = 1000
	waiter := NewFanIn[int](workerCount, 20*time.Millisecond, func(v []int) error {
		fmt.Printf("FanIn[%T].flush: %v\n", v[0], len(v))
		return nil
	})
	go waiter.ProcessLoop(ctx)

	// tick will reached first before 1000 queue filled, so flush will only around ~20
	const EventsCount = 203

	workerPool := pond.New(workerCount, EventsCount)
	go func() {
		for z := EventsCount; z > 0; z-- { // assuming there's at least 10K items to be written
			z := z
			time.Sleep(1 * time.Millisecond)
			workerPool.Submit(func() { // assuming this is goroutine of queue/pubsub/msgbroker client
				// preprocessing here
				err := <-waiter.SubmitWaitChan(z)
				if err != nil {
					t.Errorf("SubmitWaitChan error: %v", err)
				}
				// ack the event here
			})
		}
		workerPool.StopAndWaitFor(5 * time.Second)
		done <- true
	}()

	fmt.Println("awaiting signal")
	<-done
	fmt.Println("exiting")
	fmt.Println("total flushed: ", waiter.TotalFlushSucceed)

	if waiter.TotalFlushSucceed != EventsCount {
		t.Errorf("TotalFlushSucceed want %d got %d", EventsCount, waiter.TotalFlushSucceed)
	}
	if waiter.TotalFlushFailed != 0 {
		t.Errorf("TotalFlushFailed want %d got %d", 0, waiter.TotalFlushFailed)
	}
}
