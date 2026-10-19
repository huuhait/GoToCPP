// phase: 09
// Go 1.25's bytes/buffer_test.go (Buffer: Write*, Read*, Next, Truncate, Grow, ReadFrom, WriteTo, UnreadByte/Rune, the panics and the growth rules) through a small harness; the allocation counts are not checked; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"unicode/utf8"
)


const N = 10000       // make this bigger for a larger (and slower) test
var testString string // test data for write tests
var testBytes []byte  // test data; same as testString but as a slice.

type negativeReader struct{}

func (r *negativeReader) Read([]byte) (int, error) { return -1, nil }

func init() {
	testBytes = make([]byte, N)
	for i := 0; i < N; i++ {
		testBytes[i] = 'a' + byte(i%26)
	}
	testString = string(testBytes)
}

// Verify that contents of buf match the string s.
func check(t *testT, testname string, buf *bytes.Buffer, s string) {
	bytes := buf.Bytes()
	str := buf.String()
	atomic.AddInt64(&checks, 1)
	if buf.Len() != len(bytes) {
		t.Errorf("%s: buf.Len() == %d, len(buf.Bytes()) == %d", testname, buf.Len(), len(bytes))
	}

	atomic.AddInt64(&checks, 1)
	if buf.Len() != len(str) {
		t.Errorf("%s: buf.Len() == %d, len(buf.String()) == %d", testname, buf.Len(), len(str))
	}

	atomic.AddInt64(&checks, 1)
	if buf.Len() != len(s) {
		t.Errorf("%s: buf.Len() == %d, len(s) == %d", testname, buf.Len(), len(s))
	}

	atomic.AddInt64(&checks, 1)
	if string(bytes) != s {
		t.Errorf("%s: string(buf.Bytes()) == %q, s == %q", testname, string(bytes), s)
	}
}

// Fill buf through n writes of string fus.
// The initial contents of buf corresponds to the string s;
// the result is the final contents of buf returned as a string.
func fillString(t *testT, testname string, buf *bytes.Buffer, s string, n int, fus string) string {
	check(t, testname+" (fill 1)", buf, s)
	for ; n > 0; n-- {
		m, err := buf.WriteString(fus)
		atomic.AddInt64(&checks, 1)
		if m != len(fus) {
			t.Errorf(testname+" (fill 2): m == %d, expected %d", m, len(fus))
		}
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf(testname+" (fill 3): err should always be nil, found err == %s", err)
		}
		s += fus
		check(t, testname+" (fill 4)", buf, s)
	}
	return s
}

// Fill buf through n writes of byte slice fub.
// The initial contents of buf corresponds to the string s;
// the result is the final contents of buf returned as a string.
func fillBytes(t *testT, testname string, buf *bytes.Buffer, s string, n int, fub []byte) string {
	check(t, testname+" (fill 1)", buf, s)
	for ; n > 0; n-- {
		m, err := buf.Write(fub)
		atomic.AddInt64(&checks, 1)
		if m != len(fub) {
			t.Errorf(testname+" (fill 2): m == %d, expected %d", m, len(fub))
		}
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf(testname+" (fill 3): err should always be nil, found err == %s", err)
		}
		s += string(fub)
		check(t, testname+" (fill 4)", buf, s)
	}
	return s
}

func TestNewBuffer(t *testT) {
	buf := bytes.NewBuffer(testBytes)
	check(t, "NewBuffer", buf, testString)
}

var buf bytes.Buffer

// Calling NewBuffer and immediately shallow copying the Buffer struct
// should not result in any allocations.
// This can be used to reset the underlying []byte of an existing Buffer.
func TestNewBufferShallow(t *testT) {
	
	n := allocsPerRun(1000, func() {
		buf = *bytes.NewBuffer(testBytes)
	})
	atomic.AddInt64(&checks, 1)
	if n > 0 {
		t.Errorf("allocations occurred while shallow copying")
	}
	check(t, "NewBuffer", &buf, testString)
}

