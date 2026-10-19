// phase: 09
// Go 1.25's bufio/bufio_test.go (Reader: Read, Peek, Discard, ReadByte/UnreadByte, ReadRune, ReadLine, ReadBytes, ReadString, WriteTo, Reset, Size; Writer: Write*, Flush, ReadFrom, Available, AvailableBuffer, errors; ReadWriter; the readers of testing/iotest are local copies) through a small harness; the allocation count test is dropped; expected output from real Go.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)


// OneByteReader returns a Reader that implements
// each non-empty Read by reading one byte from r.
func OneByteReader(r io.Reader) io.Reader { return &oneByteReader{r} }

type oneByteReader struct {
	r io.Reader
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return r.r.Read(p[0:1])
}

// HalfReader returns a Reader that implements Read
// by reading half as many requested bytes from r.
func HalfReader(r io.Reader) io.Reader { return &halfReader{r} }

type halfReader struct {
	r io.Reader
}

func (r *halfReader) Read(p []byte) (int, error) {
	return r.r.Read(p[0 : (len(p)+1)/2])
}

// DataErrReader changes the way errors are handled by a Reader. Normally, a
// Reader returns an error (typically EOF) from the first Read call after the
// last piece of data is read. DataErrReader wraps a Reader and changes its
// behavior so the final error is returned along with the final data, instead
// of in the first call after the final data.
func DataErrReader(r io.Reader) io.Reader { return &dataErrReader{r, nil, make([]byte, 1024)} }

type dataErrReader struct {
	r      io.Reader
	unread []byte
	data   []byte
}

func (r *dataErrReader) Read(p []byte) (n int, err error) {
	// loop because first call needs two reads:
	// one to get data and a second to look for an error.
	for {
		if len(r.unread) == 0 {
			n1, err1 := r.r.Read(r.data)
			r.unread = r.data[0:n1]
			err = err1
		}
		if n > 0 || err != nil {
			break
		}
		n = copy(p, r.unread)
		r.unread = r.unread[n:]
	}
	return
}

// ErrTimeout is a fake timeout error.
var ErrTimeout = errors.New("timeout")

// TimeoutReader returns [ErrTimeout] on the second read
// with no data. Subsequent calls to read succeed.
func TimeoutReader(r io.Reader) io.Reader { return &timeoutReader{r, 0} }

type timeoutReader struct {
	r     io.Reader
	count int
}

func (r *timeoutReader) Read(p []byte) (int, error) {
	r.count++
	if r.count == 2 {
		return 0, ErrTimeout
	}
	return r.r.Read(p)
}

// ErrReader returns an [io.Reader] that returns 0, err from all Read calls.
func ErrReader(err error) io.Reader {
	return &errReader{err: err}
}

type errReader struct {
	err error
}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, r.err
}


const DefaultBufSize = 4096 // bufio's export_test.go

// Reads from a reader and rot13s the result.
type rot13Reader struct {
	r io.Reader
}

func newRot13Reader(r io.Reader) *rot13Reader {
	r13 := new(rot13Reader)
	r13.r = r
	return r13
}

func (r13 *rot13Reader) Read(p []byte) (int, error) {
	n, err := r13.r.Read(p)
	for i := 0; i < n; i++ {
		c := p[i] | 0x20 // lowercase byte
		atomic.AddInt64(&checks, 1)
		if 'a' <= c && c <= 'm' {
			p[i] += 13
		} else if 'n' <= c && c <= 'z' {
			p[i] -= 13
		}
	}
	return n, err
}

// Call ReadByte to accumulate the text of a file
func readBytes(buf *bufio.Reader) string {
	var b [1000]byte
	nb := 0
	for {
		c, err := buf.ReadByte()
		atomic.AddInt64(&checks, 1)
		if err == io.EOF {
			break
		}
		atomic.AddInt64(&checks, 1)
		if err == nil {
			b[nb] = c
			nb++
		} else if err != ErrTimeout {
			panic("Data: " + err.Error())
		}
	}
	return string(b[0:nb])
}

func TestReaderSimple(t *testT) {
	data := "hello world"
	b := bufio.NewReader(strings.NewReader(data))
	atomic.AddInt64(&checks, 1)
	if s := readBytes(b); s != "hello world" {
		t.Errorf("simple hello world test failed: got %q", s)
	}

	b = bufio.NewReader(newRot13Reader(strings.NewReader(data)))
	atomic.AddInt64(&checks, 1)
	if s := readBytes(b); s != "uryyb jbeyq" {
		t.Errorf("rot13 hello world test failed: got %q", s)
	}
}

type readMaker struct {
	name string
	fn   func(io.Reader) io.Reader
}

var readMakers = []readMaker{
	{"full", func(r io.Reader) io.Reader { return r }},
	{"byte", OneByteReader},
	{"half", HalfReader},
	{"data+err", DataErrReader},
	{"timeout", TimeoutReader},
}

// Call ReadString (which ends up calling everything else)
// to accumulate the text of a file.
func readLines(b *bufio.Reader) string {
	s := ""
	for {
		s1, err := b.ReadString('\n')
		atomic.AddInt64(&checks, 1)
		if err == io.EOF {
			break
		}
		atomic.AddInt64(&checks, 1)
		if err != nil && err != ErrTimeout {
			panic("GetLines: " + err.Error())
		}
		s += s1
	}
	return s
}

// Call Read to accumulate the text of a file
func reads(buf *bufio.Reader, m int) string {
	var b [1000]byte
	nb := 0
	for {
		n, err := buf.Read(b[nb : nb+m])
		nb += n
		atomic.AddInt64(&checks, 1)
		if err == io.EOF {
			break
		}
	}
	return string(b[0:nb])
}

type bufReader struct {
	name string
	fn   func(*bufio.Reader) string
}

var bufreaders = []bufReader{
	{"1", func(b *bufio.Reader) string { return reads(b, 1) }},
	{"2", func(b *bufio.Reader) string { return reads(b, 2) }},
	{"3", func(b *bufio.Reader) string { return reads(b, 3) }},
	{"4", func(b *bufio.Reader) string { return reads(b, 4) }},
	{"5", func(b *bufio.Reader) string { return reads(b, 5) }},
	{"7", func(b *bufio.Reader) string { return reads(b, 7) }},
	{"bytes", readBytes},
	{"lines", readLines},
}

const minReadBufferSize = 16

var bufsizes = []int{
	0, minReadBufferSize, 23, 32, 46, 64, 93, 128, 1024, 4096,
}

func TestReader(t *testT) {
	var texts [31]string
	str := ""
	all := ""
	for i := 0; i < len(texts)-1; i++ {
		texts[i] = str + "\n"
		all += texts[i]
		str += string(rune(i%26 + 'a'))
	}
	texts[len(texts)-1] = all

	for h := 0; h < len(texts); h++ {
		text := texts[h]
		for i := 0; i < len(readMakers); i++ {
			for j := 0; j < len(bufreaders); j++ {
				for k := 0; k < len(bufsizes); k++ {
					readmaker := readMakers[i]
					bufreader := bufreaders[j]
					bufsize := bufsizes[k]
					read := readmaker.fn(strings.NewReader(text))
					buf := bufio.NewReaderSize(read, bufsize)
					s := bufreader.fn(buf)
					atomic.AddInt64(&checks, 1)
					if s != text {
						t.Errorf("reader=%s fn=%s bufsize=%d want=%q got=%q",
							readmaker.name, bufreader.name, bufsize, text, s)
					}
				}
			}
		}
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	return 0, nil
}

func TestZeroReader(t *testT) {
	var z zeroReader
	r := bufio.NewReader(z)

	c := make(chan error)
	go func() {
		_, err := r.ReadByte()
		c <- err
	}()

	select {
	case err := <-c:
		atomic.AddInt64(&checks, 1)
		if err == nil {
			t.Error("error expected")
		} else if err != io.ErrNoProgress {
			t.Error("unexpected error:", err)
		}
	case <-time.After(time.Second):
		t.Error("test timed out (endless loop in ReadByte?)")
	}
}

// A StringReader delivers its data one string segment at a time via Read.
type StringReader struct {
	data []string
	step int
}

func (r *StringReader) Read(p []byte) (n int, err error) {
	atomic.AddInt64(&checks, 1)
	if r.step < len(r.data) {
		s := r.data[r.step]
		n = copy(p, s)
		r.step++
	} else {
		err = io.EOF
	}
	return
}

func readRuneSegments(t *testT, segments []string) {
	got := ""
	want := strings.Join(segments, "")
	r := bufio.NewReader(&StringReader{data: segments})
	for {
		r, _, err := r.ReadRune()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			atomic.AddInt64(&checks, 1)
			if err != io.EOF {
				return
			}
			break
		}
		got += string(r)
	}
	atomic.AddInt64(&checks, 1)
	if got != want {
		t.Errorf("segments=%v got=%s want=%s", segments, got, want)
	}
}

var segmentList = [][]string{
	{},
	{""},
	{"日", "本語"},
	{"\u65e5", "\u672c", "\u8a9e"},
	{"\U000065e5", "\U0000672c", "\U00008a9e"},
	{"\xe6", "\x97\xa5\xe6", "\x9c\xac\xe8\xaa\x9e"},
	{"Hello", ", ", "World", "!"},
	{"Hello", ", ", "", "World", "!"},
}

func TestReadRune(t *testT) {
	for _, s := range segmentList {
		readRuneSegments(t, s)
	}
}

