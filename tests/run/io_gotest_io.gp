// phase: 09
// Go 1.25's io/io_test.go (Copy, CopyN, CopyBuffer, WriterTo/ReaderFrom shortcuts, TeeReader, SectionReader, OffsetWriter, ReadAtLeast, ReadFull, WriteString, Discard, NopCloser, MultiWriter, LimitReader, the panics of negative reads) through a small harness; expected output from real Go.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)


// io's export_test.go: the three error values of the package, taken from the package itself (their identity is what the tests compare).
var ErrInvalidWrite, ErrWhence, ErrOffset = ioErrors()

type badWriter struct{}

func (badWriter) Write(p []byte) (int, error) { return len(p) + 1, nil }

func ioErrors() (a, b, c error) {
	_, a = io.Copy(badWriter{}, io.LimitReader(strings.NewReader("x"), 1))
	r := io.NewSectionReader(strings.NewReader("abc"), 0, 3)
	_, b = r.Seek(0, 99)
	_, c = r.Seek(-1, io.SeekStart)
	return
}

func (t *testT) TempDir() string {
	d, err := os.MkdirTemp("", "iotest")
	if err != nil {
		panic(err)
	}
	return d
}

// A version of bytes.Buffer without ReadFrom and WriteTo
type Buffer struct {
	bytes.Buffer
	io.ReaderFrom // conflicts with and hides bytes.Buffer's ReaderFrom.
	io.WriterTo   // conflicts with and hides bytes.Buffer's WriterTo.
}

// Simple tests, primarily to verify the ReadFrom and WriteTo callouts inside Copy, CopyBuffer and CopyN.

func TestCopy(t *testT) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	io.Copy(wb, rb)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	}
}

func TestCopyNegative(t *testT) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello")
	io.Copy(wb, &io.LimitedReader{R: rb, N: -1})
	atomic.AddInt64(&checks, 1)
	if wb.String() != "" {
		t.Errorf("Copy on LimitedReader with N<0 copied data")
	}

	io.CopyN(wb, rb, -1)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "" {
		t.Errorf("CopyN with N<0 copied data")
	}
}

func TestCopyBuffer(t *testT) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	io.CopyBuffer(wb, rb, make([]byte, 1)) // Tiny buffer to keep it honest.
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello, world." {
		t.Errorf("CopyBuffer did not work properly")
	}
}

func TestCopyBufferNil(t *testT) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	io.CopyBuffer(wb, rb, nil) // Should allocate a buffer.
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello, world." {
		t.Errorf("CopyBuffer did not work properly")
	}
}

func TestCopyReadFrom(t *testT) {
	rb := new(Buffer)
	wb := new(bytes.Buffer) // implements ReadFrom.
	rb.WriteString("hello, world.")
	io.Copy(wb, rb)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	}
}

func TestCopyWriteTo(t *testT) {
	rb := new(bytes.Buffer) // implements WriteTo.
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	io.Copy(wb, rb)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	}
}

// Version of bytes.Buffer that checks whether WriteTo was called or not
type writeToChecker struct {
	bytes.Buffer
	writeToCalled bool
}

func (wt *writeToChecker) WriteTo(w io.Writer) (int64, error) {
	wt.writeToCalled = true
	return wt.Buffer.WriteTo(w)
}

// It's preferable to choose WriterTo over ReaderFrom, since a WriterTo can issue one large write,
// while the ReaderFrom must read until EOF, potentially allocating when running out of buffer.
// Make sure that we choose WriterTo when both are implemented.
func TestCopyPriority(t *testT) {
	rb := new(writeToChecker)
	wb := new(bytes.Buffer)
	rb.WriteString("hello, world.")
	io.Copy(wb, rb)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	} else if !rb.writeToCalled {
		t.Errorf("WriteTo was not prioritized over ReadFrom")
	}
}

type zeroErrReader struct {
	err error
}

func (r zeroErrReader) Read(p []byte) (int, error) {
	return copy(p, []byte{0}), r.err
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

// In case a Read results in an error with non-zero bytes read, and
// the subsequent Write also results in an error, the error from Write
// is returned, as it is the one that prevented progressing further.
func TestCopyReadErrWriteErr(t *testT) {
	er, ew := errors.New("readError"), errors.New("writeError")
	r, w := zeroErrReader{err: er}, errWriter{err: ew}
	n, err := io.Copy(w, r)
	atomic.AddInt64(&checks, 1)
	if n != 0 || err != ew {
		t.Errorf("Copy(zeroErrReader, errWriter) = %d, %v; want 0, writeError", n, err)
	}
}

func TestCopyN(t *testT) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	io.CopyN(wb, rb, 5)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello" {
		t.Errorf("CopyN did not work properly")
	}
}