func TestNewBufferString(t *testT) {
	buf := bytes.NewBufferString(testString)
	check(t, "NewBufferString", buf, testString)
}

// Empty buf through repeated reads into fub.
// The initial contents of buf corresponds to the string s.
func empty(t *testT, testname string, buf *bytes.Buffer, s string, fub []byte) {
	check(t, testname+" (empty 1)", buf, s)

	for {
		n, err := buf.Read(fub)
		atomic.AddInt64(&checks, 1)
		if n == 0 {
			break
		}
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf(testname+" (empty 2): err should always be nil, found err == %s", err)
		}
		s = s[n:]
		check(t, testname+" (empty 3)", buf, s)
	}

	check(t, testname+" (empty 4)", buf, "")
}

func TestBasicOperations(t *testT) {
	var buf bytes.Buffer

	for i := 0; i < 5; i++ {
		check(t, "TestBasicOperations (1)", &buf, "")

		buf.Reset()
		check(t, "TestBasicOperations (2)", &buf, "")

		buf.Truncate(0)
		check(t, "TestBasicOperations (3)", &buf, "")

		n, err := buf.Write(testBytes[0:1])
		atomic.AddInt64(&checks, 1)
		if want := 1; err != nil || n != want {
			t.Errorf("Write: got (%d, %v), want (%d, %v)", n, err, want, nil)
		}
		check(t, "TestBasicOperations (4)", &buf, "a")

		buf.WriteByte(testString[1])
		check(t, "TestBasicOperations (5)", &buf, "ab")

		n, err = buf.Write(testBytes[2:26])
		atomic.AddInt64(&checks, 1)
		if want := 24; err != nil || n != want {
			t.Errorf("Write: got (%d, %v), want (%d, %v)", n, err, want, nil)
		}
		check(t, "TestBasicOperations (6)", &buf, testString[0:26])

		buf.Truncate(26)
		check(t, "TestBasicOperations (7)", &buf, testString[0:26])

		buf.Truncate(20)
		check(t, "TestBasicOperations (8)", &buf, testString[0:20])

		empty(t, "TestBasicOperations (9)", &buf, testString[0:20], make([]byte, 5))
		empty(t, "TestBasicOperations (10)", &buf, "", make([]byte, 100))

		buf.WriteByte(testString[1])
		c, err := buf.ReadByte()
		atomic.AddInt64(&checks, 1)
		if want := testString[1]; err != nil || c != want {
			t.Errorf("ReadByte: got (%q, %v), want (%q, %v)", c, err, want, nil)
		}
		c, err = buf.ReadByte()
		atomic.AddInt64(&checks, 1)
		if err != io.EOF {
			t.Errorf("ReadByte: got (%q, %v), want (%q, %v)", c, err, byte(0), io.EOF)
		}
	}
}

func TestLargeStringWrites(t *testT) {
	var buf bytes.Buffer
	limit := 30
	atomic.AddInt64(&checks, 1)
	if true {
		limit = 9
	}
	for i := 3; i < limit; i += 3 {
		s := fillString(t, "TestLargeWrites (1)", &buf, "", 5, testString)
		empty(t, "TestLargeStringWrites (2)", &buf, s, make([]byte, len(testString)/i))
	}
	check(t, "TestLargeStringWrites (3)", &buf, "")
}

func TestLargeByteWrites(t *testT) {
	var buf bytes.Buffer
	limit := 30
	atomic.AddInt64(&checks, 1)
	if true {
		limit = 9
	}
	for i := 3; i < limit; i += 3 {
		s := fillBytes(t, "TestLargeWrites (1)", &buf, "", 5, testBytes)
		empty(t, "TestLargeByteWrites (2)", &buf, s, make([]byte, len(testString)/i))
	}
	check(t, "TestLargeByteWrites (3)", &buf, "")
}

