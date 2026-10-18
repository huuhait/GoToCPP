package util

import "fmt"

type T struct{ V int }

func (t T) Name() string { return fmt.Sprint("x.T ", t.V) }

func F() string { return "x.F" }

var Reg = map[string]int{"x": 1}

type I interface{ Name() string }

func init() { fmt.Println("init x/util") }
