// phase: 09
// Go 1.25's bufio/example_test.go (Writer, Scanner with lines, words, custom split functions, ReadWriter) run in order; expected output from real Go.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)


func ExampleWriter() {
	w := bufio.NewWriter(os.Stdout)
	fmt.Fprint(w, "Hello, ")
	fmt.Fprint(w, "world!")
	w.Flush() // Don't forget to flush!

}

func ExampleWriter_AvailableBuffer() {
	w := bufio.NewWriter(os.Stdout)
	for _, i := range []int64{1, 2, 3, 4} {
		b := w.AvailableBuffer()
		b = strconv.AppendInt(b, i, 10)
		b = append(b, ' ')
		w.Write(b)
	}
	w.Flush()

}

// ExampleWriter_ReadFrom demonstrates how to use the ReadFrom method of Writer.
func ExampleWriter_ReadFrom() {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	data := "Hello, world!\nThis is a ReadFrom example."
	reader := strings.NewReader(data)

	n, err := writer.ReadFrom(reader)
	if err != nil {
		fmt.Println("ReadFrom Error:", err)
		return
	}

	if err = writer.Flush(); err != nil {
		fmt.Println("Flush Error:", err)
		return
	}

	fmt.Println("Bytes written:", n)
	fmt.Println("Buffer contents:", buf.String())

}

// The simplest use of a Scanner, to read standard input as a set of lines.
func ExampleScanner_lines() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Println(scanner.Text()) // Println will add back the final '\n'
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

// Return the most recent call to Scan as a []byte.
func ExampleScanner_Bytes() {
	scanner := bufio.NewScanner(strings.NewReader("gopher"))
	for scanner.Scan() {
		fmt.Println(len(scanner.Bytes()) == 6)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "shouldn't see an error scanning a string")
	}

}

// Use a Scanner to implement a simple word-count utility by scanning the
// input as a sequence of space-delimited tokens.
func ExampleScanner_words() {
	// An artificial input source.
	const input = "Now is the winter of our discontent,\nMade glorious summer by this sun of York.\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	// Set the split function for the scanning operation.
	scanner.Split(bufio.ScanWords)
	// Count the words.
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading input:", err)
	}
	fmt.Printf("%d\n", count)

}

// Use a Scanner with a custom split function (built by wrapping ScanWords) to validate
// 32-bit decimal input.
func ExampleScanner_custom() {
	// An artificial input source.
	const input = "1234 5678 1234567901234567890"
	scanner := bufio.NewScanner(strings.NewReader(input))
	// Create a custom split function by wrapping the existing ScanWords function.
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = bufio.ScanWords(data, atEOF)
		if err == nil && token != nil {
			_, err = strconv.ParseInt(string(token), 10, 32)
		}
		return
	}
	// Set the split function for the scanning operation.
	scanner.Split(split)
	// Validate the input
	for scanner.Scan() {
		fmt.Printf("%s\n", scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Invalid input: %s", err)
	}

}

// Use a Scanner with a custom split function to parse a comma-separated
// list with an empty final value.
func ExampleScanner_emptyFinalToken() {
	// Comma-separated list; last entry is empty.
	const input = "1,2,3,4,"
	scanner := bufio.NewScanner(strings.NewReader(input))
	// Define a split function that separates on commas.
	onComma := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		for i := 0; i < len(data); i++ {
			if data[i] == ',' {
				return i + 1, data[:i], nil
			}
		}
		if !atEOF {
			return 0, nil, nil
		}
		// There is one final token to be delivered, which may be the empty string.
		// Returning bufio.ErrFinalToken here tells Scan there are no more tokens after this
		// but does not trigger an error to be returned from Scan itself.
		return 0, data, bufio.ErrFinalToken
	}
	scanner.Split(onComma)
	// Scan.
	for scanner.Scan() {
		fmt.Printf("%q ", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading input:", err)
	}

}

// Use a Scanner with a custom split function to parse a comma-separated
// list with an empty final value but stops at the token "STOP".
func ExampleScanner_earlyStop() {
	onComma := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		i := bytes.IndexByte(data, ',')
		if i == -1 {
			if !atEOF {
				return 0, nil, nil
			}
			// If we have reached the end, return the last token.
			return 0, data, bufio.ErrFinalToken
		}
		// If the token is "STOP", stop the scanning and ignore the rest.
		if string(data[:i]) == "STOP" {
			return i + 1, nil, bufio.ErrFinalToken
		}
		// Otherwise, return the token before the comma.
		return i + 1, data[:i], nil
	}
	const input = "1,2,STOP,4,"
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(onComma)
	for scanner.Scan() {
		fmt.Printf("Got a token %q\n", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading input:", err)
	}

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
	fmt.Println("== ExampleWriter")
	ExampleWriter()
	fmt.Println("== ExampleWriter_AvailableBuffer")
	ExampleWriter_AvailableBuffer()
	fmt.Println("== ExampleWriter_ReadFrom")
	ExampleWriter_ReadFrom()
	fmt.Println("== ExampleScanner_lines")
	ExampleScanner_lines()
	fmt.Println("== ExampleScanner_Bytes")
	ExampleScanner_Bytes()
	fmt.Println("== ExampleScanner_words")
	ExampleScanner_words()
	fmt.Println("== ExampleScanner_custom")
	ExampleScanner_custom()
	fmt.Println("== ExampleScanner_emptyFinalToken")
	ExampleScanner_emptyFinalToken()
	fmt.Println("== ExampleScanner_earlyStop")
	ExampleScanner_earlyStop()
}

// Output:
// == ExampleWriter
// Hello, world!== ExampleWriter_AvailableBuffer
// 1 2 3 4 == ExampleWriter_ReadFrom
// Bytes written: 41
// Buffer contents: Hello, world!
// This is a ReadFrom example.
// == ExampleScanner_lines
// == ExampleScanner_Bytes
// true
// == ExampleScanner_words
// 15
// == ExampleScanner_custom
// 1234
// 5678
// Invalid input: strconv.ParseInt: parsing "1234567901234567890": value out of range== ExampleScanner_emptyFinalToken
// "1" "2" "3" "4" "" == ExampleScanner_earlyStop
// Got a token "1"
// Got a token "2"
