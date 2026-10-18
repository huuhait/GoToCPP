package bc2

import "fmt"

type T struct{ N int }

var V = "bc2"

const K = "k-bc2"

func Name() string { return "bc2" }
func (t T) Show() string { return fmt.Sprint("bc2", t.N) }
func Make() any { return T{1} }
