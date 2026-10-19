// phase: 07

package main

import "fmt"

type IntSeq func(yield func(int) bool)

func Sum[S ~func(yield func(int) bool)](s S) (t int) {
	for x := range s {
		t += x
	}
	return
}

func Gen[K comparable, V any, S ~func(yield func(K, V) bool)](s S) map[K]V {
	m := map[K]V{}
	for k, v := range s {
		m[k] = v
	}
	return m
}

func main() {
	fmt.Println(Sum(IntSeq(func(yield func(int) bool) { yield(1); yield(2) })))
	fmt.Println(Sum(func(yield func(int) bool) { yield(5) }))
	fmt.Println(Gen(func(yield func(string, bool) bool) { yield("a", true) }))
}

// Output:
// 3
// 5
// map[a:true]
