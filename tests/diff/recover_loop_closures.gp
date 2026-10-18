// phase: 05

package main

import "fmt"

func main() {
	var fs []func()
	for i := 0; i < 3; i++ {
		fs = append(fs, func() { fmt.Println("captured", i) })
		defer func(n int) { fmt.Println("arg", n, "loop var", i) }(i * 10)
	}
	for _, f := range fs {
		f()
	}
	for _, s := range []string{"x", "y"} {
		defer func() { fmt.Println("range var", s) }()
	}
}