func TestLargeStringReads(t *testT) {
	var buf bytes.Buffer
	for i := 3; i < 30; i += 3 {
		s := fillString(t, "TestLargeReads (1)", &buf, "", 5, testString[:len(testString)/i])
		empty(t, "TestLargeReads (2)", &buf, s, make([]byte, len(testString)))
	}
	check(t, "TestLargeStringReads (3)", &buf, "")
}

func TestLargeByteReads(t *testT) {
	var buf bytes.Buffer
	for i := 3; i < 30; i += 3 {
		s := fillBytes(t, "TestLargeReads (1)", &buf, "", 5, testBytes[:len(testBytes)/i])
		empty(t, "TestLargeReads (2)", &buf, s, make([]byte, len(testString)))
	}
	check(t, "TestLargeByteReads (3)", &buf, "")
}

func TestMixedReadsAndWrites(t *testT) {
	var buf bytes.Buffer
	s := ""
	for i := 0; i < 50; i++ {
		wlen := rng.Intn(len(testString))
		atomic.AddInt64(&checks, 1)
		if i%2 == 0 {
			s = fillString(t, "TestMixedReadsAndWrites (1)", &buf, s, 1, testString[0:wlen])
		} else {
			s = fillBytes(t, "TestMixedReadsAndWrites (1)", &buf, s, 1, testBytes[0:wlen])
		}

		rlen := rng.Intn(len(testString))
		fub := make([]byte, rlen)
		n, _ := buf.Read(fub)
		s = s[n:]
	}
	empty(t, "TestMixedReadsAndWrites (2)", &buf, s, make([]byte, buf.Len()))
}

func TestCapWithPreallocatedSlice(t *testT) {
	buf := bytes.NewBuffer(make([]byte, 10))
	n := buf.Cap()
	atomic.AddInt64(&checks, 1)
	if n != 10 {
		t.Errorf("expected 10, got %d", n)
	}
}

func TestCapWithSliceAndWrittenData(t *testT) {
	buf := bytes.NewBuffer(make([]byte, 0, 10))
	buf.Write([]byte("test"))
	n := buf.Cap()
	atomic.AddInt64(&checks, 1)
	if n != 10 {
		t.Errorf("expected 10, got %d", n)
	}
}

func TestNil(t *testT) {
	var b *bytes.Buffer
	atomic.AddInt64(&checks, 1)
	if b.String() != "<nil>" {
		t.Errorf("expected <nil>; got %q", b.String())
	}
}

func TestReadFrom(t *testT) {
	var buf bytes.Buffer
	for i := 3; i < 30; i += 3 {
		s := fillBytes(t, "TestReadFrom (1)", &buf, "", 5, testBytes[:len(testBytes)/i])
		var b bytes.Buffer
		b.ReadFrom(&buf)
		empty(t, "TestReadFrom (2)", &b, s, make([]byte, len(testString)))
	}
}

type panicReader struct{ panic bool }

func (r panicReader) Read(p []byte) (int, error) {
	atomic.AddInt64(&checks, 1)
	if r.panic {
		panic("oops")
	}
	return 0, io.EOF
}

// Make sure that an empty Buffer remains empty when
// it is "grown" before a Read that panics
func TestReadFromPanicReader(t *testT) {

	// First verify non-panic behaviour
	var buf bytes.Buffer
	i, err := buf.ReadFrom(panicReader{})
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatal(err)
	}
	atomic.AddInt64(&checks, 1)
	if i != 0 {
		t.Fatalf("unexpected return from bytes.ReadFrom (1): got: %d, want %d", i, 0)
	}
	check(t, "TestReadFromPanicReader (1)", &buf, "")

	// Confirm that when Reader panics, the empty buffer remains empty
	var buf2 bytes.Buffer
	defer func() {
		recover()
		check(t, "TestReadFromPanicReader (2)", &buf2, "")
	}()
	buf2.ReadFrom(panicReader{panic: true})
}

