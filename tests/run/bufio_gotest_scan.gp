// phase: 09
// Go 1.25's bufio/scan_test.go (Scanner: ScanLines, ScanWords, ScanRunes, ScanBytes, long lines, ErrTooLong, custom split functions, empty tokens, bad readers, Buffer; MaxTokenSize of bufio's export_test.go is Buffer(nil, n)) through a small harness; expected output from real Go.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)


const smallMaxTokenSize = 256 // Much smaller for more efficient testing.


var scanTests = []string{
	"",
	"a",
	"¼",
	"☹",
	"\x81",   // UTF-8 error
	"\uFFFD", // correctly encoded RuneError
	"abcdefgh",
	"abc def\n\t\tgh    ",
	"abc¼☹\x81\uFFFD日本語\x82abc",
}

func TestScanByte(t *testT) {
	for n, test := range scanTests {
		buf := strings.NewReader(test)
		s := bufio.NewScanner(buf)
		s.Split(bufio.ScanBytes)
		var i int
		for i = 0; s.Scan(); i++ {
			atomic.AddInt64(&checks, 1)
			if b := s.Bytes(); len(b) != 1 || b[0] != test[i] {
				t.Errorf("#%d: %d: expected %q got %q", n, i, test, b)
			}
		}
		atomic.AddInt64(&checks, 1)
		if i != len(test) {
			t.Errorf("#%d: termination expected at %d; got %d", n, len(test), i)
		}
		err := s.Err()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("#%d: %v", n, err)
		}
	}
}

// Test that the rune splitter returns same sequence of runes (not bytes) as for range string.
func TestScanRune(t *testT) {
	for n, test := range scanTests {
		buf := strings.NewReader(test)
		s := bufio.NewScanner(buf)
		s.Split(bufio.ScanRunes)
		var i, runeCount int
		var expect rune
		// Use a string range loop to validate the sequence of runes.
		for i, expect = range test {
			atomic.AddInt64(&checks, 1)
			if !s.Scan() {
				break
			}
			runeCount++
			got, _ := utf8.DecodeRune(s.Bytes())
			atomic.AddInt64(&checks, 1)
			if got != expect {
				t.Errorf("#%d: %d: expected %q got %q", n, i, expect, got)
			}
		}
		atomic.AddInt64(&checks, 1)
		if s.Scan() {
			t.Errorf("#%d: scan ran too long, got %q", n, s.Text())
		}
		testRuneCount := utf8.RuneCountInString(test)
		atomic.AddInt64(&checks, 1)
		if runeCount != testRuneCount {
			t.Errorf("#%d: termination expected at %d; got %d", n, testRuneCount, runeCount)
		}
		err := s.Err()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("#%d: %v", n, err)
		}
	}
}

var wordScanTests = []string{
	"",
	" ",
	"\n",
	"a",
	" a ",
	"abc def",
	" abc def ",
	" abc\tdef\nghi\rjkl\fmno\vpqr\u0085stu\u00a0\n",
}

// Test that the word splitter returns the same data as strings.Fields.
func TestScanWords(t *testT) {
	for n, test := range wordScanTests {
		buf := strings.NewReader(test)
		s := bufio.NewScanner(buf)
		s.Split(bufio.ScanWords)
		words := strings.Fields(test)
		var wordCount int
		for wordCount = 0; wordCount < len(words); wordCount++ {
			atomic.AddInt64(&checks, 1)
			if !s.Scan() {
				break
			}
			got := s.Text()
			atomic.AddInt64(&checks, 1)
			if got != words[wordCount] {
				t.Errorf("#%d: %d: expected %q got %q", n, wordCount, words[wordCount], got)
			}
		}
		atomic.AddInt64(&checks, 1)
		if s.Scan() {
			t.Errorf("#%d: scan ran too long, got %q", n, s.Text())
		}
		atomic.AddInt64(&checks, 1)
		if wordCount != len(words) {
			t.Errorf("#%d: termination expected at %d; got %d", n, len(words), wordCount)
		}
		err := s.Err()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("#%d: %v", n, err)
		}
	}
}

// slowReader is a reader that returns only a few bytes at a time, to test the incremental
// reads in Scanner.Scan.
type slowReader struct {
	max int
	buf io.Reader
}

func (sr *slowReader) Read(p []byte) (n int, err error) {
	atomic.AddInt64(&checks, 1)
	if len(p) > sr.max {
		p = p[0:sr.max]
	}
	return sr.buf.Read(p)
}

