package main

import (
	"errors"
	"fmt"
)

type shape interface{ area() int }

type sq struct{ n int }

func (s sq) area() int { return s.n * s.n }

func init() { fmt.Println("init") }

func main() {
	var fs []func() int
	for i := 0; i < 3; i++ {
		fs = append(fs, func() int { return i })
	}
	var sh shape = sq{3}
	err := fmt.Errorf("wrapped: %w", errors.New("x"))
	k := uint(len(fs))
	fmt.Println(fs[0](), sh.area(), err, 1<<k, 7>>k)
	defer fmt.Println("done")
	go func() {}()
}
