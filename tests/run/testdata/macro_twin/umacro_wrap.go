// The hand-written Go twin of umacro_wrap.gp: each macro is a function with the place of the use and
// the code of the decorated function (a func literal) as arguments, and each function keeps its
// results in named variables that the code sets before it returns.

package main

import (
	"fmt"
	"sync"
)

const file = "umacro_wrap.gp"

func VM(gp_file string, gp_line int, gp_func string, gp_body func()) {
	fmt.Println("vm enter", gp_func)
	defer fmt.Println("vm exit", gp_func)
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

func Once(gp_file string, gp_line int, gp_func string, gp_body func(), o *sync.Once) {
	o.Do(func() { gp_body() })
}

func Retry(gp_file string, gp_line int, gp_func string, gp_body func(), n int) {
	for i := 1; i <= n; i++ {
		fmt.Println("attempt", i, "of", n)
		gp_body()
	}
}

func Skip(gp_file string, gp_line int, gp_func string, gp_body func(), skip bool) {
	if skip {
		fmt.Println("skipped", gp_func)
		return
	}
	gp_body()
}

func Trace(gp_file string, gp_line int, gp_func string) func() {
	fmt.Println("enter", gp_func)
	return func() { fmt.Println("exit ", gp_func) }
}

func secret(key string) (gp_r0 int, gp_r1 error) {
	if gp_defer1 := Trace(file, 65, "main.secret"); gp_defer1 != nil {
		defer gp_defer1()
	}
	VM(file, 66, "main.secret", func() {
		defer fmt.Println("body defer of secret")
		if key == "" {
			gp_r0, gp_r1 = 0, fmt.Errorf("empty key")
			return
		}
		gp_r0, gp_r1 = len(key)*42, nil
		return
	})
	return
}

func risky(a, b int) (gp_r0 int) {
	Recover(file, 75, "main.risky", func() {
		gp_r0 = a / b
		return
	})
	return
}

func square(n int) (gp_r0 int) {
	InThread(file, 80, "main.square", func() {
		gp_r0 = n * n
		return
	})
	return
}

var setups int

func setup(name string) (gp_r0 int) {
	Once(file, 87, "main.setup", func() {
		setups++
		fmt.Println("setup", name)
		gp_r0 = setups
		return
	}, &setupOnce)
	return
}

var setupOnce sync.Once

var calls int

func attempt() (n int) {
	Retry(file, 98, "main.attempt", func() {
		calls++
		n = calls * 10
		return
	}, 3)
	return
}

func maybe(skip bool, v int) (gp_r0 int, gp_r1 string) {
	Skip(file, 105, "main.maybe", func() {
		gp_r0, gp_r1 = v, "ran"
		return
	}, skip)
	return
}

type Counter struct{ n int }

func (c *Counter) Inc() (total int, err error) {
	VM(file, 112, "main.(*Counter).Inc", func() {
		defer func() { total *= 2 }()
		c.n++
		if c.n > 2 {
			err = fmt.Errorf("too many: %d", c.n)
			return
		}
		total, err = c.n, nil
		return
	})
	return
}

func (c Counter) Safe(i int) (gp_r0 int, ok bool) {
	Recover(file, 123, "main.Counter.Safe", func() {
		arr := []int{1, 2, 3}
		gp_r0, ok = arr[i], true
		return
	})
	return
}

func sum(xs ...int) (t int) {
	VM(file, 129, "main.sum", func() {
		for _, x := range xs {
			t += x
		}
		return
	})
	return
}

func Max[T int | string](a, b T) (gp_r0 T) {
	Recover(file, 137, "main.Max[...]", func() {
		VM(file, 138, "main.Max[...]", func() {
			if a > b {
				gp_r0 = a
				return
			}
			gp_r0 = b
			return
		})
		return
	})
	return
}

func noResult(n *int) {
	VM(file, 146, "main.noResult", func() {
		if *n > 0 {
			*n--
			return
		}
		*n = 100
	})
}

func main() {
	fmt.Println(secret("key"))
	fmt.Println(secret(""))

	fmt.Println(risky(8, 2))
	fmt.Println(risky(1, 0))

	results := make([]int, 4)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() { results[i] = square(i + 1) })
	}
	wg.Wait()
	fmt.Println("squares:", results)

	fmt.Println(setup("a"), setup("b"), setup("c"), "setups:", setups)

	fmt.Println(attempt(), "calls:", calls)

	for _, skip := range []bool{false, true} {
		v, s := maybe(skip, 7)
		fmt.Printf("%d %q\n", v, s)
	}

	c := &Counter{}
	fmt.Println(c.Inc())
	fmt.Println(c.Inc())
	fmt.Println(c.Inc())
	fmt.Println(Counter{}.Safe(1))
	fmt.Println(Counter{}.Safe(5))

	fmt.Println(sum(), sum(1, 2, 3))
	fmt.Println(Max(3, 9), Max("b", "a"))
	k := 1
	noResult(&k)
	noResult(&k)
	fmt.Println(k)
}
