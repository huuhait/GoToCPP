// bench: quicksort
// category: cpu
// size: 3000000
// smoke: 3000
// twin: yes
// about: hand-written quicksort (median of three, insertion sort for short ranges) of an LCG-generated []int
package main

import "fmt"

func insertion(a []int, lo, hi int) {
	for i := lo + 1; i <= hi; i++ {
		v := a[i]
		j := i - 1
		for j >= lo && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}

func quicksort(a []int, lo, hi int) {
	for hi-lo > 12 {
		mid := lo + (hi-lo)/2
		if a[mid] < a[lo] {
			a[mid], a[lo] = a[lo], a[mid]
		}
		if a[hi] < a[lo] {
			a[hi], a[lo] = a[lo], a[hi]
		}
		if a[hi] < a[mid] {
			a[hi], a[mid] = a[mid], a[hi]
		}
		pivot := a[mid]
		i, j := lo, hi
		for i <= j {
			for a[i] < pivot {
				i++
			}
			for a[j] > pivot {
				j--
			}
			if i <= j {
				a[i], a[j] = a[j], a[i]
				i++
				j--
			}
		}
		if j-lo < hi-i {
			quicksort(a, lo, j)
			lo = i
		} else {
			quicksort(a, i, hi)
			hi = j
		}
	}
	insertion(a, lo, hi)
}

func main() {
	n := benchN
	a := make([]int, n)
	seed := uint64(12345)
	for i := range a {
		seed = seed*6364136223846793005 + 1442695040888963407
		a[i] = int(seed >> 40)
	}
	quicksort(a, 0, n-1)
	ok := true
	h := uint64(1469598103934665603)
	for i, v := range a {
		if i > 0 && a[i-1] > v {
			ok = false
		}
		h = (h ^ uint64(v)) * 1099511628211
	}
	fmt.Println(ok, a[0], a[n/2], a[n-1], h)
}
