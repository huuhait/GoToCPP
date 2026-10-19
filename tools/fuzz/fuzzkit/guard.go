package fuzzkit

import (
	"fmt"
	"os"
	"runtime"
	"runtime/metrics"
	"strconv"
	"sync"
	"time"
)

// A fuzz target that hangs or that eats memory does not fail by itself: the engine waits, and
// the machine swaps. Guard turns both into a crash of the worker process, which the engine saves
// as a failing input ("fuzzing process hung or terminated unexpectedly"), with a dump of the
// stacks on stderr that the campaign runner reads for its signature.
//
// The limits: GOCPP_FUZZ_HANG seconds for one execution (10), GOCPP_FUZZ_MEM megabytes of live
// heap (3000).

var (
	guardOnce sync.Once
	guardMu   sync.Mutex
	guardWhat string
	guardAt   time.Time
	memLimit  uint64
	hangLimit time.Duration
)

func envInt(name string, def int) int {
	if s := os.Getenv(name); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func startGuard() {
	guardOnce.Do(func() {
		memLimit = uint64(envInt("GOCPP_FUZZ_MEM", 3000)) << 20
		hangLimit = time.Duration(envInt("GOCPP_FUZZ_HANG", 10)) * time.Second
		go func() {
			sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
			for range time.Tick(100 * time.Millisecond) {
				guardMu.Lock()
				what, at := guardWhat, guardAt
				guardMu.Unlock()
				if what == "" {
					continue
				}
				metrics.Read(sample)
				switch {
				case sample[0].Value.Uint64() > memLimit:
					die("MEMORY BLOWUP: %d MB of live heap in %s", sample[0].Value.Uint64()>>20, what)
				case time.Since(at) > hangLimit:
					die("HANG: %s runs for more than %v", what, hangLimit)
				}
			}
		}()
	})
}

// die prints the message and the stacks of all goroutines and ends the process.
func die(format string, args ...any) {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	fmt.Fprintf(os.Stderr, "fuzz: "+format+"\n%s\n", append(args, buf)...)
	os.Exit(3)
}

// Guard runs f, and ends the process if f runs too long or the heap grows too big. what names the
// target in the message.
func Guard(what string, f func()) {
	startGuard()
	guardMu.Lock()
	guardWhat, guardAt = what, time.Now()
	guardMu.Unlock()
	defer func() {
		guardMu.Lock()
		guardWhat = ""
		guardMu.Unlock()
	}()
	f()
}
