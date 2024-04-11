
# FanIn

A library similar to [ch-timed-buffer](//github.com/kokizzu/ch-timed-buffer), 
used to wait synchronization of multiple goroutines.

So something like this:

```
goroutine1 -> writer
goroutine2 -> writer
goroutine3 -> writer

writer done flushing all 3
  
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