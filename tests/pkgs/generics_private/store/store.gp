// Package store: exported generic functions whose bodies use unexported names of the package.
package store

import "fmt"

type entry struct {
	key string
	n   int
}

var calls int

var registry = map[string]int{}

const limit = 3

func note(key string) int {
	calls++
	registry[key]++
	return registry[key]
}

func describe(e entry) string { return fmt.Sprintf("%s#%d", e.key, e.n) }

func Put[T any](key string, v T) string {
	n := note(key)
	if n > limit {
		panic("too many " + key)
	}
	return describe(entry{key, n}) + fmt.Sprint("=", v)
}

func Calls() int { return calls }

func Reset() {
	calls = 0
	clear(registry)
}

func Wrap[T any](vs ...T) []string {
	var out []string
	for i, v := range vs {
		out = append(out, Put(fmt.Sprint("w", i%2), v))
	}
	return out
}

func Hidden() any { return entry{"h", 1} }
