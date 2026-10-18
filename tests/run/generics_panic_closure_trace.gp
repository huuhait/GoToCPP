// phase: 07
// ExitCode: 2
// Stderr: ^panic: bad value 2
// Stderr: (?m)^goroutine 1 \[running\]:
// Stderr: (?m)^main\.Each\[\.\.\.\]\(\.\.\.\)\n
// Stderr: (?m)^main\.Check\[\.\.\.\]\(\.\.\.\)\n

// A panic inside a func literal of a generic function: the frame is main.F[...].func1.
package main

import "fmt"

func Each[T any](xs []T, f func(T)) {
	for _, x := range xs {
		f(x)
	}
}

func Check[T comparable](xs []T, bad T) {
	Each(xs, func(x T) {
		if x == bad {
			panic(fmt.Sprint("bad value ", x))
		}
	})
}

func main() {
	Check([]string{"a", "b"}, "z")
	Check([]int{1, 2, 3}, 2)
}

// Output:
