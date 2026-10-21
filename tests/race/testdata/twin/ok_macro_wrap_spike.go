package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var entered, left, inVM, vmCalls, recovered atomic.Int64

func Trace() func() {
	entered.Add(1)
	return func() { left.Add(1) }
}

func VM(body func()) {
	inVM.Add(1)
	defer inVM.Add(-1)
	vmCalls.Add(1)
	body()
}

func Recover(body func()) {
	defer func() {
		if recover() != nil {
			recovered.Add(1)
		}
	}()
	body()
}

func InThread(body func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		body()
	}()
	<-done
}

func Once(o *sync.Once, body func()) {
	o.Do(func() { body() })
}

func secret(key string) (gp_r0 int, gp_r1 bool) {
	if f := Trace(); f != nil {
		defer f()
	}
	VM(func() {
		gp_r0, gp_r1 = func() (int, bool) {
			if key == "" {
				return 0, false
			}
			return len(key) * 42, true
		}()
	})
	return
}

func risky(n int) (gp_r0 int) {
	Recover(func() {
		gp_r0 = func() int { return 10 / n }()
	})
	return
}

func work(id int) (gp_r0 string) {
	InThread(func() {
		gp_r0 = func() string { return fmt.Sprint("job ", id, " done") }()
	})
	return
}

var (
	setupOnce sync.Once
	setups    int
)

func setup() (gp_r0 int) {
	Once(&setupOnce, func() {
		gp_r0 = func() int {
			setups++
			return setups
		}()
	})
	return
}

func main() {
	const goroutines = 16
	var wg sync.WaitGroup
	secrets := make([]int, goroutines)
	divided := make([]int, goroutines)
	jobs := make([]string, goroutines)
	for g := range goroutines {
		wg.Go(func() {
			key := "key"
			if g%4 == 0 {
				key = ""
			}
			secrets[g], _ = secret(key)
			divided[g] = risky(g % 3)
			jobs[g] = work(g)
			setup()
		})
	}
	wg.Wait()

	sum := 0
	for _, n := range secrets {
		sum += n
	}
	fmt.Println("secrets", sum)
	fmt.Println("divided", divided)
	fmt.Println(jobs[0], "/", jobs[15])
	fmt.Println("setups", setups, "recovered", recovered.Load())
	fmt.Println("trace", entered.Load(), left.Load(), "vm", inVM.Load(), vmCalls.Load())
}
