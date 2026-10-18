// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A nil *User in an interface: the typed nil is not a nil interface; a promoted value method
// derefs the pointer (an ordinary nil dereference panic, not panicwrap's message, which only
// declared value methods of *T have), and a promoted pointer method derefs it too, to find
// the address of the embedded field.
package main

import "fmt"

type Getter interface{ Get() int }
type Setter interface{ Set(int) }

type Base struct{ ID int }

func (b Base) Get() int   { return b.ID }
func (b *Base) Set(v int) { b.ID = v }

type User struct {
	Base
	Name string
}

//go:noinline
func call(g Getter) int { return g.Get() }

//go:noinline
func store(s Setter) { s.Set(1) }

func main() {
	var u *User
	var g Getter = u
	var s Setter = u
	fmt.Println(g == nil, s != nil)
	fmt.Println(call(g))
	fmt.Println("not reached")
}

// Output:
// false true
