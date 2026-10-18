package slash

import "fmt"

type T struct{ N int }

var V = "slash"

func (t T) Show() string { return fmt.Sprint("slash", t.N) }
func Make() any          { return T{1} }
func F() int             { return len(V) }
