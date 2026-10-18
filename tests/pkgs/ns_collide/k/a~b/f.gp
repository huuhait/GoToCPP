package tilde

import "fmt"

type T struct{ N int }

var V = "tilde"

func (t T) Show() string { return fmt.Sprint("tilde", t.N) }
func Make() any          { return T{1} }
func F() int             { return len(V) }
