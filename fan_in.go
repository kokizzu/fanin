package fanin

import (
	"context"
	"time"
)

func NewFanIn[T any](n int, tickDuration time.Duration, flushFunc func([]T)) *FanIn[T] {
	return &FanIn[T]{
		queue:        make(chan DataAndCallback[T], n),
		writeStream:  make([]T, 0, n),
		callbacks:    make([]func(), 0, n),
		flushFunc:    flushFunc,
		tickDuration: tickDuration,
	}
}

type FanIn[T any] struct {
	// write queue so no deadlock
	queue chan DataAndCallback[T]

	// maximum duration before automatically flush
	tickDuration time.Duration

	// list of items to be processed, and callbacks
	writeStream []T
	callbacks   []func()

	// just for statistsics
	TotalFlushed uint64

	// function that will called when it's time to flush
	flushFunc func([]T)
}

type DataAndCallback[T any] struct {
	data     T
	callback func()
}

func (w *FanIn[T]) ProcessLoop(ctx context.Context) {

	tick := time.NewTicker(w.tickDuration)

mainLoop:
	for { // synchronize writer and flusher
		select {
		case <-tick.C:
			w.flush(true)
		case v := <-w.queue:
			w.writeStream = append(w.writeStream, v.data)
			w.callbacks = append(w.callbacks, v.callback)
			w.flush(false)
		case <-ctx.Done():
			w.flush(true)
			break mainLoop
		}
	}
}

func (w *FanIn[T]) SubmitCallback(z T, callback func()) {
	w.queue <- DataAndCallback[T]{data: z, callback: callback}
}

func (w *FanIn[T]) SubmitWaitChan(z T) chan struct{} {
	isDataFlushed := make(chan struct{})
	w.queue <- DataAndCallback[T]{data: z, callback: func() {
		isDataFlushed <- struct{}{}
	}}
	return isDataFlushed
}

func (w *FanIn[T]) flush(force bool) {
	if len(w.callbacks) == 0 { // nothing to flush
		return
	}
	if !force && len(w.callbacks) < cap(w.callbacks) { // not yet time to flush, data too small
		return
	}
	w.flushFunc(w.writeStream)
	w.TotalFlushed += uint64(len(w.writeStream))

	// call callbacks
	for _, callback := range w.callbacks {
		callback()
	}
	// clear the stream
	w.writeStream = w.writeStream[:0]
	w.callbacks = w.callbacks[:0]
}
