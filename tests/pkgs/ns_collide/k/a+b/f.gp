package plus

import "fmt"

type T struct{ N int }

var V = "plus"

func (t T) Show() string { return fmt.Sprint("plus", t.N) }
func Make() any          { return T{1} }
func F() int             { return len(V) }
