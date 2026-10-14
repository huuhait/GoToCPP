// phase: 03

// append(s, nil...) appends nothing: the result is s itself (nil for a nil s), also for a
// []byte, and also when go/types leaves the nil untyped.
package main

import "fmt"

func main() {
	fmt.Println(len(append([]int(nil), nil...)), append([]int(nil), nil...) == nil)
	s := []int{1, 2}
	s = append(s, nil...)
	fmt.Println(len(s), s[1])
	b := append([]byte(nil), nil...)
	fmt.Println(len(b), b == nil)
	b = append([]byte("go"), nil...)
	fmt.Println(string(b))
	var ss [][]string
	ss = append(ss, nil...)
	ss = append(ss, nil)
	fmt.Println(len(ss), ss[0] == nil)
}

// Output:
// 0 true
// 2 2
// 0 true
// go
// 1 true
