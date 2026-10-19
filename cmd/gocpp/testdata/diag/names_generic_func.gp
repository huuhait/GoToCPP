package main

import "<cstdlib>"

func First[T any](s []T) T { return s[0] }

func main() {
	words := []string{"a", "b"}
	_ = cstdlib.abs(First[string](words))
	_ = cstdlib.abs(First(words))
}
