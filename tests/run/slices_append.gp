// phase: 03

package main

import "fmt"

func main() {
	s := []int{1, 2}
	t := append(s, 3)
	s[0] = 9
	fmt.Println(s, t, len(t), cap(t) >= 3)
}

// Output:
// [9 2] [1 2 3] 3 true
