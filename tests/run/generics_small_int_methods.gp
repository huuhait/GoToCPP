// phase: 07

// Methods of generic types over small integers: the arithmetic of a type-parameter-typed field wraps
// at the width of the type argument (int8, uint8, int16, uint16, int32) as in typed code, and the
// division by zero of the method panics with Go's text.
package main

import "fmt"

type Int interface {
	~int8 | ~uint8 | ~int16 | ~uint16 | ~int32 | ~uint32 | ~int64 | ~uint64 | ~int | ~uint
}

type Acc[T Int] struct {
	sum, prod T
	n         int
}

func (a *Acc[T]) Add(v T) {
	a.sum += v
	a.prod *= v
	a.n++
}

func (a Acc[T]) Mean() T { return a.sum / T(a.n) }

func (a Acc[T]) Scaled(k T) Acc[T] { return Acc[T]{a.sum * k, a.prod - k, a.n} }

func (a Acc[T]) Shifted(s uint) (T, T) { return a.sum << s, a.sum >> s }

func (a Acc[T]) Negated() (T, T) { return -a.sum, ^a.prod }

func newAcc[T Int]() *Acc[T] { return &Acc[T]{prod: 1} }

func run[T Int](vals ...T) {
	a := newAcc[T]()
	for _, v := range vals {
		a.Add(v)
	}
	s := a.Scaled(7)
	l, r := a.Shifted(3)
	n1, n2 := a.Negated()
	fmt.Printf("%T sum=%v prod=%v n=%d scaled=%v/%v shifted=%v/%v neg=%v/%v\n", a.sum, a.sum, a.prod, a.n, s.sum, s.prod, l, r, n1, n2)
}

func divide[T Int](a Acc[T]) (res T, err any) {
	defer func() { err = recover() }()
	return a.Mean(), nil
}

func main() {
	run[int8](100, 27, 1)
	run[int8](-128, -1)
	run[uint8](200, 100, 56)
	run[uint8](255, 255)
	run[int16](30000, 3000)
	run[uint16](65535, 2)
	run[int32](2147483647, 1)
	run[uint32](4294967295, 3)
	run[int64](9223372036854775807, 1)
	run[uint64](18446744073709551615, 2)
	run[int](-9223372036854775808, -1)
	run[uint](1, 2, 3, 4, 5)
	fmt.Println(divide(Acc[int8]{sum: 50, n: 3}))
	fmt.Println(divide(Acc[uint8]{sum: 250, n: 0}))
	fmt.Println(divide(Acc[int16]{sum: -32768, n: 0}))
	fmt.Println(divide(Acc[int64]{sum: -9223372036854775808, n: 1}))
	var m Acc[int8]
	m.sum = -128
	fmt.Println(m.sum/-1, m.sum%-1, -m.sum)
}

// Output:
// int8 sum=-128 prod=-116 n=3 scaled=-128/-123 shifted=0/-16 neg=-128/115
// int8 sum=127 prod=-128 n=2 scaled=121/121 shifted=-8/15 neg=-127/127
// uint8 sum=100 prod=0 n=3 scaled=188/249 shifted=32/12 neg=156/255
// uint8 sum=254 prod=1 n=2 scaled=242/250 shifted=240/31 neg=2/254
// int16 sum=-32536 prod=19072 n=2 scaled=-31144/19065 shifted=1856/-4067 neg=32536/-19073
// uint16 sum=1 prod=65534 n=2 scaled=7/65527 shifted=8/0 neg=65535/1
// int32 sum=-2147483648 prod=2147483647 n=2 scaled=-2147483648/2147483640 shifted=0/-268435456 neg=-2147483648/-2147483648
// uint32 sum=2 prod=4294967293 n=2 scaled=14/4294967286 shifted=16/0 neg=4294967294/2
// int64 sum=-9223372036854775808 prod=9223372036854775807 n=2 scaled=-9223372036854775808/9223372036854775800 shifted=0/-1152921504606846976 neg=-9223372036854775808/-9223372036854775808
// uint64 sum=1 prod=18446744073709551614 n=2 scaled=7/18446744073709551607 shifted=8/0 neg=18446744073709551615/1
// int sum=9223372036854775807 prod=-9223372036854775808 n=2 scaled=9223372036854775801/9223372036854775801 shifted=-8/1152921504606846975 neg=-9223372036854775807/9223372036854775807
// uint sum=15 prod=120 n=5 scaled=105/113 shifted=120/1 neg=18446744073709551601/18446744073709551495
// 16 <nil>
// 0 runtime error: integer divide by zero
// 0 runtime error: integer divide by zero
// -9223372036854775808 <nil>
// -128 0 -128
