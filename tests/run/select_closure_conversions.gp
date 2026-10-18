// phase: 08

// Values sent in select cases are converted to the element type like arguments: untyped constants,
// concrete types to interfaces, named types; the receive side assigns to variables of another type
// where Go allows it.
package main

import "fmt"

type Celsius float64

type Stringer interface{ String() string }

func (c Celsius) String() string { return fmt.Sprintf("%.1fC", float64(c)) }

func main() {
	fl := make(chan float64, 1)
	ifc := make(chan Stringer, 1)
	anyc := make(chan any, 1)
	temps := make(chan Celsius, 1)
	bytes := make(chan byte, 1)
	runes := make(chan rune, 1)

	select {
	case fl <- 3:
	}
	select {
	case ifc <- Celsius(21.5):
	}
	select {
	case anyc <- 42:
	}
	select {
	case temps <- 36.6:
	}
	select {
	case bytes <- 'A':
	}
	select {
	case runes <- 'é':
	}
	var s Stringer
	var a any
	var f float64
	var t Celsius
	var b byte
	var r rune
	select {
	case f = <-fl:
	}
	select {
	case s = <-ifc:
	}
	select {
	case a = <-anyc:
	}
	select {
	case t = <-temps:
	}
	select {
	case b = <-bytes:
	}
	select {
	case r = <-runes:
	}
	fmt.Println(f, s, a, t, b, string(r))
	fmt.Printf("%T %T %T\n", f, s, a)

	// receive a concrete value into an interface variable
	var st Stringer
	c2 := make(chan Celsius, 1)
	c2 <- 5
	select {
	case v := <-c2:
		st = v
	}
	fmt.Println(st.String())
}

// Output:
// 3 21.5C 42 36.6C 65 é
// float64 main.Celsius int
// 5.0C
