// phase: 09
// Go 1.25's errors/example_test.go (New, Is, As, Unwrap, Join examples; io/fs and os are local stand-ins) run in order; expected output from real Go.

package main

import (
	"errors"
	"fmt"
	"time"
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

// MyError is an error implementation that includes a time and message.
type MyError struct {
	When time.Time
	What string
}

func (e MyError) Error() string {
	return fmt.Sprintf("%v: %v", e.When, e.What)
}

func oops() error {
	return MyError{
		time.Date(1989, 3, 15, 22, 30, 0, 0, time.UTC),
		"the file system has gone away",
	}
}

func Example() {
	if err := oops(); err != nil {
		fmt.Println(err)
	}

}

func ExampleNew() {
	err := errors.New("emit macho dwarf: elf header corrupted")
	if err != nil {
		fmt.Print(err)
	}

}

// The fmt package's Errorf function lets us use the package's formatting
// features to create descriptive error messages.
func ExampleNew_errorf() {
	const name, id = "bimmler", 17
	err := fmt.Errorf("user %q (id %d) not found", name, id)
	if err != nil {
		fmt.Print(err)
	}

}

func ExampleJoin() {
	err1 := errors.New("err1")
	err2 := errors.New("err2")
	err := errors.Join(err1, err2)
	fmt.Println(err)
	if errors.Is(err, err1) {
		fmt.Println("err is err1")
	}
	if errors.Is(err, err2) {
		fmt.Println("err is err2")
	}
	fmt.Println(err.(interface{ Unwrap() []error }).Unwrap())

}

func ExampleIs() {
	if _, err := openFile("non-existing"); err != nil {
		if errors.Is(err, ErrNotExist) {
			fmt.Println("file does not exist")
		} else {
			fmt.Println(err)
		}
	}

}

func ExampleAs() {
	if _, err := openFile("non-existing"); err != nil {
		var pathError *PathError
		if errors.As(err, &pathError) {
			fmt.Println("Failed at path:", pathError.Path)
		} else {
			fmt.Println(err)
		}
	}

}

func ExampleUnwrap() {
	err1 := errors.New("error1")
	err2 := fmt.Errorf("error2: [%w]", err1)
	fmt.Println(err2)
	fmt.Println(errors.Unwrap(err2))

}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

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
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else if subtests > 0 {
			fmt.Println("ok", tc.name, subtests)
		} else {
			fmt.Println("ok", tc.name)
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
	fmt.Println("== Example")
	Example()
	fmt.Println("== ExampleNew")
	ExampleNew()
	fmt.Println("== ExampleNew_errorf")
	ExampleNew_errorf()
	fmt.Println("== ExampleJoin")
	ExampleJoin()
	fmt.Println("== ExampleIs")
	ExampleIs()
	fmt.Println("== ExampleAs")
	ExampleAs()
	fmt.Println("== ExampleUnwrap")
	ExampleUnwrap()
}

// Output:
// == Example
// 1989-03-15 22:30:00 +0000 UTC: the file system has gone away
// == ExampleNew
// emit macho dwarf: elf header corrupted== ExampleNew_errorf
// user "bimmler" (id 17) not found== ExampleJoin
// err1
// err2
// err is err1
// err is err2
// [err1 err2]
// == ExampleIs
// file does not exist
// == ExampleAs
// Failed at path: non-existing
// == ExampleUnwrap
// error2: [error1]
// error1
