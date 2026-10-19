// phase: 10

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// step prints its name and fails for "bad".
func step(name string) error {
	fmt.Println("step", name)
	if name == "bad" {
		return fmt.Errorf("step %s: %w", name, ErrBad)
	}
	return nil
}

func spawn!(name string) {
	done := make(chan string, 1)
	go func() {
		err := step(name)
		done <- fmt.Sprint("worker ", name, " ", err != nil)
	}()
	fmt.Println(<-done) // the worker is finished before step runs in this goroutine
	step(name + "-main")
}

func main() {
	fmt.Println(spawn("a"))
	fmt.Println(spawn("bad"))
}

// Output:
// step a
// worker a false
// step a-main
// <nil>
// step bad
// worker bad true
// step bad-main
// <nil>
