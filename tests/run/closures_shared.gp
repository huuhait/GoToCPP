// phase: 03

// A variable that closures capture and that is larger than 128 bytes is shared with them (boxed),
// as Go does, even when it never changes: 2,000 closures of a 94 KiB array hold one array, not
// 2,000 copies (about 190 MB). Smaller ones are copied into each closure. The values are the same
// either way, as the variables never change.
package main

import "fmt"

type big struct {
	name  string
	table [12000]int
}

func readers(b big, n int) []func(int) int {
	var fs []func(int) int
	for i := range n {
		fs = append(fs, func(k int) int { return b.table[k] + i })
	}
	return fs
}

func fill() [12000]int {
	var a [12000]int
	for i := range a {
		a[i] = i * 2
	}
	return a
}

func main() {
	arr := fill() // never changed: shared all the same
	small := [16]int{1, 2, 3}
	var fs []func() int
	for i := 0; i < 2000; i++ {
		k := i % len(arr)
		fs = append(fs, func() int { return arr[k] + small[2] })
	}
	total := 0
	for _, f := range fs {
		total += f()
	}
	fmt.Println(len(fs), total)

	b := big{name: "b"}
	b.table[7] = 70
	rs := readers(b, 1500)
	fmt.Println(len(rs), rs[0](7), rs[1499](7), b.name)
}

// Output:
// 2000 4004000
// 1500 70 1569 b
