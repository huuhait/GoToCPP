// phase: 09
// Differential sweep of package io: 5000 pseudo-random texts go through Copy, CopyN, CopyBuffer, ReadAll, ReadFull,
// ReadAtLeast, LimitReader, MultiReader, MultiWriter, TeeReader, SectionReader (Read, ReadAt, Seek), OffsetWriter,
// WriteString and Pipe (a goroutine writes, the main goroutine reads) with readers and writers that return random chunk
// sizes, short writes and errors; one hash per operation is printed. The expected output comes from real Go 1.25
// (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type rngT struct{ s uint64 }

func (r *rngT) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) intn(n int) int { return int(r.next()>>1) % n }

var errBoom = errors.New("boom")

type chunkReader struct {
	s        string
	r        *rngT
	max      int
	errAtEnd bool
	dataEOF  bool // the last chunk comes together with io.EOF
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.s) == 0 {
		if c.errAtEnd {
			return 0, errBoom
		}
		return 0, io.EOF
	}
	n := 1 + c.r.intn(c.max)
	if n > len(p) {
		n = len(p)
	}
	if n > len(c.s) {
		n = len(c.s)
	}
	copy(p, c.s[:n])
	c.s = c.s[n:]
	if len(c.s) == 0 && c.dataEOF {
		return n, io.EOF
	}
	return n, nil
}

// failWriter writes at most limit bytes and then fails; sometimes it writes less than asked without an error.
type failWriter struct {
	buf   bytes.Buffer
	limit int
	r     *rngT
	short bool
}

func (w *failWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.limit {
		n := w.limit - w.buf.Len()
		w.buf.Write(p[:n])
		return n, errBoom
	}
	if w.short && len(p) > 1 && w.r.intn(4) == 0 {
		n := w.r.intn(len(p))
		w.buf.Write(p[:n])
		return n, nil // a short write without an error: io.Copy reports ErrShortWrite
	}
	return w.buf.Write(p)
}

type hasher struct{ h uint64 }

func newHasher() *hasher { return &hasher{14695981039346656037} }
func (x *hasher) bytes(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}
func (x *hasher) int(n int) { x.bytes(fmt.Sprint(n)) }
func (x *hasher) err(e error) {
	if e == nil {
		x.bytes("nil")
	} else {
		x.bytes(e.Error())
	}
}

func genText(r *rngT) string {
	n := r.intn(300)
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + r.intn(26))
		if r.intn(20) == 0 {
			b[i] = '\n'
		}
	}
	return string(b)
}

