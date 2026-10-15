// phase: 03

// Sorting without package sort (Phase 07): insertion sort and quicksort over slices of
// structs, driven by func values (less functions, closures over a key), a stable merge sort,
// and a sort of an array through a slice of it.
package main

import "fmt"

type Person struct {
	Name string
	Age  int
}

type Less func(a, b Person) bool

func insertion(ps []Person, less Less) {
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && less(ps[j], ps[j-1]); j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}

func quick(xs []int) {
	if len(xs) < 2 {
		return
	}
	p := xs[len(xs)/2]
	i, j := 0, len(xs)-1
	for i <= j {
		for xs[i] < p {
			i++
		}
		for xs[j] > p {
			j--
		}
		if i <= j {
			xs[i], xs[j] = xs[j], xs[i]
			i++
			j--
		}
	}
	quick(xs[:j+1])
	quick(xs[i:])
}

func mergeSort(ps []Person, less Less) []Person {
	if len(ps) <= 1 {
		return ps
	}
	mid := len(ps) / 2
	a, b := mergeSort(ps[:mid], less), mergeSort(ps[mid:], less)
	out := make([]Person, 0, len(ps))
	for len(a) > 0 && len(b) > 0 {
		if less(b[0], a[0]) {
			out, b = append(out, b[0]), b[1:]
		} else {
			out, a = append(out, a[0]), a[1:]
		}
	}
	return append(append(out, a...), b...)
}

func by(key func(Person) int) Less {
	return func(a, b Person) bool { return key(a) < key(b) }
}

func main() {
	ps := []Person{{"eve", 31}, {"bob", 25}, {"amy", 31}, {"dan", 19}, {"cat", 25}}
	byName := func(a, b Person) bool { return a.Name < b.Name }
	cp := append([]Person(nil), ps...)
	insertion(cp, byName)
	fmt.Println(cp)
	fmt.Println(mergeSort(ps, by(func(p Person) int { return p.Age })))
	fmt.Println(mergeSort(ps, by(func(p Person) int { return -len(p.Name) - p.Age })))
	fmt.Println(ps[0], len(ps))

	arr := [10]int{5, 3, 9, 1, 7, 3, 8, 2, 6, 0}
	quick(arr[:])
	fmt.Println(arr)
	xs := []int{}
	for i := 0; i < 50; i++ {
		xs = append(xs, (i*37+11)%50)
	}
	quick(xs)
	sorted := true
	for i := 1; i < len(xs); i++ {
		if xs[i-1] > xs[i] {
			sorted = false
		}
	}
	fmt.Println(sorted, xs[0], xs[49], xs[:5])
}

// Output:
// [{amy 31} {bob 25} {cat 25} {dan 19} {eve 31}]
// [{dan 19} {bob 25} {cat 25} {eve 31} {amy 31}]
// [{eve 31} {amy 31} {bob 25} {cat 25} {dan 19}]
// {eve 31} 5
// [0 1 2 3 3 5 6 7 8 9]
// true 0 49 [0 1 2 3 4]
