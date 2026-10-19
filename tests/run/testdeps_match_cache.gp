// phase: 09
// std-internal: tests testing/internal/testdeps, which only the standard library may import

// The matcher behind -test.run, -skip, -list and -bench keeps the compiled patterns (gocpp compiles slower than Go, and a pattern
// with / alternates between the patterns of its elements): two patterns used in turn compile once each, a pattern that does not
// compile is compiled once and gives the same error again, and the cache stays small (at most 64 entries) while giving the right answers.

package main

import (
	"fmt"
	"strconv"
	"testing/internal/testdeps"
)

func main() {
	d := testdeps.TestDeps{}
	base := testdeps.Compiles
	ok := true
	for i := 0; i < 1000; i++ {
		a, err := d.MatchString("^Test", "TestMany")
		b, _ := d.MatchString("sub[0-9]+", "sub"+strconv.Itoa(i))
		c, _ := d.MatchString("^Test", "xTest")
		ok = ok && err == nil && a && b && !c
	}
	fmt.Println("two patterns in turn:", testdeps.Compiles-base, "compiles, answers right:", ok)

	base = testdeps.Compiles
	ok = true
	for i := 0; i < 100; i++ {
		m, _ := d.MatchString("^p"+strconv.Itoa(i)+"$", "p"+strconv.Itoa(i))
		n, _ := d.MatchString("^p"+strconv.Itoa(i)+"$", "p"+strconv.Itoa(i+1))
		ok = ok && m && !n
	}
	fmt.Println("100 different patterns:", testdeps.Compiles-base, "compiles, answers right:", ok)
	base = testdeps.Compiles
	for i := 0; i < 100; i++ {
		m, _ := d.MatchString("^p"+strconv.Itoa(i)+"$", "p"+strconv.Itoa(i))
		ok = ok && m
	}
	fmt.Println("again, at most 100 compiles:", testdeps.Compiles-base <= 100, ok)

	base = testdeps.Compiles
	_, err1 := d.MatchString("(", "x")
	_, err2 := d.MatchString("(", "y")
	fmt.Println("invalid pattern:", testdeps.Compiles-base, "compile")
	fmt.Println(err1)
	fmt.Println(err1 == err2)
}

// Output:
// two patterns in turn: 2 compiles, answers right: true
// 100 different patterns: 100 compiles, answers right: true
// again, at most 100 compiles: true true
// invalid pattern: 1 compile
// error parsing regexp: missing closing ): `(`
// true
