// phase: 07

package main

import "fmt"

type Yield func(int) bool

type Iter func(yield Yield)

type Iter2 func(yield func(string, int) bool)

type Gen interface {
	Each(func(int) bool)
}

type upTo int

func (n upTo) Each(yield func(int) bool) {
	for i := range int(n) {
		if !yield(i) {
			return
		}
	}
}

func upTwo() Iter {
	return func(yield Yield) {
		yield(1)
		yield(2)
	}
}

func main() {
	for x := range upTwo() {
		fmt.Println(x)
	}
	var it Iter = func(yield Yield) {
		for i := 10; i < 13; i++ {
			if !yield(i) {
				return
			}
		}
	}
	for x := range it {
		if x == 12 {
			break
		}
		fmt.Println(x)
	}
	var it2 Iter2 = func(yield func(string, int) bool) { yield("k", 1) }
	for k, v := range it2 {
		fmt.Println(k, v)
	}
	var g Gen = upTo(3)
	for x := range g.Each {
		fmt.Println("gen", x)
	}
}

// Output:
// 1
// 2
// 10
// 11
// k 1
// gen 0
// gen 1
// gen 2
