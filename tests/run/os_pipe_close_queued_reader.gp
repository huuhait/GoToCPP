// phase: 09

// Close with a reader blocked on the pipe and a second reader queued behind it for the read lock.
// Close wakes the blocked reader (the poller's wake descriptor) and the queued reader, which got
// the file's read lock only after Close, sees that the file is closed (the re-check in
// fdMutex.rwlock): both fail with ErrClosed, as in Go.
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
	time.Sleep(200 * time.Millisecond) // one reader is blocked on the pipe, the other waits for the lock
	fmt.Println("close:", r.Close())
	fmt.Println("close again:", r.Close())
	wg.Wait()
	w.Write([]byte("x")) // nobody reads it
	w.Close()
	fmt.Println(results[0])
	fmt.Println(results[1])
}

// Output:
// close: <nil>
// close again: close |0: file already closed
// error: read |0: file already closed
// error: read |0: file already closed
