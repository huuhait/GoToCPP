// The hand-written Go twin of umacro_wrap_spike.gp: spikes/2026-10-13/macros-v3-wrap/main.go with the
// place of each macro use as arguments and the code of each wrapped function moved into a func literal.

package main

import (
	"errors"
	"fmt"
	"sync"
)

func Trace(gp_file string, gp_line int, gp_func string) func() {
	fmt.Println("enter", gp_func)
	return func() { fmt.Println("exit ", gp_func) }
}

func VM(gp_file string, gp_line int, gp_func string, gp_body func()) {
	fmt.Println("  vm enter:", gp_func)
	defer fmt.Println("  vm exit: ", gp_func)
	gp_body()
}

func Recover(gp_file string, gp_line int, gp_func string, gp_body func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", gp_func+":", r)
		}
	}()
	gp_body()
}

func InThread(gp_file string, gp_line int, gp_func string, gp_body func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		gp_body()
	}()
	<-done
}

func Once(gp_file string, gp_line int, gp_func string, o *sync.Once, gp_body func()) {
	o.Do(func() { gp_body() })
}

func secret(key string) (gp_r0 int, gp_r1 error) {
	if gp_defer1 := Trace("umacro_wrap_spike.gp", 49, "main.secret"); gp_defer1 != nil {
		defer gp_defer1()
	}
	VM("umacro_wrap_spike.gp", 50, "main.secret", func() {
		gp_r0, gp_r1 = func() (int, error) {
			if key == "" {
				return 0, errors.New("empty key")
			}
			defer fmt.Println("  secret: body defer")
			return len(key) * 42, nil
		}()
	})
	return
}

func risky(n int) (gp_r0 int) {
	Recover("umacro_wrap_spike.gp", 59, "main.risky", func() {
		gp_r0 = func() int { return 10 / n }()
	})
	return
}

func work(id int) (gp_r0 string) {
	InThread("umacro_wrap_spike.gp", 62, "main.work", func() {
		gp_r0 = func() string { return fmt.Sprint("job ", id, " done") }()
	})
	return
}

var (
	setupOnce sync.Once
	calls     int
)

func setup() (gp_r0 int) {
	Once("umacro_wrap_spike.gp", 70, "main.setup", &setupOnce, func() {
		gp_r0 = func() int {
			calls++
			return calls
		}()
	})
	return
}

func main() {
	fmt.Println(secret("key"))
	fmt.Println(secret(""))
	fmt.Println("risky(2) =", risky(2))
	fmt.Println("risky(0) =", risky(0))

	var wg sync.WaitGroup
	results := make([]string, 4)
	for i := range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); results[i] = work(i) }()
	}
	wg.Wait()
	fmt.Println(results)

	fmt.Println("setup:", setup(), setup(), "calls:", calls)
}
