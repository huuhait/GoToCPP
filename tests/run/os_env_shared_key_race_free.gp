// phase: 09

package main

import (
	"fmt"
	"os"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				switch (g + i) % 4 {
				case 0:
					os.Setenv("GPOS_SHARED", "v")
				case 1:
					os.Unsetenv("GPOS_SHARED")
				case 2:
					os.Getenv("GPOS_SHARED")
				default:
					os.Environ()
				}
			}
		}(g)
	}
	wg.Wait()
	v := os.Getenv("GPOS_SHARED")
	fmt.Println(v == "" || v == "v")
}

// Output:
// true
