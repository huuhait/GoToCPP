// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[0\] with length 0
// Stderr: (?m)^goroutine 1 \[running\]:
// Stderr: (?m)^main\.At\[\.\.\.\]\(\.\.\.\)\n
// Stderr: (?m)^main\.Middle\[\.\.\.\]\(\.\.\.\)\n
// Stderr: (?m)^main\.main\(\)\n

// An unrecovered panic inside generic code: the exit code, the message, and the frames named main.F[...].
package main

import "fmt"

func At[T any](xs []T, i int) T { return xs[i] }

func Middle[K comparable, V any](m map[K][]V, k K, i int) V {
	return At(m[k], i)
}

func main() {
	m := map[string][]int{"a": {1, 2, 3}}
	fmt.Println(Middle(m, "a", 1))
	fmt.Println(Middle(m, "b", 0))
}

// Output:
// 2