// genLine writes to buf a predictable but non-trivial line of text of length
// n, including the terminal newline and an occasional carriage return.
// If addNewline is false, the \r and \n are not emitted.
func genLine(buf *bytes.Buffer, lineNum, n int, addNewline bool) {
	buf.Reset()
	doCR := lineNum%5 == 0
	atomic.AddInt64(&checks, 1)
	if doCR {
		n--
	}
	for i := 0; i < n-1; i++ { // Stop early for \n.
		c := 'a' + byte(lineNum+i)
		atomic.AddInt64(&checks, 1)
		if c == '\n' || c == '\r' { // Don't confuse us.
			c = 'N'
		}
		buf.WriteByte(c)
	}
	atomic.AddInt64(&checks, 1)
	if addNewline {
		atomic.AddInt64(&checks, 1)
		if doCR {
			buf.WriteByte('\r')
		}
		buf.WriteByte('\n')
	}
}

// Test the line splitter, including some carriage returns but no long lines.
func TestScanLongLines(t *testT) {
	// Build a buffer of lots of line lengths up to but not exceeding smallMaxTokenSize.
	tmp := new(bytes.Buffer)
	buf := new(bytes.Buffer)
	lineNum := 0
	j := 0
	for i := 0; i < 2*smallMaxTokenSize; i++ {
		genLine(tmp, lineNum, j, true)
		atomic.AddInt64(&checks, 1)
		if j < smallMaxTokenSize {
			j++
		} else {
			j--
		}
		buf.Write(tmp.Bytes())
		lineNum++
	}
	s := bufio.NewScanner(&slowReader{1, buf})
	s.Split(bufio.ScanLines)
	s.Buffer(nil, smallMaxTokenSize)
	j = 0
	for lineNum := 0; s.Scan(); lineNum++ {
		genLine(tmp, lineNum, j, false)
		atomic.AddInt64(&checks, 1)
		if j < smallMaxTokenSize {
			j++
		} else {
			j--
		}
		line := tmp.String() // We use the string-valued token here, for variety.
		atomic.AddInt64(&checks, 1)
		if s.Text() != line {
			t.Errorf("%d: bad line: %d %d\n%.100q\n%.100q\n", lineNum, len(s.Bytes()), len(line), s.Text(), line)
		}
	}
	err := s.Err()
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatal(err)
	}
}

// Test that the line splitter errors out on a long line.
func TestScanLineTooLong(t *testT) {
	const smallMaxTokenSize = 256 // Much smaller for more efficient testing.
	// Build a buffer of lots of line lengths up to but not exceeding smallMaxTokenSize.
	tmp := new(bytes.Buffer)
	buf := new(bytes.Buffer)
	lineNum := 0
	j := 0
	for i := 0; i < 2*smallMaxTokenSize; i++ {
		genLine(tmp, lineNum, j, true)
		j++
		buf.Write(tmp.Bytes())
		lineNum++
	}
	s := bufio.NewScanner(&slowReader{3, buf})
	s.Split(bufio.ScanLines)
	s.Buffer(nil, smallMaxTokenSize)
	j = 0
	for lineNum := 0; s.Scan(); lineNum++ {
		genLine(tmp, lineNum, j, false)
		atomic.AddInt64(&checks, 1)
		if j < smallMaxTokenSize {
			j++
		} else {
			j--
		}
		line := tmp.Bytes()
		atomic.AddInt64(&checks, 1)
		if !bytes.Equal(s.Bytes(), line) {
			t.Errorf("%d: bad line: %d %d\n%.100q\n%.100q\n", lineNum, len(s.Bytes()), len(line), s.Bytes(), line)
		}
	}
	err := s.Err()
	atomic.AddInt64(&checks, 1)
	if err != bufio.ErrTooLong {
		t.Fatalf("expected ErrTooLong; got %s", err)
	}
}

// Test that the line splitter handles a final line without a newline.
func testNoNewline(text string, lines []string, t *testT) {
	buf := strings.NewReader(text)
	s := bufio.NewScanner(&slowReader{7, buf})
	s.Split(bufio.ScanLines)
	for lineNum := 0; s.Scan(); lineNum++ {
		line := lines[lineNum]
		atomic.AddInt64(&checks, 1)
		if s.Text() != line {
			t.Errorf("%d: bad line: %d %d\n%.100q\n%.100q\n", lineNum, len(s.Bytes()), len(line), s.Bytes(), line)
		}
	}
	err := s.Err()
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatal(err)
	}
}

