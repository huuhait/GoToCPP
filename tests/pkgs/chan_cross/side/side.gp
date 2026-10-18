package side

import "example.com/chans/pipe"

// Relay takes tickets from a pipe package's channel type and passes them on.
func Relay(in <-chan pipe.Ticket, out chan<- pipe.Namer, n int) {
	for i := 0; i < n; i++ {
		out <- <-in
	}
}

var Boxes = make(chan pipe.Pipe, 2)
