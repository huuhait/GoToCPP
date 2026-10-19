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

func spawn(name string) error {
	done := make(chan string, 1)
	go func() {
		err := step(name)
		done <- fmt.Sprint("worker ", name, " ", err != nil)
	}()
	fmt.Println(<-done)
	if err := step(name + "-main"); err != nil {
		return err
	}
	return nil
}

func main() {
	fmt.Println(spawn("a"))
	fmt.Println(spawn("bad"))
}