func TestReadFromNegativeReader(t *testT) {
	var b bytes.Buffer
	defer func() {
		switch err := recover().(type) {
		case nil:
			t.Fatal("bytes.Buffer.ReadFrom didn't panic")
		case error:
			// this is the error string of errNegativeRead
			wantError := "bytes.Buffer: reader returned negative count from Read"
			atomic.AddInt64(&checks, 1)
			if err.Error() != wantError {
				t.Fatalf("recovered panic: got %v, want %v", err.Error(), wantError)
			}
		default:
			t.Fatalf("unexpected panic value: %#v", err)
		}
	}()

	b.ReadFrom(new(negativeReader))
}

func TestWriteTo(t *testT) {
	var buf bytes.Buffer
	for i := 3; i < 30; i += 3 {
		s := fillBytes(t, "TestWriteTo (1)", &buf, "", 5, testBytes[:len(testBytes)/i])
		var b bytes.Buffer
		buf.WriteTo(&b)
		empty(t, "TestWriteTo (2)", &b, s, make([]byte, len(testString)))
	}
}

func TestWriteAppend(t *testT) {
	var got bytes.Buffer
	var want []byte
	for i := 0; i < 1000; i++ {
		b := got.AvailableBuffer()
		b = strconv.AppendInt(b, int64(i), 10)
		want = strconv.AppendInt(want, int64(i), 10)
		got.Write(b)
	}
	atomic.AddInt64(&checks, 1)
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("Bytes() = %q, want %q", &got, want)
	}

	// With a sufficiently sized buffer, there should be no allocations.
	n := allocsPerRun(100, func() {
		got.Reset()
		for i := 0; i < 1000; i++ {
			b := got.AvailableBuffer()
			b = strconv.AppendInt(b, int64(i), 10)
			got.Write(b)
		}
	})
	atomic.AddInt64(&checks, 1)
	if n > 0 {
		t.Errorf("allocations occurred while appending")
	}
}

func TestRuneIO(t *testT) {
	const NRune = 1000
	// Built a test slice while we write the data
	b := make([]byte, utf8.UTFMax*NRune)
	var buf bytes.Buffer
	n := 0
	for r := rune(0); r < NRune; r++ {
		size := utf8.EncodeRune(b[n:], r)
		nbytes, err := buf.WriteRune(r)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Fatalf("WriteRune(%U) error: %s", r, err)
		}
		atomic.AddInt64(&checks, 1)
		if nbytes != size {
			t.Fatalf("WriteRune(%U) expected %d, got %d", r, size, nbytes)
		}
		n += size
	}
	b = b[0:n]

	// Check the resulting bytes
	atomic.AddInt64(&checks, 1)
	if !bytes.Equal(buf.Bytes(), b) {
		t.Fatalf("incorrect result from WriteRune: %q not %q", buf.Bytes(), b)
	}

	p := make([]byte, utf8.UTFMax)
	// Read it back with ReadRune
	for r := rune(0); r < NRune; r++ {
		size := utf8.EncodeRune(p, r)
		nr, nbytes, err := buf.ReadRune()
		atomic.AddInt64(&checks, 1)
		if nr != r || nbytes != size || err != nil {
			t.Fatalf("ReadRune(%U) got %U,%d not %U,%d (err=%s)", r, nr, nbytes, r, size, err)
		}
	}

	// Check that UnreadRune works
	buf.Reset()

	// check at EOF
	atomic.AddInt64(&checks, 1)
	if err := buf.UnreadRune(); err == nil {
		t.Fatal("UnreadRune at EOF: got no error")
	}
	atomic.AddInt64(&checks, 1)
	if _, _, err := buf.ReadRune(); err == nil {
		t.Fatal("ReadRune at EOF: got no error")
	}
	atomic.AddInt64(&checks, 1)
	if err := buf.UnreadRune(); err == nil {
		t.Fatal("UnreadRune after ReadRune at EOF: got no error")
	}

	// check not at EOF
	buf.Write(b)
	for r := rune(0); r < NRune; r++ {
		r1, size, _ := buf.ReadRune()
		atomic.AddInt64(&checks, 1)
		if err := buf.UnreadRune(); err != nil {
			t.Fatalf("UnreadRune(%U) got error %q", r, err)
		}
		r2, nbytes, err := buf.ReadRune()
		atomic.AddInt64(&checks, 1)
		if r1 != r2 || r1 != r || nbytes != size || err != nil {
			t.Fatalf("ReadRune(%U) after UnreadRune got %U,%d not %U,%d (err=%s)", r, r2, nbytes, r, size, err)
		}
	}
}

