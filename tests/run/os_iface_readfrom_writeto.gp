// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
)

var tmpDir string

func setup() string {
	d, err := os.MkdirTemp("", "gpos")
	if err != nil {
		panic(err)
	}
	tmpDir = d
	return d
}

func cleanup() { os.RemoveAll(tmpDir) }

func idx(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// norm replaces the temporary directory in s by $D.
func norm(s string) string {
	for {
		i := idx(s, tmpDir)
		if i < 0 {
			return s
		}
		s = s[:i] + "$D" + s[i+len(tmpDir):]
	}
}

// e is the text of err, with the temporary directory replaced.
func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return norm(err.Error())
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type sliceReader struct{ s string }

func (r *sliceReader) Read(p []byte) (int, error) {
	if len(r.s) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.s)
	r.s = r.s[n:]
	return n, nil
}

type sink struct{ b []byte }

func (s *sink) Write(p []byte) (int, error) {
	s.b = append(s.b, p...)
	return len(p), nil
}

func main() {
	d := setup()
	defer cleanup()
	f, _ := os.Create(d + "/rf")
	n, err := f.ReadFrom(&sliceReader{"from a reader"})
	fmt.Println(n, err)
	f.Seek(0, 0)
	buf := &sink{}
	n, err = f.WriteTo(buf)
	fmt.Println(n, err, string(buf.b))
	f.Close()
	n, err = f.ReadFrom(&sliceReader{"x"})
	fmt.Println(n, e(err))
	n, err = f.WriteTo(buf)
	fmt.Println(n, e(err))
	g, _ := os.Open(d + "/rf")
	var rt io.WriterTo = g
	var rf io.ReaderFrom = f
	fmt.Println(rt != nil, rf != nil)
	g.Close()
}

// Output:
// 13 <nil>
// 13 <nil> from a reader
// 0 write $D/rf: file already closed
// 0 read $D/rf: file already closed
// true true
