// phase: 08

// The ok of a comma-ok assignment is an untyped bool: a variable of a named
// bool type takes it (Go issue 8475; our own program).
package main

import "fmt"

type flag bool

func main() {
	var ok flag

	var i interface{} = 3
	_, ok = i.(int)
	fmt.Println(ok)
	_, ok = i.(string)
	fmt.Println(ok)

	m := map[string]int{"a": 1}
	var n int
	n, ok = m["a"]
	fmt.Println(n, ok)
	_, ok = m["b"]
	fmt.Println(ok)

	c := make(chan int, 1)
	c <- 7
	n, ok = <-c
	fmt.Println(n, ok)
	close(c)
	n, ok = <-c
	fmt.Println(n, ok)
}

// Output:
// true
// false
// 1 true
// false
// 7 true
// 0 false
