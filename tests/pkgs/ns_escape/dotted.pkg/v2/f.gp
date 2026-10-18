package dotv2

import "fmt"

type T struct{ N int }

var V = "dotv2"

const K = "k-dotv2"

func Name() string { return "dotv2" }
func (t T) Show() string { return fmt.Sprint("dotv2", t.N) }
func Make() any { return T{1} }
