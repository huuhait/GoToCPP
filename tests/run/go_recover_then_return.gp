// phase: 08

// A panic that is recovered leaves nothing for main's return to wait for: main recovers, a goroutine
// recovers, a String method that panics is swallowed by fmt, and the program ends with code 0.
package main

import "fmt"

type T struct{}

func (T) String() string {
	defer func() {}()
	panic("in String")
}

func main() {
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		panic("main's panic")
	}()
	done := make(chan bool)
	go func() {
		defer close(done)
		defer func() { fmt.Println("goroutine recovered:", recover()) }()
		panic("goroutine's panic")
	}()
	<-done
	fmt.Println(T{})
}

// Output:
// recovered: main's panic
// goroutine recovered: goroutine's panic
// %!v(PANIC=String method: in String)
