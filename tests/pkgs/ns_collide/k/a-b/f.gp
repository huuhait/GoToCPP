package dash

import "fmt"

type T struct{ N int }

var V = "dash"

func (t T) Show() string { return fmt.Sprint("dash", t.N) }
func Make() any          { return T{1} }
func F() int             { return len(V) }
