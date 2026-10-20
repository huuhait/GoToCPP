// phase: 09

// Multi-result calls supply fixed parameters and variadic operands separately to native fmt.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

var number int
var calls int
var buffer bytes.Buffer

type Number int

var defined Number

func stringArgs() (string, *int) {
	calls++
	return "7\n", &number
}

func stringFormatArgs() (string, string, *Number) {
	return "8", "%d", &defined
}

func readerArgs() (*strings.Reader, *int) {
	return strings.NewReader("9\n"), &number
}

func readerFormatArgs() (io.Reader, string, *Number) {
	return strings.NewReader("10"), "%d", &defined
}

func stdinArgs() (string, *int)                        { return "%d", &number }
func formatArgs() (string, int64)                      { return "%T", int64(7) }
func writerArgs() (*bytes.Buffer, int64)               { return &buffer, int64(7) }
func writerFormatArgs() (*bytes.Buffer, string, int64) { return &buffer, "%T", int64(7) }
func appendArgs() ([]byte, int64)                      { return []byte("v"), int64(7) }
func appendFormatArgs() ([]byte, string, int64)        { return nil, "%T", int64(7) }

type state struct{}

func (state) Width() (int, bool)              { return 4, true }
func (state) Precision() (int, bool)          { return 2, true }
func (state) Flag(flag int) bool              { return flag == '+' }
func (state) Write(value []byte) (int, error) { return len(value), nil }
func stateArgs() (state, rune)                { return state{}, 'x' }

func scanStandardInput() {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	writer.WriteString("11\n")
	writer.Close()
	previous := os.Stdin
	os.Stdin = reader
	defer func() {
		os.Stdin = previous
		reader.Close()
	}()
	count, err := fmt.Scanf(stdinArgs())
	fmt.Println("Scanf", count, err, number)
}

func main() {
	count, err := fmt.Sscan(stringArgs())
	fmt.Println("Sscan", count, err, number, calls)

	count, err = fmt.Sscanln(stringArgs())
	fmt.Println("Sscanln", count, err, number, calls)

	count, err = fmt.Sscanf(stringFormatArgs())
	fmt.Println("Sscanf", count, err, defined)

	count, err = fmt.Fscan(readerArgs())
	fmt.Println("Fscan", count, err, number)

	count, err = fmt.Fscanln(readerArgs())
	fmt.Println("Fscanln", count, err, number)

	count, err = fmt.Fscanf(readerFormatArgs())
	fmt.Println("Fscanf", count, err, defined)

	scanStandardInput()
	fmt.Println("Sprintf", fmt.Sprintf(formatArgs()))
	fmt.Println("Errorf", fmt.Errorf(formatArgs()))
	fmt.Println("Append", string(fmt.Append(appendArgs())))
	fmt.Println("Appendf", string(fmt.Appendf(appendFormatArgs())))
	fmt.Println("FormatString", fmt.FormatString(stateArgs()))

	count, err = fmt.Fprint(writerArgs())
	fmt.Println("Fprint", count, err, buffer.String())
	buffer.Reset()
	count, err = fmt.Fprintf(writerFormatArgs())
	fmt.Println("Fprintf", count, err, buffer.String())

	// Calls through func values use the general tuple path and must keep the same conversions.
	scan := fmt.Sscan
	count, err = scan(stringArgs())
	fmt.Println("func Sscan", count, err, number, calls)
}

// Output:
// Sscan 1 <nil> 7 1
// Sscanln 1 <nil> 7 2
// Sscanf 1 <nil> 8
// Fscan 1 <nil> 9
// Fscanln 1 <nil> 9
// Fscanf 1 <nil> 10
// Scanf 1 <nil> 11
// Sprintf int64
// Errorf int64
// Append v7
// Appendf int64
// FormatString %+4.2x
// Fprint 1 <nil> 7
// Fprintf 5 <nil> int64
// func Sscan 1 <nil> 7 3