func TestCopyNReadFrom(t *testT) {
	rb := new(Buffer)
	wb := new(bytes.Buffer) // implements ReadFrom.
	rb.WriteString("hello")
	io.CopyN(wb, rb, 5)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello" {
		t.Errorf("CopyN did not work properly")
	}
}

func TestCopyNWriteTo(t *testT) {
	rb := new(bytes.Buffer) // implements WriteTo.
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	io.CopyN(wb, rb, 5)
	atomic.AddInt64(&checks, 1)
	if wb.String() != "hello" {
		t.Errorf("CopyN did not work properly")
	}
}



type noReadFrom struct {
	w io.Writer
}

func (w *noReadFrom) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

type wantedAndErrReader struct{}

func (wantedAndErrReader) Read(p []byte) (int, error) {
	return len(p), errors.New("wantedAndErrReader error")
}

func TestCopyNEOF(t *testT) {
	// Test that EOF behavior is the same regardless of whether
	// argument to CopyN has ReadFrom.

	b := new(bytes.Buffer)

	n, err := io.CopyN(&noReadFrom{b}, strings.NewReader("foo"), 3)
	atomic.AddInt64(&checks, 1)
	if n != 3 || err != nil {
		t.Errorf("CopyN(noReadFrom, foo, 3) = %d, %v; want 3, nil", n, err)
	}

	n, err = io.CopyN(&noReadFrom{b}, strings.NewReader("foo"), 4)
	atomic.AddInt64(&checks, 1)
	if n != 3 || err != io.EOF {
		t.Errorf("CopyN(noReadFrom, foo, 4) = %d, %v; want 3, EOF", n, err)
	}

	n, err = io.CopyN(b, strings.NewReader("foo"), 3) // b has read from
	atomic.AddInt64(&checks, 1)
	if n != 3 || err != nil {
		t.Errorf("CopyN(bytes.Buffer, foo, 3) = %d, %v; want 3, nil", n, err)
	}

	n, err = io.CopyN(b, strings.NewReader("foo"), 4) // b has read from
	atomic.AddInt64(&checks, 1)
	if n != 3 || err != io.EOF {
		t.Errorf("CopyN(bytes.Buffer, foo, 4) = %d, %v; want 3, EOF", n, err)
	}

	n, err = io.CopyN(b, wantedAndErrReader{}, 5)
	atomic.AddInt64(&checks, 1)
	if n != 5 || err != nil {
		t.Errorf("CopyN(bytes.Buffer, wantedAndErrReader, 5) = %d, %v; want 5, nil", n, err)
	}

	n, err = io.CopyN(&noReadFrom{b}, wantedAndErrReader{}, 5)
	atomic.AddInt64(&checks, 1)
	if n != 5 || err != nil {
		t.Errorf("CopyN(noReadFrom, wantedAndErrReader, 5) = %d, %v; want 5, nil", n, err)
	}
}

func TestReadAtLeast(t *testT) {
	var rb bytes.Buffer
	testReadAtLeast(t, &rb)
}

// A version of bytes.Buffer that returns n > 0, err on Read
// when the input is exhausted.
type dataAndErrorBuffer struct {
	err error
	bytes.Buffer
}

func (r *dataAndErrorBuffer) Read(p []byte) (n int, err error) {
	n, err = r.Buffer.Read(p)
	atomic.AddInt64(&checks, 1)
	if n > 0 && r.Buffer.Len() == 0 && err == nil {
		err = r.err
	}
	return
}

func TestReadAtLeastWithDataAndEOF(t *testT) {
	var rb dataAndErrorBuffer
	rb.err = io.EOF
	testReadAtLeast(t, &rb)
}

func TestReadAtLeastWithDataAndError(t *testT) {
	var rb dataAndErrorBuffer
	rb.err = fmt.Errorf("fake error")
	testReadAtLeast(t, &rb)
}

