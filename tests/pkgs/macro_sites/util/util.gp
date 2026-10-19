package util

import (
	_ "embed"
	"fmt"
)

//go:embed note.txt
var Note string

type Widget struct{}

func Where() string { return fmt.Sprint(#file(), ":", #line(), " ", #func()) }

func (w *Widget) Where() string { return fmt.Sprint(#file(), " ", #func()) }

func Closure() string { return func() string { return #func() }() }

func Gen[T any](x T) string { return #func() }

var Init = #func()

func Text(a, b int) string { return #text(a * b) }

func EmbedHere() string { return #embed("note.txt") }
