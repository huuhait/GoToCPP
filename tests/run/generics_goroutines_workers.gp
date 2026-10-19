// phase: 08

// Goroutines, mutexes and atomics with generic types: a worker pool over generic jobs, a typed
// concurrent map, atomic.Pointer[T], sync.OnceValue(s), a WaitGroup in a generic struct, results
// collected through channels and sorted, and a panic in a worker turned into an error.
package main

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
)

type Job[T any] struct {
	ID   int
	Data T
}

type Result[R any] struct {
	ID  int
	Val R
	Err error
}

func Pool[T, R any](workers int, jobs []Job[T], f func(T) R) []Result[R] {
	in := make(chan Job[T])
	out := make(chan Result[R], len(jobs))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range in {
				func() {
					defer func() {
						if r := recover(); r != nil {
							out <- Result[R]{ID: j.ID, Err: fmt.Errorf("job %d: %v", j.ID, r)}
						}
					}()
					out <- Result[R]{ID: j.ID, Val: f(j.Data)}
				}()
			}
		}()
	}
	for _, j := range jobs {
		in <- j
	}
	close(in)
	wg.Wait()
	close(out)
	var rs []Result[R]
	for r := range out {
		rs = append(rs, r)
	}
	slices.SortFunc(rs, func(a, b Result[R]) int { return a.ID - b.ID })
	return rs
}

type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func (s *SafeMap[K, V]) Update(k K, f func(V, bool) V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[K]V{}
	}
	old, ok := s.m[k]
	s.m[k] = f(old, ok)
}

func (s *SafeMap[K, V]) Get(k K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[k]
	return v, ok
}

type Config struct {
	Name string
	N    int
}

func main() {
	jobs := make([]Job[int], 8)
	for i := range jobs {
		jobs[i] = Job[int]{i, i * 10}
	}
	for _, r := range Pool(3, jobs, func(x int) int8 {
		if x == 30 {
			panic("thirty")
		}
		return int8(x * 3)
	}) {
		fmt.Println(r.ID, r.Val, r.Err)
	}
	rs := Pool(4, []Job[string]{{1, "a"}, {2, "bb"}, {3, ""}}, func(s string) byte { return s[0] })
	for _, r := range rs {
		fmt.Println(r.ID, r.Val, r.Err)
	}
	var sm SafeMap[string, []int]
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.Update(fmt.Sprint("k", i%3), func(old []int, _ bool) []int { return append(old, i) })
		}()
	}
	wg.Wait()
	total := 0
	for _, k := range []string{"k0", "k1", "k2"} {
		v, ok := sm.Get(k)
		total += len(v)
		fmt.Println(k, ok, len(v))
	}
	fmt.Println("total", total)

	var cfg atomic.Pointer[Config]
	fmt.Println(cfg.Load() == nil)
	cfg.Store(&Config{"a", 1})
	old := cfg.Swap(&Config{"b", 2})
	fmt.Println(old.Name, cfg.Load().Name, cfg.CompareAndSwap(old, &Config{"c", 3}), cfg.Load().N)
	var cnt atomic.Int64
	var big atomic.Pointer[[]int]
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cnt.Add(int64(i))
			s := []int{i}
			big.Store(&s)
		}()
	}
	wg.Wait()
	fmt.Println(cnt.Load(), len(*big.Load()))

	calls := 0
	once := sync.OnceValue(func() Config { calls++; return Config{"once", calls} })
	once2 := sync.OnceValues(func() (int8, error) { calls++; return 7, nil })
	for range 3 {
		wg.Add(1)
		go func() { defer wg.Done(); once(); once2() }()
	}
	wg.Wait()
	v, err := once2()
	fmt.Println(once(), v, err, calls)
}

// Output:
// 0 0 <nil>
// 1 30 <nil>
// 2 60 <nil>
// 3 0 job 3: thirty
// 4 120 <nil>
// 5 -106 <nil>
// 6 -76 <nil>
// 7 -46 <nil>
// 1 97 <nil>
// 2 98 <nil>
// 3 0 job 3: runtime error: index out of range [0] with length 0
// k0 true 7
// k1 true 7
// k2 true 6
// total 20
// true
// a b false 2
// 1225 1
// {once 1} 7 <nil> 2
