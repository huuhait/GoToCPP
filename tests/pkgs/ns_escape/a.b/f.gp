package ab1

import "fmt"

type T struct{ N int }

var V = "ab1"

const K = "k-ab1"

func Name() string { return "ab1" }
func (t T) Show() string { return fmt.Sprint("ab1", t.N) }
func Make() any { return T{1} }