func TestWriteInvalidRune(t *testT) {
	// Invalid runes, including negative ones, should be written as
	// utf8.RuneError.
	for _, r := range []rune{-1, utf8.MaxRune + 1} {
		var buf bytes.Buffer
		buf.WriteRune(r)
		check(t, fmt.Sprintf("TestWriteInvalidRune (%d)", r), &buf, "\uFFFD")
	}
}

func TestNext(t *testT) {
	b := []byte{0, 1, 2, 3, 4}
	tmp := make([]byte, 5)
	for i := 0; i <= 5; i++ {
		for j := i; j <= 5; j++ {
			for k := 0; k <= 6; k++ {
				// 0 <= i <= j <= 5; 0 <= k <= 6
				// Check that if we start with a buffer
				// of length j at offset i and ask for
				// Next(k), we get the right bytes.
				buf := bytes.NewBuffer(b[0:j])
				n, _ := buf.Read(tmp[0:i])
				atomic.AddInt64(&checks, 1)
				if n != i {
					t.Fatalf("Read %d returned %d", i, n)
				}
				bb := buf.Next(k)
				want := k
				atomic.AddInt64(&checks, 1)
				if want > j-i {
					want = j - i
				}
				atomic.AddInt64(&checks, 1)
				if len(bb) != want {
					t.Fatalf("in %d,%d: len(Next(%d)) == %d", i, j, k, len(bb))
				}
				for l, v := range bb {
					atomic.AddInt64(&checks, 1)
					if v != byte(l+i) {
						t.Fatalf("in %d,%d: Next(%d)[%d] = %d, want %d", i, j, k, l, v, l+i)
					}
				}
			}
		}
	}
}

var readBytesTests = []struct {
	buffer   string
	delim    byte
	expected []string
	err      error
}{
	{"", 0, []string{""}, io.EOF},
	{"a\x00", 0, []string{"a\x00"}, nil},
	{"abbbaaaba", 'b', []string{"ab", "b", "b", "aaab"}, nil},
	{"hello\x01world", 1, []string{"hello\x01"}, nil},
	{"foo\nbar", 0, []string{"foo\nbar"}, io.EOF},
	{"alpha\nbeta\ngamma\n", '\n', []string{"alpha\n", "beta\n", "gamma\n"}, nil},
	{"alpha\nbeta\ngamma", '\n', []string{"alpha\n", "beta\n", "gamma"}, io.EOF},
}

func TestReadBytes(t *testT) {
	for _, test := range readBytesTests {
		buf := bytes.NewBufferString(test.buffer)
		var err error
		for _, expected := range test.expected {
			var bytes []byte
			bytes, err = buf.ReadBytes(test.delim)
			atomic.AddInt64(&checks, 1)
			if string(bytes) != expected {
				t.Errorf("expected %q, got %q", expected, bytes)
			}
			atomic.AddInt64(&checks, 1)
			if err != nil {
				break
			}
		}
		atomic.AddInt64(&checks, 1)
		if err != test.err {
			t.Errorf("expected error %v, got %v", test.err, err)
		}
	}
}

func TestReadString(t *testT) {
	for _, test := range readBytesTests {
		buf := bytes.NewBufferString(test.buffer)
		var err error
		for _, expected := range test.expected {
			var s string
			s, err = buf.ReadString(test.delim)
			atomic.AddInt64(&checks, 1)
			if s != expected {
				t.Errorf("expected %q, got %q", expected, s)
			}
			atomic.AddInt64(&checks, 1)
			if err != nil {
				break
			}
		}
		atomic.AddInt64(&checks, 1)
		if err != test.err {
			t.Errorf("expected error %v, got %v", test.err, err)
		}
	}
}


