// phase: 08

// What the type checker rejects about channels and go statements (steps 8.2 to 8.4): the direction
// of a channel is part of its type, a send needs a value of the element type, close needs a
// channel that can send, and go needs a call.
package main

func f() int { return 1 }

func main() {
	ro := make(<-chan int)
	wo := make(chan<- int)
	c := make(chan int)
	ro <- 1                         // ERROR "cannot send to receive-only channel"
	<-wo                            // ERROR "cannot receive from send-only channel"
	close(ro)                       // ERROR "cannot close receive-only channel"
	var bi chan int = ro            // ERROR "cannot use ro \(variable of type <-chan int\) as chan int value"
	var r2 <-chan int = wo          // ERROR "cannot use wo \(variable of type chan<- int\) as <-chan int value"
	c <- "s"                        // ERROR "cannot use \x22s\x22 \(untyped string constant\) as int value in send"
	var s string = <-c              // ERROR "cannot use <-c \(.*\) as string value"
	go int(1)                       // ERROR "go requires function call, not conversion"
	go len("x")                     // ERROR "go discards result of len\(\x22x\x22\)"
	for range wo {                  // ERROR "cannot range over wo \(variable of type chan<- int\): receive from send-only channel"
	}
	_ = make(chan int, -1)          // ERROR "index -1 \(constant of type int\) must not be negative"
	_ = make(chan int, 1, 2)        // ERROR "make\(chan int, 1, 2\) expects 1 or 2 arguments; found 3"
	n := 5
	n <- 1                          // ERROR "cannot send to non-channel"
	_ = <-n                         // ERROR "cannot receive from non-channel"
	close(n)                        // ERROR "cannot close non-channel"
	var x chan<- chan int = make(chan chan<- int) // ERROR "cannot use make\(chan chan<- int\)"
	_, _, _, _ = bi, r2, s, x
}
