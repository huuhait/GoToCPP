package upcase

import "fmt"

type T struct{ N int }

var V = "upcase"

const K = "k-upcase"

func Name() string { return "upcase" }
func (t T) Show() string { return fmt.Sprint("upcase", t.N) }
func Make() any { return T{1} }
