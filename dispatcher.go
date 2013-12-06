// Copyright 2013 Alex Belyaev (lex@alexbelyaev.com)
//
// Goevent is a tiny lib for managing events in Go
package goevent

import (
	"sync"
)

type Dispatcher struct {
	mu        sync.Mutex
	listeners map[string][]interface{}
	pchans    map[string]map[chan int]bool
	qchans    map[string][]chan int
	paused    bool
}

// Type for listening function.
type ListenerFunc func(e Event, quit chan int)

// Interface for liseners.
type Listener interface {
	Execute(event Event, quit chan int)
}

// Event. Whenever event is dispatched Event passed to listener.
type Event struct {
	Data interface{}
	Name string
}

// Returns new Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		mu:        sync.Mutex{},
		listeners: make(map[string][]interface{}, 5),
		pchans:    make(map[string]map[chan int]bool, 5),
		qchans:    make(map[string][]chan int, 5),
		paused:    false,
	}
}

// Adds listener to given event
func (d *Dispatcher) Listener(event string, l Listener) {
	d.listeners[event] = append(d.listeners[event], l)
}

// Adds listening function to given event.
//   d.Listener("sample.event", func(e Event, q chan int) {
//      //... do some stuff
//
//      q <- 1 // if you'd like to let your main gouroutine wait for this function
//             // to be executed, pass int to the channel after all manupilations
//             // inside the function
//   })
func (d *Dispatcher) ListenerFunc(event string, l ListenerFunc) {
	d.listeners[event] = append(d.listeners[event], l)
}

// Dispatch given event
func (d *Dispatcher) Dispatch(name string, e Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.pchans[name] == nil {
		d.pchans[name] = make(map[chan int]bool)
	}

	for _, l := range d.listeners[name] {
		p := make(chan int)
		d.pchans[name][p] = true

		q := make(chan int)
		d.qchans[name] = append(d.qchans[name], q)

		go d.executeListener(name, l, e, p, q)
	}
}

func (d *Dispatcher) executeListener(name string, l interface{}, e Event, p chan int, q chan int) {
	if d.paused {
		<-p
	}

	switch t := l.(type) {
	case Listener:
		t.Execute(e, q)
	case ListenerFunc:
		t(e, q)
	default:
		panic("Invalid listener")
	}

	delete(d.pchans[name], p)
}

// Pauses execution of listeners and listening functions
func (d *Dispatcher) Pause() {
	d.paused = true
}

// Resumes execution of listeners and listening functions
func (d *Dispatcher) Resume() {
	if !d.paused {
		return
	}

	for _, arr := range d.pchans {
		for c, _ := range arr {
			select {
			case c <- 1:
			default:
			}
		}
	}

	d.paused = false
}

// Waits until all listeners for given event have finished
func (d *Dispatcher) Wait(event string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, c := range d.qchans[event] {
		if c != nil {
			<-c
			d.qchans[event][i] = nil
		}
	}
}

// Waits for all listeners of all events
func (d *Dispatcher) WaitAll() {
	for e, _ := range d.qchans {
		d.Wait(e)
	}
}
