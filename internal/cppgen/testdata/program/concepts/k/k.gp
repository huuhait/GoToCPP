package k

import "fmt"

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

type Float interface{ ~float32 | ~float64 }

type Number interface{ Integer | Float }

type Ordered interface {
	Integer | Float | ~string
}

type Celsius float64

type Meters int

type Names []string

type Named struct{ Name string }

func (n Named) Hello() string   { return "hello " + n.Name }
func (n *Named) Rename(s string) { n.Name = s }

type Wrapper struct{ Named }

type Greeter interface{ Hello() string }

type Renamer interface{ Rename(s string) }

type Both interface {
	Greeter
	Renamer
}

type Text interface{ ~string | ~[]byte }

type NumGreeter interface {
	~int | ~float64
	Hello() string
}

type Tagged int

func (t Tagged) Hello() string { return fmt.Sprint("tagged ", int(t)) }

func Sum[T Number](xs []T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Max[T Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Eq[T comparable](a, b T) bool { return a == b }

func Greet[T Greeter](x T) string { return x.Hello() }

func Rename[T Renamer](x T, s string) { x.Rename(s) }

func Len[S ~[]E, E any](s S) int { return len(s) }

func Conv[T, U Number](x T) U { return U(x) }

func Size[T Text](a T) int { return len(a) }

func Both2[T Both](x T) string {
	x.Rename("both")
	return x.Hello()
}

func Show[T NumGreeter](x T) string { return x.Hello() }

func Anon[T interface {
	~int | ~string
	Hello() string
}](x T) string {
	return x.Hello()
}

func Any[T any](x T) T { return x }

func unexported[T Integer](x T) T { return x }

type Bag struct{ Items []int }