func testReadAtLeast(t *testT, rb io.ReadWriter) {
	rb.Write([]byte("0123"))
	buf := make([]byte, 2)
	n, err := io.ReadAtLeast(rb, buf, 2)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error(err)
	}
	atomic.AddInt64(&checks, 1)
	if n != 2 {
		t.Errorf("expected to have read 2 bytes, got %v", n)
	}
	n, err = io.ReadAtLeast(rb, buf, 4)
	atomic.AddInt64(&checks, 1)
	if err != io.ErrShortBuffer {
		t.Errorf("expected ErrShortBuffer got %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if n != 0 {
		t.Errorf("expected to have read 0 bytes, got %v", n)
	}
	n, err = io.ReadAtLeast(rb, buf, 1)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Error(err)
	}
	atomic.AddInt64(&checks, 1)
	if n != 2 {
		t.Errorf("expected to have read 2 bytes, got %v", n)
	}
	n, err = io.ReadAtLeast(rb, buf, 2)
	atomic.AddInt64(&checks, 1)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if n != 0 {
		t.Errorf("expected to have read 0 bytes, got %v", n)
	}
	rb.Write([]byte("4"))
	n, err = io.ReadAtLeast(rb, buf, 2)
	want := io.ErrUnexpectedEOF
	atomic.AddInt64(&checks, 1)
	if rb, ok := rb.(*dataAndErrorBuffer); ok && rb.err != io.EOF {
		want = rb.err
	}
	atomic.AddInt64(&checks, 1)
	if err != want {
		t.Errorf("expected %v, got %v", want, err)
	}
	atomic.AddInt64(&checks, 1)
	if n != 1 {
		t.Errorf("expected to have read 1 bytes, got %v", n)
	}
}

func TestTeeReader(t *testT) {
	src := []byte("hello, world")
	dst := make([]byte, len(src))
	rb := bytes.NewBuffer(src)
	wb := new(bytes.Buffer)
	r := io.TeeReader(rb, wb)
	atomic.AddInt64(&checks, 1)
	if n, err := io.ReadFull(r, dst); err != nil || n != len(src) {
		t.Fatalf("ReadFull(r, dst) = %d, %v; want %d, nil", n, err, len(src))
	}
	atomic.AddInt64(&checks, 1)
	if !bytes.Equal(dst, src) {
		t.Errorf("bytes read = %q want %q", dst, src)
	}
	atomic.AddInt64(&checks, 1)
	if !bytes.Equal(wb.Bytes(), src) {
		t.Errorf("bytes written = %q want %q", wb.Bytes(), src)
	}
	atomic.AddInt64(&checks, 1)
	if n, err := r.Read(dst); n != 0 || err != io.EOF {
		t.Errorf("r.Read at EOF = %d, %v want 0, EOF", n, err)
	}
	rb = bytes.NewBuffer(src)
	pr, pw := io.Pipe()
	pr.Close()
	r = io.TeeReader(rb, pw)
	atomic.AddInt64(&checks, 1)
	if n, err := io.ReadFull(r, dst); n != 0 || err != io.ErrClosedPipe {
		t.Errorf("closed tee: ReadFull(r, dst) = %d, %v; want 0, EPIPE", n, err)
	}
}