func TestUnreadRune(t *testT) {
	segments := []string{"Hello, world:", "日本語"}
	r := bufio.NewReader(&StringReader{data: segments})
	got := ""
	want := strings.Join(segments, "")
	// Normal execution.
	for {
		r1, _, err := r.ReadRune()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			atomic.AddInt64(&checks, 1)
			if err != io.EOF {
				t.Error("unexpected error on ReadRune:", err)
			}
			break
		}
		got += string(r1)
		// Put it back and read it again.
		atomic.AddInt64(&checks, 1)
		if err = r.UnreadRune(); err != nil {
			t.Fatal("unexpected error on UnreadRune:", err)
		}
		r2, _, err := r.ReadRune()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Fatal("unexpected error reading after unreading:", err)
		}
		atomic.AddInt64(&checks, 1)
		if r1 != r2 {
			t.Fatalf("incorrect rune after unread: got %c, want %c", r1, r2)
		}
	}
	atomic.AddInt64(&checks, 1)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNoUnreadRuneAfterPeek(t *testT) {
	br := bufio.NewReader(strings.NewReader("example"))
	br.ReadRune()
	br.Peek(1)
	atomic.AddInt64(&checks, 1)
	if err := br.UnreadRune(); err == nil {
		t.Error("UnreadRune didn't fail after Peek")
	}
}

func TestNoUnreadByteAfterPeek(t *testT) {
	br := bufio.NewReader(strings.NewReader("example"))
	br.ReadByte()
	br.Peek(1)
	atomic.AddInt64(&checks, 1)
	if err := br.UnreadByte(); err == nil {
		t.Error("UnreadByte didn't fail after Peek")
	}
}

func TestNoUnreadRuneAfterDiscard(t *testT) {
	br := bufio.NewReader(strings.NewReader("example"))
	br.ReadRune()
	br.Discard(1)
	atomic.AddInt64(&checks, 1)
	if err := br.UnreadRune(); err == nil {
		t.Error("UnreadRune didn't fail after Discard")
	}
}

func TestNoUnreadByteAfterDiscard(t *testT) {
	br := bufio.NewReader(strings.NewReader("example"))
	br.ReadByte()
	br.Discard(1)
	atomic.AddInt64(&checks, 1)
	if err := br.UnreadByte(); err == nil {
		t.Error("UnreadByte didn't fail after Discard")
	}
}

func TestNoUnreadRuneAfterWriteTo(t *testT) {
	br := bufio.NewReader(strings.NewReader("example"))
	br.WriteTo(io.Discard)
	atomic.AddInt64(&checks, 1)
	if err := br.UnreadRune(); err == nil {
		t.Error("UnreadRune didn't fail after WriteTo")
	}
}

func TestNoUnreadByteAfterWriteTo(t *testT) {
	br := bufio.NewReader(strings.NewReader("example"))
	br.WriteTo(io.Discard)
	atomic.AddInt64(&checks, 1)
	if err := br.UnreadByte(); err == nil {
		t.Error("UnreadByte didn't fail after WriteTo")
	}
}

func TestUnreadByte(t *testT) {
	segments := []string{"Hello, ", "world"}
	r := bufio.NewReader(&StringReader{data: segments})
	got := ""
	want := strings.Join(segments, "")
	// Normal execution.
	for {
		b1, err := r.ReadByte()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			atomic.AddInt64(&checks, 1)
			if err != io.EOF {
				t.Error("unexpected error on ReadByte:", err)
			}
			break
		}
		got += string(b1)
		// Put it back and read it again.
		atomic.AddInt64(&checks, 1)
		if err = r.UnreadByte(); err != nil {
			t.Fatal("unexpected error on UnreadByte:", err)
		}
		b2, err := r.ReadByte()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Fatal("unexpected error reading after unreading:", err)
		}
		atomic.AddInt64(&checks, 1)
		if b1 != b2 {
			t.Fatalf("incorrect byte after unread: got %q, want %q", b1, b2)
		}
	}
	atomic.AddInt64(&checks, 1)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnreadByteMultiple(t *testT) {
	segments := []string{"Hello, ", "world"}
	data := strings.Join(segments, "")
	for n := 0; n <= len(data); n++ {
		r := bufio.NewReader(&StringReader{data: segments})
		// Read n bytes.
		for i := 0; i < n; i++ {
			b, err := r.ReadByte()
			atomic.AddInt64(&checks, 1)
			if err != nil {
				t.Fatalf("n = %d: unexpected error on ReadByte: %v", n, err)
			}
			atomic.AddInt64(&checks, 1)
			if b != data[i] {
				t.Fatalf("n = %d: incorrect byte returned from ReadByte: got %q, want %q", n, b, data[i])
			}
		}
		// Unread one byte if there is one.
		atomic.AddInt64(&checks, 1)
		if n > 0 {
			atomic.AddInt64(&checks, 1)
			if err := r.UnreadByte(); err != nil {
				t.Errorf("n = %d: unexpected error on UnreadByte: %v", n, err)
			}
		}
		// Test that we cannot unread any further.
		atomic.AddInt64(&checks, 1)
		if err := r.UnreadByte(); err == nil {
			t.Errorf("n = %d: expected error on UnreadByte", n)
		}
	}
}

func TestUnreadByteOthers(t *testT) {
	// A list of readers to use in conjunction with UnreadByte.
	var readers = []func(*bufio.Reader, byte) ([]byte, error){
		(*bufio.Reader).ReadBytes,
		(*bufio.Reader).ReadSlice,
		func(r *bufio.Reader, delim byte) ([]byte, error) {
			data, err := r.ReadString(delim)
			return []byte(data), err
		},
		// ReadLine doesn't fit the data/pattern easily
		// so we leave it out. It should be covered via
		// the ReadSlice test since ReadLine simply calls
		// ReadSlice, and it's that function that handles
		// the last byte.
	}

	// Try all readers with UnreadByte.
	for rno, read := range readers {
		// Some input data that is longer than the minimum reader buffer size.
		const n = 10
		var buf bytes.Buffer
		for i := 0; i < n; i++ {
			buf.WriteString("abcdefg")
		}

		r := bufio.NewReaderSize(&buf, minReadBufferSize)
		readTo := func(delim byte, want string) {
			data, err := read(r, delim)
			atomic.AddInt64(&checks, 1)
			if err != nil {
				t.Fatalf("#%d: unexpected error reading to %c: %v", rno, delim, err)
			}
			atomic.AddInt64(&checks, 1)
			if got := string(data); got != want {
				t.Fatalf("#%d: got %q, want %q", rno, got, want)
			}
		}

		// Read the data with occasional UnreadByte calls.
		for i := 0; i < n; i++ {
			readTo('d', "abcd")
			for j := 0; j < 3; j++ {
				atomic.AddInt64(&checks, 1)
				if err := r.UnreadByte(); err != nil {
					t.Fatalf("#%d: unexpected error on UnreadByte: %v", rno, err)
				}
				readTo('d', "d")
			}
			readTo('g', "efg")
		}

		// All data should have been read.
		_, err := r.ReadByte()
		atomic.AddInt64(&checks, 1)
		if err != io.EOF {
			t.Errorf("#%d: got error %v; want EOF", rno, err)
		}
	}
}

// Test that UnreadRune fails if the preceding operation was not a ReadRune.
func TestUnreadRuneError(t *testT) {
	buf := make([]byte, 3) // All runes in this test are 3 bytes long
	r := bufio.NewReader(&StringReader{data: []string{"日本語日本語日本語"}})
	atomic.AddInt64(&checks, 1)
	if r.UnreadRune() == nil {
		t.Error("expected error on UnreadRune from fresh buffer")
	}
	_, _, err := r.ReadRune()
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on ReadRune (1):", err)
	}
	atomic.AddInt64(&checks, 1)
	if err = r.UnreadRune(); err != nil {
		t.Error("unexpected error on UnreadRune (1):", err)
	}
	atomic.AddInt64(&checks, 1)
	if r.UnreadRune() == nil {
		t.Error("expected error after UnreadRune (1)")
	}
	// Test error after Read.
	_, _, err = r.ReadRune() // reset state
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on ReadRune (2):", err)
	}
	_, err = r.Read(buf)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on Read (2):", err)
	}
	atomic.AddInt64(&checks, 1)
	if r.UnreadRune() == nil {
		t.Error("expected error after Read (2)")
	}
	// Test error after ReadByte.
	_, _, err = r.ReadRune() // reset state
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on ReadRune (2):", err)
	}
	for range buf {
		_, err = r.ReadByte()
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Error("unexpected error on ReadByte (2):", err)
		}
	}
	atomic.AddInt64(&checks, 1)
	if r.UnreadRune() == nil {
		t.Error("expected error after ReadByte")
	}
	// Test error after UnreadByte.
	_, _, err = r.ReadRune() // reset state
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on ReadRune (3):", err)
	}
	_, err = r.ReadByte()
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on ReadByte (3):", err)
	}
	err = r.UnreadByte()
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on UnreadByte (3):", err)
	}
	atomic.AddInt64(&checks, 1)
	if r.UnreadRune() == nil {
		t.Error("expected error after UnreadByte (3)")
	}
	// Test error after ReadSlice.
	_, _, err = r.ReadRune() // reset state
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error("unexpected error on ReadRune (4):", err)
	}
	_, err = r.ReadSlice(0)
	atomic.AddInt64(&checks, 1)
	if err != io.EOF {
		t.Error("unexpected error on ReadSlice (4):", err)
	}
	atomic.AddInt64(&checks, 1)
	if r.UnreadRune() == nil {
		t.Error("expected error after ReadSlice (4)")
	}
}

