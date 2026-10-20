// phase: 09

// Assigning os.Stdout (a fresh *os.File each time, and a copy of one) while other goroutines call
// fmt.Println is a data race in Go but never frees a file under a printer. Here the printers write to /dev/null.
package main

import (
	"fmt"
	"os"
	"sync"
)

func main() {
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	orig := os.Stdout
	os.Stdout = null
	tmpl := *null
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5000; i++ {
				fmt.Println("x", i)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50000; i++ {
			c := new(os.File)
			*c = tmpl
			os.Stdout = c
		}
	}()
	wg.Wait()
	os.Stdout = orig
	fmt.Println("done")
}

// Output:
// done