func TestSectionReader_ReadAt(t *testT) {
	dat := "a long sample data, 1234567890"
	tests := []struct {
		data   string
		off    int
		n      int
		bufLen int
		at     int
		exp    string
		err    error
	}{
		{data: "", off: 0, n: 10, bufLen: 2, at: 0, exp: "", err: io.EOF},
		{data: dat, off: 0, n: len(dat), bufLen: 0, at: 0, exp: "", err: nil},
		{data: dat, off: len(dat), n: 1, bufLen: 1, at: 0, exp: "", err: io.EOF},
		{data: dat, off: 0, n: len(dat) + 2, bufLen: len(dat), at: 0, exp: dat, err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: len(dat) / 2, at: 0, exp: dat[:len(dat)/2], err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: len(dat), at: 0, exp: dat, err: nil},
		{data: dat, off: 0, n: len(dat), bufLen: len(dat) / 2, at: 2, exp: dat[2 : 2+len(dat)/2], err: nil},
		{data: dat, off: 3, n: len(dat), bufLen: len(dat) / 2, at: 2, exp: dat[5 : 5+len(dat)/2], err: nil},
		{data: dat, off: 3, n: len(dat) / 2, bufLen: len(dat)/2 - 2, at: 2, exp: dat[5 : 5+len(dat)/2-2], err: nil},
		{data: dat, off: 3, n: len(dat) / 2, bufLen: len(dat)/2 + 2, at: 2, exp: dat[5 : 5+len(dat)/2-2], err: io.EOF},
		{data: dat, off: 0, n: 0, bufLen: 0, at: -1, exp: "", err: io.EOF},
		{data: dat, off: 0, n: 0, bufLen: 0, at: 1, exp: "", err: io.EOF},
	}
	for i, tt := range tests {
		r := strings.NewReader(tt.data)
		s := io.NewSectionReader(r, int64(tt.off), int64(tt.n))
		buf := make([]byte, tt.bufLen)
		atomic.AddInt64(&checks, 1)
		if n, err := s.ReadAt(buf, int64(tt.at)); n != len(tt.exp) || string(buf[:n]) != tt.exp || err != tt.err {
			t.Fatalf("%d: ReadAt(%d) = %q, %v; expected %q, %v", i, tt.at, buf[:n], err, tt.exp, tt.err)
		}
		atomic.AddInt64(&checks, 1)
		if _r, off, n := s.Outer(); _r != r || off != int64(tt.off) || n != int64(tt.n) {
			t.Fatalf("%d: Outer() = %v, %d, %d; expected %v, %d, %d", i, _r, off, n, r, tt.off, tt.n)
		}
	}
}

func TestSectionReader_Seek(t *testT) {
	// Verifies that NewSectionReader's Seeker behaves like bytes.NewReader (which is like strings.NewReader)
	br := bytes.NewReader([]byte("foo"))
	sr := io.NewSectionReader(br, 0, int64(len("foo")))

	for _, whence := range []int{io.SeekStart, io.SeekCurrent, io.SeekEnd} {
		for offset := int64(-3); offset <= 4; offset++ {
			brOff, brErr := br.Seek(offset, whence)
			srOff, srErr := sr.Seek(offset, whence)
			atomic.AddInt64(&checks, 1)
			if (brErr != nil) != (srErr != nil) || brOff != srOff {
				t.Errorf("For whence %d, offset %d: bytes.Reader.Seek = (%v, %v) != SectionReader.Seek = (%v, %v)",
					whence, offset, brOff, brErr, srErr, srOff)
			}
		}
	}

	// And verify we can just seek past the end and get an EOF
	got, err := sr.Seek(100, io.SeekStart)
	atomic.AddInt64(&checks, 1)
	if err != nil || got != 100 {
		t.Errorf("Seek = %v, %v; want 100, nil", got, err)
	}

	n, err := sr.Read(make([]byte, 10))
	atomic.AddInt64(&checks, 1)
	if n != 0 || err != io.EOF {
		t.Errorf("Read = %v, %v; want 0, EOF", n, err)
	}
}

func TestSectionReader_Size(t *testT) {
	tests := []struct {
		data string
		want int64
	}{
		{"a long sample data, 1234567890", 30},
		{"", 0},
	}

	for _, tt := range tests {
		r := strings.NewReader(tt.data)
		sr := io.NewSectionReader(r, 0, int64(len(tt.data)))
		atomic.AddInt64(&checks, 1)
		if got := sr.Size(); got != tt.want {
			t.Errorf("Size = %v; want %v", got, tt.want)
		}
	}
}

func TestSectionReader_Max(t *testT) {
	r := strings.NewReader("abcdef")
	const maxint64 = 1<<63 - 1
	sr := io.NewSectionReader(r, 3, maxint64)
	n, err := sr.Read(make([]byte, 3))
	atomic.AddInt64(&checks, 1)
	if n != 3 || err != nil {
		t.Errorf("Read = %v %v, want 3, nil", n, err)
	}
	n, err = sr.Read(make([]byte, 3))
	atomic.AddInt64(&checks, 1)
	if n != 0 || err != io.EOF {
		t.Errorf("Read = %v, %v, want 0, EOF", n, err)
	}
	atomic.AddInt64(&checks, 1)
	if _r, off, n := sr.Outer(); _r != r || off != 3 || n != maxint64 {
		t.Fatalf("Outer = %v, %d, %d; expected %v, %d, %d", _r, off, n, r, 3, int64(maxint64))
	}
}

