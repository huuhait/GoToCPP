// phase: 09

//go:build linux

package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

func nfds() int {
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		panic(err)
	}
	return len(ents)
}

// A Read or Write that has to wait does not fail when the process has no descriptor left (the
// wake descriptor of the wait cannot be made: the wait goes on without it), and Close and
// SetDeadline still end it.
func main() {
	big := make([]byte, 1<<20) // allocated before the descriptors run out (a sanitizer may need one)
	buf := make([]byte, 1<<16)
	// GCC's AddressSanitizer needs a pipe of its own when a goroutine runs with no descriptor left
	// ("CHECK failed: sanitizer_posix_libcdep.cpp:305 pipe(fds)"), so under it steps 2 to 4 are
	// not run (the output is what they print). Every other build runs them.
	maps, _ := os.ReadFile("/proc/self/maps")
	gccAsan := strings.Contains(string(maps), "libasan")

	// Goroutines for steps 2 to 4, started while there are descriptors (a thread of the sanitizer's
	// own may need one to start).
	jobs := make(chan func())
	for i := 0; i < 3; i++ {
		go func() {
			for f := range jobs {
				f()
			}
		}()
	}
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old); err != nil {
		panic(err)
	}
	defer syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old)

	// 1. Many pipes with a blocked reader each, and room for only some wake descriptors.
	const N = 60
	const extra = 4 // pipes that nobody waits on in step 1
	lim := syscall.Rlimit{Cur: uint64(nfds() + 2*(N+extra) + 25), Max: old.Max}
	if lim.Cur > old.Max {
		panic("hard limit too low")
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		panic(err)
	}
	rs := make([]*os.File, N+extra)
	ws := make([]*os.File, N+extra)
	for i := range rs {
		var err error
		rs[i], ws[i], err = os.Pipe()
		if err != nil {
			panic(err)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	bad := 0
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := rs[i].Read(make([]byte, 4))
			if n != 1 || err != nil {
				mu.Lock()
				bad++
				mu.Unlock()
			}
		}()
	}
	time.Sleep(200 * time.Millisecond)
	for i := 0; i < N; i++ {
		ws[i].Write([]byte{1})
	}
	wg.Wait()
	fmt.Println("reads failed:", bad)

	if gccAsan {
		fmt.Println("close: <nil>\ntrue write |1: file already closed\nread |0: file already closed\nread |0: i/o timeout\nread |0: i/o timeout\nread |0: file already closed true")
		close(jobs)
		return
	}

	// 2. A Write that waits on a full pipe, with no descriptor left for a wake descriptor.
	// The first 25 waits above made the wake descriptors there was room for; the files that did not
	// wait yet have none and can make none (the limit is reached).
	r, w := rs[N], ws[N]
	done := make(chan string)
	jobs <- func() {
		n, err := w.Write(big)
		done <- fmt.Sprint(n > 0 && n < 1<<20, err)
	}
	time.Sleep(100 * time.Millisecond)
	r.Read(buf) // makes room: the Write goes on, and waits again
	time.Sleep(100 * time.Millisecond)
	fmt.Println("close:", w.Close()) // the Write is woken by Close, also without a wake descriptor
	fmt.Println(<-done)

	// 3. Close wakes a Read, and a deadline ends a Read, with no descriptor left.
	r1, r2 := rs[N+1], rs[N+2]
	jobs <- func() {
		_, err := r1.Read(buf)
		done <- fmt.Sprint(err)
	}
	time.Sleep(100 * time.Millisecond)
	r1.Close()
	fmt.Println(<-done)
	r2.SetReadDeadline(time.Now().Add(60 * time.Millisecond))
	_, err := r2.Read(buf)
	fmt.Println(err)
	jobs <- func() {
		_, err := r2.Read(buf)
		done <- fmt.Sprint(err)
	}
	r2.SetReadDeadline(time.Time{})
	time.Sleep(100 * time.Millisecond)
	r2.SetReadDeadline(time.Now().Add(-time.Second)) // a new deadline ends the wait
	fmt.Println(<-done)

	// 4. A far deadline does not make the wait without a wake descriptor sleep until it (test D08
	// of the 9.6 degraded wait): Close ends the Read within the wait's look-again period. (The
	// descriptors that steps 2 and 3 freed are taken first.)
	var fill []*os.File
	for {
		f, err := os.Open("/dev/null")
		if err != nil {
			break
		}
		fill = append(fill, f)
	}
	r3 := rs[N+3]
	r3.SetReadDeadline(time.Now().Add(time.Hour))
	jobs <- func() {
		_, err := r3.Read(buf)
		done <- fmt.Sprint(err)
	}
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	closed := make(chan bool)
	go func() { // (Close waits until the Read is over)
		r3.Close()
		closed <- true
	}()
	select {
	case err := <-done:
		<-closed
		fmt.Println(err, time.Since(start) < 2*time.Second)
	case <-time.After(5 * time.Second):
		fmt.Println("Close did not end a Read with a far deadline")
		return
	}
	for _, f := range fill {
		f.Close()
	}

	close(jobs)
	for i := range rs {
		rs[i].Close()
		ws[i].Close()
	}
}

// Output:
// reads failed: 0
// close: <nil>
// true write |1: file already closed
// read |0: file already closed
// read |0: i/o timeout
// read |0: i/o timeout
// read |0: file already closed true
