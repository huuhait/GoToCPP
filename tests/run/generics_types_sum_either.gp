// phase: 07

// A sum type with two type parameters: methods that branch, return instances of other arguments, and print.
package main

import "fmt"

type Either[L, R any] struct {
	left  L
	right R
	isR   bool
}

func Left[L, R any](v L) Either[L, R]  { return Either[L, R]{left: v} }
func Right[L, R any](v R) Either[L, R] { return Either[L, R]{right: v, isR: true} }

func (e Either[L, R]) IsRight() bool { return e.isR }

func (e Either[L, R]) Swap() Either[R, L] {
	if e.isR {
		return Left[R, L](e.right)
	}
	return Right[R, L](e.left)
}

func (e Either[L, R]) Fold(l func(L) string, r func(R) string) string {
	if e.isR {
		return r(e.right)
	}
	return l(e.left)
}

func (e Either[L, R]) String() string {
	return e.Fold(func(l L) string { return fmt.Sprintf("Left(%v)", l) }, func(r R) string { return fmt.Sprintf("Right(%v)", r) })
}

func Divide(a, b int) Either[string, int] {
	if b == 0 {
		return Left[string, int]("div by zero")
	}
	return Right[string](a / b)
}

func main() {
	fmt.Println(Divide(7, 2), Divide(1, 0))
	fmt.Println(Divide(7, 2).Swap(), Divide(1, 0).Swap().Swap())
	fmt.Printf("%T %T\n", Divide(1, 1), Divide(1, 1).Swap())
	e := Left[[]int, map[string]bool](nil)
	fmt.Println(e, e.IsRight())
}

// Output:
// Right(3) Left(div by zero)
// Left(3) Left(div by zero)
// main.Either[string,int] main.Either[int,string]
// Left([]) false
