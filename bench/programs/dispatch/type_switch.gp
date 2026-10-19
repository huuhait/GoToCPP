// bench: type_switch
// category: dispatch
// size: 150000000
// smoke: 50000
// twin: no
// about: type switches and type assertions over a mixed []any
package main

import "fmt"

type pt struct{ a, b int }

type named int

func classify(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case string:
		return len(x)
	case float64:
		return int(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case *pt:
		return x.a + x.b
	case pt:
		return x.a - x.b
	case named:
		return int(x) * 2
	case nil:
		return -1
	}
	return 0
}

func main() {
	vals := []any{1, "hello", 2.5, true, &pt{3, 4}, pt{9, 2}, named(21), nil, 77, "x", false, int8(5)}
	var idx [1024]int
	x := uint32(7)
	for i := range idx {
		x = x*1664525 + 1013904223
		idx[i] = int(x>>16) % len(vals)
	}
	var sum int
	for i := 0; i < benchN; i++ {
		v := vals[idx[i&1023]]
		sum += classify(v)
		if n, ok := v.(int); ok {
			sum += n
		}
	}
	fmt.Println(sum)
}
