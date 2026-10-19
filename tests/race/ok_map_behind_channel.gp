// phase: 08

// A map is used by one goroutine at a time, passed on through a channel.
package main

import "fmt"

func main() {
	token := make(chan map[string]int, 1)
	token <- map[string]int{}
	done := make(chan bool)
	for g := 0; g < 6; g++ {
		go func() {
			m := <-token
			m["k"]++
			m[fmt.Sprint("g", g)] = g
			token <- m
			done <- true
		}()
	}
	for g := 0; g < 6; g++ {
		<-done
	}
	m := <-token
	fmt.Println(m["k"], len(m))
}

// Output:
// 6 7
