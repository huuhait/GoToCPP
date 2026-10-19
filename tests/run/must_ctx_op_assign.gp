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

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

func f(i, j string) (int, error) {
	total := 10
	total += num!(i)
	arr := []int{0, 0, 0, 0}
	arr[num!(i)] += num!(j)
	arr[say("k")] = num!(j)
	return total + arr[1] + arr[3], nil
}

func main() {
	fmt.Println(f("1", "2"))
	fmt.Println(f("1", "x"))
	fmt.Println(f("x", "x"))
}

// Output:
// say k
// 13 <nil>
// 0 num "x": bad
// 0 num "x": bad