// largeWriter returns an invalid count that is larger than the number
// of bytes provided (issue 39978).
type largeWriter struct {
	err error
}

func (w largeWriter) Write(p []byte) (int, error) {
	return len(p) + 1, w.err
}

func TestCopyLargeWriter(t *testT) {
	want := ErrInvalidWrite
	rb := new(Buffer)
	wb := largeWriter{}
	rb.WriteString("hello, world.")
	atomic.AddInt64(&checks, 1)
	if _, err := io.Copy(wb, rb); err != want {
		t.Errorf("Copy error: got %v, want %v", err, want)
	}

	want = errors.New("largeWriterError")
	rb = new(Buffer)
	wb = largeWriter{err: want}
	rb.WriteString("hello, world.")
	atomic.AddInt64(&checks, 1)
	if _, err := io.Copy(wb, rb); err != want {
		t.Errorf("Copy error: got %v, want %v", err, want)
	}
}

func TestNopCloserWriterToForwarding(t *testT) {
	for _, tc := range [...]struct {
		Name string
		r    io.Reader
	}{
		{"not a WriterTo", io.Reader(nil)},
		{"a WriterTo", struct {
			io.Reader
			io.WriterTo
		}{}},
	} {
		nc := io.NopCloser(tc.r)

		_, expected := tc.r.(io.WriterTo)
		_, got := nc.(io.WriterTo)
		atomic.AddInt64(&checks, 1)
		if expected != got {
			t.Errorf("NopCloser incorrectly forwards WriterTo for %s, got %t want %t", tc.Name, got, expected)
		}
	}
}

func TestOffsetWriter_Seek(t *testT) {
	tmpfilename := "TestOffsetWriter_Seek"
	tmpfile, err := os.CreateTemp(t.TempDir(), tmpfilename)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatalf("CreateTemp(%s) failed: %v", tmpfilename, err)
	}
	defer tmpfile.Close()
	w := io.NewOffsetWriter(tmpfile, 0)

	// Should throw error errWhence if whence is not valid
	t.Run("errWhence", func(t *testT) {
		for _, whence := range []int{-3, -2, -1, 3, 4, 5} {
			var offset int64 = 0
			gotOff, gotErr := w.Seek(offset, whence)
			atomic.AddInt64(&checks, 1)
			if gotOff != 0 || gotErr != ErrWhence {
				t.Errorf("For whence %d, offset %d, OffsetWriter.Seek got: (%d, %v), want: (%d, %v)",
					whence, offset, gotOff, gotErr, 0, ErrWhence)
			}
		}
	})

	// Should throw error errOffset if offset is negative
	t.Run("errOffset", func(t *testT) {
		for _, whence := range []int{io.SeekStart, io.SeekCurrent} {
			for offset := int64(-3); offset < 0; offset++ {
				gotOff, gotErr := w.Seek(offset, whence)
				atomic.AddInt64(&checks, 1)
				if gotOff != 0 || gotErr != ErrOffset {
					t.Errorf("For whence %d, offset %d, OffsetWriter.Seek got: (%d, %v), want: (%d, %v)",
						whence, offset, gotOff, gotErr, 0, ErrOffset)
				}
			}
		}
	})

	// Normal tests
	t.Run("normal", func(t *testT) {
		tests := []struct {
			offset    int64
			whence    int
			returnOff int64
		}{
			// keep in order
			{whence: io.SeekStart, offset: 1, returnOff: 1},
			{whence: io.SeekStart, offset: 2, returnOff: 2},
			{whence: io.SeekStart, offset: 3, returnOff: 3},
			{whence: io.SeekCurrent, offset: 1, returnOff: 4},
			{whence: io.SeekCurrent, offset: 2, returnOff: 6},
			{whence: io.SeekCurrent, offset: 3, returnOff: 9},
		}
		for idx, tt := range tests {
			gotOff, gotErr := w.Seek(tt.offset, tt.whence)
			atomic.AddInt64(&checks, 1)
			if gotOff != tt.returnOff || gotErr != nil {
				t.Errorf("%d:: For whence %d, offset %d, OffsetWriter.Seek got: (%d, %v), want: (%d, <nil>)",
					idx+1, tt.whence, tt.offset, gotOff, gotErr, tt.returnOff)
			}
		}
	})
}

