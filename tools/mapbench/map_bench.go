//go:build ignore

// map_bench.go is tools/mapbench/map_bench.cpp in Go: the same operations at the same sizes,
// CPU time, the best of a few repeats. Run: GOGC=off go run tools/mapbench/map_bench.go [int|str] [sizes...]
package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"syscall"
)

func cpuNS() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Utime.Nano() + ru.Stime.Nano())
}

var sink int64

func best(repeats int, ops int64, f func()) float64 {
	b := 1e30
	for r := 0; r < repeats; r++ {
		t0 := cpuNS()
		f()
		t := (cpuNS() - t0) / float64(ops)
		if t < b {
			b = t
		}
	}
	return b
}

func heap() uint64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func run[K comparable](kname string, n int64, mk func(int64) K) {
	keys := make([]K, n)
	misses := make([]K, n)
	rng := rand.New(rand.NewSource(12345))
	for i := range keys {
		keys[i] = mk(int64(rng.Uint64() >> 1))
	}
	for i := range misses {
		misses[i] = mk(int64(rng.Uint64() >> 1))
	}
	reps := max(1, 4000000/n)
	tries := 3
	if n >= 1000000 {
		tries = 2
	}
	report := func(op string, ns float64) { fmt.Printf("%s %s %d %.2f\n", op, kname, n, ns) }
	report("insert", best(tries, n*reps, func() {
		for r := int64(0); r < reps; r++ {
			m := make(map[K]int64)
			for i := int64(0); i < n; i++ {
				m[keys[i]] = i
			}
			sink = int64(len(m))
		}
	}))
	report("insert_hint", best(tries, n*reps, func() {
		for r := int64(0); r < reps; r++ {
			m := make(map[K]int64, n)
			for i := int64(0); i < n; i++ {
				m[keys[i]] = i
			}
			sink = int64(len(m))
		}
	}))
	before := heap()
	m := make(map[K]int64)
	for i := int64(0); i < n; i++ {
		m[keys[i]] = i
	}
	after := heap()
	if n >= 1000 {
		fmt.Printf("bytes_per_entry %s %d %.1f\n", kname, n, float64(after-before)/float64(n))
	}
	lreps := max(1, 8000000/n)
	report("lookup_hit", best(tries, n*lreps, func() {
		var s int64
		for r := int64(0); r < lreps; r++ {
			for i := int64(0); i < n; i++ {
				s += m[keys[i]]
			}
		}
		sink = s
	}))
	report("lookup_miss", best(tries, n*lreps, func() {
		var s int64
		for r := int64(0); r < lreps; r++ {
			for i := int64(0); i < n; i++ {
				s += m[misses[i]]
			}
		}
		sink = s
	}))
	report("range", best(tries, n*lreps, func() {
		var s int64
		for r := int64(0); r < lreps; r++ {
			for _, v := range m {
				s += v
			}
		}
		sink = s
	}))
	report("update", best(tries, n*lreps, func() {
		for r := int64(0); r < lreps; r++ {
			for i := int64(0); i < n; i++ {
				m[keys[i]]++
			}
		}
	}))
	{
		del := 1e30
		for t := 0; t < tries+2; t++ {
			tables := make([]map[K]int64, reps)
			for r := range tables {
				tables[r] = make(map[K]int64)
				for i := int64(0); i < n; i++ {
					tables[r][keys[i]] = i
				}
			}
			t0 := cpuNS()
			for _, d := range tables {
				for i := int64(0); i < n; i++ {
					delete(d, keys[i])
				}
			}
			del = min(del, (cpuNS()-t0)/float64(n*reps))
		}
		report("delete", del)
	}
	{
		c := make(map[K]int64)
		half := max(n/2, 1)
		for i := int64(0); i < half; i++ {
			c[keys[i]] = i
		}
		report("churn", best(tries, n*reps, func() {
			for r := int64(0); r < reps; r++ {
				for i := int64(0); i < n; i++ {
					delete(c, keys[i%n])
					c[keys[(i+half)%n]] = i
				}
			}
		}))
	}
}

type skey struct{ a, b int64 }

func main() {
	sizes := []int64{10, 1000, 100000, 10000000}
	ints, strs, structs := true, true, true
	if len(os.Args) > 1 {
		sizes = nil
		ints, strs, structs = false, false, false
		picked := false
		for _, a := range os.Args[1:] {
			switch a {
			case "int":
				ints, picked = true, true
			case "str":
				strs, picked = true, true
			case "struct":
				structs, picked = true, true
			default:
				v, _ := strconv.ParseInt(a, 10, 64)
				sizes = append(sizes, v)
			}
		}
		if !picked {
			ints, strs, structs = true, true, true
		}
		if sizes == nil {
			sizes = []int64{10, 1000, 100000, 10000000}
		}
	}
	for _, n := range sizes {
		if ints {
			run("int", n, func(v int64) int64 { return v })
		}
		if strs {
			run("string", n, func(v int64) string { return "key-" + strconv.FormatInt(v, 10) })
		}
		if structs {
			run("struct", n, func(v int64) skey { return skey{v, v >> 7} })
		}
	}
}
