// phase: 09

// Nil pointers to defined types take Go's reflect fallback, not its predeclared pointer cases.
package main

import "fmt"

type Number int
type Flag bool
type Unsigned uint
type Real float64
type Complex complex128
type Text string
type Bytes []byte
type Byte uint8
type NumberPointer *int

func check(name string, input string, target any) {
	defer func() {
		if value := recover(); value != nil {
			fmt.Println(name, "panic", value)
		}
	}()
	count, err := fmt.Sscan(input, target)
	fmt.Println(name, count, err)
}

func main() {
	check("number", "1", (*Number)(nil))
	check("number empty", "", (*Number)(nil))
	check("flag", "true", (*Flag)(nil))
	check("unsigned", "1", (*Unsigned)(nil))
	check("real", "1.5", (*Real)(nil))
	check("complex", "1+2i", (*Complex)(nil))
	check("text", "word", (*Text)(nil))
	check("bytes", "word", (*Bytes)(nil))
	check("byte elements", "word", (*[]Byte)(nil))
	check("defined pointer", "1", NumberPointer(nil))
	check("int fast path", "1", (*int)(nil))
	check("int empty", "", (*int)(nil))
	check("bytes fast path", "word", (*[]byte)(nil))
}

// Output:
// number 0 can't scan type: *main.Number
// number empty 0 can't scan type: *main.Number
// flag 0 can't scan type: *main.Flag
// unsigned 0 can't scan type: *main.Unsigned
// real 0 can't scan type: *main.Real
// complex 0 can't scan type: *main.Complex
// text 0 can't scan type: *main.Text
// bytes 0 can't scan type: *main.Bytes
// byte elements 0 can't scan type: *[]main.Byte
// defined pointer 0 can't scan type: main.NumberPointer
// int fast path panic runtime error: invalid memory address or nil pointer dereference
// int empty 0 EOF
// bytes fast path panic runtime error: invalid memory address or nil pointer dereference