func main() {
	r := &rngT{4242}
	names := []string{}
	hs := map[string]*hasher{}
	get := func(n string) *hasher {
		h := hs[n]
		if h == nil {
			h = newHasher()
			hs[n] = h
			names = append(names, n)
		}
		return h
	}
	for it := 0; it < 5000; it++ {
		text := genText(r)
		cr := func() *chunkReader { return &chunkReader{s: text, r: r, max: 1 + r.intn(40), errAtEnd: it%6 == 0, dataEOF: it%3 == 1} }
		{
			w := &failWriter{limit: r.intn(400), r: r, short: it%4 == 0}
			n, err := io.Copy(w, cr())
			get("Copy").int(int(n))
			get("Copy").err(err)
			get("Copy").bytes(w.buf.String())
		}
		{
			w := &failWriter{limit: 1 << 20, r: r}
			n, err := io.CopyN(w, cr(), int64(r.intn(len(text)+20)))
			get("CopyN").int(int(n))
			get("CopyN").err(err)
			get("CopyN").bytes(w.buf.String())
		}
		{
			w := &failWriter{limit: 1 << 20, r: r}
			n, err := io.CopyBuffer(w, cr(), make([]byte, 1+r.intn(16)))
			get("CopyBuffer").int(int(n))
			get("CopyBuffer").err(err)
		}
		{
			b, err := io.ReadAll(cr())
			get("ReadAll").bytes(string(b))
			get("ReadAll").err(err)
		}
		{
			p := make([]byte, r.intn(40))
			n, err := io.ReadFull(cr(), p)
			get("ReadFull").int(n)
			get("ReadFull").err(err)
			p = make([]byte, 10+r.intn(30))
			n, err = io.ReadAtLeast(cr(), p, r.intn(len(p)+3))
			get("ReadAtLeast").int(n)
			get("ReadAtLeast").err(err)
		}
		{
			b, err := io.ReadAll(io.LimitReader(cr(), int64(r.intn(len(text)+5))))
			get("LimitReader").bytes(string(b))
			get("LimitReader").err(err)
		}
		{
			parts := []io.Reader{}
			for k := 0; k < 1+r.intn(4); k++ {
				parts = append(parts, strings.NewReader(text[:r.intn(len(text)+1)]))
			}
			if r.intn(3) == 0 {
				parts = append(parts, io.MultiReader(parts[0], strings.NewReader("x")))
			}
			b, err := io.ReadAll(io.MultiReader(parts...))
			get("MultiReader").bytes(string(b))
			get("MultiReader").err(err)
		}
		{
			var a, b bytes.Buffer
			fw := &failWriter{limit: r.intn(100), r: r}
			mw := io.MultiWriter(&a, fw, &b)
			n, err := io.Copy(mw, cr())
			get("MultiWriter").int(int(n))
			get("MultiWriter").err(err)
			get("MultiWriter").bytes(a.String() + "|" + fw.buf.String() + "|" + b.String())
		}
		{
			var tee bytes.Buffer
			b, err := io.ReadAll(io.TeeReader(cr(), &tee))
			get("TeeReader").bytes(string(b) + "|" + tee.String())
			get("TeeReader").err(err)
		}
		{
			base := strings.NewReader(text)
			off := int64(r.intn(len(text) + 3))
			sr := io.NewSectionReader(base, off, int64(r.intn(len(text)+3)))
			get("SectionReader").int(int(sr.Size()))
			p := make([]byte, r.intn(20))
			n, err := sr.Read(p)
			get("SectionReader").int(n)
			get("SectionReader").err(err)
			n, err = sr.ReadAt(p, int64(r.intn(30)-5))
			get("SectionReaderAt").int(n)
			get("SectionReaderAt").err(err)
			pos, err := sr.Seek(int64(r.intn(40)-10), r.intn(3))
			get("SectionReaderSeek").int(int(pos))
			get("SectionReaderSeek").err(err)
			b, err := io.ReadAll(sr)
			get("SectionReaderAll").bytes(string(b))
			get("SectionReaderAll").err(err)
		}
		{
			var buf writerAtBuf
			ow := io.NewOffsetWriter(&buf, int64(r.intn(20)))
			n, err := ow.Write([]byte(text[:r.intn(len(text)+1)]))
			get("OffsetWriter").int(n)
			get("OffsetWriter").err(err)
			pos, err := ow.Seek(int64(r.intn(10)-2), r.intn(3))
			get("OffsetWriterSeek").int(int(pos))
			get("OffsetWriterSeek").err(err)
			n, err = ow.WriteAt([]byte("zz"), int64(r.intn(10)-1))
			get("OffsetWriterAt").int(n)
			get("OffsetWriterAt").err(err)
			get("OffsetWriter").bytes(string(buf.b))
		}
		{
			w := &failWriter{limit: r.intn(30), r: r}
			n, err := io.WriteString(w, text)
			get("WriteString").int(n)
			get("WriteString").err(err)
		}
		if it%10 == 0 {
			pr, pw := io.Pipe()
			chunks := 1 + r.intn(5)
			done := make(chan int)
			go func() {
				total := 0
				for k := 0; k < chunks; k++ {
					n, err := pw.Write([]byte(text[:r.intn(len(text)+1)]))
					total += n
					_ = err
				}
				if it%20 == 0 {
					pw.CloseWithError(errBoom)
				} else {
					pw.Close()
				}
				done <- total
			}()
			// the writer's random choices run in another goroutine: only the sum of what was read and written is compared
			b, err := io.ReadAll(pr)
			total := <-done
			get("Pipe").int(len(b) - total)
			get("Pipe").err(err)
		}
	}
	for _, n := range names {
		fmt.Printf("%-18s %016x\n", n, hs[n].h)
	}
}

type writerAtBuf struct{ b []byte }

func (w *writerAtBuf) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	if need := int(off) + len(p); need > len(w.b) {
		w.b = append(w.b, make([]byte, need-len(w.b))...)
	}
	copy(w.b[off:], p)
	return len(p), nil
}

// Output:
// Copy               78191800de7b3f30
// CopyN              936734e62a4a0288
// CopyBuffer         e64b59a5924470f5
// ReadAll            b1fcb3dfe0106aa8
// ReadFull           02437d8c51a9d18c
// ReadAtLeast        1b1acfbfe44c3ba0
// LimitReader        9ad8466a40c6fefe
// MultiReader        8defced71def8ec6
// MultiWriter        7d570b1ee20982b1
// TeeReader          01d4a493b210725f
// SectionReader      1eb2f11c686264c3
// SectionReaderAt    8ff9a65bc38e6283
// SectionReaderSeek  946692598cfae96b
// SectionReaderAll   b310130042657217
// OffsetWriter       e9339c0c67deb71a
// OffsetWriterSeek   efb3ac292357849d
// OffsetWriterAt     3cd8b11e9b302c39
// WriteString        1d902add7e678061
// Pipe               cd0b403a45c7f7e5
