// phase: 09

// While os.Setenv changes a variable, another goroutine's LookupEnv sees it set (an empty value is
// set too), never missing, as in Go: the C library's copy is made under the lock that LookupEnv
// takes shared (runtime/src/os/windows/win_syscall.cpp, env_lock). A variable nobody changes is
// never missing either.
package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

func main() {
	os.Setenv("ZV", "a")
	os.Setenv("OTHER", "keep")
	var stop atomic.Bool
	var missing, otherMissing, reads atomic.Int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20000; i++ {
			if i%2 == 0 {
				os.Setenv("ZV", "")
			} else {
				os.Setenv("ZV", "b")
			}
		}
		stop.Store(true)
	}()
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				if _, ok := os.LookupEnv("ZV"); !ok {
					missing.Add(1)
				}
				if _, ok := os.LookupEnv("OTHER"); !ok {
					otherMissing.Add(1)
				}
				reads.Add(1)
			}
		}()
	}
	wg.Wait()
	fmt.Println("ZV missing:", missing.Load() > 0, "OTHER missing:", otherMissing.Load() > 0, "reads>0:", reads.Load() > 0)
}
