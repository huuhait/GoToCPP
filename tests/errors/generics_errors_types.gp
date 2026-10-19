// phase: 07

// Errors in generic types: methods, instantiations of the receiver, the type parameter list of a
// method that differs from the type's, and uses of instances.
package main

type List[T any] struct{ items []T }

func (l List[T]) Len() int { return len(l.items) }

func (l *List[U]) Push(x U) { l.items = append(l.items, x) } // fine: U names T here

func (l List) NoParams() {} // ERROR "cannot use generic type List\[T any\] without instantiation"

func (l List[T, V]) TooMany() {} // ERROR "receiver declares 2 type parameters, but receiver base type declares 1"

func (l List[_]) Anon() {}

type Num interface{ ~int | ~float64 }

type Vec[T Num] []T

func (v Vec[T]) Sum() T {
	var s T
	for _, x := range v {
		s += x
	}
	return s
}

func (v Vec[T]) Bad(s string) T { return s } // ERROR "cannot use s \(variable of type string\) as T value in return statement"

func main() {
	var l List[int]
	l.Push("a")        // ERROR "cannot use \x22a\x22 \(untyped string constant\) as int value in argument to l.Push"
	l.Missing()        // ERROR "l.Missing undefined \(type List\[int\] has no field or method Missing\)"
	_ = l.items[0].foo // ERROR "l.items\[0\].foo undefined \(type int has no field or method foo\)"
	var v Vec[string]  // ERROR "string does not satisfy Num \(string missing in ~int \| ~float64\)"
	_ = v
	var w Vec[float64]
	w.Sum()
	_ = w.Sum() + 1
	_ = w.Sum() + "x" // ERROR "invalid operation: w.Sum\(\) \+ \x22x\x22 \(mismatched types float64 and untyped string\)"
	var x List[int] = List[int8]{} // ERROR "cannot use List\[int8\]\{\} \(value of struct type List\[int8\]\) as List\[int\] value in variable declaration"
	_ = x
}
