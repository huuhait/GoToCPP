// phase: 09

// The scanner's error handler recognizes panic(io.EOF) by identity, not by its error text.
package main

import (
	"errors"
	"fmt"
	"io"
)

var cleaned int

type panicScanner struct{ value any }

func (scanner panicScanner) Scan(state fmt.ScanState, verb rune) error {
	defer func() { cleaned++ }()
	panic(scanner.value)
}

type returnEOF struct{}

func (returnEOF) Scan(state fmt.ScanState, verb rune) error {
	defer func() { cleaned++ }()
	return io.EOF
}

type panicReader struct{}

func (panicReader) Read(buffer []byte) (int, error) {
	defer func() { cleaned++ }()
	panic(io.EOF)
}

func check(name string, scan func() (int, error)) {
	cleaned = 0
	defer func() {
		if value := recover(); value != nil {
			fmt.Printf("%s panic %T %v cleaned=%d\n", name, value, value, cleaned)
		}
	}()
	count, err := scan()
	fmt.Println(name, count, err, err == io.EOF, err == io.ErrUnexpectedEOF, cleaned)
}

func main() {
	check("panic EOF", func() (int, error) {
		return fmt.Sscan("7", panicScanner{io.EOF})
	})
	check("partial EOF", func() (int, error) {
		var number int
		return fmt.Sscan("7", &number, panicScanner{io.EOF})
	})
	check("return EOF", func() (int, error) {
		return fmt.Sscan("7", returnEOF{})
	})
	check("other error", func() (int, error) {
		return fmt.Sscan("7", panicScanner{errors.New("EOF")})
	})
	check("string", func() (int, error) {
		return fmt.Sscan("7", panicScanner{"EOF"})
	})
	check("unexpected EOF", func() (int, error) {
		return fmt.Sscan("7", panicScanner{io.ErrUnexpectedEOF})
	})
	check("reader EOF", func() (int, error) {
		var number int
		return fmt.Fscan(panicReader{}, &number)
	})
}

// Output:
// panic EOF 0 EOF true false 1
// partial EOF 1 EOF true false 1
// return EOF 0 unexpected EOF false true 1
// other error panic *errors.errorString EOF cleaned=1
// string panic string EOF cleaned=1
// unexpected EOF panic *errors.errorString unexpected EOF cleaned=1
// reader EOF 0 EOF true false 1