func TestUnreadRuneAtEOF(t *testT) {
	// UnreadRune/ReadRune should error at EOF (was a bug; used to panic)
	r := bufio.NewReader(strings.NewReader("x"))
	r.ReadRune()
	r.ReadRune()
	r.UnreadRune()
	_, _, err := r.ReadRune()
	atomic.AddInt64(&checks, 1)
	if err == nil {
		t.Error("expected error at EOF")
	} else if err != io.EOF {
		t.Error("expected EOF; got", err)
	}
}

func TestReadWriteRune(t *testT) {
	const NRune = 1000
	byteBuf := new(bytes.Buffer)
	w := bufio.NewWriter(byteBuf)
	// Write the runes out using WriteRune
	buf := make([]byte, utf8.UTFMax)
	for r := rune(0); r < NRune; r++ {
		size := utf8.EncodeRune(buf, r)
		nbytes, err := w.WriteRune(r)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Fatalf("WriteRune(0x%x) error: %s", r, err)
		}
		atomic.AddInt64(&checks, 1)
		if nbytes != size {
			t.Fatalf("WriteRune(0x%x) expected %d, got %d", r, size, nbytes)
		}
	}
	w.Flush()

	r := bufio.NewReader(byteBuf)
	// Read them back with ReadRune
	for r1 := rune(0); r1 < NRune; r1++ {
		size := utf8.EncodeRune(buf, r1)
		nr, nbytes, err := r.ReadRune()
		atomic.AddInt64(&checks, 1)
		if nr != r1 || nbytes != size || err != nil {
			t.Fatalf("ReadRune(0x%x) got 0x%x,%d not 0x%x,%d (err=%s)", r1, nr, nbytes, r1, size, err)
		}
	}
}

func TestWriteInvalidRune(t *testT) {
	// Invalid runes, including negative ones, should be written as the
	// replacement character.
	for _, r := range []rune{-1, utf8.MaxRune + 1} {
		var buf strings.Builder
		w := bufio.NewWriter(&buf)
		w.WriteRune(r)
		w.Flush()
		atomic.AddInt64(&checks, 1)
		if s := buf.String(); s != "\uFFFD" {
			t.Errorf("WriteRune(%d) wrote %q, not replacement character", r, s)
		}
	}
}


func TestWriter(t *testT) {
	var data [8192]byte

	for i := 0; i < len(data); i++ {
		data[i] = byte(' ' + i%('~'-' '))
	}
	w := new(bytes.Buffer)
	for i := 0; i < len(bufsizes); i++ {
		for j := 0; j < len(bufsizes); j++ {
			nwrite := bufsizes[i]
			bs := bufsizes[j]

			// Write nwrite bytes using buffer size bs.
			// Check that the right amount makes it out
			// and that the data is correct.

			w.Reset()
			buf := bufio.NewWriterSize(w, bs)
			context := fmt.Sprintf("nwrite=%d bufsize=%d", nwrite, bs)
			n, e1 := buf.Write(data[0:nwrite])
			atomic.AddInt64(&checks, 1)
			if e1 != nil || n != nwrite {
				t.Errorf("%s: buf.Write %d = %d, %v", context, nwrite, n, e1)
				continue
			}
			atomic.AddInt64(&checks, 1)
			if e := buf.Flush(); e != nil {
				t.Errorf("%s: buf.Flush = %v", context, e)
			}

			written := w.Bytes()
			atomic.AddInt64(&checks, 1)
			if len(written) != nwrite {
				t.Errorf("%s: %d bytes written", context, len(written))
			}
			for l := 0; l < len(written); l++ {
				atomic.AddInt64(&checks, 1)
				if written[l] != data[l] {
					t.Errorf("wrong bytes written")
					t.Errorf("want=%q", data[:len(written)])
					t.Errorf("have=%q", written)
				}
			}
		}
	}
}

func TestWriterAppend(t *testT) {
	got := new(bytes.Buffer)
	var want []byte
	rn := rng
	w := bufio.NewWriterSize(got, 64)
	for i := 0; i < 100; i++ {
		// Obtain a buffer to append to.
		b := w.AvailableBuffer()
		atomic.AddInt64(&checks, 1)
		if w.Available() != cap(b) {
			t.Fatalf("Available() = %v, want %v", w.Available(), cap(b))
		}

		// While not recommended, it is valid to append to a shifted buffer.
		// This forces Write to copy the input.
		atomic.AddInt64(&checks, 1)
		if rn.Intn(8) == 0 && cap(b) > 0 {
			b = b[1:1:cap(b)]
		}

		// Append a random integer of varying width.
		n := int64(rn.Intn(1 << rn.Intn(30)))
		want = append(strconv.AppendInt(want, n, 10), ' ')
		b = append(strconv.AppendInt(b, n, 10), ' ')
		w.Write(b)
	}
	w.Flush()

	atomic.AddInt64(&checks, 1)
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("output mismatch:\ngot  %s\nwant %s", got.Bytes(), want)
	}
}

// Check that write errors are returned properly.

type errorWriterTest struct {
	n, m   int
	err    error
	expect error
}

func (w errorWriterTest) Write(p []byte) (int, error) {
	return len(p) * w.n / w.m, w.err
}

var errorWriterTests = []errorWriterTest{
	{0, 1, nil, io.ErrShortWrite},
	{1, 2, nil, io.ErrShortWrite},
	{1, 1, nil, nil},
	{0, 1, io.ErrClosedPipe, io.ErrClosedPipe},
	{1, 2, io.ErrClosedPipe, io.ErrClosedPipe},
	{1, 1, io.ErrClosedPipe, io.ErrClosedPipe},
}

func TestWriteErrors(t *testT) {
	for _, w := range errorWriterTests {
		buf := bufio.NewWriter(w)
		_, e := buf.Write([]byte("hello world"))
		atomic.AddInt64(&checks, 1)
		if e != nil {
			t.Errorf("Write hello to %v: %v", w, e)
			continue
		}
		// Two flushes, to verify the error is sticky.
		for i := 0; i < 2; i++ {
			e = buf.Flush()
			atomic.AddInt64(&checks, 1)
			if e != w.expect {
				t.Errorf("Flush %d/2 %v: got %v, wanted %v", i+1, w, e, w.expect)
			}
		}
	}
}

func TestNewReaderSizeIdempotent(t *testT) {
	const BufSize = 1000
	b := bufio.NewReaderSize(strings.NewReader("hello world"), BufSize)
	// Does it recognize itself?
	b1 := bufio.NewReaderSize(b, BufSize)
	atomic.AddInt64(&checks, 1)
	if b1 != b {
		t.Error("NewReaderSize did not detect underlying Reader")
	}
	// Does it wrap if existing buffer is too small?
	b2 := bufio.NewReaderSize(b, 2*BufSize)
	atomic.AddInt64(&checks, 1)
	if b2 == b {
		t.Error("NewReaderSize did not enlarge buffer")
	}
}

func TestNewWriterSizeIdempotent(t *testT) {
	const BufSize = 1000
	b := bufio.NewWriterSize(new(bytes.Buffer), BufSize)
	// Does it recognize itself?
	b1 := bufio.NewWriterSize(b, BufSize)
	atomic.AddInt64(&checks, 1)
	if b1 != b {
		t.Error("NewWriterSize did not detect underlying Writer")
	}
	// Does it wrap if existing buffer is too small?
	b2 := bufio.NewWriterSize(b, 2*BufSize)
	atomic.AddInt64(&checks, 1)
	if b2 == b {
		t.Error("NewWriterSize did not enlarge buffer")
	}
}

func TestWriteString(t *testT) {
	const BufSize = 8
	buf := new(strings.Builder)
	b := bufio.NewWriterSize(buf, BufSize)
	b.WriteString("0")                         // easy
	b.WriteString("123456")                    // still easy
	b.WriteString("7890")                      // easy after flush
	b.WriteString("abcdefghijklmnopqrstuvwxy") // hard
	b.WriteString("z")
	atomic.AddInt64(&checks, 1)
	if err := b.Flush(); err != nil {
		t.Error("WriteString", err)
	}
	s := "01234567890abcdefghijklmnopqrstuvwxyz"
	atomic.AddInt64(&checks, 1)
	if buf.String() != s {
		t.Errorf("WriteString wants %q gets %q", s, buf.String())
	}
}

func TestWriteStringStringWriter(t *testT) {
	const BufSize = 8
	{
		tw := &teststringwriter{}
		b := bufio.NewWriterSize(tw, BufSize)
		b.WriteString("1234")
		tw.check(t, "", "")
		b.WriteString("56789012")   // longer than BufSize
		tw.check(t, "12345678", "") // but not enough (after filling the partially-filled buffer)
		b.Flush()
		tw.check(t, "123456789012", "")
	}
	{
		tw := &teststringwriter{}
		b := bufio.NewWriterSize(tw, BufSize)
		b.WriteString("123456789")   // long string, empty buffer:
		tw.check(t, "", "123456789") // use WriteString
	}
	{
		tw := &teststringwriter{}
		b := bufio.NewWriterSize(tw, BufSize)
		b.WriteString("abc")
		tw.check(t, "", "")
		b.WriteString("123456789012345")      // long string, non-empty buffer
		tw.check(t, "abc12345", "6789012345") // use Write and then WriteString since the remaining part is still longer than BufSize
	}
	{
		tw := &teststringwriter{}
		b := bufio.NewWriterSize(tw, BufSize)
		b.Write([]byte("abc")) // same as above, but use Write instead of WriteString
		tw.check(t, "", "")
		b.WriteString("123456789012345")
		tw.check(t, "abc12345", "6789012345") // same as above
	}
}

