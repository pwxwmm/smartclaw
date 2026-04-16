package utils

import (
	"fmt"
	"log"
	"runtime/debug"
)

// Go launches a goroutine with panic recovery. If the goroutine panics,
// it logs the panic value and stack trace instead of crashing the process.
func Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC RECOVERED] goroutine panicked: %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}

// GoWithErr launches a goroutine with panic recovery and error channel.
// The goroutine's error is sent to the returned channel on completion.
func GoWithErr(fn func() error) <-chan error {
	ch := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- fmt.Errorf("goroutine panicked: %v", r)
				close(ch)
			}
		}()
		ch <- fn()
		close(ch)
	}()
	return ch
}
