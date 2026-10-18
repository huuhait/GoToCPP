// phase: 08

// What the type checker rejects about select (step 8.5): a case must be a send or a receive,
// possibly with an assignment; one default at most; the direction of a channel is checked in a
// case like anywhere else.
package main

func f() int { return 1 }

func main() {
	var ro <-chan int
	var so chan<- int
	c := make(chan int)
	var x int
	select {
	case f(): // ERROR "select case must be send or receive \(possibly with assignment\)"
	case x: // ERROR "select case must be send or receive \(possibly with assignment\)"
	case x = 1: // ERROR "select case must be send or receive \(possibly with assignment\)"
	case ro <- 1: // ERROR "cannot send to receive-only channel"
	case <-so: // ERROR "cannot receive from send-only channel"
	case c <- "s": // ERROR "cannot use \x22s\x22 \(untyped string constant\) as int value in send"
	case x = <-"s": // ERROR "cannot receive from non-channel"
	}
	select {
	default:
	default: // ERROR "multiple defaults"
	}
	select {
	case y := <-so: // ERROR "declared and not used: y" "cannot receive from send-only channel"
	}
	select {
	case x := 5: // ERROR "select case must be send or receive \(possibly with assignment\)"
	}
	_ = x
}
