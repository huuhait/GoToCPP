// phase: 09

// A fmt.State or fmt.ScanState that a method keeps after the call that made it: in Go the printer
// or scanner is back in its pool, so the State holds stale data and the ScanState has no reader
// (ReadRune, SkipSpace and Token meet the end of the input, UnreadRune panics). Nothing crashes.
package main

import "fmt"

var saved fmt.State
var savedScan fmt.ScanState

type F struct{}

func (F) Format(s fmt.State, c rune) { saved = s }

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
	_ = fmt.Sprintf("%+7.2v", F{})
	// (What a freed printer reports is Go's pool: stale data. The State is used, and nothing crashes.)
	saved.Width()
	saved.Precision()
	saved.Flag('+')
	saved.Write(nil)
	fmt.Println("State used")

	var s S
	n, err := fmt.Sscan("tok", &s)
	fmt.Println(n, err)
	wd, wok := savedScan.Width()
	fmt.Println(wd, wok)
	try("ReadRune", func() { savedScan.ReadRune() })
	try("SkipSpace", func() { savedScan.SkipSpace() })
	try("Token", func() { savedScan.Token(true, nil) })
	try("UnreadRune", func() { savedScan.UnreadRune() })
}

// Output:
// State used
// 1 <nil>
// 0 false
// ReadRune -> <nil>
// SkipSpace -> <nil>
// Token -> <nil>
// UnreadRune -> runtime error: invalid memory address or nil pointer dereference
