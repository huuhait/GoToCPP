// phase: 07

// A generic struct with two type parameters: literals, fields, methods, copies and %v, %+v, %T.
package main

import "fmt"

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

func (p Pair[K, V]) String() string { return fmt.Sprintf("%v=>%v", p.Key, p.Val) }

func (p *Pair[K, V]) Set(v V) { p.Val = v }

func (p Pair[K, V]) With(v V) Pair[K, V] {
	p.Val = v
	return p
}

func MakePair[K comparable, V any](k K, v V) Pair[K, V] { return Pair[K, V]{k, v} }

func main() {
	p := Pair[string, int]{"a", 1}
	q := p
	q.Set(2)
	fmt.Println(p, q, p.With(9), p.Val)
	fmt.Printf("%v|%+v|%T\n", p, p, p)
	r := MakePair("k", []string{"x", "y"})
	fmt.Printf("%v %T\n", r, r)
	fmt.Println(MakePair(1.5, true), MakePair('c', "rune"))
	var z Pair[int, *int]
	fmt.Println(z.Key, z.Val == nil)
	pp := &Pair[bool, string]{Key: true}
	pp.Set("s")
	fmt.Println(*pp, pp.Key)
}

// Output:
// a=>1 a=>2 a=>9 1
// a=>1|a=>1|main.Pair[string,int]
// k=>[x y] main.Pair[string,[]string]
// 1.5=>true 99=>rune
// 0 true
// true=>s true
