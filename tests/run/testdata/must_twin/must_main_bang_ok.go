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

// step prints its name and fails for "bad".
func step(name string) error {
	fmt.Println("step", name)
	if name == "bad" {
		return fmt.Errorf("step %s: %w", name, ErrBad)
	}
	return nil
}

// exitErr is the end of main!: "<program>: <error>" on stderr and exit code 1.
func exitErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}

func main() {
	exitErr(func() (err error) {
		defer fmt.Println("deferred")
		if err = step("a"); err != nil {
			return err
		}
		n, err := num("5")
		if err != nil {
			return err
		}
		fmt.Println("n =", n)
		return nil
	}())
}
