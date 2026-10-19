// phase: 09
// Go 1.25's errors/wrap_test.go (the tables of Is, As, AsValidation (the panics) and Unwrap; io/fs.PathError and os.Open are local stand-ins) through a small harness; expected output from real Go.

package main

import (
	"errors"
	"fmt"
	"reflect"
)

type PathError struct { // io/fs.PathError (package io/fs is step 9.6)
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string { return e.Op + " " + e.Path + ": " + e.Err.Error() }
func (e *PathError) Unwrap() error { return e.Err }
func (e *PathError) Timeout() bool {
	t, ok := e.Err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}

var ErrNotExist = errors.New("file does not exist")

type enoent struct{}

func (enoent) Error() string        { return "no such file or directory" }
func (enoent) Is(target error) bool { return target == ErrNotExist }

func openFile(name string) (any, error) { return nil, &PathError{"open", name, enoent{}} }

func TestIs(t *testT) {
	err1 := errors.New("1")
	erra := wrapped{"wrap 2", err1}
	errb := wrapped{"wrap 3", erra}

	err3 := errors.New("3")

	poser := &poser{"either 1 or 3", func(err error) bool {
		return err == err1 || err == err3
	}}

	testCases := []struct {
		err    error
		target error
		match  bool
	}{
		{nil, nil, true},
		{nil, err1, false},
		{err1, nil, false},
		{err1, err1, true},
		{erra, err1, true},
		{errb, err1, true},
		{err1, err3, false},
		{erra, err3, false},
		{errb, err3, false},
		{poser, err1, true},
		{poser, err3, true},
		{poser, erra, false},
		{poser, errb, false},
		{errorUncomparable{}, errorUncomparable{}, true},
		{errorUncomparable{}, &errorUncomparable{}, false},
		{&errorUncomparable{}, errorUncomparable{}, true},
		{&errorUncomparable{}, &errorUncomparable{}, false},
		{errorUncomparable{}, err1, false},
		{&errorUncomparable{}, err1, false},
		{multiErr{}, err1, false},
		{multiErr{err1, err3}, err1, true},
		{multiErr{err3, err1}, err1, true},
		{multiErr{err1, err3}, errors.New("x"), false},
		{multiErr{err3, errb}, errb, true},
		{multiErr{err3, errb}, erra, true},
		{multiErr{err3, errb}, err1, true},
		{multiErr{errb, err3}, err1, true},
		{multiErr{poser}, err1, true},
		{multiErr{poser}, err3, true},
		{multiErr{nil}, nil, false},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testT) {
			checks++
			if got := errors.Is(tc.err, tc.target); got != tc.match {
				t.Errorf("Is(%v, %v) = %v, want %v", tc.err, tc.target, got, tc.match)
			}
		})
	}
}

type poser struct {
	msg string
	f   func(error) bool
}

var poserPathErr = &PathError{Op: "poser"}

func (p *poser) Error() string     { return p.msg }
func (p *poser) Is(err error) bool { return p.f(err) }
func (p *poser) As(err any) bool {
	switch x := err.(type) {
	case **poser:
		*x = p
	case *errorT:
		*x = errorT{"poser"}
	case **PathError:
		*x = poserPathErr
	default:
		return false
	}
	return true
}