// Test that the line splitter handles a final line without a newline.
func TestScanLineNoNewline(t *testT) {
	const text = "abcdefghijklmn\nopqrstuvwxyz"
	lines := []string{
		"abcdefghijklmn",
		"opqrstuvwxyz",
	}
	testNoNewline(text, lines, t)
}

// Test that the line splitter handles a final line with a carriage return but no newline.
func TestScanLineReturnButNoNewline(t *testT) {
	const text = "abcdefghijklmn\nopqrstuvwxyz\r"
	lines := []string{
		"abcdefghijklmn",
		"opqrstuvwxyz",
	}
	testNoNewline(text, lines, t)
}

// Test that the line splitter handles a final empty line.
func TestScanLineEmptyFinalLine(t *testT) {
	const text = "abcdefghijklmn\nopqrstuvwxyz\n\n"
	lines := []string{
		"abcdefghijklmn",
		"opqrstuvwxyz",
		"",
	}
	testNoNewline(text, lines, t)
}

// Test that the line splitter handles a final empty line with a carriage return but no newline.
func TestScanLineEmptyFinalLineWithCR(t *testT) {
	const text = "abcdefghijklmn\nopqrstuvwxyz\n\r"
	lines := []string{
		"abcdefghijklmn",
		"opqrstuvwxyz",
		"",
	}
	testNoNewline(text, lines, t)
}

var testError = errors.New("testError")

// Test the correct error is returned when the split function errors out.
func TestSplitError(t *testT) {
	// Create a split function that delivers a little data, then a predictable error.
	numSplits := 0
	const okCount = 7
	errorSplit := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		atomic.AddInt64(&checks, 1)
		if atEOF {
			panic("didn't get enough data")
		}
		atomic.AddInt64(&checks, 1)
		if numSplits >= okCount {
			return 0, nil, testError
		}
		numSplits++
		return 1, data[0:1], nil
	}
	// Read the data.
	const text = "abcdefghijklmnopqrstuvwxyz"
	buf := strings.NewReader(text)
	s := bufio.NewScanner(&slowReader{1, buf})
	s.Split(errorSplit)
	var i int
	for i = 0; s.Scan(); i++ {
		atomic.AddInt64(&checks, 1)
		if len(s.Bytes()) != 1 || text[i] != s.Bytes()[0] {
			t.Errorf("#%d: expected %q got %q", i, text[i], s.Bytes()[0])
		}
	}
	// Check correct termination location and error.
	atomic.AddInt64(&checks, 1)
	if i != okCount {
		t.Errorf("unexpected termination; expected %d tokens got %d", okCount, i)
	}
	err := s.Err()
	atomic.AddInt64(&checks, 1)
	if err != testError {
		t.Fatalf("expected %q got %v", testError, err)
	}
}


// Test for issue 5268.
type alwaysError struct{}

