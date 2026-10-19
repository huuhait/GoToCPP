// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
	"sync"
)

func main() {
	r, w, _ := os.Pipe()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				w.Write([]byte{byte('a' + i)})
			}
		}(i)
	}
	go func() { wg.Wait(); w.Close() }()
	b, err := io.ReadAll(r)
	counts := map[byte]int{}
	for _, c := range b {
		counts[c]++
	}
	fmt.Println(len(b), err, len(counts))
	for i := 0; i < 8; i++ {
		fmt.Print(" ", counts[byte('a'+i)])
	}
	fmt.Println()
}

// Output:
// 800 <nil> 8
//  100 100 100 100 100 100 100 100
