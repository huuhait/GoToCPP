// phase: 07

// The pointer-method constraint pattern: [T any, PT interface{ *T; Set(string) }].
package main

import "fmt"

type Conf struct{ v string }

func (c *Conf) Set(s string) { c.v = "conf:" + s }

type Num struct{ n int }

func (n *Num) Set(s string) { n.n = len(s) }

func Build[T any, PT interface {
	*T
	Set(string)
}](s string) T {
	var t T
	PT(&t).Set(s)
	return t
}

func BuildAll[T any, PT interface {
	*T
	Set(string)
}](ss ...string) []T {
	out := make([]T, len(ss))
	for i, s := range ss {
		PT(&out[i]).Set(s)
	}
	return out
}

type Resetter interface{ Reset() }

type Buf struct{ data []int }

func (b *Buf) Reset() { b.data = b.data[:0] }

func ResetAll[T Resetter](xs []T) {
	for _, x := range xs {
		x.Reset()
	}
}

func main() {
	c := Build[Conf]("hi")
	n := Build[Num]("four")
	fmt.Println(c, n, c.v, n.n)
	fmt.Println(BuildAll[Conf]("a", "b"), BuildAll[Num]("x", "yy", "zzz"))
	bufs := []*Buf{{data: []int{1, 2}}, {data: []int{3}}}
	ResetAll(bufs)
	fmt.Println(len(bufs[0].data), len(bufs[1].data), cap(bufs[0].data))
}

// Output:
// {conf:hi} {4} conf:hi 4
// [{conf:a} {conf:b}] [{1} {2} {3}]
// 0 0 2
