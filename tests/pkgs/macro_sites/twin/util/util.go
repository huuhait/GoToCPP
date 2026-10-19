package util

import (
	_ "embed"
	"fmt"
)

//go:embed note.txt
var Note string

type Widget struct{}

// The built-in macros are written as the constants the compiler puts in their place.

func Where() string { return fmt.Sprint("util/util.gp", ":", 13, " ", "example.com/sites/util.Where") }

func (w *Widget) Where() string {
	return fmt.Sprint("util/util.gp", " ", "example.com/sites/util.(*Widget).Where")
}

func Closure() string {
	return func() string { return "example.com/sites/util.Closure.func1" }()
}

func Gen[T any](x T) string { return "example.com/sites/util.Gen[...]" }

var Init = "example.com/sites/util.init"

func Text(a, b int) string { return "a * b" }

func EmbedHere() string { return "util data\n" }