func TestGrow(t *testT) {
	x := []byte{'x'}
	y := []byte{'y'}
	tmp := make([]byte, 72)
	for _, growLen := range []int{0, 100, 1000, 10000, 100000} {
		for _, startLen := range []int{0, 100, 1000, 10000, 100000} {
			xBytes := bytes.Repeat(x, startLen)

			buf := bytes.NewBuffer(xBytes)
			// If we read, this affects buf.off, which is good to test.
			readBytes, _ := buf.Read(tmp)
			yBytes := bytes.Repeat(y, growLen)
			allocs := allocsPerRun(100, func() {
				buf.Grow(growLen)
				buf.Write(yBytes)
			})
			// Check no allocation occurs in write, as long as we're single-threaded.
			atomic.AddInt64(&checks, 1)
			if allocs != 0 {
				t.Errorf("allocation occurred during write")
			}
			// Check that buffer has correct data.
			atomic.AddInt64(&checks, 1)
			if !bytes.Equal(buf.Bytes()[0:startLen-readBytes], xBytes[readBytes:]) {
				t.Errorf("bad initial data at %d %d", startLen, growLen)
			}
			atomic.AddInt64(&checks, 1)
			if !bytes.Equal(buf.Bytes()[startLen-readBytes:startLen-readBytes+growLen], yBytes) {
				t.Errorf("bad written data at %d %d", startLen, growLen)
			}
		}
	}
}

func TestGrowOverflow(t *testT) {
	defer func() {
		atomic.AddInt64(&checks, 1)
		if err := recover(); err != bytes.ErrTooLarge {
			t.Errorf("after too-large Grow, recover() = %v; want %v", err, bytes.ErrTooLarge)
		}
	}()

	buf := bytes.NewBuffer(make([]byte, 1))
	const maxInt = int(^uint(0) >> 1)
	buf.Grow(maxInt)
}

// Was a bug: used to give EOF reading empty slice at EOF.
func TestReadEmptyAtEOF(t *testT) {
	b := new(bytes.Buffer)
	slice := make([]byte, 0)
	n, err := b.Read(slice)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Errorf("read error: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if n != 0 {
		t.Errorf("wrong count; got %d want 0", n)
	}
}

func TestUnreadByte(t *testT) {
	b := new(bytes.Buffer)

	// check at EOF
	atomic.AddInt64(&checks, 1)
	if err := b.UnreadByte(); err == nil {
		t.Fatal("UnreadByte at EOF: got no error")
	}
	atomic.AddInt64(&checks, 1)
	if _, err := b.ReadByte(); err == nil {
		t.Fatal("ReadByte at EOF: got no error")
	}
	atomic.AddInt64(&checks, 1)
	if err := b.UnreadByte(); err == nil {
		t.Fatal("UnreadByte after ReadByte at EOF: got no error")
	}

	// check not at EOF
	b.WriteString("abcdefghijklmnopqrstuvwxyz")

	// after unsuccessful read
	atomic.AddInt64(&checks, 1)
	if n, err := b.Read(nil); n != 0 || err != nil {
		t.Fatalf("Read(nil) = %d,%v; want 0,nil", n, err)
	}
	atomic.AddInt64(&checks, 1)
	if err := b.UnreadByte(); err == nil {
		t.Fatal("UnreadByte after Read(nil): got no error")
	}

	// after successful read
	atomic.AddInt64(&checks, 1)
	if _, err := b.ReadBytes('m'); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if err := b.UnreadByte(); err != nil {
		t.Fatalf("UnreadByte: %v", err)
	}
	c, err := b.ReadByte()
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if c != 'm' {
		t.Errorf("ReadByte = %q; want %q", c, 'm')
	}
}