func TestOffsetWriter_WriteAt(t *testT) {
	defer func() { atomic.StoreInt64(&checks, -1) }() // goroutines count the checks: the number is not deterministic
	const content = "0123456789ABCDEF"
	contentSize := int64(len(content))
	tmpdir := t.TempDir()

	work := func(off, at int64) {
		position := fmt.Sprintf("off_%d_at_%d", off, at)
		tmpfile, err := os.CreateTemp(tmpdir, position)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Fatalf("CreateTemp(%s) failed: %v", position, err)
		}
		defer tmpfile.Close()

		var writeN int64
		var wg sync.WaitGroup
		// Concurrent writes, one byte at a time
		for step, value := range []byte(content) {
			wg.Add(1)
			go func(wg *sync.WaitGroup, tmpfile *os.File, value byte, off, at int64, step int) {
				defer wg.Done()

				w := io.NewOffsetWriter(tmpfile, off)
				n, e := w.WriteAt([]byte{value}, at+int64(step))
				atomic.AddInt64(&checks, 1)
				if e != nil {
					t.Errorf("WriteAt failed. off: %d, at: %d, step: %d\n error: %v", off, at, step, e)
				}
				atomic.AddInt64(&writeN, int64(n))
			}(&wg, tmpfile, value, off, at, step)
		}
		wg.Wait()

		// Read one more byte to reach EOF
		buf := make([]byte, contentSize+1)
		readN, err := tmpfile.ReadAt(buf, off+at)
		atomic.AddInt64(&checks, 1)
		if err != io.EOF {
			t.Fatalf("ReadAt failed: %v", err)
		}
		readContent := string(buf[:contentSize])
		atomic.AddInt64(&checks, 1)
		if writeN != int64(readN) || writeN != contentSize || readContent != content {
			t.Fatalf("%s:: WriteAt(%s, %d) error. \ngot n: %v, content: %s \nexpected n: %v, content: %v",
				position, content, at, readN, readContent, contentSize, content)
		}
	}
	for off := int64(0); off < 2; off++ {
		for at := int64(0); at < 2; at++ {
			work(off, at)
		}
	}
}

func TestWriteAt_PositionPriorToBase(t *testT) {
	tmpdir := t.TempDir()
	tmpfilename := "TestOffsetWriter_WriteAt"
	tmpfile, err := os.CreateTemp(tmpdir, tmpfilename)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatalf("CreateTemp(%s) failed: %v", tmpfilename, err)
	}
	defer tmpfile.Close()

	// start writing position in OffsetWriter
	offset := int64(10)
	// position we want to write to the tmpfile
	at := int64(-1)
	w := io.NewOffsetWriter(tmpfile, offset)
	_, e := w.WriteAt([]byte("hello"), at)
	atomic.AddInt64(&checks, 1)
	if e == nil {
		t.Errorf("error expected to be not nil")
	}
}

