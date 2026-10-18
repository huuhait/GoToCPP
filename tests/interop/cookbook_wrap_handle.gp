// phase: 05

package main

import (
	"errors"
	"fmt"

	"<stdio.h>"
)

// LineWriter wraps a C FILE* in a Go type with Go methods, so Go code (and an interface) can use it
// without knowing about C.
type LineWriter struct {
	f *stdio.FILE // a raw C pointer: C owns the FILE, Close gives it back
}

var errClosed = errors.New("write to a closed LineWriter")

func Open() *LineWriter {
	return &LineWriter{f: stdio.tmpfile()}
}

func (w *LineWriter) WriteLine(s string) error {
	if w.f == nil {
		return errClosed
	}
	// C.cstr is a const char* that lives for this statement; fputs does not keep it.
	if stdio.fputs(C.cstr(s), w.f) == stdio.EOF || stdio.fputc('\n', w.f) == stdio.EOF {
		return errors.New("write failed")
	}
	return nil
}

// Contents reads the file back into a Go string.
func (w *LineWriter) Contents() string {
	stdio.rewind(w.f)
	var buf [256]C.char
	n := stdio.fread(&buf[0], 1, 255, w.f)
	return C.GoStringN(&buf[0], C.int(n))
}

func (w *LineWriter) Close() error {
	if w.f != nil {
		stdio.fclose(w.f)
		w.f = nil
	}
	return nil
}

// Sink is what the rest of the program needs to know.
type Sink interface {
	WriteLine(s string) error
	Close() error
}

func log(out Sink, lines ...string) {
	for _, l := range lines {
		if err := out.WriteLine(l); err != nil {
			fmt.Println("error:", err)
		}
	}
}

func main() {
	w := Open()
	var out Sink = w // *LineWriter implements Sink
	log(out, "first", "second")
	fmt.Print(w.Contents())
	out.Close()
	log(out, "third")
}

// Output:
// first
// second
// error: write to a closed LineWriter
