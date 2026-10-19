package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrBad = errors.New("bad")

// num parses one digit.
func num(s string) (int, error) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return int(s[0] - '0'), nil
	}
	return 0, fmt.Errorf("num %q: %w", s, ErrBad)
}

func main() {
	var err error
	func() {
		defer fmt.Println("main defer")
		fmt.Println("start")
		var n int
		n, err = num("z")
		if err != nil {
			return
		}
		fmt.Println("n =", n)
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}
