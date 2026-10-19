// phase: 08

// Two goroutines write different fields of the same struct: no race.
package main

import "fmt"

type Pair struct {
	a int
	b int
}

func main() {
	p := &Pair{}
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			p.a++
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			p.b += 2
		}
		done <- true
	}()
	<-done
	<-done
	fmt.Println(p.a, p.b)
}

// Output:
// 100 200
