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

func feed(ch chan int, xs ...string) error {
	for _, x := range xs {
		ch <- num!(x)
	}
	return nil
}

func main() {
	ch := make(chan int, 8)
	fmt.Println(feed(ch, "1", "2"))
	fmt.Println(feed(ch, "3", "z", "4"))
	close(ch)
	for v := range ch {
		fmt.Println(v)
	}
}

// Output:
// <nil>
// num "z": bad
// 1
// 2
// 3
