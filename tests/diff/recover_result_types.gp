// phase: 05

package main

import "fmt"

type pt struct {
	x, y int
	name string
}

func boolean() bool {
	defer func() { recover() }()
	panic(1)
}

func float() float64 {
	defer func() { recover() }()
	panic(1)
}

func str() string {
	defer func() { recover() }()
	panic(1)
}

func structure() pt {
	defer func() { recover() }()
	panic(1)
}

func pointer() *pt {
	defer func() { recover() }()
	panic(1)
}

func slice() []int {
	defer func() { recover() }()
	panic(1)
}

func mapping() map[string]int {
	defer func() { recover() }()
	panic(1)
}

func array() [3]int {
	defer func() { recover() }()
	panic(1)
}

func function() func() {
	defer func() { recover() }()
	panic(1)
}

func namedSet() (p pt, s []int) {
	defer func() { recover() }()
	p = pt{1, 2, "set"}
	s = []int{9}
	panic(1)
}

func main() {
	fmt.Println(boolean(), float(), str() == "", structure(), pointer() == nil)
	fmt.Println(slice() == nil, mapping() == nil, array(), function() == nil)
	fmt.Println(namedSet())
}
