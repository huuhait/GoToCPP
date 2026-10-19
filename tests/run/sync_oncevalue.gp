// phase: 08

// OnceValue and OnceValues call f once and return its result(s) to every caller.
package main

import (
	"fmt"
	"sync"
)

func main() {
	n := 0
	v := sync.OnceValue(func() int { n++; return n * 10 })
	fmt.Println(v(), v(), v(), "calls:", n)

	vs := sync.OnceValue(func() []string { return []string{"a", "b"} })
	fmt.Println(vs(), len(vs()))

	two := sync.OnceValues(func() (string, error) {
		n++
		return fmt.Sprint("value", n), fmt.Errorf("err %d", n)
	})
	s, err := two()
	s2, err2 := two()
	fmt.Println(s, err, s2, err2)

	bad := sync.OnceValue(func() int { panic("no value") })
	for i := 0; i < 2; i++ {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			bad()
		}()
	}
}

// Output:
// 10 10 10 calls: 1
// [a b] 2
// value2 err 2 value2 err 2
// recovered: no value
// recovered: no value
