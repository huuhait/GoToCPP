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

// exitErr is the end of main!: "<program>: <error>" on stderr and exit code 1.
func exitErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}

func init() {
	exitErr(func() error {
		fmt.Println("init start")
		if _, err := num("q"); err != nil {
			return err
		}
		fmt.Println("not reached")
		return nil
	}())
}

func main() {
	fmt.Println("main must not run")
}
