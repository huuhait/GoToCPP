// phase: 05

// The nil-interface gotcha in all forms: typed nils of every kind, errors, comparisons, map keys, errors.Is/As, nil receivers.
package main

import (
	"errors"
	"fmt"
)

type E struct{ msg string }

func (e *E) Error() string {
	if e == nil {
		return "nil E"
	}
	return e.msg
}

type V struct{ n int }

func (v V) Error() string { return fmt.Sprint("V", v.n) }

type I interface{ M() }
type T struct{}

func (*T) M() {}

func retPtr(fail bool) *E {
	if fail {
		return &E{"fail"}
	}
	return nil
}
func retErr(fail bool) error {
	return retPtr(fail)
}
func retErrOK(fail bool) error {
	if p := retPtr(fail); p != nil {
		return p
	}
	return nil
}

func isNil(a any) string {
	switch a {
	case nil:
		return "nil"
	}
	return "non-nil"
}

func main() {
	var (
		p  *E
		m  map[string]int
		s  []int
		f  func()
		ip *int
	)
	vals := []any{p, m, s, f, ip, nil, (*T)(nil)}
	for i, v := range vals {
		fmt.Printf("%d: %v %T %s\n", i, v == nil, v, isNil(v))
	}
	var e1 error = p
	var e2 error
	fmt.Println(e1 == nil, e2 == nil, e1 == e2, e1 != nil)
	fmt.Println(retErr(false) == nil, retErrOK(false) == nil, retErr(true), retErrOK(true))
	fmt.Println(e1.Error(), e1)
	fmt.Printf("%v|%+v|%s|%q|%d\n", e1, e1, e1, e1, e1)
	var i I = (*T)(nil)
	fmt.Println(i == nil, i != nil)
	i.M()
	var any1 any = p
	var any2 any = (*E)(nil)
	var any3 any = (*V)(nil)
	fmt.Println(any1 == any2, any1 == any3, any1 == nil)
	err := error(V{1})
	fmt.Println(errors.Is(err, V{1}), errors.Is(err, V{2}), errors.Is(e1, nil), errors.Is(nil, nil), errors.Is(nil, err))
	var target *E
	fmt.Println(errors.As(fmt.Errorf("x: %w", &E{"inner"}), &target), target)
	target = nil
	fmt.Println(errors.As(fmt.Errorf("x: %w", p), &target), target == nil)
	type pair struct {
		a, b error
	}
	pr := pair{p, nil}
	fmt.Println(pr.a == nil, pr.b == nil, pr == pair{p, nil}, pr == pair{})
	arr := [2]any{p, nil}
	fmt.Println(arr == [2]any{(*E)(nil), nil}, arr == [2]any{})
	mm := map[any]int{p: 1, nil: 2, (*V)(nil): 3}
	fmt.Println(mm[p], mm[nil], mm[(*V)(nil)], mm[(*T)(nil)], len(mm))
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		var ee error
		fmt.Println(ee.Error())
	}()
	var np *E
	fmt.Println(np.Error())
}
