// phase: 03

// Variadic functions: calls with no arguments (a nil slice), one, several, f(s...) (the
// callee shares s's array, as in Go), an append-like function, variadic func values and
// literals, a variadic parameter after others, and a variadic of structs.
package main

import "fmt"

func count(xs ...int) (int, bool) {
	return len(xs), xs == nil
}

func sum(xs ...int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func appendAll(dst []string, xs ...string) []string {
	for _, x := range xs {
		dst = append(dst, x)
	}
	return dst
}

func double(xs ...int) {
	for i := range xs {
		xs[i] *= 2
	}
}

type pt struct{ x, y int }

func far(origin pt, ps ...pt) int {
	best := 0
	for _, p := range ps {
		d := (p.x-origin.x)*(p.x-origin.x) + (p.y-origin.y)*(p.y-origin.y)
		if d > best {
			best = d
		}
	}
	return best
}

func main() {
	fmt.Println(count())
	fmt.Println(count(1))
	fmt.Println(count(1, 2, 3))
	var none []int
	fmt.Println(count(none...))
	fmt.Println(count([]int{}...))

	nums := []int{1, 2, 3}
	fmt.Println(sum(), sum(nums...), sum(4, 5))
	double(nums...)
	fmt.Println(nums[0], nums[1], nums[2])
	double(nums[1:]...)
	fmt.Println(nums[0], nums[1], nums[2])

	words := appendAll(nil, "a", "b")
	words = appendAll(words, words...)
	words = appendAll(words)
	fmt.Println(len(words), words[0], words[3])

	vf := sum
	fmt.Println(vf(), vf(10, 20), vf(nums...))
	join := func(sep string, parts ...string) string {
		s := ""
		for i, p := range parts {
			if i > 0 {
				s += sep
			}
			s += p
		}
		return s
	}
	fmt.Println(join("-"), join("-", "x"), join("-", words...))
	fmt.Println(func(xs ...int) int { return len(xs) }(1, 2, 3, 4))

	fmt.Println(far(pt{}, pt{1, 1}, pt{3, 4}, pt{-2, 0}), far(pt{1, 1}))
}

// Output:
// 0 true
// 1 false
// 3 false
// 0 true
// 0 false
// 0 6 9
// 2 4 6
// 2 8 12
// 4 a b
// 0 30 22
//  x a-b-a-b
// 4
// 25 0
