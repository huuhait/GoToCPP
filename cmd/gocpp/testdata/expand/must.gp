// Must shows what ! becomes: every context of a must call, a must function
// with a declared error, and main!.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// plain is not touched: it is printed as written, even the spacing.
func plain(a,b   int) int {
	x  :=  a+b   // two spaces stay
	s := `raw
string`
	_ = s
	return x
}

// load is a must function: the dropped errors are returned.
func load!(path string) (n int, err error) {
	defer errors.Wrapf(&err, "load %s", path)
	f := os.Open(path)
	defer f.Close()

	// the first line
	n = strconv.Atoi(path)
	return n, nil
}

// half is a must function without a declared error.
func half!(s string) int {
	n := strconv.Atoi(s)
	if n < 0 {
		return 0, fmt.Errorf("%d is negative", n)
	}
	return n / 2
}

func contexts(s string) (int, error) {
	// an assignment
	a := strconv.Atoi!(s)

	// a condition, a loop and a short-circuit
	if v := strconv.Atoi!(s); v > a {
		a = v
	}
	for i := 0; i < strconv.Atoi!(s); i++ {
		a++
	}
	ok := a > 0 && strconv.Itoa(strconv.Atoi!(s)) != ""
	_ = ok
	switch strconv.Atoi!(s) {
	case 1:
		a++
	}
	defer fmt.Println(strconv.Atoi!(s))
	return a, nil
}

func nopanic(s string) int {
	return strconv.Atoi!(s) // panics on an error
}

func main!() {
	defer fmt.Println("done")
	n := half("8")
	fmt.Println(n, plain(1, 2))
}
