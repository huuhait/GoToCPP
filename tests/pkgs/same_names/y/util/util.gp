package util

import "fmt"

type T struct{ V int }

func (t T) Name() string { return fmt.Sprint("y.T ", t.V) }

func F() string { return "y.F" }

var Reg = map[string]int{"y": 1}

type I interface{ Name() string }

func init() { fmt.Println("init y/util") }
