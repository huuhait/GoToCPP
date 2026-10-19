package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// exitErr is the end of main!: "<program>: <error>" on stderr and exit code 1.
func exitErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}

func init() {
	fmt.Println("init 1")
}

func init() {
	fmt.Println("init 2")
	exitErr(errors.New("init says no"))
}

func main() {
	fmt.Println("main must not run")
}
