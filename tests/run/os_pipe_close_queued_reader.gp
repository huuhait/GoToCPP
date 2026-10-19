// phase: 09
// divergence: D7

// Close with a reader blocked in the kernel and a second reader queued behind it. Every goroutine
// is an OS thread (D7), so Close does not wake the blocked reader (Go's poller does, and fails
// both readers at once with "file already closed"). Here the blocked reader keeps waiting until the
// writer writes; the queued reader, which got the file's read lock only after Close, must see that
// the file is closed (the re-check in fdMutex.rwlock) and fail with ErrClosed instead of reading.
package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

func main() {
	r, w, _ := os.Pipe()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []string
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 8)
			n, err := r.Read(buf)
			mu.Lock()
			if err == nil {
				results = append(results, fmt.Sprint("read ", n, " bytes"))
			} else {
				results = append(results, fmt.Sprint("error: ", err))
			}
			mu.Unlock()
		}()
	}
	time.Sleep(200 * time.Millisecond) // one reader is blocked in read(2), the other waits for the lock
	fmt.Println("close:", r.Close())
	fmt.Println("close again:", r.Close())
	w.Write([]byte("x")) // the blocked reader gets this byte
	w.Close()
	wg.Wait()
	if results[0] > results[1] {
		results[0], results[1] = results[1], results[0]
	}
	fmt.Println(results[0])
	fmt.Println(results[1])
}

// Output:
// close: <nil>
// close again: close |0: file already closed
// error: read |0: file already closed
// read 1 bytes
