// phase: 05

// A type switch over the basic types, with a default (step 5.5).
package main

import "fmt"

func describe(x any) string {
	switch v := x.(type) {
	case int:
		return fmt.Sprint("int ", v+1)
	case int64:
		return fmt.Sprint("int64 ", v*2)
	case float64:
		return fmt.Sprint("float64 ", v/2)
	case string:
		return "string " + v + "!"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []int:
		return fmt.Sprint("slice ", len(v))
	case map[string]int:
		return fmt.Sprint("map ", len(v))
	case func() int:
		return fmt.Sprint("func ", v())
	case [2]int:
		return fmt.Sprint("array ", v[0]+v[1])
	case *int:
		return fmt.Sprint("ptr ", *v)
	default:
		return "unknown"
	}
}

func main() {
	n := 5
	vals := []any{1, int64(2), 3.0, "s", true, false, []int{1, 2}, map[string]int{"a": 1}, func() int { return 9 }, [2]int{3, 4}, &n, nil, 'x', uint(1), struct{}{}}
	for _, v := range vals {
		fmt.Println(describe(v))
	}
}

// Output:
// int 2
// int64 4
// float64 1.5
// string s!
// true
// false
// slice 2
// map 1
// func 9
// array 7
// ptr 5
// unknown
// unknown
// unknown
// unknown
