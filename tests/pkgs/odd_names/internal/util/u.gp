package utils

import "fmt"

// the folder is util, the package name is utils

func Hello() { fmt.Println("hello from", "utils") }

type string_ struct{ s string }

func Wrap(s string) string_ { return string_{s} }
func (w string_) Get() string { return w.s }