func TestOffsetWriter_Write(t *testT) {
	const content = "0123456789ABCDEF"
	contentSize := len(content)
	tmpdir := t.TempDir()

	makeOffsetWriter := func(name string) (*io.OffsetWriter, *os.File) {
		tmpfilename := "TestOffsetWriter_Write_" + name
		tmpfile, err := os.CreateTemp(tmpdir, tmpfilename)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Fatalf("CreateTemp(%s) failed: %v", tmpfilename, err)
		}
		return io.NewOffsetWriter(tmpfile, 0), tmpfile
	}
	checkContent := func(name string, f *os.File) {
		// Read one more byte to reach EOF
		buf := make([]byte, contentSize+1)
		readN, err := f.ReadAt(buf, 0)
		atomic.AddInt64(&checks, 1)
		if err != io.EOF {
			t.Fatalf("ReadAt failed, err: %v", err)
		}
		readContent := string(buf[:contentSize])
		atomic.AddInt64(&checks, 1)
		if readN != contentSize || readContent != content {
			t.Fatalf("%s error. \ngot n: %v, content: %s \nexpected n: %v, content: %v",
				name, readN, readContent, contentSize, content)
		}
	}

	var name string
	name = "Write"
	t.Run(name, func(t *testT) {
		// Write directly (off: 0, at: 0)
		// Write content to file
		w, f := makeOffsetWriter(name)
		defer f.Close()
		for _, value := range []byte(content) {
			n, err := w.Write([]byte{value})
			atomic.AddInt64(&checks, 1)
			if err != nil {
				t.Fatalf("Write failed, n: %d, err: %v", n, err)
			}
		}
		checkContent(name, f)

		// Copy -> Write
		// Copy file f to file f2
		name = "Copy"
		w2, f2 := makeOffsetWriter(name)
		defer f2.Close()
		io.Copy(w2, f)
		checkContent(name, f2)
	})

	// Copy -> WriteTo -> Write
	// Note: strings.Reader implements the io.WriterTo interface.
	name = "Write_Of_Copy_WriteTo"
	t.Run(name, func(t *testT) {
		w, f := makeOffsetWriter(name)
		defer f.Close()
		io.Copy(w, strings.NewReader(content))
		checkContent(name, f)
	})
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
		{"TestCopy", TestCopy},
		{"TestCopyNegative", TestCopyNegative},
		{"TestCopyBuffer", TestCopyBuffer},
		{"TestCopyBufferNil", TestCopyBufferNil},
		{"TestCopyReadFrom", TestCopyReadFrom},
		{"TestCopyWriteTo", TestCopyWriteTo},
		{"TestCopyPriority", TestCopyPriority},
		{"TestCopyReadErrWriteErr", TestCopyReadErrWriteErr},
		{"TestCopyN", TestCopyN},
		{"TestCopyNReadFrom", TestCopyNReadFrom},
		{"TestCopyNWriteTo", TestCopyNWriteTo},
		{"TestCopyNEOF", TestCopyNEOF},
		{"TestReadAtLeast", TestReadAtLeast},
		{"TestReadAtLeastWithDataAndEOF", TestReadAtLeastWithDataAndEOF},
		{"TestReadAtLeastWithDataAndError", TestReadAtLeastWithDataAndError},
		{"TestTeeReader", TestTeeReader},
		{"TestSectionReader_ReadAt", TestSectionReader_ReadAt},
		{"TestSectionReader_Seek", TestSectionReader_Seek},
		{"TestSectionReader_Size", TestSectionReader_Size},
		{"TestSectionReader_Max", TestSectionReader_Max},
		{"TestCopyLargeWriter", TestCopyLargeWriter},
		{"TestNopCloserWriterToForwarding", TestNopCloserWriterToForwarding},
		{"TestOffsetWriter_Seek", TestOffsetWriter_Seek},
		{"TestOffsetWriter_WriteAt", TestOffsetWriter_WriteAt},
		{"TestWriteAt_PositionPriorToBase", TestWriteAt_PositionPriorToBase},
		{"TestOffsetWriter_Write", TestOffsetWriter_Write},
	}
	runTests(tests_)
}

// Output:
// ok TestCopy subtests: 0 checks: 1
// ok TestCopyNegative subtests: 0 checks: 2
// ok TestCopyBuffer subtests: 0 checks: 1
// ok TestCopyBufferNil subtests: 0 checks: 1
// ok TestCopyReadFrom subtests: 0 checks: 1
// ok TestCopyWriteTo subtests: 0 checks: 1
// ok TestCopyPriority subtests: 0 checks: 1
// ok TestCopyReadErrWriteErr subtests: 0 checks: 1
// ok TestCopyN subtests: 0 checks: 1
// ok TestCopyNReadFrom subtests: 0 checks: 1
// ok TestCopyNWriteTo subtests: 0 checks: 1
// ok TestCopyNEOF subtests: 0 checks: 6
// ok TestReadAtLeast subtests: 0 checks: 11
// ok TestReadAtLeastWithDataAndEOF subtests: 0 checks: 15
// ok TestReadAtLeastWithDataAndError subtests: 0 checks: 15
// ok TestTeeReader subtests: 0 checks: 5
// ok TestSectionReader_ReadAt subtests: 0 checks: 24
// ok TestSectionReader_Seek subtests: 0 checks: 26
// ok TestSectionReader_Size subtests: 0 checks: 2
// ok TestSectionReader_Max subtests: 0 checks: 3
// ok TestCopyLargeWriter subtests: 0 checks: 2
// ok TestNopCloserWriterToForwarding subtests: 0 checks: 2
// ok TestOffsetWriter_Seek subtests: 3 checks: 19
// ok TestOffsetWriter_WriteAt subtests: 0 checks: -1
// ok TestWriteAt_PositionPriorToBase subtests: 0 checks: 2
// ok TestOffsetWriter_Write subtests: 2 checks: 25
// tests: 26 failed: 0
