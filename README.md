
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
	"time"

	"github.com/alitto/pond"
	"github.com/kokizzu/fanin"
)

func main() {
	ctx := context.Background()
	done := make(chan bool)

	const workerCount = 1000
	waiter := fanin.NewFanIn[int](workerCount, 1 * time.Second, func(v []int) {
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

	<-done
	fmt.Println("total flushed: ", waiter.TotalFlushed)
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
