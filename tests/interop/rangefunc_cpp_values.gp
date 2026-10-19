// phase: 08

// Range over a func whose yield function takes C/C++ values (step 7.8): the range variable takes the
// type of the yield parameter, a std::string, a std::vector<int>, a C number; the key and the value
// of a two-value yield; a jump out of the body with a C++ value (return, break, labeled continue);
// the assignment form; a method value and a func literal as the iterator; a closure that keeps the
// range variable.
package main

import (
	"fmt"

	"<cstdlib>"
	"<string>"
	"<vector>"
)

func strs(yield func(std.string) bool) {
	for _, w := range []string{"alpha", "beta", "gamma"} {
		if !yield(std.string(w)) {
			return
		}
	}
}

func vecs(yield func(std.vector[int]) bool) {
	if yield(std.vector[int]{1, 20}) {
		yield(std.vector[int]{3, 4, 5})
	}
}

func ints(yield func(C.int) bool) {
	for i := 1; i <= 3; i++ {
		if !yield(C.int(i * 10)) {
			return
		}
	}
}

func pairs(yield func(std.string, C.int) bool) {
	_ = yield(std.string("one"), 1) && yield(std.string("two"), 2) && yield(std.string("three"), 3)
}

func none(yield func() bool) {
	_ = yield() && yield()
}

type bag struct{ n int }

func (b *bag) All(yield func(std.string) bool) {
	for i := 0; i < b.n; i++ {
		var s std.string = "item"
		s.append("!")
		if !yield(s) {
			return
		}
	}
}

func firstLong() std.string {
	for s := range strs {
		if int(s.size()) > 4 {
			return s
		}
	}
	return std.string("none")
}

func main() {
	for s := range strs {
		var t std.string = s
		fmt.Println(string(t), int(s.size()))
	}
	for v := range vecs {
		sum := 0
		for i := 0; i < int(v.size()); i++ {
			sum += int(v[i])
		}
		fmt.Println("vec", int(v.size()), sum)
	}
	for x := range ints {
		fmt.Println(int(x) + 1)
	}
	for s, n := range pairs {
		fmt.Println(string(s), int(n)*2)
	}
	for _, n := range pairs {
		fmt.Println("n", int(n))
	}
	count := 0
	for range none {
		count++
	}
	fmt.Println("none", count)

	fmt.Println(string(firstLong()))

	// break and labeled continue with C++ values
outer:
	for x := range ints {
		for s := range strs {
			if int(x) == 20 {
				continue outer
			}
			if string(s) == "beta" {
				break
			}
			fmt.Println(int(x), string(s))
		}
	}

	// the assignment form
	var last std.string
	for last = range strs {
	}
	fmt.Println("last", string(last))
	var key std.string
	var val C.int
	for key, val = range pairs {
		if int(val) == 2 {
			break
		}
	}
	fmt.Println("pair", string(key), int(val))

	// a method value, a func literal, and a closure that keeps the range variable
	b := &bag{2}
	for s := range b.All {
		fmt.Println(string(s))
	}
	lit := func(yield func(std.string) bool) {
		_ = yield(std.string("lit"))
	}
	var keep []func() string
	for s := range lit {
		keep = append(keep, func() string { return string(s) })
	}
	for s := range strs {
		keep = append(keep, func() string { return string(s) })
	}
	for _, f := range keep {
		fmt.Println(f())
	}
}

// Output:
// alpha 5
// beta 4
// gamma 5
// vec 2 21
// vec 3 12
// 11
// 21
// 31
// one 2
// two 4
// three 6
// n 1
// n 2
// n 3
// none 2
// alpha
// 10 alpha
// 30 alpha
// last gamma
// pair two 2
// item!
// item!
// lit
// alpha
// beta
// gamma
