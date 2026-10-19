// phase: 10

// Many wrap macros on one func (step 10.9): 3, 8 and 20 stacked on a func with no result, with unnamed and
// with named results, and on a method. How many there can be does not depend on the layout of the
// source (the macros nest by position, and gofmt can change the space between ) and {). The twin
// (testdata/macro_twin/umacro_stack.go) is plain Go: the entering and leaving written as a defer.

package main

import (
	"errors"
	"fmt"
)

// Step says when the function's code starts and ends.
macro Step(name string) {
	fmt.Println("enter", name)
	#body()
	fmt.Println("leave", name)
}

func fail(on bool) error {
	if on {
		return errors.New("failed")
	}
	return nil
}

type Counter struct{ n int }

#Step("s01")
#Step("s02")
#Step("s03")
func void3(c *Counter) {
	defer fmt.Println("body defer", 3)
	c.n += 3
}

#Step("s01")
#Step("s02")
#Step("s03")
func unnamed3(a int) (int, string) {
	if a > 0 {
		return a * 3, "positive"
	}
	return 0, "zero"
}

#Step("s01")
#Step("s02")
#Step("s03")
func named3(on bool) (n int, err error) {
	n = 3
	if err := fail(on); err != nil {
		return -1, err
	}
	return n * 2, nil
}

#Step("s01")
#Step("s02")
#Step("s03")
func (c *Counter) method3() int {
	c.n++
	return c.n * 3
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
func void8(c *Counter) {
	defer fmt.Println("body defer", 8)
	c.n += 8
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
func unnamed8(a int) (int, string) {
	if a > 0 {
		return a * 8, "positive"
	}
	return 0, "zero"
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
func named8(on bool) (n int, err error) {
	n = 8
	if err := fail(on); err != nil {
		return -1, err
	}
	return n * 2, nil
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
func (c *Counter) method8() int {
	c.n++
	return c.n * 8
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
#Step("s09")
#Step("s10")
#Step("s11")
#Step("s12")
#Step("s13")
#Step("s14")
#Step("s15")
#Step("s16")
#Step("s17")
#Step("s18")
#Step("s19")
#Step("s20")
func void20(c *Counter) {
	defer fmt.Println("body defer", 20)
	c.n += 20
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
#Step("s09")
#Step("s10")
#Step("s11")
#Step("s12")
#Step("s13")
#Step("s14")
#Step("s15")
#Step("s16")
#Step("s17")
#Step("s18")
#Step("s19")
#Step("s20")
func unnamed20(a int) (int, string) {
	if a > 0 {
		return a * 20, "positive"
	}
	return 0, "zero"
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
#Step("s09")
#Step("s10")
#Step("s11")
#Step("s12")
#Step("s13")
#Step("s14")
#Step("s15")
#Step("s16")
#Step("s17")
#Step("s18")
#Step("s19")
#Step("s20")
func named20(on bool) (n int, err error) {
	n = 20
	if err := fail(on); err != nil {
		return -1, err
	}
	return n * 2, nil
}

#Step("s01")
#Step("s02")
#Step("s03")
#Step("s04")
#Step("s05")
#Step("s06")
#Step("s07")
#Step("s08")
#Step("s09")
#Step("s10")
#Step("s11")
#Step("s12")
#Step("s13")
#Step("s14")
#Step("s15")
#Step("s16")
#Step("s17")
#Step("s18")
#Step("s19")
#Step("s20")
func (c *Counter) method20() int {
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

// Output:
// enter s01
// enter s02
// enter s03
// body defer 3
// leave s03
// leave s02
// leave s01
// enter s01
// enter s02
// enter s03
// leave s03
// leave s02
// leave s01
// 6 positive
// enter s01
// enter s02
// enter s03
// leave s03
// leave s02
// leave s01
// 0 zero
// enter s01
// enter s02
// enter s03
// leave s03
// leave s02
// leave s01
// 6 <nil>
// enter s01
// enter s02
// enter s03
// leave s03
// leave s02
// leave s01
// -1 failed
// enter s01
// enter s02
// enter s03
// leave s03
// leave s02
// leave s01
// 12 4
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// body defer 8
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 16 positive
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 0 zero
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 16 <nil>
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// -1 failed
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 104 13
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// enter s09
// enter s10
// enter s11
// enter s12
// enter s13
// enter s14
// enter s15
// enter s16
// enter s17
// enter s18
// enter s19
// enter s20
// body defer 20
// leave s20
// leave s19
// leave s18
// leave s17
// leave s16
// leave s15
// leave s14
// leave s13
// leave s12
// leave s11
// leave s10
// leave s09
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// enter s09
// enter s10
// enter s11
// enter s12
// enter s13
// enter s14
// enter s15
// enter s16
// enter s17
// enter s18
// enter s19
// enter s20
// leave s20
// leave s19
// leave s18
// leave s17
// leave s16
// leave s15
// leave s14
// leave s13
// leave s12
// leave s11
// leave s10
// leave s09
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 40 positive
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// enter s09
// enter s10
// enter s11
// enter s12
// enter s13
// enter s14
// enter s15
// enter s16
// enter s17
// enter s18
// enter s19
// enter s20
// leave s20
// leave s19
// leave s18
// leave s17
// leave s16
// leave s15
// leave s14
// leave s13
// leave s12
// leave s11
// leave s10
// leave s09
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 0 zero
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// enter s09
// enter s10
// enter s11
// enter s12
// enter s13
// enter s14
// enter s15
// enter s16
// enter s17
// enter s18
// enter s19
// enter s20
// leave s20
// leave s19
// leave s18
// leave s17
// leave s16
// leave s15
// leave s14
// leave s13
// leave s12
// leave s11
// leave s10
// leave s09
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 40 <nil>
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// enter s09
// enter s10
// enter s11
// enter s12
// enter s13
// enter s14
// enter s15
// enter s16
// enter s17
// enter s18
// enter s19
// enter s20
// leave s20
// leave s19
// leave s18
// leave s17
// leave s16
// leave s15
// leave s14
// leave s13
// leave s12
// leave s11
// leave s10
// leave s09
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// -1 failed
// enter s01
// enter s02
// enter s03
// enter s04
// enter s05
// enter s06
// enter s07
// enter s08
// enter s09
// enter s10
// enter s11
// enter s12
// enter s13
// enter s14
// enter s15
// enter s16
// enter s17
// enter s18
// enter s19
// enter s20
// leave s20
// leave s19
// leave s18
// leave s17
// leave s16
// leave s15
// leave s14
// leave s13
// leave s12
// leave s11
// leave s10
// leave s09
// leave s08
// leave s07
// leave s06
// leave s05
// leave s04
// leave s03
// leave s02
// leave s01
// 680 34
