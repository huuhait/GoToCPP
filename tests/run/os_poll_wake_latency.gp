// phase: 09

//go:build unix

// A Read that waits on a pipe is woken at once by Close and by a new deadline (the wait's wake
// descriptor; test D02 of the 9.6 degraded wait). Without the wake descriptor the wait would
// only look again after 25 ms and then less often, so these wakes would be some 45 ms late. Each
// kind is tried 20 times, and its median latency is compared with the median of wakes by data
// (which ppoll sees on the pipe itself, whatever the wake descriptor does), so that a loaded
// machine slows both: the median of Close and of a new deadline must be within 15 ms of it.
// GOCPP_POLL_LATENCY=on checks the absolute bound too (15 of 20 within 5 ms; an idle machine).
package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"time"
)

const tries = 20

// wakes runs a Read on a new pipe, waits until it is blocked, ends it with end, and returns the
// latencies from end to the Read's return, sorted, and the last error of the Read.
func wakes(end func(r, w *os.File)) (lat []time.Duration, last error) {
	for i := 0; i < tries; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			panic(err)
		}
		done := make(chan error)
		go func() {
			_, err := r.Read(make([]byte, 1))
			done <- err
		}()
		time.Sleep(30 * time.Millisecond) // the Read is blocked by now
		t := time.Now()
		end(r, w)
		last = <-done
		lat = append(lat, time.Since(t))
		r.Close()
		w.Close()
	}
	slices.Sort(lat)
	return lat, last
}

func main() {
	data, derr := wakes(func(r, w *os.File) { w.Write([]byte{1}) })
	closed, cerr := wakes(func(r, w *os.File) { r.Close() })
	deadline, perr := wakes(func(r, w *os.File) { r.SetReadDeadline(time.Now().Add(-time.Second)) })
	base := data[tries/2]
	fmt.Println("data:", derr)
	fmt.Println("close:", closed[tries/2] <= base+15*time.Millisecond, errors.Is(cerr, os.ErrClosed))
	fmt.Println("new deadline:", deadline[tries/2] <= base+15*time.Millisecond, errors.Is(perr, os.ErrDeadlineExceeded))
	if os.Getenv("GOCPP_POLL_LATENCY") == "on" {
		for _, k := range []struct {
			name string
			lat  []time.Duration
		}{{"data", data}, {"close", closed}, {"new deadline", deadline}} {
			if k.lat[14] >= 5*time.Millisecond {
				fmt.Println("slow:", k.name, k.lat)
			}
		}
	}
}

// Output:
// data: <nil>
// close: true true
// new deadline: true true
