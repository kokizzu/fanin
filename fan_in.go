package fanin

import (
	"context"
	"time"
)

func NewFanIn[T any](n int, tickDuration time.Duration, flushFunc func([]T) error) *FanIn[T] {
	return &FanIn[T]{
		queue:        make(chan DataAndCallback[T], n),
		writeStream:  make([]T, 0, n),
		callbacks:    make([]func(error), 0, n),
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
	callbacks   []func(error)

	// just for statistsics
	TotalFlushSucceed uint64
	TotalFlushFailed  uint64

	// function that will be called when it's time to flush
	flushFunc func([]T) error
}

type DataAndCallback[T any] struct {
	data     T
	callback func(err error)
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
			if w.flush(false) {
				tick.Reset(w.tickDuration)
			}
		case <-ctx.Done():
			w.flush(true)
			break mainLoop
		}
	}
}

func (w *FanIn[T]) SubmitCallback(z T, callback func(error)) {
	w.queue <- DataAndCallback[T]{data: z, callback: callback}
}

func (w *FanIn[T]) SubmitWaitChan(z T) chan error {
	isDataFlushed := make(chan error)
	w.queue <- DataAndCallback[T]{data: z, callback: func(err error) {
		isDataFlushed <- err
	}}
	return isDataFlushed
}

func (w *FanIn[T]) flush(force bool) bool {
	if len(w.callbacks) == 0 { // nothing to flush
		return false
	}
	if !force && len(w.callbacks) < cap(w.callbacks) { // not yet time to flush, data too small
		return false
	}
	err := w.flushFunc(w.writeStream)
	if err != nil {
		w.TotalFlushFailed += uint64(len(w.writeStream))
	} else {
		w.TotalFlushSucceed += uint64(len(w.writeStream))
	}

	// call callbacks
	for _, callback := range w.callbacks {
		callback(err)
	}
	// clear the stream
	w.writeStream = w.writeStream[:0]
	w.callbacks = w.callbacks[:0]
	return true
}
