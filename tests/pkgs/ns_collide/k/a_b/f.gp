package under

import "fmt"

type T struct{ N int }

var V = "under"

func (t T) Show() string { return fmt.Sprint("under", t.N) }
func Make() any          { return T{1} }
func F() int             { return len(V) }
