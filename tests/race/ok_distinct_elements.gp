// phase: 08

// Goroutines write different elements of one slice, one array and one map-free table.
package main

import "fmt"

var arr [4]int

func main() {
	s := make([]int, 4)
	done := make(chan bool)
	for i := 0; i < 4; i++ {
		go func() {
			s[i] = i + 1
			arr[i] = i + 10
			done <- true
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	fmt.Println(s, arr)
}

// Output:
// [1 2 3 4] [10 11 12 13]
