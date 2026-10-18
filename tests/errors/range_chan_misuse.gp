// phase: 08

// What the type checker rejects about range over channels (step 8.6): a send-only channel can't be
// ranged over, and there is one iteration variable at most.
package main

func main() {
	so := make(chan<- int)
	c := make(chan int)
	var a, b int
	for range so { // ERROR "cannot range over so \(variable of type chan<- int\): receive from send-only channel"
	}
	for v := range so { // ERROR "cannot range over so \(variable of type chan<- int\): receive from send-only channel"
		_ = v
	}
	for x, y := range c { // ERROR "range over c \(variable of type chan int\) permits only one iteration variable"
		_, _ = x, y
	}
	for a, b = range c { // ERROR "range over c \(variable of type chan int\) permits only one iteration variable"
	}
	for i := range c {
		var s string = i // ERROR "cannot use i \(variable of type int\) as string value in variable declaration"
		_ = s
	}
	_, _ = a, b
	var nilc chan string
	for n := range nilc {
		_ = n + 1 // ERROR "mismatched types string and untyped int|cannot convert 1 \(untyped int constant\) to type string"
	}
}
