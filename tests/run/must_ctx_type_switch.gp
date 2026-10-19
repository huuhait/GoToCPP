// phase: 10

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// num parses one digit.
func num(s string) (int, error) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return int(s[0] - '0'), nil
	}
	return 0, fmt.Errorf("num %q: %w", s, ErrBad)
}

func kind(n int) (any, error) {
	switch n {
	case 0:
		return 1, nil
	case 1:
		return "s", nil
	}
	return nil, fmt.Errorf("kind %d: %w", n, ErrBad)
}

func describe(n int) (string, error) {
	switch v := kind!(n).(type) {
	case int:
		return fmt.Sprint("int ", v+1), nil
	case string:
		return "string " + v, nil
	}
	return "other", nil
}

func main() {
	fmt.Println(describe(0))
	fmt.Println(describe(1))
	fmt.Println(describe(2))
}

// Output:
// int 2 <nil>
// string s <nil>
//  kind 2: bad
