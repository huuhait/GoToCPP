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

var ErrNotFound = errors.New("not found")

func lookup(key string) (string, error) {
	if key == "" {
		return "", ErrNotFound
	}
	return "v:" + key, nil
}

func fetch!(key string) string {
	v := lookup!(key)
	return v
}

func serve!(key string) string {
	r := fetch!(key)
	return r
}

func direct!(key string) string {
	v := lookup!(key)
	return v
}

func outer!(key string) string {
	r := direct!(key)
	return r
}

func main() {
	_, err := serve("")
	fmt.Println(err == ErrNotFound)
	show("request 1: serve, fetch, lookup", err)

	_, err = outer("")
	fmt.Println(err == ErrNotFound)
	show("request 2: the same value again, a different path", err)
}
