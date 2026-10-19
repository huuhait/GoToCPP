// phase: 05

// The switch variable captured by a closure and changed in the clause (step 5.5).
package main

import "fmt"

func main() {
	var fs []func() int
	for _, x := range []any{1, 2, "a", 3} {
		switch v := x.(type) {
		case int:
			fs = append(fs, func() int { v++; return v })
		case string:
			fs = append(fs, func() int { return len(v) })
		}
	}
	for _, f := range fs {
		fmt.Println(f(), f())
	}
	var x any = 10
	switch v := x.(type) {
	case int:
		p := &v
		*p += 5
		fmt.Println(v)
	}
}

// Output:
// 2 3
// 3 4
// 1 1
// 4 5
// 15
