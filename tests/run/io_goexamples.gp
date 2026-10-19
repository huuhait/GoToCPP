// phase: 09
// Go 1.25's io/example_test.go (Copy, CopyBuffer, CopyN, ReadAtLeast, ReadFull, WriteString, TeeReader, SectionReader, LimitReader, MultiReader/Writer, Pipe, ReadAll, Discard) run in order; expected output from real Go.

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
)


func ExampleCopy() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	if _, err := io.Copy(os.Stdout, r); err != nil {
		panic(err)
	}

}

func ExampleCopyBuffer() {
	r1 := strings.NewReader("first reader\n")
	r2 := strings.NewReader("second reader\n")
	buf := make([]byte, 8)

	// buf is used here...
	if _, err := io.CopyBuffer(os.Stdout, r1, buf); err != nil {
		panic(err)
	}

	// ... reused here also. No need to allocate an extra buffer.
	if _, err := io.CopyBuffer(os.Stdout, r2, buf); err != nil {
		panic(err)
	}

}

func ExampleCopyN() {
	r := strings.NewReader("some io.Reader stream to be read")

	if _, err := io.CopyN(os.Stdout, r, 4); err != nil {
		panic(err)
	}

}

func ExampleReadAtLeast() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	buf := make([]byte, 14)
	if _, err := io.ReadAtLeast(r, buf, 4); err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", buf)

	// buffer smaller than minimal read size.
	shortBuf := make([]byte, 3)
	if _, err := io.ReadAtLeast(r, shortBuf, 4); err != nil {
		fmt.Println("error:", err)
	}

	// minimal read size bigger than io.Reader stream
	longBuf := make([]byte, 64)
	if _, err := io.ReadAtLeast(r, longBuf, 64); err != nil {
		fmt.Println("error:", err)
	}

}

func ExampleReadFull() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", buf)

	// minimal read size bigger than io.Reader stream
	longBuf := make([]byte, 64)
	if _, err := io.ReadFull(r, longBuf); err != nil {
		fmt.Println("error:", err)
	}

}

func ExampleWriteString() {
	if _, err := io.WriteString(os.Stdout, "Hello World"); err != nil {
		panic(err)
	}

}

func ExampleLimitReader() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	lr := io.LimitReader(r, 4)

	if _, err := io.Copy(os.Stdout, lr); err != nil {
		panic(err)
	}

}

func ExampleMultiReader() {
	r1 := strings.NewReader("first reader ")
	r2 := strings.NewReader("second reader ")
	r3 := strings.NewReader("third reader\n")
	r := io.MultiReader(r1, r2, r3)

	if _, err := io.Copy(os.Stdout, r); err != nil {
		panic(err)
	}

}

func ExampleTeeReader() {
	var r io.Reader = strings.NewReader("some io.Reader stream to be read\n")

	r = io.TeeReader(r, os.Stdout)

	// Everything read from r will be copied to stdout.
	if _, err := io.ReadAll(r); err != nil {
		panic(err)
	}

}

func ExampleSectionReader() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	if _, err := io.Copy(os.Stdout, s); err != nil {
		panic(err)
	}

}

func ExampleSectionReader_Read() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	buf := make([]byte, 9)
	if _, err := s.Read(buf); err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", buf)

}

func ExampleSectionReader_ReadAt() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	buf := make([]byte, 6)
	if _, err := s.ReadAt(buf, 10); err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", buf)

}

func ExampleSectionReader_Seek() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	if _, err := s.Seek(10, io.SeekStart); err != nil {
		panic(err)
	}

	if _, err := io.Copy(os.Stdout, s); err != nil {
		panic(err)
	}

}

func ExampleSectionReader_Size() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	fmt.Println(s.Size())

}

func ExampleSeeker_Seek() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	r.Seek(5, io.SeekStart) // move to the 5th char from the start
	if _, err := io.Copy(os.Stdout, r); err != nil {
		panic(err)
	}

	r.Seek(-5, io.SeekEnd)
	if _, err := io.Copy(os.Stdout, r); err != nil {
		panic(err)
	}

}

