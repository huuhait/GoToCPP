// phase: 09

// A ScanState kept after its scan: in Go the scanner is back in its pool with a nil reader but its other
// fields, so ReadRune, SkipSpace and Token meet io.EOF if the scan had reached the end of its input and panic
// with a nil dereference if it had not.
package main

import "fmt"

var savedScan fmt.ScanState

type S struct{}

func (*S) Scan(s fmt.ScanState, verb rune) error {
	savedScan = s
	_, err := s.Token(true, nil)
	return err
}

func try(name string, f func()) {
	defer func() { fmt.Println(name, "->", recover()) }()
	f()
}

func main() {
	var s S
	fmt.Sscan("tok", &s)
	r, size, err := savedScan.ReadRune()
	fmt.Println(r, size, err)
	tok, err := savedScan.Token(true, nil)
	fmt.Printf("%q %v\n", tok, err)
	fmt.Sscan("tok rest", &s)
	try("ReadRune not at EOF", func() { savedScan.ReadRune() })
}

// Output:
// 0 0 EOF
// "" <nil>
// ReadRune not at EOF -> runtime error: invalid memory address or nil pointer dereference