func TestAs(t *testT) {
	var errT errorT
	var errP *PathError
	var timeout interface{ Timeout() bool }
	var p *poser
	_, errF := openFile("non-existing")
	poserErr := &poser{"oh no", nil}

	testCases := []struct {
		err    error
		target any
		match  bool
		want   any // value of target on match
	}{{
		nil,
		&errP,
		false,
		nil,
	}, {
		wrapped{"pitied the fool", errorT{"T"}},
		&errT,
		true,
		errorT{"T"},
	}, {
		errF,
		&errP,
		true,
		errF,
	}, {
		errorT{},
		&errP,
		false,
		nil,
	}, {
		wrapped{"wrapped", nil},
		&errT,
		false,
		nil,
	}, {
		&poser{"error", nil},
		&errT,
		true,
		errorT{"poser"},
	}, {
		&poser{"path", nil},
		&errP,
		true,
		poserPathErr,
	}, {
		poserErr,
		&p,
		true,
		poserErr,
	}, {
		errors.New("err"),
		&timeout,
		false,
		nil,
	}, {
		errF,
		&timeout,
		true,
		errF,
	}, {
		wrapped{"path error", errF},
		&timeout,
		true,
		errF,
	}, {
		multiErr{},
		&errT,
		false,
		nil,
	}, {
		multiErr{errors.New("a"), errorT{"T"}},
		&errT,
		true,
		errorT{"T"},
	}, {
		multiErr{errorT{"T"}, errors.New("a")},
		&errT,
		true,
		errorT{"T"},
	}, {
		multiErr{errorT{"a"}, errorT{"b"}},
		&errT,
		true,
		errorT{"a"},
	}, {
		multiErr{multiErr{errors.New("a"), errorT{"a"}}, errorT{"b"}},
		&errT,
		true,
		errorT{"a"},
	}, {
		multiErr{wrapped{"path error", errF}},
		&timeout,
		true,
		errF,
	}, {
		multiErr{nil},
		&errT,
		false,
		nil,
	}}
	for i, tc := range testCases {
		name := fmt.Sprintf("%d:As(Errorf(..., %v), %v)", i, tc.err, tc.target)
		// Clear the target pointer, in case it was set in a previous test.
		rtarget := reflect.ValueOf(tc.target)
		rtarget.Elem().Set(reflect.Zero(reflect.TypeOf(tc.target).Elem()))
		t.Run(name, func(t *testT) {
			match := errors.As(tc.err, tc.target)
			checks++
			if match != tc.match {
				t.Fatalf("match: got %v; want %v", match, tc.match)
			}
			checks++
			if !match {
				return
			}
			checks++
			if got := rtarget.Elem().Interface(); got != tc.want {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestAsValidation(t *testT) {
	var s string
	testCases := []any{
		nil,
		(*int)(nil),
		"error",
		&s,
	}
	err := errors.New("error")
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%T(%v)", tc, tc), func(t *testT) {
			defer func() {
				recover()
			}()
			checks++
			if errors.As(err, tc) {
				t.Errorf("As(err, %T(%v)) = true, want false", tc, tc)
				return
			}
			t.Errorf("As(err, %T(%v)) did not panic", tc, tc)
		})
	}
}

func TestUnwrap(t *testT) {
	err1 := errors.New("1")
	erra := wrapped{"wrap 2", err1}

	testCases := []struct {
		err  error
		want error
	}{
		{nil, nil},
		{wrapped{"wrapped", nil}, nil},
		{err1, nil},
		{erra, err1},
		{wrapped{"wrap 3", erra}, erra},
	}
	for _, tc := range testCases {
		checks++
		if got := errors.Unwrap(tc.err); got != tc.want {
			t.Errorf("Unwrap(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

type errorT struct{ s string }

func (e errorT) Error() string { return fmt.Sprintf("errorT(%s)", e.s) }

type wrapped struct {
	msg string
	err error
}

func (e wrapped) Error() string { return e.msg }
func (e wrapped) Unwrap() error { return e.err }

type multiErr []error

func (m multiErr) Error() string   { return "multiError" }
func (m multiErr) Unwrap() []error { return []error(m) }

type errorUncomparable struct {
	f []string
}

func (errorUncomparable) Error() string {
	return "uncomparable error"
}

func (errorUncomparable) Is(target error) bool {
	_, ok := target.(errorUncomparable)
	return ok
}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

// checks counts the if statements that the tests of the file run: a Test that returns early shows in the expected output.
var checks int

func (t *testT) Helper()                           {}
func (t *testT) Parallel()                         {}
func (t *testT) Name() string                      { return t.name }
func (t *testT) Log(args ...any)                   {}
func (t *testT) Logf(format string, args ...any)   {}
func (t *testT) Skip(args ...any)                  { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)       { panic(skipExit{}) }
func (t *testT) report(msg string)                 { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                 { t.report(fmt.Sprint(args...)) }
func (t *testT) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	panic(fatalExit{})
}
func (t *testT) Fatal(args ...any) {
	t.Error(args...)
	panic(fatalExit{})
}

func (t *testT) Run(name string, f func(t *testT)) bool {
	sub := &testT{name: t.name + "/" + name}
	subtests++
	runTest(sub, f)
	if sub.failed {
		t.failed = true
	}
	return !sub.failed
}

func runTest(t *testT, f func(t *testT)) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case fatalExit, skipExit:
			default:
				t.failed = true
				fmt.Println("PANIC", t.name+":", r)
			}
		}
	}()
	f(t)
}

type testCase struct {
	name string
	f    func(t *testT)
}

func runTests(tests []testCase) {
	failed := 0
	for _, tc := range tests {
		t := &testT{name: tc.name}
		subtests = 0
		checks = 0
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", checks)
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// A deterministic generator (splitmix64) in place of math/rand, so that Go and gocpp run the same inputs.
type rngT struct{ s uint64 }

var rng = &rngT{0x9E3779B97F4A7C15}

func (r *rngT) Uint64() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) Uint32() uint32   { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int   { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestIs", TestIs},
		{"TestAs", TestAs},
		{"TestAsValidation", TestAsValidation},
		{"TestUnwrap", TestUnwrap},
	}
	runTests(tests_)
}

// Output:
// ok TestIs subtests: 30 checks: 30
// ok TestAs subtests: 18 checks: 48
// ok TestAsValidation subtests: 4 checks: 4
// ok TestUnwrap subtests: 0 checks: 5
// tests: 4 failed: 0
