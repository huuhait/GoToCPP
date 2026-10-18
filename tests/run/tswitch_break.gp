// phase: 05

// break, continue and labels in the clauses of a type switch (step 5.5).
package main

import "fmt"

func main() {
	items := []any{1, "a", 2.5, nil, 3, "stop", 4}
	sum := 0
loop:
	for i, it := range items {
		switch v := it.(type) {
		case int:
			if v == 3 {
				break
			}
			sum += v
		case string:
			if v == "stop" {
				break loop
			}
			continue
		case nil:
			sum += 100
			continue loop
		default:
			sum += 1000
		}
		sum += 10 * i
	}
	fmt.Println(sum)
	count := 0
	for _, it := range []any{1, 2, 3, "x", 4} {
		switch it.(type) {
		case int:
			count++
			if count == 2 {
				break
			}
			count += 10
		case string:
			count += 100
		}
		count += 1000
	}
	fmt.Println(count)
}

// Output:
// 1161
// 5144
