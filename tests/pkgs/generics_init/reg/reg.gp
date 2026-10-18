package reg

import "fmt"

var Log []string

func Note[T any](v T) T {
	Log = append(Log, fmt.Sprint(v))
	return v
}

func Map[T, U any](xs []T, f func(T) U) []U {
	var out []U
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

var Doubled = Map([]int{1, 2, 3}, func(i int) int { return Note(i * 2) })

var First = Note("reg first")

func init() { Note("reg init") }