func (alwaysError) Read(p []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestNonEOFWithEmptyRead(t *testT) {
	scanner := bufio.NewScanner(alwaysError{})
	for scanner.Scan() {
		t.Fatal("read should fail")
	}
	err := scanner.Err()
	atomic.AddInt64(&checks, 1)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test that Scan finishes if we have endless empty reads.
type endlessZeros struct{}

func (endlessZeros) Read(p []byte) (int, error) {
	return 0, nil
}

func TestBadReader(t *testT) {
	scanner := bufio.NewScanner(endlessZeros{})
	for scanner.Scan() {
		t.Fatal("read should fail")
	}
	err := scanner.Err()
	atomic.AddInt64(&checks, 1)
	if err != io.ErrNoProgress {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestScanWordsExcessiveWhiteSpace(t *testT) {
	const word = "ipsum"
	s := strings.Repeat(" ", 4*smallMaxTokenSize) + word
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Buffer(nil, smallMaxTokenSize)
	scanner.Split(bufio.ScanWords)
	atomic.AddInt64(&checks, 1)
	if !scanner.Scan() {
		t.Fatalf("scan failed: %v", scanner.Err())
	}
	atomic.AddInt64(&checks, 1)
	if token := scanner.Text(); token != word {
		t.Fatalf("unexpected token: %v", token)
	}
}

// Test that empty tokens, including at end of line or end of file, are found by the scanner.
// Issue 8672: Could miss final empty token.

func commaSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i < len(data); i++ {
		atomic.AddInt64(&checks, 1)
		if data[i] == ',' {
			return i + 1, data[:i], nil
		}
	}
	return 0, data, bufio.ErrFinalToken
}

func testEmptyTokens(t *testT, text string, values []string) {
	s := bufio.NewScanner(strings.NewReader(text))
	s.Split(commaSplit)
	var i int
	for i = 0; s.Scan(); i++ {
		atomic.AddInt64(&checks, 1)
		if i >= len(values) {
			t.Fatalf("got %d fields, expected %d", i+1, len(values))
		}
		atomic.AddInt64(&checks, 1)
		if s.Text() != values[i] {
			t.Errorf("%d: expected %q got %q", i, values[i], s.Text())
		}
	}
	atomic.AddInt64(&checks, 1)
	if i != len(values) {
		t.Fatalf("got %d fields, expected %d", i, len(values))
	}
	atomic.AddInt64(&checks, 1)
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyTokens(t *testT) {
	testEmptyTokens(t, "1,2,3,", []string{"1", "2", "3", ""})
}

func TestWithNoEmptyTokens(t *testT) {
	testEmptyTokens(t, "1,2,3", []string{"1", "2", "3"})
}

func loopAtEOFSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	atomic.AddInt64(&checks, 1)
	if len(data) > 0 {
		return 1, data[:1], nil
	}
	return 0, data, nil
}

func TestDontLoopForever(t *testT) {
	s := bufio.NewScanner(strings.NewReader("abc"))
	s.Split(loopAtEOFSplit)
	// Expect a panic
	defer func() {
		err := recover()
		atomic.AddInt64(&checks, 1)
		if err == nil {
			t.Fatal("should have panicked")
		}
		atomic.AddInt64(&checks, 1)
		if msg, ok := err.(string); !ok || !strings.Contains(msg, "empty tokens") {
			panic(err)
		}
	}()
	for count := 0; s.Scan(); count++ {
		atomic.AddInt64(&checks, 1)
		if count > 1000 {
			t.Fatal("looping")
		}
	}
	atomic.AddInt64(&checks, 1)
	if s.Err() != nil {
		t.Fatal("after scan:", s.Err())
	}
}

func TestBlankLines(t *testT) {
	s := bufio.NewScanner(strings.NewReader(strings.Repeat("\n", 1000)))
	for count := 0; s.Scan(); count++ {
		atomic.AddInt64(&checks, 1)
		if count > 2000 {
			t.Fatal("looping")
		}
	}
	atomic.AddInt64(&checks, 1)
	if s.Err() != nil {
		t.Fatal("after scan:", s.Err())
	}
}

type countdown int

func (c *countdown) split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	atomic.AddInt64(&checks, 1)
	if *c > 0 {
		*c--
		return 1, data[:1], nil
	}
	return 0, nil, nil
}

// Check that the looping-at-EOF check doesn't trigger for merely empty tokens.
func TestEmptyLinesOK(t *testT) {
	c := countdown(10000)
	s := bufio.NewScanner(strings.NewReader(strings.Repeat("\n", 10000)))
	s.Split(c.split)
	for s.Scan() {
	}
	atomic.AddInt64(&checks, 1)
	if s.Err() != nil {
		t.Fatal("after scan:", s.Err())
	}
	atomic.AddInt64(&checks, 1)
	if c != 0 {
		t.Fatalf("stopped with %d left to process", c)
	}
}

// Make sure we can read a huge token if a big enough buffer is provided.
func TestHugeBuffer(t *testT) {
	text := strings.Repeat("x", 2*bufio.MaxScanTokenSize)
	s := bufio.NewScanner(strings.NewReader(text + "\n"))
	s.Buffer(make([]byte, 100), 3*bufio.MaxScanTokenSize)
	for s.Scan() {
		token := s.Text()
		atomic.AddInt64(&checks, 1)
		if token != text {
			t.Errorf("scan got incorrect token of length %d", len(token))
		}
	}
	atomic.AddInt64(&checks, 1)
	if s.Err() != nil {
		t.Fatal("after scan:", s.Err())
	}
}

// negativeEOFReader returns an invalid -1 at the end, as though it
// were wrapping the read system call.
type negativeEOFReader int

func (r *negativeEOFReader) Read(p []byte) (int, error) {
	atomic.AddInt64(&checks, 1)
	if *r > 0 {
		c := int(*r)
		atomic.AddInt64(&checks, 1)
		if c > len(p) {
			c = len(p)
		}
		for i := 0; i < c; i++ {
			p[i] = 'a'
		}
		p[c-1] = '\n'
		*r -= negativeEOFReader(c)
		return c, nil
	}
	return -1, io.EOF
}

// Test that the scanner doesn't panic and returns ErrBadReadCount
// on a reader that returns a negative count of bytes read (issue 38053).
func TestNegativeEOFReader(t *testT) {
	r := negativeEOFReader(10)
	scanner := bufio.NewScanner(&r)
	c := 0
	for scanner.Scan() {
		c++
		atomic.AddInt64(&checks, 1)
		if c > 1 {
			t.Error("read too many lines")
			break
		}
	}
	atomic.AddInt64(&checks, 1)
	if got, want := scanner.Err(), bufio.ErrBadReadCount; got != want {
		t.Errorf("scanner.Err: got %v, want %v", got, want)
	}
}

// largeReader returns an invalid count that is larger than the number
// of bytes requested.
type largeReader struct{}

func (largeReader) Read(p []byte) (int, error) {
	return len(p) + 1, nil
}

// Test that the scanner doesn't panic and returns ErrBadReadCount
// on a reader that returns an impossibly large count of bytes read (issue 38053).
func TestLargeReader(t *testT) {
	scanner := bufio.NewScanner(largeReader{})
	for scanner.Scan() {
	}
	atomic.AddInt64(&checks, 1)
	if got, want := scanner.Err(), bufio.ErrBadReadCount; got != want {
		t.Errorf("scanner.Err: got %v, want %v", got, want)
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
	tests_ := []testCase{
		{"TestScanByte", TestScanByte},
		{"TestScanRune", TestScanRune},
		{"TestScanWords", TestScanWords},
		{"TestScanLongLines", TestScanLongLines},
		{"TestScanLineTooLong", TestScanLineTooLong},
		{"TestScanLineNoNewline", TestScanLineNoNewline},
		{"TestScanLineReturnButNoNewline", TestScanLineReturnButNoNewline},
		{"TestScanLineEmptyFinalLine", TestScanLineEmptyFinalLine},
		{"TestScanLineEmptyFinalLineWithCR", TestScanLineEmptyFinalLineWithCR},
		{"TestSplitError", TestSplitError},
		{"TestNonEOFWithEmptyRead", TestNonEOFWithEmptyRead},
		{"TestBadReader", TestBadReader},
		{"TestScanWordsExcessiveWhiteSpace", TestScanWordsExcessiveWhiteSpace},
		{"TestEmptyTokens", TestEmptyTokens},
		{"TestWithNoEmptyTokens", TestWithNoEmptyTokens},
		{"TestDontLoopForever", TestDontLoopForever},
		{"TestBlankLines", TestBlankLines},
		{"TestEmptyLinesOK", TestEmptyLinesOK},
		{"TestHugeBuffer", TestHugeBuffer},
		{"TestNegativeEOFReader", TestNegativeEOFReader},
		{"TestLargeReader", TestLargeReader},
	}
	runTests(tests_)
}

// Output:
// ok TestScanByte subtests: 0 checks: 77
// ok TestScanRune subtests: 0 checks: 113
// ok TestScanWords subtests: 0 checks: 50
// ok TestScanLongLines subtests: 0 checks: 297018
// ok TestScanLineTooLong subtests: 0 checks: 176410
// ok TestScanLineNoNewline subtests: 0 checks: 8
// ok TestScanLineReturnButNoNewline subtests: 0 checks: 8
// ok TestScanLineEmptyFinalLine subtests: 0 checks: 10
// ok TestScanLineEmptyFinalLineWithCR subtests: 0 checks: 10
// ok TestSplitError subtests: 0 checks: 33
// ok TestNonEOFWithEmptyRead subtests: 0 checks: 1
// ok TestBadReader subtests: 0 checks: 1
// ok TestScanWordsExcessiveWhiteSpace subtests: 0 checks: 2
// ok TestEmptyTokens subtests: 0 checks: 16
// ok TestWithNoEmptyTokens subtests: 0 checks: 13
// ok TestDontLoopForever subtests: 0 checks: 209
// ok TestBlankLines subtests: 0 checks: 1001
// ok TestEmptyLinesOK subtests: 0 checks: 10003
// ok TestHugeBuffer subtests: 0 checks: 2
// ok TestNegativeEOFReader subtests: 0 checks: 5
// ok TestLargeReader subtests: 0 checks: 1
// tests: 21 failed: 0
