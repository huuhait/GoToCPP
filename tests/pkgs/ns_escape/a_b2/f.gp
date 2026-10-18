package ab2

import "fmt"

type T struct{ N int }

var V = "ab2"

const K = "k-ab2"

func Name() string { return "ab2" }
func (t T) Show() string { return fmt.Sprint("ab2", t.N) }
func Make() any { return T{1} }
