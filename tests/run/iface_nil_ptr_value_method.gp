// phase: 05
// ExitCode: 2
// Stderr: ^panic: value method main\.T\.Name called using nil \*T pointer

// A value method called through a nil *T held by an interface: Go's wrapper panics with "value
// method main.T.Name called using nil *T pointer" (step 5.4). (The conversion is in a function
// that gc doesn't inline: gc devirtualizes a call on an interface whose type it knows, and then
// the panic is a plain nil dereference.)
package main

import "fmt"

type Namer interface{ Name() string }

type T struct{ n string }

func (t T) Name() string { return t.n }

//go:noinline
func mk(p *T) Namer { return p }

func main() {
	n := mk(nil)
	fmt.Println(n != nil)
	fmt.Println(n.Name())
}

// Output:
// true
