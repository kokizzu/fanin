
# FanIn

A library similar to [ch-timed-buffer](//github.com/kokizzu/ch-timed-buffer), 
used to wait synchronization of multiple goroutines (`pond` can be used to fan-out, this library used to synchronize fan-in, so just like inverse of wait-group). The difference is that this one can notify the caller when it's done flushing using callback or channel, database-agnostics, and no double buffering (so slow flush function might have effect).

So something like this:

```
# enqueue things to write
goroutine1 -> writer
goroutine2 -> writer
goroutine3 -> writer

# writer done flushing all 3

# notify back the callers
writer -> goroutine1   
writer -> goroutine2
writer -> goroutine3  
```

For example if you got multiple goroutine from message-broker/pubsub, 
write in batch, and you only want to ack the message if already flushed so no data loss.

## Example Usage

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kokizzu/fanin"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const workerCount = 1000
	waiter := fanin.NewFanIn[int](workerCount, 1 * time.Second, func(v []int) error {
		fmt.Printf("FanIn[%T].flush: %v\n", v[0], len(v))
		return nil // no error when flushing
	})
	go waiter.ProcessLoop(ctx)

	const EventsCount = 10_203

	var wg sync.WaitGroup
	workerLimit := make(chan struct{}, workerCount)
	for z := EventsCount; z > 0; z-- { // assuming there's at least 10K items to be written
		z := z
		workerLimit <- struct{}{}
		wg.Add(1)
		go func() { // assuming this is goroutine of queue/pubsub/msgbroker client
			defer wg.Done()
			defer func() { <-workerLimit }()
			// preprocessing here
			err := <-waiter.SubmitWaitChan(z)
			if err != nil {
				// handle error here
				return
			}
			// ack the event here
		}()
	}
	wg.Wait()
	fmt.Println("total flushed: ", waiter.TotalFlushSucceed)
}
```

output:

```
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 1000
FanIn[int].flush: 203
total flushed:  10203
```

## Maintenance checklist

- [x] Go runtime updated to 1.26.5.
- [x] Removed unnecessary external test dependency; the module is dependency-free.
- [x] `make test` covers full-batch flushing, tick flushing, and flush error propagation.
- [x] `make verify-dependency-security` and `make vulncheck` check dependency security.
