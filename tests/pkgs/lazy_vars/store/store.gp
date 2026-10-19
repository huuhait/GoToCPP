package store

import "./late.hpp"

// Holder is a struct of ours with a C++ class field: its variable is lazy too.
type Holder struct {
	N int
	L late.Late
}

var H Holder

var L late.Late

var Pair [2]late.Late

// Order is initialized from the other variables: gp_var_init runs after they are built.
var Order = describe()

func describe() string { return "built " + itoa(int(late.built())) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

func init() {
	L.set(7)
	Pair[1].set(11)
}

// Sum reads the variables of this package, through the references.
func Sum() int {
	return int(L.get() + Pair[0].get() + Pair[1].get() + H.L.get())
}

// Get, Set and Bump are what other packages use: a method call on a C++ value of another package's
// variable (store.L.get()) is not built yet.
func Get() (int, int, int) { return int(L.get()), int(Pair[0].get()), int(Pair[1].get()) }

func SetL(x int) { L.set(C.int(x)) }

func Bump() int {
	p := &Pair[0]
	p.set(5)
	return int(Pair[0].get()) + H.N
}
