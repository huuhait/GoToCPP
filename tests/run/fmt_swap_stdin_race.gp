// phase: 09

// Assigning os.Stdin while other goroutines call fmt.Scan never frees a file under a scanner.
package main

import (
	"fmt"
	"os"
	"sync"
)

func main() {
	// (The scanners must not block on the real standard input: it starts as an empty pipe.)
	r0, w0, _ := os.Pipe()
	w0.Close()
	os.Stdin = r0
	var wg sync.WaitGroup
	stop := make(chan bool)
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				var x int
				fmt.Scan(&x)
			}
		}()
	}
	for i := 0; i < 1000; i++ {
		r, w, _ := os.Pipe()
		w.WriteString("1 2 3 ")
		w.Close()
		os.Stdin = r
	}
	close(stop)
	wg.Wait()
	fmt.Println("done")
}

// Output:
// done
