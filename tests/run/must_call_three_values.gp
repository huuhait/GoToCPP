// phase: 10

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func triple(s string) (int, string, bool, error) {
	if s == "" {
		return 0, "", false, fmt.Errorf("triple: %w", ErrBad)
	}
	return len(s), s + s, len(s) > 1, nil
}

func use(s string) (string, error) {
	n, d, long := triple!(s)
	return fmt.Sprint(n, d, long), nil
}

func main() {
	fmt.Println(use("ab"))
	fmt.Println(use(""))
}

// Output:
// 2ababtrue <nil>
//  triple: bad
