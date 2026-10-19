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
			key := "GPOS_CONC_" + fmt.Sprint(g)
			for i := 0; i < 300; i++ {
				os.Setenv(key, fmt.Sprint(i))
				if os.Getenv(key) != fmt.Sprint(i) {
					panic("lost update")
				}
				_ = os.Environ()
				if i%50 == 49 {
					os.Unsetenv(key)
					if _, ok := os.LookupEnv(key); ok {
						panic("still set")
					}
				}
			}
		}(g)
	}
	wg.Wait()
	n := 0
	for g := 0; g < 8; g++ {
		if _, ok := os.LookupEnv("GPOS_CONC_" + fmt.Sprint(g)); !ok {
			n++
		}
	}
	fmt.Println(n)
}

// Output:
// 8
