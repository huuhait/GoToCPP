// phase: 09
// iter.Pull and iter.Pull2 beyond Go's own tests: many pulls in a row, deferred calls that run at stop, driving one
// iterator from several goroutines in turn, pulls inside pulls, a zip of two sequences, panics of several kinds
// (a value, an error, a nil dereference, an index out of range), an iterator that is abandoned in the middle, a
// sequence over a channel, and eight goroutines that pull at the same time. The expected output comes from real Go
// 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"runtime"
	"slices"
	"sync"
	"time"
)

func count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

// settle waits until runtime.NumGoroutine has been the same for 100 checks in a row (goroutines that have called
// wg.Done may still be ending), at most 5 seconds.
func settle() int {
	last, same := -1, 0
	for i := 0; i < 50000 && same < 100; i++ {
		n := runtime.NumGoroutine()
		if n == last {
			same++
		} else {
			last, same = n, 0
		}
		runtime.Gosched()
		time.Sleep(100 * time.Microsecond)
	}
	return last
}

func zip[A, B any](a iter.Seq[A], b iter.Seq[B]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		na, sa := iter.Pull(a)
		defer sa()
		nb, sb := iter.Pull(b)
		defer sb()
		for {
			x, ok1 := na()
			y, ok2 := nb()
			if !ok1 || !ok2 {
				return
			}
			if !yield(x, y) {
				return
			}
		}
	}
}

type myErr struct{ code int }

func (e *myErr) Error() string { return fmt.Sprint("myErr ", e.code) }

func try(f func()) (r any) {
	defer func() { r = recover() }()
	f()
	return nil
}

func main() {
	base := settle()

	// 1. many pulls, each stopped early or run to the end
	total := 0
	for i := 0; i < 2000; i++ {
		next, stop := iter.Pull(count(10))
		for j := 0; j < i%12; j++ {
			v, ok := next()
			if !ok {
				break
			}
			total += v
		}
		if i%3 != 0 {
			stop()
		} else {
			for {
				if _, ok := next(); !ok {
					break
				}
			}
			stop()
		}
	}
	fmt.Println("many:", total, settle()-base)

	// 2. deferred calls of the sequence run at stop, in order, and the sequence sees yield return false
	var log []string
	seq := func(yield func(string) bool) {
		defer func() { log = append(log, "outer defer") }()
		for _, s := range []string{"a", "b", "c"} {
			func() {
				defer func() { log = append(log, "defer "+s) }()
				if !yield(s) {
					log = append(log, "yield false at "+s)
				}
			}()
			if len(log) > 0 && log[len(log)-1] != "defer "+s {
				return
			}
		}
	}
	next, stop := iter.Pull(seq)
	v, _ := next()
	v2, _ := next()
	log = append(log, fmt.Sprint("got ", v, v2))
	stop()
	stop()
	fmt.Println(log)

	// 3. one iterator driven by several goroutines in turn
	n3, s3 := iter.Pull(count(100))
	var got []int
	for g := 0; g < 5; g++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 4; k++ {
				v, ok := n3()
				if ok {
					got = append(got, v)
				}
			}
		}()
		wg.Wait()
		v, _ := n3()
		got = append(got, -v)
	}
	s3()
	fmt.Println(got, settle()-base)

	// 4. zip, with pulls inside a pull
	for a, b := range zip(count(5), slices.Values([]string{"x", "y", "z"})) {
		fmt.Print(a, b, ";")
	}
	fmt.Println()
	var inner iter.Seq[int] = func(yield func(int) bool) {
		n, st := iter.Pull2(zip(count(4), count(6)))
		defer st()
		for {
			a, b, ok := n()
			if !ok || !yield(a*10+b) {
				return
			}
		}
	}
	for v, c := range zip(inner, maps.Keys(map[string]bool{"only": true})) {
		fmt.Println(v, c)
	}
	fmt.Println("nested:", settle()-base)

	// 5. panics: a value, an error, a nil dereference, an index out of range; then the iterator is dead
	var np *myErr
	idx := 5
	arr := []int{1, 2}
	for i, f := range []func(yield func(int) bool){
		func(yield func(int) bool) { yield(1); panic("boom") },
		func(yield func(int) bool) { yield(1); panic(&myErr{42}) },
		func(yield func(int) bool) { yield(1); fmt.Print(np.code) },
		func(yield func(int) bool) { yield(1); fmt.Print(arr[idx]) },
		func(yield func(int) bool) { yield(1); panic(fmt.Errorf("wrapped: %w", errors.New("inner"))) },
	} {
		next, stop := iter.Pull(iter.Seq[int](f))
		v, ok := next()
		r := try(func() { next() })
		_, ok2 := next()
		r2 := try(stop)
		fmt.Println(i, v, ok, r, ok2, r2)
	}
	fmt.Println("panics:", settle()-base)

	// 6. a panic at stop (the sequence panics when yield returns false), with Pull2
	n6, s6 := iter.Pull2(func(yield func(int, string) bool) {
		for i := 0; ; i++ {
			if !yield(i, fmt.Sprint("s", i)) {
				panic(fmt.Sprint("cleanup failed after ", i))
			}
		}
	})
	k, s, ok := n6()
	k, s, ok = n6()
	fmt.Println(k, s, ok)
	fmt.Println(try(s6), try(s6))
	k, s, ok = n6()
	fmt.Println(k, s, ok, settle()-base)

	// 7. a sequence over a channel, pulled
	ch := make(chan int)
	go func() {
		for i := 0; i < 5; i++ {
			ch <- i * i
		}
		close(ch)
	}()
	chseq := func(yield func(int) bool) {
		for v := range ch {
			if !yield(v) {
				return
			}
		}
	}
	n7, s7 := iter.Pull(chseq)
	sum := 0
	for {
		v, ok := n7()
		if !ok {
			break
		}
		sum += v
	}
	s7()
	fmt.Println("chan:", sum, settle()-base)

	// 8. eight goroutines pull at the same time
	var wg sync.WaitGroup
	results := make([]int, 8)
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < 200; r++ {
				next, stop := iter.Pull(count(20 + g))
				for {
					v, ok := next()
					if !ok {
						break
					}
					results[g] += v
				}
				stop()
			}
		}()
	}
	wg.Wait()
	fmt.Println(results, settle()-base)

	// 9. a Pull that is abandoned in the middle keeps its goroutine, as in Go; one that was never started too
	na, _ := iter.Pull(count(10))
	na()
	nb, _ := iter.Pull(count(10))
	_ = nb
	fmt.Println("abandoned:", settle()-base)

	// 10. Go's documented use: values of a slice and map as pull iterators
	nx, sx := iter.Pull2(maps.All(map[int]int{1: 1}))
	fmt.Println(nx())
	fmt.Println(nx())
	sx()
}

// Output:
// many: 34916 0
// [defer a got ab yield false at b defer b yield false at c defer c outer defer]
// [0 1 2 3 -4 5 6 7 8 -9 10 11 12 13 -14 15 16 17 18 -19 20 21 22 23 -24] 0
// 0x;1y;2z;
// 0 only
// nested: 0
// 0 1 true boom false <nil>
// 1 1 true myErr 42 false <nil>
// 2 1 true runtime error: invalid memory address or nil pointer dereference false <nil>
// 3 1 true runtime error: index out of range [5] with length 2 false <nil>
// 4 1 true wrapped: inner false <nil>
// panics: 0
// 1 s1 true
// cleanup failed after 1 <nil>
// 0  false 0
// chan: 30 0
// [38000 42000 46200 50600 55200 60000 65000 70200] 0
// abandoned: 2
// 1 1 true
// 0 0 false
