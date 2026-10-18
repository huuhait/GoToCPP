// phase: 07

// Large values as type arguments: arrays and structs bigger than a stack frame are passed and returned by value.
package main

import "fmt"

type Big struct {
	data [40000]int64
	name string
}

func Fill[T any](n int, v T) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func Copy[T any](v T) (T, T) { return v, v }

func Id[T any](v T) T { return v }

func Modify[T any](v T, f func(*T)) T {
	f(&v)
	return v
}

func First[T any](xs ...T) T { return xs[0] }

func main() {
	var b Big
	b.data[0], b.data[39999], b.name = 7, 9, "big"
	c, d := Copy(b)
	d.data[0] = 100
	fmt.Println(c.data[0], d.data[0], c.data[39999], c.name)
	m := Modify(b, func(p *Big) { p.data[1] = 5; p.name += "!" })
	fmt.Println(b.data[1], m.data[1], m.name, Id(b).data[39999])
	fs := Fill(3, [30000]int{1})
	fs[1][0] = 2
	fmt.Println(len(fs), fs[0][0], fs[1][0], fs[2][0])
	fmt.Println(First(b, c).name, Id([50000]int8{})[49999], len(Fill(2, b)))
}

// Output:
// 7 100 9 big
// 0 5 big! 9
// 3 1 2 1
// big 0 2
