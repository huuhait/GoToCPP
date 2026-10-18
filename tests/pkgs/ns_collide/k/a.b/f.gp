package dot

import "fmt"

type T struct{ N int }

var V = "dot"

func (t T) Show() string { return fmt.Sprint("dot", t.N) }
func Make() any          { return T{1} }
func F() int             { return len(V) }
