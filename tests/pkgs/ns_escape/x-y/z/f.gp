package xyz

import "fmt"

type T struct{ N int }

var V = "xyz"

const K = "k-xyz"

func Name() string { return "xyz" }
func (t T) Show() string { return fmt.Sprint("xyz", t.N) }
func Make() any { return T{1} }