type teststringwriter struct {
	write       string
	writeString string
}

func (w *teststringwriter) Write(b []byte) (int, error) {
	w.write += string(b)
	return len(b), nil
}

func (w *teststringwriter) WriteString(s string) (int, error) {
	w.writeString += s
	return len(s), nil
}

func (w *teststringwriter) check(t *testT, write, writeString string) {
	t.Helper()
	atomic.AddInt64(&checks, 1)
	if w.write != write {
		t.Errorf("write: expected %q, got %q", write, w.write)
	}
	atomic.AddInt64(&checks, 1)
	if w.writeString != writeString {
		t.Errorf("writeString: expected %q, got %q", writeString, w.writeString)
	}
}

func TestBufferFull(t *testT) {
	const longString = "And now, hello, world! It is the time for all good men to come to the aid of their party"
	buf := bufio.NewReaderSize(strings.NewReader(longString), minReadBufferSize)
	line, err := buf.ReadSlice('!')
	atomic.AddInt64(&checks, 1)
	if string(line) != "And now, hello, " || err != bufio.ErrBufferFull {
		t.Errorf("first ReadSlice(,) = %q, %v", line, err)
	}
	line, err = buf.ReadSlice('!')
	atomic.AddInt64(&checks, 1)
	if string(line) != "world!" || err != nil {
		t.Errorf("second ReadSlice(,) = %q, %v", line, err)
	}
}

