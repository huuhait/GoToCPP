// phase: 07

// Several results of different type parameters, and a generic function returning an unnamed struct.
package main

import "fmt"

func Swap[A, B any](a A, b B) (B, A) { return b, a }

func Pair[A, B any](a A, b B) struct {
	First  A
	Second B
} {
	return struct {
		First  A
		Second B
	}{a, b}
}

func Divmod[T ~int | ~int64](a, b T) (q, r T) {
	q = a / b
	r = a % b
	return
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func Try[T any](f func() (T, error)) (res T, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	return Must(f()), true
}

func Zip[A, B any](as []A, bs []B) []struct {
	A A
	B B
} {
	n := min(len(as), len(bs))
	out := make([]struct {
		A A
		B B
	}, n)
	for i := range n {
		out[i].A, out[i].B = as[i], bs[i]
	}
	return out
}

func main() {
	s, i := Swap(1, "one")
	fmt.Println(s, i)
	x, y := Swap(Swap(2.5, true))
	fmt.Println(x, y)
	p := Pair("k", []int{1, 2})
	fmt.Println(p.First, p.Second, p)
	fmt.Printf("%v %+v %T\n", p, p, p)
	fmt.Println(Divmod(17, 5))
	fmt.Println(Divmod[int64](-17, 5))
	fmt.Println(Try(func() (int, error) { return 4, nil }))
	fmt.Println(Try(func() (string, error) { return "", fmt.Errorf("bad") }))
	fmt.Println(Zip([]int{1, 2, 3}, []string{"a", "b"}))
}

// Output:
// one 1
// 2.5 true
// k [1 2] {k [1 2]}
// {k [1 2]} {First:k Second:[1 2]} struct { First string; Second []int }
// 3 2
// -3 -2
// 4 true
//  false
// [{1 a} {2 b}]
