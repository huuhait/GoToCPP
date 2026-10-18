// phase: 05

package main

import "fmt"

type counter struct {
	n    int
	hits []int
}

func (c *counter) guard(i int) {
	defer func() {
		if r := recover(); r != nil {
			c.hits = append(c.hits, i)
		}
	}()
	c.n++
	if i%3 == 0 {
		panic(fmt.Sprint("multiple of three: ", i))
	}
}

func makeAdder() (func(int), func() int) {
	total := 0
	return func(n int) {
			defer func() { total += n }()
			if n < 0 {
				panic("negative")
			}
		}, func() int {
			return total
		}
}

func main() {
	c := &counter{}
	for i := 1; i <= 9; i++ {
		c.guard(i)
	}
	fmt.Println(c.n, c.hits)
	add, get := makeAdder()
	for _, n := range []int{5, 3} {
		add(n)
	}
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		add(-4)
	}()
	fmt.Println(get())
}
