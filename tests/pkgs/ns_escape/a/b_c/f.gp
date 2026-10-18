package bc1

import "fmt"

type T struct{ N int }

var V = "bc1"

const K = "k-bc1"

func Name() string { return "bc1" }
func (t T) Show() string { return fmt.Sprint("bc1", t.N) }
func Make() any { return T{1} }
