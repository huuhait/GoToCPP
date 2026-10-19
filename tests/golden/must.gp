// phase: 05

package main

import "fmt"

func atoi(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return len(s), nil
}

// A must function: callers see func load(s string) (int, error); every dropped error is returned.
func load!(s string) int {
	n := atoi(s)
	fmt.Println("loaded", n)
	return n
}

// A must call in a function that returns an error: the error is returned, the other results are zero.
func twice(s string) (int, error) {
	a := load!(s)
	return a * 2, nil
}

// The call to the left runs first, then the must call; a call in a loop condition runs on every iteration.
func count(s string, limit int) (n int, err error) {
	for i := 0; i < limit+atoi!(s); i++ {
		n += i + atoi!(s)
	}
	return n, nil
}

// A function with no error result: the error is a panic.
func must(s string) int {
	return atoi!(s)
}

// defer f.Close!(): the error of the deferred call is returned when the function would return nil.
func closing(c interface{ Close() error }) (err error) {
	defer c.Close!()
	return nil
}

func main!() {
	fmt.Println(twice("7"))
	fmt.Println(must("8"))
}