// Tests that we occasionally compact. Issue 5154.
func TestBufferGrowth(t *testT) {
	var b bytes.Buffer
	buf := make([]byte, 1024)
	b.Write(buf[0:1])
	var cap0 int
	for i := 0; i < 5<<10; i++ {
		b.Write(buf)
		b.Read(buf)
		atomic.AddInt64(&checks, 1)
		if i == 0 {
			cap0 = b.Cap()
		}
	}
	cap1 := b.Cap()
	// (*Buffer).grow allows for 2x capacity slop before sliding,
	// so set our error threshold at 3x.
	atomic.AddInt64(&checks, 1)
	if cap1 > cap0*3 {
		t.Errorf("buffer cap = %d; too big (grew from %d)", cap1, cap0)
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
		{"TestNewBuffer", TestNewBuffer},
		{"TestNewBufferShallow", TestNewBufferShallow},
		{"TestNewBufferString", TestNewBufferString},
		{"TestBasicOperations", TestBasicOperations},
		{"TestLargeStringWrites", TestLargeStringWrites},
		{"TestLargeByteWrites", TestLargeByteWrites},
		{"TestLargeStringReads", TestLargeStringReads},
		{"TestLargeByteReads", TestLargeByteReads},
		{"TestMixedReadsAndWrites", TestMixedReadsAndWrites},
		{"TestCapWithPreallocatedSlice", TestCapWithPreallocatedSlice},
		{"TestCapWithSliceAndWrittenData", TestCapWithSliceAndWrittenData},
		{"TestNil", TestNil},
		{"TestReadFrom", TestReadFrom},
		{"TestReadFromPanicReader", TestReadFromPanicReader},
		{"TestReadFromNegativeReader", TestReadFromNegativeReader},
		{"TestWriteTo", TestWriteTo},
		{"TestWriteAppend", TestWriteAppend},
		{"TestRuneIO", TestRuneIO},
		{"TestWriteInvalidRune", TestWriteInvalidRune},
		{"TestNext", TestNext},
		{"TestReadBytes", TestReadBytes},
		{"TestReadString", TestReadString},
		{"TestGrow", TestGrow},
		{"TestGrowOverflow", TestGrowOverflow},
		{"TestReadEmptyAtEOF", TestReadEmptyAtEOF},
		{"TestUnreadByte", TestUnreadByte},
		{"TestBufferGrowth", TestBufferGrowth},
	}
	runTests(tests_)
}

// Output:
// ok TestNewBuffer subtests: 0 checks: 4
// ok TestNewBufferShallow subtests: 0 checks: 5
// ok TestNewBufferString subtests: 0 checks: 4
// ok TestBasicOperations subtests: 0 checks: 390
// ok TestLargeStringWrites subtests: 0 checks: 373
// ok TestLargeByteWrites subtests: 0 checks: 373
// ok TestLargeStringReads subtests: 0 checks: 451
// ok TestLargeByteReads subtests: 0 checks: 451
// ok TestMixedReadsAndWrites subtests: 0 checks: 565
// ok TestCapWithPreallocatedSlice subtests: 0 checks: 1
// ok TestCapWithSliceAndWrittenData subtests: 0 checks: 1
// ok TestNil subtests: 0 checks: 1
// ok TestReadFrom subtests: 0 checks: 447
// ok TestReadFromPanicReader subtests: 0 checks: 12
// ok TestReadFromNegativeReader subtests: 0 checks: 1
// ok TestWriteTo subtests: 0 checks: 447
// ok TestWriteAppend subtests: 0 checks: 2
// ok TestRuneIO subtests: 0 checks: 5004
// ok TestWriteInvalidRune subtests: 0 checks: 8
// ok TestNext subtests: 0 checks: 616
// ok TestReadBytes subtests: 0 checks: 35
// ok TestReadString subtests: 0 checks: 35
// ok TestGrow subtests: 0 checks: 75
// ok TestGrowOverflow subtests: 0 checks: 1
// ok TestReadEmptyAtEOF subtests: 0 checks: 2
// ok TestUnreadByte subtests: 0 checks: 9
// ok TestBufferGrowth subtests: 0 checks: 5121
// tests: 27 failed: 0
