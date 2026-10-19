// phase: 08

// A baton goes around a ring of goroutines: only its holder touches the shared array.
package main

import "fmt"

var board [6]int

func main() {
	const n = 6
	chans := make([]chan int, n)
	for i := range chans {
		chans[i] = make(chan int)
	}
	done := make(chan bool)
	for i := 0; i < n; i++ {
		go func() {
			for round := 0; round < 3; round++ {
				v := <-chans[i]
				board[i] += v
				if i == n-1 && round == 2 {
					done <- true
					return
				}
				chans[(i+1)%n] <- v + 1
			}
		}()
	}
	chans[0] <- 1
	<-done
	fmt.Println(board)
}

// Output:
// [21 24 27 30 33 36]
