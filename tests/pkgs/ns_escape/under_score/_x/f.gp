package ux

import "fmt"

type T struct{ N int }

var V = "ux"

const K = "k-ux"

func Name() string { return "ux" }
func (t T) Show() string { return fmt.Sprint("ux", t.N) }
func Make() any { return T{1} }
