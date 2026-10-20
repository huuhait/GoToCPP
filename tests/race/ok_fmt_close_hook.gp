// phase: 09
// Stderr: ^done\n$

// The standard-stream close hook runs for every File.Close. Closing the originals concurrently
// with unrelated files must not race on their pfd.Sysfd fields (which Close changes to -1).
package main

import (
	"fmt"
	"os"
	"sync"
)

func main() {
	const workers = 8
	var wg sync.WaitGroup
	ready := make(chan bool, workers)
	start := make(chan bool)
	for g := 0; g < workers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			files := make([]*os.File, 64)
			for i := range files {
				f, err := os.Open(os.DevNull)
				if err != nil {
					panic(err)
				}
				files[i] = f
			}
			ready <- true
			<-start
			for _, f := range files {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}
		}()
	}
	for g := 0; g < workers; g++ {
		<-ready
	}
	close(start)
	if err := os.Stdout.Close(); err != nil {
		panic(err)
	}
	if err := os.Stdin.Close(); err != nil {
		panic(err)
	}
	wg.Wait()
	fmt.Fprintln(os.Stderr, "done")
}

// Output:
