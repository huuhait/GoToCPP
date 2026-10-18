// phase: 05

// A type switch with an init statement and without a variable (step 5.5).
package main

import "fmt"

func lookup(m map[string]any, k string) any { return m[k] }

func main() {
	m := map[string]any{"a": 1, "b": "two", "c": 3.0}
	for _, k := range []string{"a", "b", "c", "d"} {
		switch v := lookup(m, k).(type) {
		case int:
			fmt.Println(k, "int", v)
		case string:
			fmt.Println(k, "string", v)
		case nil:
			fmt.Println(k, "missing")
		default:
			fmt.Println(k, "other")
		}
		switch x := m[k]; x.(type) {
		case int, string:
			fmt.Println("int or string", x != nil)
		}
		switch n := len(k); y := m[k].(type) {
		case float64:
			fmt.Println(n, y)
		}
	}
}

// Output:
// a int 1
// int or string true
// b string two
// int or string true
// c other
// 1 3
// d missing
