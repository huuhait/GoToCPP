package lib

import (
	"testing"
	"time"
)

// TestHang blocks the test goroutine and a helper on channels; the timeout prints every goroutine,
// as Go's testing does (it sets the traceback to "all" before the panic).
func TestHang(t *testing.T) {
	started := make(chan bool)
	never := make(chan bool)
	go func() {
		started <- true
		<-never
	}()
	<-started

	// Both goroutines are parked on the channel after a moment.
	time.Sleep(100 * time.Millisecond)
	<-never
}
