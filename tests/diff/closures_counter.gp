// phase: 03

package main

import "fmt"

func gen(start, step int) (next func() int, peek func() int, reset func()) {
	n := start
	next = func() int {
		v := n
		n += step
		return v
	}
	peek = func() int { return n }
	reset = func() { n = start }
	return
}

func memo(f func(int) int) func(int) int {
	cache := map[int]int{}
	calls := 0
	return func(x int) int {
		if v, ok := cache[x]; ok {
			return v
		}
		calls++
		v := f(x)
		cache[x] = v
		fmt.Println("computed", x, "calls", calls)
		return v
	}
}

func main() {
	next, peek, reset := gen(10, 5)
	fmt.Println(next(), next(), peek())
	reset()
	fmt.Println(next(), peek())
	n2, p2, _ := gen(0, 1)
	fmt.Println(n2(), n2(), p2(), peek())

	sq := memo(func(x int) int { return x * x })
	fmt.Println(sq(3), sq(4), sq(3))

	var chain []func(int) int
	for i := 1; i <= 3; i++ {
		prev := func(x int) int { return x }
		if len(chain) > 0 {
			prev = chain[len(chain)-1]
		}
		chain = append(chain, func(x int) int { return prev(x)*10 + i })
	}
	fmt.Println(chain[0](0), chain[1](0), chain[2](0))
}
