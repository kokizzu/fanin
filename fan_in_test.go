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
	waiter := NewFanIn[int](workerCount, 500*time.Millisecond, func(v []int) {
		fmt.Printf("FanIn[%T].flush: %v\n", v[0], len(v))
	})
	go waiter.ProcessLoop(ctx)

	const EventsCount = 10_203

	workerPool := pond.New(workerCount, EventsCount)
	go func() {
		for z := EventsCount; z > 0; z-- { // assuming there's at least 10K items to be written
			z := z
			workerPool.Submit(func() { // assuming this is goroutine of queue/pubsub/msgbroker client
				// preprocessing here
				<-waiter.SubmitWaitChan(z)
				// ack the event here
			})
		}
		workerPool.StopAndWaitFor(5 * time.Second)
		done <- true
	}()

	fmt.Println("awaiting signal")
	<-done
	fmt.Println("exiting")
	fmt.Println("total flushed: ", waiter.TotalFlushed)

	if waiter.TotalFlushed != 10203 {
		t.Errorf("TotalFlushed want %d got %d", 10203, waiter.TotalFlushed)
	}
}
