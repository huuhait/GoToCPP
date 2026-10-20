// phase: 09

package main

import (
	"errors"
	"fmt"
)

var saved fmt.ScanState

type S struct{ v string }

func (s *S) Scan(st fmt.ScanState, verb rune) error {
	saved = st
	tok, _ := st.Token(false, nil)
	s.v = string(tok)
	return errors.New("bad " + s.v)
}

func try(name string, f func()) {
	defer func() { fmt.Println(name, "->", recover()) }()
	f()
}

func main() {
	var s S
	// The Scan method fails at its width limit: Go's scanner keeps argLimit = 3 = count, so the freed
	// ScanState answers io.EOF (not a nil dereference).
	n, err := fmt.Sscanf("abcdef", "%3v", &s)
	fmt.Println(n, err, s.v)
	try("ReadRune", func() {
		r, size, err := saved.ReadRune()
		fmt.Println(r, size, err)
	})
	try("SkipSpace", func() { saved.SkipSpace() })
	try("Token", func() {
		t, err := saved.Token(true, nil)
		fmt.Printf("%q %v\n", t, err)
	})
	// Not at the limit (2 of 4 runes read): a nil dereference.
	n, err = fmt.Sscanf("abcdef", "%4v", &s)
	fmt.Println(n, err, s.v)
}

// Output:
// 0 bad abc abc
// 0 0 EOF
// ReadRune -> <nil>
// SkipSpace -> <nil>
// "" <nil>
// Token -> <nil>
// 0 bad abcd abcd
