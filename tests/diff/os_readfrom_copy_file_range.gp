// phase: 09
//go:build linux

// File.ReadFrom (and io.Copy into a *File) tries copy_file_range(2) first, as Go's
// zero_copy_linux.go does: from a *File or a LimitedReader of one (whose N goes down), with both
// file offsets moved; its errors carry the "copy_file_range:" prefix (a destination opened
// read-only, a source opened write-only); a pipe on either side, an append-mode destination and an
// empty source fall back to the generic copy (dev.syscalldiffs).

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var tmp string

func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return strings.ReplaceAll(err.Error(), tmp, "$D")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func content(name string) string {
	b, err := os.ReadFile(name)
	must(err)
	return string(b)
}

func pos(f *os.File) int64 {
	n, err := f.Seek(0, io.SeekCurrent)
	must(err)
	return n
}

func main() {
	d, err := os.MkdirTemp("", "cfr")
	must(err)
	tmp = d
	defer os.RemoveAll(d)
	must(os.WriteFile(d+"/src", []byte("0123456789"), 0o644))

	// file to file: the offsets of both files move
	src, _ := os.Open(d + "/src")
	src.Seek(2, io.SeekStart)
	dst, _ := os.Create(d + "/dst")
	dst.WriteString("ab")
	n, err := io.Copy(dst, src)
	fmt.Println(n, e(err), pos(src), pos(dst), content(d+"/dst"))

	// a LimitedReader: N goes down by what was copied
	src.Seek(0, io.SeekStart)
	lr := &io.LimitedReader{R: src, N: 4}
	n, err = dst.ReadFrom(lr)
	fmt.Println(n, e(err), lr.N, pos(src), content(d+"/dst"))
	lr.N = 0
	n, err = dst.ReadFrom(lr)
	fmt.Println(n, e(err), lr.N)
	dst.Close()

	// at the end of the source
	n, err = io.Copy(io.Discard, src)
	fmt.Println(n, e(err))

	// the destination opened read-only, write-only source
	ro, _ := os.Open(d + "/dst")
	src.Seek(0, io.SeekStart)
	n, err = ro.ReadFrom(src)
	fmt.Println(n, e(err))
	ro.Close()
	wo, _ := os.OpenFile(d+"/src", os.O_WRONLY, 0)
	out, _ := os.Create(d + "/out")
	n, err = out.ReadFrom(wo)
	fmt.Println(n, e(err))
	wo.Close()

	// append mode: no copy_file_range, the generic copy
	ap, _ := os.OpenFile(d+"/out", os.O_WRONLY|os.O_APPEND, 0)
	src.Seek(5, io.SeekStart)
	n, err = ap.ReadFrom(src)
	fmt.Println(n, e(err), content(d+"/out"))
	ap.Close()

	// a pipe on either side: EINVAL, then the generic copy
	r, w, _ := os.Pipe()
	go func() {
		w.WriteString("through a pipe")
		w.Close()
	}()
	n, err = out.ReadFrom(r)
	fmt.Println(n, e(err), content(d+"/out"))
	r.Close()
	r, w, _ = os.Pipe()
	src.Seek(0, io.SeekStart)
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	n, err = w.ReadFrom(src)
	w.Close()
	fmt.Println(n, e(err), <-done)

	// an empty source and a closed one
	must(os.WriteFile(d+"/empty", nil, 0o644))
	em, _ := os.Open(d + "/empty")
	n, err = out.ReadFrom(em)
	fmt.Println(n, e(err))
	em.Close()
	n, err = out.ReadFrom(em)
	fmt.Println(n, e(err))
	src.Close()
	out.Close()
	n, err = out.ReadFrom(strings.NewReader("x"))
	fmt.Println(n, e(err))
}
