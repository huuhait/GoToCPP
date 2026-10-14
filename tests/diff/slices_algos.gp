// phase: 03

// Everyday slice code: an insertion sort, a sieve, a binary search, prefix sums, a stack of
// slices, reversing and filtering in place, and building words from runes.
package main

import "fmt"

func insertionSort(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func sieve(n int) []int {
	composite := make([]bool, n+1)
	var primes []int
	for i := 2; i <= n; i++ {
		if composite[i] {
			continue
		}
		primes = append(primes, i)
		for j := i * i; j <= n; j += i {
			composite[j] = true
		}
	}
	return primes
}

func search(a []int, x int) int {
	lo, hi := 0, len(a)
	for lo < hi {
		mid := (lo + hi) / 2
		if a[mid] < x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

func reverse(s []rune) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func filter(a []int, keep int) []int {
	out := a[:0]
	for _, v := range a {
		if v%keep == 0 {
			out = append(out, v)
		}
	}
	return out
}

func words(s string) []string {
	var out []string
	var cur []rune
	for _, r := range s {
		if r == ' ' {
			if len(cur) > 0 {
				out = append(out, string(cur))
				cur = cur[:0]
			}
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

func main() {
	a := []int{5, 2, 9, 1, 5, 6, -3, 0}
	insertionSort(a)
	for _, v := range a {
		fmt.Print(v, " ")
	}
	fmt.Println(search(a, 5), search(a, 100), search(a, -10))

	p := sieve(100)
	fmt.Println(len(p), p[0], p[len(p)-1], search(p, 53))

	sums := make([]int, len(p)+1)
	for i, v := range p {
		sums[i+1] = sums[i] + v
	}
	fmt.Println(sums[len(sums)-1], sums[10]-sums[5])

	var stack [][]int
	stack = append(stack, []int{1}, []int{2, 3})
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(top) > 1 {
			stack = append(stack, top[1:])
		}
		fmt.Print(top[0], " ")
	}
	fmt.Println()

	r := []rune("héllo, wörld")
	reverse(r)
	fmt.Println(string(r))
	f := filter([]int{1, 2, 3, 4, 5, 6, 7, 8, 9}, 3)
	fmt.Println(len(f), f[0], f[1], f[2])
	w := words("  the quick  brown fox ")
	fmt.Println(len(w), w[0], w[1], w[2], w[3])
	var b []byte
	for i := len(w) - 1; i >= 0; i-- {
		b = append(b, w[i]...)
		if i > 0 {
			b = append(b, '-')
		}
	}
	fmt.Println(string(b))
}
