// The plain-Go twin of umacro_stack.gp: the Step macros as a function that prints on entry and, deferred, on
// exit.

package main

import (
	"errors"
	"fmt"
)

func steps(names ...string) func() {
	for _, n := range names {
		fmt.Println("enter", n)
	}
	return func() {
		for i := len(names) - 1; i >= 0; i-- {
			fmt.Println("leave", names[i])
		}
	}
}

func fail(on bool) error {
	if on {
		return errors.New("failed")
	}
	return nil
}

type Counter struct{ n int }

func void3(c *Counter) {
	defer steps("s01", "s02", "s03")()
	defer fmt.Println("body defer", 3)
	c.n += 3
}

func unnamed3(a int) (int, string) {
	defer steps("s01", "s02", "s03")()
	if a > 0 {
		return a * 3, "positive"
	}
	return 0, "zero"
}

func named3(on bool) (n int, err error) {
	defer steps("s01", "s02", "s03")()
	n = 3
	if err := fail(on); err != nil {
		return -1, err
	}
	return n * 2, nil
}

func (c *Counter) method3() int {
	defer steps("s01", "s02", "s03")()
	c.n++
	return c.n * 3
}

func void8(c *Counter) {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08")()
	defer fmt.Println("body defer", 8)
	c.n += 8
}

func unnamed8(a int) (int, string) {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08")()
	if a > 0 {
		return a * 8, "positive"
	}
	return 0, "zero"
}

func named8(on bool) (n int, err error) {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08")()
	n = 8
	if err := fail(on); err != nil {
		return -1, err
	}
	return n * 2, nil
}

func (c *Counter) method8() int {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08")()
	c.n++
	return c.n * 8
}

func void20(c *Counter) {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08", "s09", "s10", "s11", "s12", "s13", "s14", "s15", "s16", "s17", "s18", "s19", "s20")()
	defer fmt.Println("body defer", 20)
	c.n += 20
}

func unnamed20(a int) (int, string) {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08", "s09", "s10", "s11", "s12", "s13", "s14", "s15", "s16", "s17", "s18", "s19", "s20")()
	if a > 0 {
		return a * 20, "positive"
	}
	return 0, "zero"
}

func named20(on bool) (n int, err error) {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08", "s09", "s10", "s11", "s12", "s13", "s14", "s15", "s16", "s17", "s18", "s19", "s20")()
	n = 20
	if err := fail(on); err != nil {
		return -1, err
	}
	return n * 2, nil
}

func (c *Counter) method20() int {
	defer steps("s01", "s02", "s03", "s04", "s05", "s06", "s07", "s08", "s09", "s10", "s11", "s12", "s13", "s14", "s15", "s16", "s17", "s18", "s19", "s20")()
	c.n++
	return c.n * 20
}

func main() {
	c := &Counter{}
	void3(c)
	fmt.Println(unnamed3(2))
	fmt.Println(unnamed3(0))
	fmt.Println(named3(false))
	fmt.Println(named3(true))
	fmt.Println(c.method3(), c.n)
	void8(c)
	fmt.Println(unnamed8(2))
	fmt.Println(unnamed8(0))
	fmt.Println(named8(false))
	fmt.Println(named8(true))
	fmt.Println(c.method8(), c.n)
	void20(c)
	fmt.Println(unnamed20(2))
	fmt.Println(unnamed20(0))
	fmt.Println(named20(false))
	fmt.Println(named20(true))
	fmt.Println(c.method20(), c.n)
}
