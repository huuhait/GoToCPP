// phase: 10

package main

import (
	"errors"
	"fmt"
	"path/filepath"
)

func show(label string, err error) {
	fmt.Println("--", label)
	frames := errors.Trace(err)
	if len(frames) == 0 {
		fmt.Println("  (no trace)")
	}
	for _, f := range frames {
		if f.File == "" {
			fmt.Printf("  ... %d hops omitted\n", f.Line)
			continue
		}
		fmt.Printf("  %s:%d %s\n", filepath.Base(f.File), f.Line, f.Function)
	}
}

var ErrPerm = errors.New("permission denied")

func open(path string) (int, error) {
	return 0, ErrPerm
}

func load!(path string) (n int, err error) {
	defer func() {
		_ = fmt.Errorf("audit: %w", ErrPerm)
	}()
	defer errors.Wrapf(&err, "load %s", path)
	n = open!(path)
	return n, nil
}

func boot!(path string) int {
	n := load!(path)
	return n
}

func loadLog!(path string) (n int, err error) {
	defer errors.Wrapf(&err, "loadLog %s", path)
	defer func() {
		if err != nil {
			_ = fmt.Errorf("log: %w", err)
			_ = fmt.Errorf("log again: %w", err)
		}
	}()
	n = open!(path)
	return n, nil
}

func bootLog!(path string) int {
	n := loadLog!(path)
	return n
}

func main() {
	_, err := boot("/etc/shadow")
	show("an unrelated wrapper of the sentinel, made after Wrapf: ignored", err)

	_, err = bootLog("/etc/shadow")
	show("COST: a log-only wrapper takes the trace, so the Wrapf that follows does not match", err)
}