func ExampleMultiWriter() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	var buf1, buf2 strings.Builder
	w := io.MultiWriter(&buf1, &buf2)

	if _, err := io.Copy(w, r); err != nil {
		panic(err)
	}

	fmt.Print(buf1.String())
	fmt.Print(buf2.String())

}

func ExamplePipe() {
	r, w := io.Pipe()

	go func() {
		fmt.Fprint(w, "some io.Reader stream to be read\n")
		w.Close()
	}()

	if _, err := io.Copy(os.Stdout, r); err != nil {
		panic(err)
	}

}

func ExampleReadAll() {
	r := strings.NewReader("Go is a general-purpose language designed with systems programming in mind.")

	b, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s", b)

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
// It is atomic: some tests run their checks in several goroutines.
var checks int64

func (t *testT) Helper()                          {}
func (t *testT) Parallel()                        {}
func (t *testT) Name() string                     { return t.name }
func (t *testT) Log(args ...any)                  {}
func (t *testT) Logf(format string, args ...any)  {}
func (t *testT) Skip(args ...any)                 { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)      { panic(skipExit{}) }
func (t *testT) report(msg string)                { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                { t.report(fmt.Sprint(args...)) }
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
		atomic.StoreInt64(&checks, 0)
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", atomic.LoadInt64(&checks))
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// allocsPerRun runs f runs times and returns 0: gocpp does not count heap allocations (the tests of allocation counts are
// dropped or their comparison is).
func allocsPerRun(runs int, f func()) float64 {
	for i := 0; i < runs; i++ {
		f()
	}
	return 0
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
func (r *rngT) Uint32() uint32 { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	fmt.Println("== ExampleCopy")
	ExampleCopy()
	fmt.Println("== ExampleCopyBuffer")
	ExampleCopyBuffer()
	fmt.Println("== ExampleCopyN")
	ExampleCopyN()
	fmt.Println("== ExampleReadAtLeast")
	ExampleReadAtLeast()
	fmt.Println("== ExampleReadFull")
	ExampleReadFull()
	fmt.Println("== ExampleWriteString")
	ExampleWriteString()
	fmt.Println("== ExampleLimitReader")
	ExampleLimitReader()
	fmt.Println("== ExampleMultiReader")
	ExampleMultiReader()
	fmt.Println("== ExampleTeeReader")
	ExampleTeeReader()
	fmt.Println("== ExampleSectionReader")
	ExampleSectionReader()
	fmt.Println("== ExampleSectionReader_Read")
	ExampleSectionReader_Read()
	fmt.Println("== ExampleSectionReader_ReadAt")
	ExampleSectionReader_ReadAt()
	fmt.Println("== ExampleSectionReader_Seek")
	ExampleSectionReader_Seek()
	fmt.Println("== ExampleSectionReader_Size")
	ExampleSectionReader_Size()
	fmt.Println("== ExampleSeeker_Seek")
	ExampleSeeker_Seek()
	fmt.Println("== ExampleMultiWriter")
	ExampleMultiWriter()
	fmt.Println("== ExamplePipe")
	ExamplePipe()
	fmt.Println("== ExampleReadAll")
	ExampleReadAll()
}

// Output:
// == ExampleCopy
// some io.Reader stream to be read
// == ExampleCopyBuffer
// first reader
// second reader
// == ExampleCopyN
// some== ExampleReadAtLeast
// some io.Reader
// error: short buffer
// error: unexpected EOF
// == ExampleReadFull
// some
// error: unexpected EOF
// == ExampleWriteString
// Hello World== ExampleLimitReader
// some== ExampleMultiReader
// first reader second reader third reader
// == ExampleTeeReader
// some io.Reader stream to be read
// == ExampleSectionReader
// io.Reader stream == ExampleSectionReader_Read
// io.Reader
// == ExampleSectionReader_ReadAt
// stream
// == ExampleSectionReader_Seek
// stream == ExampleSectionReader_Size
// 17
// == ExampleSeeker_Seek
// io.Reader stream to be read
// read
// == ExampleMultiWriter
// some io.Reader stream to be read
// some io.Reader stream to be read
// == ExamplePipe
// some io.Reader stream to be read
// == ExampleReadAll
// Go is a general-purpose language designed with systems programming in mind.