func TestPeek(t *testT) {
	p := make([]byte, 10)
	// string is 16 (minReadBufferSize) long.
	buf := bufio.NewReaderSize(strings.NewReader("abcdefghijklmnop"), minReadBufferSize)
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(1); string(s) != "a" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "a", string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(4); string(s) != "abcd" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "abcd", string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := buf.Peek(-1); err != bufio.ErrNegativeCount {
		t.Fatalf("want ErrNegativeCount got %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(32); string(s) != "abcdefghijklmnop" || err != bufio.ErrBufferFull {
		t.Fatalf("want %q, ErrBufFull got %q, err=%v", "abcdefghijklmnop", string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := buf.Read(p[0:3]); string(p[0:3]) != "abc" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "abc", string(p[0:3]), err)
	}
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(1); string(s) != "d" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "d", string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(2); string(s) != "de" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "de", string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := buf.Read(p[0:3]); string(p[0:3]) != "def" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "def", string(p[0:3]), err)
	}
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(4); string(s) != "ghij" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "ghij", string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := buf.Read(p[0:]); string(p[0:]) != "ghijklmnop" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "ghijklmnop", string(p[0:minReadBufferSize]), err)
	}
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(0); string(s) != "" || err != nil {
		t.Fatalf("want %q got %q, err=%v", "", string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := buf.Peek(1); err != io.EOF {
		t.Fatalf("want EOF got %v", err)
	}

	// Test for issue 3022, not exposing a reader's error on a successful Peek.
	buf = bufio.NewReaderSize(dataAndEOFReader("abcd"), 32)
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(2); string(s) != "ab" || err != nil {
		t.Errorf(`Peek(2) on "abcd", EOF = %q, %v; want "ab", nil`, string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if s, err := buf.Peek(4); string(s) != "abcd" || err != nil {
		t.Errorf(`Peek(4) on "abcd", EOF = %q, %v; want "abcd", nil`, string(s), err)
	}
	atomic.AddInt64(&checks, 1)
	if n, err := buf.Read(p[0:5]); string(p[0:n]) != "abcd" || err != nil {
		t.Fatalf("Read after peek = %q, %v; want abcd, EOF", p[0:n], err)
	}
	atomic.AddInt64(&checks, 1)
	if n, err := buf.Read(p[0:1]); string(p[0:n]) != "" || err != io.EOF {
		t.Fatalf(`second Read after peek = %q, %v; want "", EOF`, p[0:n], err)
	}
}

type dataAndEOFReader string

func (r dataAndEOFReader) Read(p []byte) (int, error) {
	return copy(p, r), io.EOF
}

func TestPeekThenUnreadRune(t *testT) {
	// This sequence used to cause a crash.
	r := bufio.NewReader(strings.NewReader("x"))
	r.ReadRune()
	r.Peek(1)
	r.UnreadRune()
	r.ReadRune() // Used to panic here
}

var testOutput = []byte("0123456789abcdefghijklmnopqrstuvwxy")
var testInput = []byte("012\n345\n678\n9ab\ncde\nfgh\nijk\nlmn\nopq\nrst\nuvw\nxy")
var testInputrn = []byte("012\r\n345\r\n678\r\n9ab\r\ncde\r\nfgh\r\nijk\r\nlmn\r\nopq\r\nrst\r\nuvw\r\nxy\r\n\n\r\n")

// TestReader wraps a []byte and returns reads of a specific length.
type testReader struct {
	data   []byte
	stride int
}

func (t *testReader) Read(buf []byte) (n int, err error) {
	n = t.stride
	atomic.AddInt64(&checks, 1)
	if n > len(t.data) {
		n = len(t.data)
	}
	atomic.AddInt64(&checks, 1)
	if n > len(buf) {
		n = len(buf)
	}
	copy(buf, t.data)
	t.data = t.data[n:]
	atomic.AddInt64(&checks, 1)
	if len(t.data) == 0 {
		err = io.EOF
	}
	return
}

func testReadLine(t *testT, input []byte) {
	for stride := 1; stride < 2; stride++ {
		done := 0
		reader := testReader{input, stride}
		l := bufio.NewReaderSize(&reader, len(input)+1)
		for {
			line, isPrefix, err := l.ReadLine()
			atomic.AddInt64(&checks, 1)
			if len(line) > 0 && err != nil {
				t.Errorf("ReadLine returned both data and error: %s", err)
			}
			atomic.AddInt64(&checks, 1)
			if isPrefix {
				t.Errorf("ReadLine returned prefix")
			}
			atomic.AddInt64(&checks, 1)
			if err != nil {
				atomic.AddInt64(&checks, 1)
				if err != io.EOF {
					t.Fatalf("Got unknown error: %s", err)
				}
				break
			}
			atomic.AddInt64(&checks, 1)
			if want := testOutput[done : done+len(line)]; !bytes.Equal(want, line) {
				t.Errorf("Bad line at stride %d: want: %x got: %x", stride, want, line)
			}
			done += len(line)
		}
		atomic.AddInt64(&checks, 1)
		if done != len(testOutput) {
			t.Errorf("ReadLine didn't return everything: got: %d, want: %d (stride: %d)", done, len(testOutput), stride)
		}
	}
}

func TestReadLine(t *testT) {
	testReadLine(t, testInput)
	testReadLine(t, testInputrn)
}

func TestLineTooLong(t *testT) {
	data := make([]byte, 0)
	for i := 0; i < minReadBufferSize*5/2; i++ {
		data = append(data, '0'+byte(i%10))
	}
	buf := bytes.NewReader(data)
	l := bufio.NewReaderSize(buf, minReadBufferSize)
	line, isPrefix, err := l.ReadLine()
	atomic.AddInt64(&checks, 1)
	if !isPrefix || !bytes.Equal(line, data[:minReadBufferSize]) || err != nil {
		t.Errorf("bad result for first line: got %q want %q %v", line, data[:minReadBufferSize], err)
	}
	data = data[len(line):]
	line, isPrefix, err = l.ReadLine()
	atomic.AddInt64(&checks, 1)
	if !isPrefix || !bytes.Equal(line, data[:minReadBufferSize]) || err != nil {
		t.Errorf("bad result for second line: got %q want %q %v", line, data[:minReadBufferSize], err)
	}
	data = data[len(line):]
	line, isPrefix, err = l.ReadLine()
	atomic.AddInt64(&checks, 1)
	if isPrefix || !bytes.Equal(line, data[:minReadBufferSize/2]) || err != nil {
		t.Errorf("bad result for third line: got %q want %q %v", line, data[:minReadBufferSize/2], err)
	}
	line, isPrefix, err = l.ReadLine()
	atomic.AddInt64(&checks, 1)
	if isPrefix || err == nil {
		t.Errorf("expected no more lines: %x %s", line, err)
	}
}

func TestReadAfterLines(t *testT) {
	line1 := "this is line1"
	restData := "this is line2\nthis is line 3\n"
	inbuf := bytes.NewReader([]byte(line1 + "\n" + restData))
	outbuf := new(strings.Builder)
	maxLineLength := len(line1) + len(restData)/2
	l := bufio.NewReaderSize(inbuf, maxLineLength)
	line, isPrefix, err := l.ReadLine()
	atomic.AddInt64(&checks, 1)
	if isPrefix || err != nil || string(line) != line1 {
		t.Errorf("bad result for first line: isPrefix=%v err=%v line=%q", isPrefix, err, string(line))
	}
	n, err := io.Copy(outbuf, l)
	atomic.AddInt64(&checks, 1)
	if int(n) != len(restData) || err != nil {
		t.Errorf("bad result for Read: n=%d err=%v", n, err)
	}
	atomic.AddInt64(&checks, 1)
	if outbuf.String() != restData {
		t.Errorf("bad result for Read: got %q; expected %q", outbuf.String(), restData)
	}
}

func TestReadEmptyBuffer(t *testT) {
	l := bufio.NewReaderSize(new(bytes.Buffer), minReadBufferSize)
	line, isPrefix, err := l.ReadLine()
	atomic.AddInt64(&checks, 1)
	if err != io.EOF {
		t.Errorf("expected EOF from ReadLine, got '%s' %t %s", line, isPrefix, err)
	}
}

func TestLinesAfterRead(t *testT) {
	l := bufio.NewReaderSize(bytes.NewReader([]byte("foo")), minReadBufferSize)
	_, err := io.ReadAll(l)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error(err)
		return
	}

	line, isPrefix, err := l.ReadLine()
	atomic.AddInt64(&checks, 1)
	if err != io.EOF {
		t.Errorf("expected EOF from ReadLine, got '%s' %t %s", line, isPrefix, err)
	}
}

func TestReadLineNonNilLineOrError(t *testT) {
	r := bufio.NewReader(strings.NewReader("line 1\n"))
	for i := 0; i < 2; i++ {
		l, _, err := r.ReadLine()
		atomic.AddInt64(&checks, 1)
		if l != nil && err != nil {
			t.Fatalf("on line %d/2; ReadLine=%#v, %v; want non-nil line or Error, but not both",
				i+1, l, err)
		}
	}
}

type readLineResult struct {
	line     []byte
	isPrefix bool
	err      error
}

var readLineNewlinesTests = []struct {
	input  string
	expect []readLineResult
}{
	{"012345678901234\r\n012345678901234\r\n", []readLineResult{
		{[]byte("012345678901234"), true, nil},
		{nil, false, nil},
		{[]byte("012345678901234"), true, nil},
		{nil, false, nil},
		{nil, false, io.EOF},
	}},
	{"0123456789012345\r012345678901234\r", []readLineResult{
		{[]byte("0123456789012345"), true, nil},
		{[]byte("\r012345678901234"), true, nil},
		{[]byte("\r"), false, nil},
		{nil, false, io.EOF},
	}},
}

func TestReadLineNewlines(t *testT) {
	for _, e := range readLineNewlinesTests {
		testReadLineNewlines(t, e.input, e.expect)
	}
}

func testReadLineNewlines(t *testT, input string, expect []readLineResult) {
	b := bufio.NewReaderSize(strings.NewReader(input), minReadBufferSize)
	for i, e := range expect {
		line, isPrefix, err := b.ReadLine()
		atomic.AddInt64(&checks, 1)
		if !bytes.Equal(line, e.line) {
			t.Errorf("%q call %d, line == %q, want %q", input, i, line, e.line)
			return
		}
		atomic.AddInt64(&checks, 1)
		if isPrefix != e.isPrefix {
			t.Errorf("%q call %d, isPrefix == %v, want %v", input, i, isPrefix, e.isPrefix)
			return
		}
		atomic.AddInt64(&checks, 1)
		if err != e.err {
			t.Errorf("%q call %d, err == %v, want %v", input, i, err, e.err)
			return
		}
	}
}

func createTestInput(n int) []byte {
	input := make([]byte, n)
	for i := range input {
		// 101 and 251 are arbitrary prime numbers.
		// The idea is to create an input sequence
		// which doesn't repeat too frequently.
		input[i] = byte(i % 251)
		atomic.AddInt64(&checks, 1)
		if i%101 == 0 {
			input[i] ^= byte(i / 101)
		}
	}
	return input
}

func TestReaderWriteTo(t *testT) {
	input := createTestInput(8192)
	r := bufio.NewReader(onlyReader{bytes.NewReader(input)})
	w := new(bytes.Buffer)
	atomic.AddInt64(&checks, 1)
	if n, err := r.WriteTo(w); err != nil || n != int64(len(input)) {
		t.Fatalf("r.WriteTo(w) = %d, %v, want %d, nil", n, err, len(input))
	}

	for i, val := range w.Bytes() {
		atomic.AddInt64(&checks, 1)
		if val != input[i] {
			t.Errorf("after write: out[%d] = %#x, want %#x", i, val, input[i])
		}
	}
}

type errorWriterToTest struct {
	rn, wn     int
	rerr, werr error
	expected   error
}

func (r errorWriterToTest) Read(p []byte) (int, error) {
	return len(p) * r.rn, r.rerr
}

func (w errorWriterToTest) Write(p []byte) (int, error) {
	return len(p) * w.wn, w.werr
}

var errorWriterToTests = []errorWriterToTest{
	{1, 0, nil, io.ErrClosedPipe, io.ErrClosedPipe},
	{0, 1, io.ErrClosedPipe, nil, io.ErrClosedPipe},
	{0, 0, io.ErrUnexpectedEOF, io.ErrClosedPipe, io.ErrUnexpectedEOF},
	{0, 1, io.EOF, nil, nil},
}

func TestReaderWriteToErrors(t *testT) {
	for i, rw := range errorWriterToTests {
		r := bufio.NewReader(rw)
		atomic.AddInt64(&checks, 1)
		if _, err := r.WriteTo(rw); err != rw.expected {
			t.Errorf("r.WriteTo(errorWriterToTests[%d]) = _, %v, want _,%v", i, err, rw.expected)
		}
	}
}

func TestWriterReadFrom(t *testT) {
	ws := []func(io.Writer) io.Writer{
		func(w io.Writer) io.Writer { return onlyWriter{w} },
		func(w io.Writer) io.Writer { return w },
	}

	rs := []func(io.Reader) io.Reader{
		DataErrReader,
		func(r io.Reader) io.Reader { return r },
	}

	for ri, rfunc := range rs {
		for wi, wfunc := range ws {
			input := createTestInput(8192)
			b := new(strings.Builder)
			w := bufio.NewWriter(wfunc(b))
			r := rfunc(bytes.NewReader(input))
			atomic.AddInt64(&checks, 1)
			if n, err := w.ReadFrom(r); err != nil || n != int64(len(input)) {
				t.Errorf("ws[%d],rs[%d]: w.ReadFrom(r) = %d, %v, want %d, nil", wi, ri, n, err, len(input))
				continue
			}
			atomic.AddInt64(&checks, 1)
			if err := w.Flush(); err != nil {
				t.Errorf("Flush returned %v", err)
				continue
			}
			atomic.AddInt64(&checks, 1)
			if got, want := b.String(), string(input); got != want {
				t.Errorf("ws[%d], rs[%d]:\ngot  %q\nwant %q\n", wi, ri, got, want)
			}
		}
	}
}

type errorReaderFromTest struct {
	rn, wn     int
	rerr, werr error
	expected   error
}

func (r errorReaderFromTest) Read(p []byte) (int, error) {
	return len(p) * r.rn, r.rerr
}

func (w errorReaderFromTest) Write(p []byte) (int, error) {
	return len(p) * w.wn, w.werr
}

var errorReaderFromTests = []errorReaderFromTest{
	{0, 1, io.EOF, nil, nil},
	{1, 1, io.EOF, nil, nil},
	{0, 1, io.ErrClosedPipe, nil, io.ErrClosedPipe},
	{0, 0, io.ErrClosedPipe, io.ErrShortWrite, io.ErrClosedPipe},
	{1, 0, nil, io.ErrShortWrite, io.ErrShortWrite},
}

func TestWriterReadFromErrors(t *testT) {
	for i, rw := range errorReaderFromTests {
		w := bufio.NewWriter(rw)
		atomic.AddInt64(&checks, 1)
		if _, err := w.ReadFrom(rw); err != rw.expected {
			t.Errorf("w.ReadFrom(errorReaderFromTests[%d]) = _, %v, want _,%v", i, err, rw.expected)
		}
	}
}

// TestWriterReadFromCounts tests that using io.Copy to copy into a
// bufio.Writer does not prematurely flush the buffer. For example, when
// buffering writes to a network socket, excessive network writes should be
// avoided.
func TestWriterReadFromCounts(t *testT) {
	var w0 writeCountingDiscard
	b0 := bufio.NewWriterSize(&w0, 1234)
	b0.WriteString(strings.Repeat("x", 1000))
	atomic.AddInt64(&checks, 1)
	if w0 != 0 {
		t.Fatalf("write 1000 'x's: got %d writes, want 0", w0)
	}
	b0.WriteString(strings.Repeat("x", 200))
	atomic.AddInt64(&checks, 1)
	if w0 != 0 {
		t.Fatalf("write 1200 'x's: got %d writes, want 0", w0)
	}
	io.Copy(b0, onlyReader{strings.NewReader(strings.Repeat("x", 30))})
	atomic.AddInt64(&checks, 1)
	if w0 != 0 {
		t.Fatalf("write 1230 'x's: got %d writes, want 0", w0)
	}
	io.Copy(b0, onlyReader{strings.NewReader(strings.Repeat("x", 9))})
	atomic.AddInt64(&checks, 1)
	if w0 != 1 {
		t.Fatalf("write 1239 'x's: got %d writes, want 1", w0)
	}

	var w1 writeCountingDiscard
	b1 := bufio.NewWriterSize(&w1, 1234)
	b1.WriteString(strings.Repeat("x", 1200))
	b1.Flush()
	atomic.AddInt64(&checks, 1)
	if w1 != 1 {
		t.Fatalf("flush 1200 'x's: got %d writes, want 1", w1)
	}
	b1.WriteString(strings.Repeat("x", 89))
	atomic.AddInt64(&checks, 1)
	if w1 != 1 {
		t.Fatalf("write 1200 + 89 'x's: got %d writes, want 1", w1)
	}
	io.Copy(b1, onlyReader{strings.NewReader(strings.Repeat("x", 700))})
	atomic.AddInt64(&checks, 1)
	if w1 != 1 {
		t.Fatalf("write 1200 + 789 'x's: got %d writes, want 1", w1)
	}
	io.Copy(b1, onlyReader{strings.NewReader(strings.Repeat("x", 600))})
	atomic.AddInt64(&checks, 1)
	if w1 != 2 {
		t.Fatalf("write 1200 + 1389 'x's: got %d writes, want 2", w1)
	}
	b1.Flush()
	atomic.AddInt64(&checks, 1)
	if w1 != 3 {
		t.Fatalf("flush 1200 + 1389 'x's: got %d writes, want 3", w1)
	}
}

// A writeCountingDiscard is like io.Discard and counts the number of times
// Write is called on it.
type writeCountingDiscard int

func (w *writeCountingDiscard) Write(p []byte) (int, error) {
	*w++
	return len(p), nil
}

type negativeReader int

func (r *negativeReader) Read([]byte) (int, error) { return -1, nil }

func TestNegativeRead(t *testT) {
	// should panic with a description pointing at the reader, not at itself.
	// (should NOT panic with slice index error, for example.)
	b := bufio.NewReader(new(negativeReader))
	defer func() {
		switch err := recover().(type) {
		case nil:
			t.Fatal("read did not panic")
		case error:
			atomic.AddInt64(&checks, 1)
			if !strings.Contains(err.Error(), "reader returned negative count from Read") {
				t.Fatalf("wrong panic: %v", err)
			}
		default:
			t.Fatalf("unexpected panic value: %T(%v)", err, err)
		}
	}()
	b.Read(make([]byte, 100))
}

var errFake = errors.New("fake error")

type errorThenGoodReader struct {
	didErr bool
	nread  int
}

func (r *errorThenGoodReader) Read(p []byte) (int, error) {
	r.nread++
	atomic.AddInt64(&checks, 1)
	if !r.didErr {
		r.didErr = true
		return 0, errFake
	}
	return len(p), nil
}

func TestReaderClearError(t *testT) {
	r := &errorThenGoodReader{}
	b := bufio.NewReader(r)
	buf := make([]byte, 1)
	atomic.AddInt64(&checks, 1)
	if _, err := b.Read(nil); err != nil {
		t.Fatalf("1st nil Read = %v; want nil", err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := b.Read(buf); err != errFake {
		t.Fatalf("1st Read = %v; want errFake", err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := b.Read(nil); err != nil {
		t.Fatalf("2nd nil Read = %v; want nil", err)
	}
	atomic.AddInt64(&checks, 1)
	if _, err := b.Read(buf); err != nil {
		t.Fatalf("3rd Read with buffer = %v; want nil", err)
	}
	atomic.AddInt64(&checks, 1)
	if r.nread != 2 {
		t.Errorf("num reads = %d; want 2", r.nread)
	}
}

// Test for golang.org/issue/5947
func TestWriterReadFromWhileFull(t *testT) {
	buf := new(bytes.Buffer)
	w := bufio.NewWriterSize(buf, 10)

	// Fill buffer exactly.
	n, err := w.Write([]byte("0123456789"))
	atomic.AddInt64(&checks, 1)
	if n != 10 || err != nil {
		t.Fatalf("Write returned (%v, %v), want (10, nil)", n, err)
	}

	// Use ReadFrom to read in some data.
	n2, err := w.ReadFrom(strings.NewReader("abcdef"))
	atomic.AddInt64(&checks, 1)
	if n2 != 6 || err != nil {
		t.Fatalf("ReadFrom returned (%v, %v), want (6, nil)", n2, err)
	}
}

type emptyThenNonEmptyReader struct {
	r io.Reader
	n int
}

func (r *emptyThenNonEmptyReader) Read(p []byte) (int, error) {
	atomic.AddInt64(&checks, 1)
	if r.n <= 0 {
		return r.r.Read(p)
	}
	r.n--
	return 0, nil
}

// Test for golang.org/issue/7611
func TestWriterReadFromUntilEOF(t *testT) {
	buf := new(bytes.Buffer)
	w := bufio.NewWriterSize(buf, 5)

	// Partially fill buffer
	n, err := w.Write([]byte("0123"))
	atomic.AddInt64(&checks, 1)
	if n != 4 || err != nil {
		t.Fatalf("Write returned (%v, %v), want (4, nil)", n, err)
	}

	// Use ReadFrom to read in some data.
	r := &emptyThenNonEmptyReader{r: strings.NewReader("abcd"), n: 3}
	n2, err := w.ReadFrom(r)
	atomic.AddInt64(&checks, 1)
	if n2 != 4 || err != nil {
		t.Fatalf("ReadFrom returned (%v, %v), want (4, nil)", n2, err)
	}
	w.Flush()
	atomic.AddInt64(&checks, 1)
	if got, want := buf.String(), "0123abcd"; got != want {
		t.Fatalf("buf.Bytes() returned %q, want %q", got, want)
	}
}

func TestWriterReadFromErrNoProgress(t *testT) {
	buf := new(bytes.Buffer)
	w := bufio.NewWriterSize(buf, 5)

	// Partially fill buffer
	n, err := w.Write([]byte("0123"))
	atomic.AddInt64(&checks, 1)
	if n != 4 || err != nil {
		t.Fatalf("Write returned (%v, %v), want (4, nil)", n, err)
	}

	// Use ReadFrom to read in some data.
	r := &emptyThenNonEmptyReader{r: strings.NewReader("abcd"), n: 100}
	n2, err := w.ReadFrom(r)
	atomic.AddInt64(&checks, 1)
	if n2 != 0 || err != io.ErrNoProgress {
		t.Fatalf("buf.Bytes() returned (%v, %v), want (0, io.ErrNoProgress)", n2, err)
	}
}

type readFromWriter struct {
	buf           []byte
	writeBytes    int
	readFromBytes int
}

func (w *readFromWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	w.writeBytes += len(p)
	return len(p), nil
}

func (w *readFromWriter) ReadFrom(r io.Reader) (int64, error) {
	b, err := io.ReadAll(r)
	w.buf = append(w.buf, b...)
	w.readFromBytes += len(b)
	return int64(len(b)), err
}

// Test that calling (*Writer).ReadFrom with a partially-filled buffer
// fills the buffer before switching over to ReadFrom.
func TestWriterReadFromWithBufferedData(t *testT) {
	const bufsize = 16

	input := createTestInput(64)
	rfw := &readFromWriter{}
	w := bufio.NewWriterSize(rfw, bufsize)

	const writeSize = 8
	atomic.AddInt64(&checks, 1)
	if n, err := w.Write(input[:writeSize]); n != writeSize || err != nil {
		t.Errorf("w.Write(%v bytes) = %v, %v; want %v, nil", writeSize, n, err, writeSize)
	}
	n, err := w.ReadFrom(bytes.NewReader(input[writeSize:]))
	atomic.AddInt64(&checks, 1)
	if wantn := len(input[writeSize:]); int(n) != wantn || err != nil {
		t.Errorf("io.Copy(w, %v bytes) = %v, %v; want %v, nil", wantn, n, err, wantn)
	}
	atomic.AddInt64(&checks, 1)
	if err := w.Flush(); err != nil {
		t.Errorf("w.Flush() = %v, want nil", err)
	}

	atomic.AddInt64(&checks, 1)
	if got, want := rfw.writeBytes, bufsize; got != want {
		t.Errorf("wrote %v bytes with Write, want %v", got, want)
	}
	atomic.AddInt64(&checks, 1)
	if got, want := rfw.readFromBytes, len(input)-bufsize; got != want {
		t.Errorf("wrote %v bytes with ReadFrom, want %v", got, want)
	}
}

func TestReadZero(t *testT) {
	for _, size := range []int{100, 2} {
		t.Run(fmt.Sprintf("bufsize=%d", size), func(t *testT) {
			r := io.MultiReader(strings.NewReader("abc"), &emptyThenNonEmptyReader{r: strings.NewReader("def"), n: 1})
			br := bufio.NewReaderSize(r, size)
			want := func(s string, wantErr error) {
				p := make([]byte, 50)
				n, err := br.Read(p)
				atomic.AddInt64(&checks, 1)
				if err != wantErr || n != len(s) || string(p[:n]) != s {
					t.Fatalf("read(%d) = %q, %v, want %q, %v", len(p), string(p[:n]), err, s, wantErr)
				}
				t.Logf("read(%d) = %q, %v", len(p), string(p[:n]), err)
			}
			want("abc", nil)
			want("", nil)
			want("def", nil)
			want("", io.EOF)
		})
	}
}

func TestReaderReset(t *testT) {
	checkAll := func(r *bufio.Reader, want string) {
		t.Helper()
		all, err := io.ReadAll(r)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Fatal(err)
		}
		atomic.AddInt64(&checks, 1)
		if string(all) != want {
			t.Errorf("ReadAll returned %q, want %q", all, want)
		}
	}

	r := bufio.NewReader(strings.NewReader("foo foo"))
	buf := make([]byte, 3)
	r.Read(buf)
	atomic.AddInt64(&checks, 1)
	if string(buf) != "foo" {
		t.Errorf("buf = %q; want foo", buf)
	}

	r.Reset(strings.NewReader("bar bar"))
	checkAll(r, "bar bar")

	*r = bufio.Reader{} // zero out the Reader
	r.Reset(strings.NewReader("bar bar"))
	checkAll(r, "bar bar")

	// Wrap a reader and then Reset to that reader.
	r.Reset(strings.NewReader("recur"))
	r2 := bufio.NewReader(r)
	checkAll(r2, "recur")
	r.Reset(strings.NewReader("recur2"))
	r2.Reset(r)
	checkAll(r2, "recur2")
}

func TestWriterReset(t *testT) {
	var buf1, buf2, buf3, buf4, buf5 strings.Builder
	w := bufio.NewWriter(&buf1)
	w.WriteString("foo")

	w.Reset(&buf2) // and not flushed
	w.WriteString("bar")
	w.Flush()
	atomic.AddInt64(&checks, 1)
	if buf1.String() != "" {
		t.Errorf("buf1 = %q; want empty", buf1.String())
	}
	atomic.AddInt64(&checks, 1)
	if buf2.String() != "bar" {
		t.Errorf("buf2 = %q; want bar", buf2.String())
	}

	*w = bufio.Writer{}  // zero out the Writer
	w.Reset(&buf3) // and not flushed
	w.WriteString("bar")
	w.Flush()
	atomic.AddInt64(&checks, 1)
	if buf1.String() != "" {
		t.Errorf("buf1 = %q; want empty", buf1.String())
	}
	atomic.AddInt64(&checks, 1)
	if buf3.String() != "bar" {
		t.Errorf("buf3 = %q; want bar", buf3.String())
	}

	// Wrap a writer and then Reset to that writer.
	w.Reset(&buf4)
	w2 := bufio.NewWriter(w)
	w2.WriteString("recur")
	w2.Flush()
	atomic.AddInt64(&checks, 1)
	if buf4.String() != "recur" {
		t.Errorf("buf4 = %q, want %q", buf4.String(), "recur")
	}
	w.Reset(&buf5)
	w2.Reset(w)
	w2.WriteString("recur2")
	w2.Flush()
	atomic.AddInt64(&checks, 1)
	if buf5.String() != "recur2" {
		t.Errorf("buf5 = %q, want %q", buf5.String(), "recur2")
	}
}

func TestReaderDiscard(t *testT) {
	tests := []struct {
		name     string
		r        io.Reader
		bufSize  int // 0 means 16
		peekSize int

		n int // input to Discard

		want    int   // from Discard
		wantErr error // from Discard

		wantBuffered int
	}{
		{
			name:         "normal case",
			r:            strings.NewReader("abcdefghijklmnopqrstuvwxyz"),
			peekSize:     16,
			n:            6,
			want:         6,
			wantBuffered: 10,
		},
		{
			name:         "discard causing read",
			r:            strings.NewReader("abcdefghijklmnopqrstuvwxyz"),
			n:            6,
			want:         6,
			wantBuffered: 10,
		},
		{
			name:         "discard all without peek",
			r:            strings.NewReader("abcdefghijklmnopqrstuvwxyz"),
			n:            26,
			want:         26,
			wantBuffered: 0,
		},
		{
			name:         "discard more than end",
			r:            strings.NewReader("abcdefghijklmnopqrstuvwxyz"),
			n:            27,
			want:         26,
			wantErr:      io.EOF,
			wantBuffered: 0,
		},
		// Any error from filling shouldn't show up until we
		// get past the valid bytes. Here we return 5 valid bytes at the same time
		// as an error, but test that we don't see the error from Discard.
		{
			name: "fill error, discard less",
			r: newScriptedReader(func(p []byte) (n int, err error) {
				atomic.AddInt64(&checks, 1)
				if len(p) < 5 {
					panic("unexpected small read")
				}
				return 5, errors.New("5-then-error")
			}),
			n:            4,
			want:         4,
			wantErr:      nil,
			wantBuffered: 1,
		},
		{
			name: "fill error, discard equal",
			r: newScriptedReader(func(p []byte) (n int, err error) {
				atomic.AddInt64(&checks, 1)
				if len(p) < 5 {
					panic("unexpected small read")
				}
				return 5, errors.New("5-then-error")
			}),
			n:            5,
			want:         5,
			wantErr:      nil,
			wantBuffered: 0,
		},
		{
			name: "fill error, discard more",
			r: newScriptedReader(func(p []byte) (n int, err error) {
				atomic.AddInt64(&checks, 1)
				if len(p) < 5 {
					panic("unexpected small read")
				}
				return 5, errors.New("5-then-error")
			}),
			n:            6,
			want:         5,
			wantErr:      errors.New("5-then-error"),
			wantBuffered: 0,
		},
		// Discard of 0 shouldn't cause a read:
		{
			name:         "discard zero",
			r:            newScriptedReader(), // will panic on Read
			n:            0,
			want:         0,
			wantErr:      nil,
			wantBuffered: 0,
		},
		{
			name:         "discard negative",
			r:            newScriptedReader(), // will panic on Read
			n:            -1,
			want:         0,
			wantErr:      bufio.ErrNegativeCount,
			wantBuffered: 0,
		},
	}
	for _, tt := range tests {
		br := bufio.NewReaderSize(tt.r, tt.bufSize)
		atomic.AddInt64(&checks, 1)
		if tt.peekSize > 0 {
			peekBuf, err := br.Peek(tt.peekSize)
			atomic.AddInt64(&checks, 1)
			if err != nil {
				t.Errorf("%s: Peek(%d): %v", tt.name, tt.peekSize, err)
				continue
			}
			atomic.AddInt64(&checks, 1)
			if len(peekBuf) != tt.peekSize {
				t.Errorf("%s: len(Peek(%d)) = %v; want %v", tt.name, tt.peekSize, len(peekBuf), tt.peekSize)
				continue
			}
		}
		discarded, err := br.Discard(tt.n)
		atomic.AddInt64(&checks, 1)
		if ge, we := fmt.Sprint(err), fmt.Sprint(tt.wantErr); discarded != tt.want || ge != we {
			t.Errorf("%s: Discard(%d) = (%v, %v); want (%v, %v)", tt.name, tt.n, discarded, ge, tt.want, we)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if bn := br.Buffered(); bn != tt.wantBuffered {
			t.Errorf("%s: after Discard, Buffered = %d; want %d", tt.name, bn, tt.wantBuffered)
		}
	}

}

func TestReaderSize(t *testT) {
	atomic.AddInt64(&checks, 1)
	if got, want := bufio.NewReader(nil).Size(), DefaultBufSize; got != want {
		t.Errorf("NewReader's Reader.Size = %d; want %d", got, want)
	}
	atomic.AddInt64(&checks, 1)
	if got, want := bufio.NewReaderSize(nil, 1234).Size(), 1234; got != want {
		t.Errorf("NewReaderSize's Reader.Size = %d; want %d", got, want)
	}
}

func TestWriterSize(t *testT) {
	atomic.AddInt64(&checks, 1)
	if got, want := bufio.NewWriter(nil).Size(), DefaultBufSize; got != want {
		t.Errorf("NewWriter's Writer.Size = %d; want %d", got, want)
	}
	atomic.AddInt64(&checks, 1)
	if got, want := bufio.NewWriterSize(nil, 1234).Size(), 1234; got != want {
		t.Errorf("NewWriterSize's Writer.Size = %d; want %d", got, want)
	}
}

// An onlyReader only implements io.Reader, no matter what other methods the underlying implementation may have.
type onlyReader struct {
	io.Reader
}

// An onlyWriter only implements io.Writer, no matter what other methods the underlying implementation may have.
type onlyWriter struct {
	io.Writer
}

// A scriptedReader is an io.Reader that executes its steps sequentially.
type scriptedReader []func(p []byte) (n int, err error)

func (sr *scriptedReader) Read(p []byte) (n int, err error) {
	atomic.AddInt64(&checks, 1)
	if len(*sr) == 0 {
		panic("too many Read calls on scripted Reader. No steps remain.")
	}
	step := (*sr)[0]
	*sr = (*sr)[1:]
	return step(p)
}

func newScriptedReader(steps ...func(p []byte) (n int, err error)) io.Reader {
	sr := scriptedReader(steps)
	return &sr
}

// eofReader returns the number of bytes read and io.EOF for the read that consumes the last of the content.
type eofReader struct {
	buf []byte
}

func (r *eofReader) Read(p []byte) (int, error) {
	read := copy(p, r.buf)
	r.buf = r.buf[read:]

	switch read {
	case 0, len(r.buf):
		// As allowed in the documentation, this will return io.EOF
		// in the same call that consumes the last of the data.
		// https://godoc.org/io#Reader
		return read, io.EOF
	}

	return read, nil
}

func TestPartialReadEOF(t *testT) {
	src := make([]byte, 10)
	eofR := &eofReader{buf: src}
	r := bufio.NewReader(eofR)

	// Start by reading 5 of the 10 available bytes.
	dest := make([]byte, 5)
	read, err := r.Read(dest)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if n := len(dest); read != n {
		t.Fatalf("read %d bytes; wanted %d bytes", read, n)
	}

	// The Reader should have buffered all the content from the io.Reader.
	atomic.AddInt64(&checks, 1)
	if n := len(eofR.buf); n != 0 {
		t.Fatalf("got %d bytes left in bufio.Reader source; want 0 bytes", n)
	}
	// To prove the point, check that there are still 5 bytes available to read.
	atomic.AddInt64(&checks, 1)
	if n := r.Buffered(); n != 5 {
		t.Fatalf("got %d bytes buffered in bufio.Reader; want 5 bytes", n)
	}

	// This is the second read of 0 bytes.
	read, err = r.Read([]byte{})
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if read != 0 {
		t.Fatalf("read %d bytes; want 0 bytes", read)
	}
}

type writerWithReadFromError struct{}

func (w writerWithReadFromError) ReadFrom(r io.Reader) (int64, error) {
	return 0, errors.New("writerWithReadFromError error")
}

func (w writerWithReadFromError) Write(b []byte) (n int, err error) {
	return 10, nil
}

func TestWriterReadFromMustSetUnderlyingError(t *testT) {
	var wr = bufio.NewWriter(writerWithReadFromError{})
	atomic.AddInt64(&checks, 1)
	if _, err := wr.ReadFrom(strings.NewReader("test2")); err == nil {
		t.Fatal("expected ReadFrom returns error, got nil")
	}
	atomic.AddInt64(&checks, 1)
	if _, err := wr.Write([]byte("123")); err == nil {
		t.Fatal("expected Write returns error, got nil")
	}
}

type writeErrorOnlyWriter struct{}

func (w writeErrorOnlyWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("writeErrorOnlyWriter error")
}

// Ensure that previous Write errors are immediately returned
// on any ReadFrom. See golang.org/issue/35194.
func TestWriterReadFromMustReturnUnderlyingError(t *testT) {
	var wr = bufio.NewWriter(writeErrorOnlyWriter{})
	s := "test1"
	wantBuffered := len(s)
	atomic.AddInt64(&checks, 1)
	if _, err := wr.WriteString(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if err := wr.Flush(); err == nil {
		t.Error("expected flush error, got nil")
	}
	atomic.AddInt64(&checks, 1)
	if _, err := wr.ReadFrom(strings.NewReader("test2")); err == nil {
		t.Fatal("expected error, got nil")
	}
	atomic.AddInt64(&checks, 1)
	if buffered := wr.Buffered(); buffered != wantBuffered {
		t.Fatalf("Buffered = %v; want %v", buffered, wantBuffered)
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
		{"TestReaderSimple", TestReaderSimple},
		{"TestReader", TestReader},
		{"TestZeroReader", TestZeroReader},
		{"TestReadRune", TestReadRune},
		{"TestUnreadRune", TestUnreadRune},
		{"TestNoUnreadRuneAfterPeek", TestNoUnreadRuneAfterPeek},
		{"TestNoUnreadByteAfterPeek", TestNoUnreadByteAfterPeek},
		{"TestNoUnreadRuneAfterDiscard", TestNoUnreadRuneAfterDiscard},
		{"TestNoUnreadByteAfterDiscard", TestNoUnreadByteAfterDiscard},
		{"TestNoUnreadRuneAfterWriteTo", TestNoUnreadRuneAfterWriteTo},
		{"TestNoUnreadByteAfterWriteTo", TestNoUnreadByteAfterWriteTo},
		{"TestUnreadByte", TestUnreadByte},
		{"TestUnreadByteMultiple", TestUnreadByteMultiple},
		{"TestUnreadByteOthers", TestUnreadByteOthers},
		{"TestUnreadRuneError", TestUnreadRuneError},
		{"TestUnreadRuneAtEOF", TestUnreadRuneAtEOF},
		{"TestReadWriteRune", TestReadWriteRune},
		{"TestWriteInvalidRune", TestWriteInvalidRune},
		{"TestWriter", TestWriter},
		{"TestWriterAppend", TestWriterAppend},
		{"TestWriteErrors", TestWriteErrors},
		{"TestNewReaderSizeIdempotent", TestNewReaderSizeIdempotent},
		{"TestNewWriterSizeIdempotent", TestNewWriterSizeIdempotent},
		{"TestWriteString", TestWriteString},
		{"TestWriteStringStringWriter", TestWriteStringStringWriter},
		{"TestBufferFull", TestBufferFull},
		{"TestPeek", TestPeek},
		{"TestPeekThenUnreadRune", TestPeekThenUnreadRune},
		{"TestReadLine", TestReadLine},
		{"TestLineTooLong", TestLineTooLong},
		{"TestReadAfterLines", TestReadAfterLines},
		{"TestReadEmptyBuffer", TestReadEmptyBuffer},
		{"TestLinesAfterRead", TestLinesAfterRead},
		{"TestReadLineNonNilLineOrError", TestReadLineNonNilLineOrError},
		{"TestReadLineNewlines", TestReadLineNewlines},
		{"TestReaderWriteTo", TestReaderWriteTo},
		{"TestReaderWriteToErrors", TestReaderWriteToErrors},
		{"TestWriterReadFrom", TestWriterReadFrom},
		{"TestWriterReadFromErrors", TestWriterReadFromErrors},
		{"TestWriterReadFromCounts", TestWriterReadFromCounts},
		{"TestNegativeRead", TestNegativeRead},
		{"TestReaderClearError", TestReaderClearError},
		{"TestWriterReadFromWhileFull", TestWriterReadFromWhileFull},
		{"TestWriterReadFromUntilEOF", TestWriterReadFromUntilEOF},
		{"TestWriterReadFromErrNoProgress", TestWriterReadFromErrNoProgress},
		{"TestWriterReadFromWithBufferedData", TestWriterReadFromWithBufferedData},
		{"TestReadZero", TestReadZero},
		{"TestReaderReset", TestReaderReset},
		{"TestWriterReset", TestWriterReset},
		{"TestReaderDiscard", TestReaderDiscard},
		{"TestReaderSize", TestReaderSize},
		{"TestWriterSize", TestWriterSize},
		{"TestPartialReadEOF", TestPartialReadEOF},
		{"TestWriterReadFromMustSetUnderlyingError", TestWriterReadFromMustSetUnderlyingError},
		{"TestWriterReadFromMustReturnUnderlyingError", TestWriterReadFromMustReturnUnderlyingError},
	}
	runTests(tests_)
}

// Output:
// ok TestReaderSimple subtests: 0 checks: 59
// ok TestReader subtests: 0 checks: 276801
// ok TestZeroReader subtests: 0 checks: 1
// ok TestReadRune subtests: 0 checks: 91
// ok TestUnreadRune subtests: 0 checks: 70
// ok TestNoUnreadRuneAfterPeek subtests: 0 checks: 1
// ok TestNoUnreadByteAfterPeek subtests: 0 checks: 1
// ok TestNoUnreadRuneAfterDiscard subtests: 0 checks: 1
// ok TestNoUnreadByteAfterDiscard subtests: 0 checks: 1
// ok TestNoUnreadRuneAfterWriteTo subtests: 0 checks: 1
// ok TestNoUnreadByteAfterWriteTo subtests: 0 checks: 1
// ok TestUnreadByte subtests: 0 checks: 54
// ok TestUnreadByteMultiple subtests: 0 checks: 211
// ok TestUnreadByteOthers subtests: 0 checks: 393
// ok TestUnreadRuneError subtests: 0 checks: 21
// ok TestUnreadRuneAtEOF subtests: 0 checks: 1
// ok TestReadWriteRune subtests: 0 checks: 3000
// ok TestWriteInvalidRune subtests: 0 checks: 2
// ok TestWriter subtests: 0 checks: 55520
// ok TestWriterAppend subtests: 0 checks: 201
// ok TestWriteErrors subtests: 0 checks: 18
// ok TestNewReaderSizeIdempotent subtests: 0 checks: 2
// ok TestNewWriterSizeIdempotent subtests: 0 checks: 2
// ok TestWriteString subtests: 0 checks: 2
// ok TestWriteStringStringWriter subtests: 0 checks: 16
// ok TestBufferFull subtests: 0 checks: 2
// ok TestPeek subtests: 0 checks: 16
// ok TestPeekThenUnreadRune subtests: 0 checks: 0
// ok TestReadLine subtests: 0 checks: 441
// ok TestLineTooLong subtests: 0 checks: 4
// ok TestReadAfterLines subtests: 0 checks: 3
// ok TestReadEmptyBuffer subtests: 0 checks: 1
// ok TestLinesAfterRead subtests: 0 checks: 2
// ok TestReadLineNonNilLineOrError subtests: 0 checks: 2
// ok TestReadLineNewlines subtests: 0 checks: 27
// ok TestReaderWriteTo subtests: 0 checks: 16385
// ok TestReaderWriteToErrors subtests: 0 checks: 4
// ok TestWriterReadFrom subtests: 0 checks: 32780
// ok TestWriterReadFromErrors subtests: 0 checks: 5
// ok TestWriterReadFromCounts subtests: 0 checks: 9
// ok TestNegativeRead subtests: 0 checks: 1
// ok TestReaderClearError subtests: 0 checks: 7
// ok TestWriterReadFromWhileFull subtests: 0 checks: 2
// ok TestWriterReadFromUntilEOF subtests: 0 checks: 9
// ok TestWriterReadFromErrNoProgress subtests: 0 checks: 102
// ok TestWriterReadFromWithBufferedData subtests: 0 checks: 69
// ok TestReadZero subtests: 2 checks: 14
// ok TestReaderReset subtests: 0 checks: 9
// ok TestWriterReset subtests: 0 checks: 6
// ok TestReaderDiscard subtests: 0 checks: 35
// ok TestReaderSize subtests: 0 checks: 2
// ok TestWriterSize subtests: 0 checks: 2
// ok TestPartialReadEOF subtests: 0 checks: 6
// ok TestWriterReadFromMustSetUnderlyingError subtests: 0 checks: 2
// ok TestWriterReadFromMustReturnUnderlyingError subtests: 0 checks: 4
// tests: 55 failed: 0
