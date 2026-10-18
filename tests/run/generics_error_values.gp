// phase: 07

// errors.Is, errors.As and wrapping in generic helpers.
package main

import (
	"errors"
	"fmt"
)

type NotFound struct{ key string }

func (e *NotFound) Error() string { return "not found: " + e.key }

type Timeout struct{ ms int }

func (e Timeout) Error() string { return fmt.Sprint("timeout after ", e.ms, "ms") }

var ErrBase = errors.New("base")

func As[T error](err error) (T, bool) {
	var t T
	ok := errors.As(err, &t)
	return t, ok
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func Wrap[T any](v T, err error, msg string) (T, error) {
	if err != nil {
		return v, fmt.Errorf("%s: %w", msg, err)
	}
	return v, nil
}

func Lookup[V any](m map[string]V, k string) (V, error) {
	if v, ok := m[k]; ok {
		return v, nil
	}
	var zero V
	return zero, &NotFound{k}
}

func Collect[T any](fs ...func() (T, error)) ([]T, error) {
	var out []T
	var errs []error
	for _, f := range fs {
		v, err := f()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, v)
	}
	return out, errors.Join(errs...)
}

func main() {
	_, err := Lookup(map[string]int{"a": 1}, "b")
	_, err = Wrap(0, err, "load")
	nf, ok := As[*NotFound](err)
	fmt.Println(err, ok, nf.key)
	_, ok = As[Timeout](err)
	fmt.Println(ok)
	te, ok := As[Timeout](fmt.Errorf("ctx: %w", Timeout{250}))
	fmt.Println(te, ok, errors.Is(fmt.Errorf("x: %w", ErrBase), ErrBase))
	vals, err := Collect(func() (int, error) { return 1, nil }, func() (int, error) { return 0, ErrBase }, func() (int, error) { return 3, nil })
	fmt.Println(vals, err, errors.Is(err, ErrBase))
	fmt.Println(Must(Lookup(map[string]string{"k": "v"}, "k")))
	defer func() { fmt.Println("recovered:", recover()) }()
	Must(Lookup(map[string]float64{}, "zz"))
}

// Output:
// load: not found: b true b
// false
// timeout after 250ms true true
// [1 3] base true
// v
// recovered: not found: zz
