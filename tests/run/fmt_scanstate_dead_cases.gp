// phase: 09

// What a ScanState kept by a Scan method answers after its scan (ReadRune, SkipSpace, Token, Width, UnreadRune)
// for 24 scans: with a width, to the end of the input or not, with an error, from a string and from readers.
package main

import (
	"bufio"
	"fmt"
	"strings"
)

var saved fmt.ScanState

// T reads one token.
type T struct{ s string }

func (t *T) Scan(st fmt.ScanState, verb rune) error {
	saved = st
	tok, err := st.Token(true, nil)
	t.s = string(tok)
	return err
}

// R reads one rune only.
type R struct{ r rune }

func (t *R) Scan(st fmt.ScanState, verb rune) error {
	saved = st
	r, _, err := st.ReadRune()
	t.r = r
	return err
}

// E fails after reading one rune.
type E struct{}

func (t *E) Scan(st fmt.ScanState, verb rune) error {
	saved = st
	st.ReadRune()
	return fmt.Errorf("custom fail")
}

func try(name string, f func() string) {
	defer func() {
		if r := recover(); r != nil {
			_, isErr := r.(error)
			fmt.Printf("  %s: PANIC %T iserr=%v %v\n", name, r, isErr, r)
		}
	}()
	fmt.Printf("  %s: %s\n", name, f())
}

func probe() {
	try("Width", func() string { w, ok := saved.Width(); return fmt.Sprint(w, ok) })
	try("Read", func() string { n, err := saved.Read(make([]byte, 4)); return fmt.Sprint(n, err) })
	try("ReadRune", func() string { r, sz, err := saved.ReadRune(); return fmt.Sprint(r, sz, err) })
	try("SkipSpace", func() string { saved.SkipSpace(); return "ok" })
	try("Token", func() string {
		tok, err := saved.Token(true, nil)
		return fmt.Sprintf("%q nil=%v %v", tok, tok == nil, err)
	})
	try("TokenNoSkip", func() string {
		tok, err := saved.Token(false, func(r rune) bool { return true })
		return fmt.Sprintf("%q %v", tok, err)
	})
	try("UnreadRune", func() string { return fmt.Sprint(saved.UnreadRune()) })
	try("ReadRune2", func() string { r, sz, err := saved.ReadRune(); return fmt.Sprint(r, sz, err) })
}

func run(c string) {
	saved = nil
	var t T
	var r R
	var e E
	var n int
	var res string
	switch c {
	case "sscan_full":
		k, err := fmt.Sscan("tok", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscan_rest":
		k, err := fmt.Sscan("tok rest", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscan_trailsp":
		k, err := fmt.Sscan("tok ", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscan_rune":
		k, err := fmt.Sscan("ab", &r)
		res = fmt.Sprint(k, err, r.r)
	case "sscan_rune_last":
		k, err := fmt.Sscan("a", &r)
		res = fmt.Sprint(k, err, r.r)
	case "sscanf_w3":
		k, err := fmt.Sscanf("abcdef", "%3v", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanf_w5_short":
		k, err := fmt.Sscanf("ab", "%5v", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanf_w5_sp":
		k, err := fmt.Sscanf("ab cdefg", "%5v", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanf_w3_two":
		k, err := fmt.Sscanf("abcdef 7", "%3v %d", &t, &n)
		res = fmt.Sprint(k, err, t.s, n)
	case "sscanln_full":
		k, err := fmt.Sscanln("tok", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanln_nl_rest":
		k, err := fmt.Sscanln("tok\nrest", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanln_extra":
		k, err := fmt.Sscanln("tok x\n", &t)
		res = fmt.Sprint(k, err, t.s)
	case "fscan_strreader_rest":
		k, err := fmt.Fscan(strings.NewReader("tok rest"), &t)
		res = fmt.Sprint(k, err, t.s)
	case "fscan_strreader_full":
		k, err := fmt.Fscan(strings.NewReader("tok"), &t)
		res = fmt.Sprint(k, err, t.s)
	case "fscan_bufio_rest":
		k, err := fmt.Fscan(bufio.NewReader(strings.NewReader("tok rest")), &t)
		res = fmt.Sprint(k, err, t.s)
	case "fscanln_bufio":
		k, err := fmt.Fscanln(bufio.NewReader(strings.NewReader("tok\nmore")), &t)
		res = fmt.Sprint(k, err, t.s)
	case "err_custom":
		k, err := fmt.Sscan("xyz", &e)
		res = fmt.Sprint(k, err)
	case "err_custom_last":
		k, err := fmt.Sscan("x", &e)
		res = fmt.Sprint(k, err)
	case "err_int_after":
		k, err := fmt.Sscan("tok rest", &t, &n)
		res = fmt.Sprint(k, err, t.s)
	case "err_int_eof":
		k, err := fmt.Sscan("tok", &t, &n)
		res = fmt.Sprint(k, err, t.s)
	case "sscanf_v_rest":
		k, err := fmt.Sscanf("tok rest", "%v", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanf_v_full":
		k, err := fmt.Sscanf("tok", "%v", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanf_v_lit":
		k, err := fmt.Sscanf("tok!", "%v!", &t)
		res = fmt.Sprint(k, err, t.s)
	case "sscanf_s_rest":
		k, err := fmt.Sscanf("tok rest", "%s", &t)
		res = fmt.Sprint(k, err, t.s)
	}
	fmt.Println(c, "->", res)
	probe()
}

func main() {
	for _, c := range []string{"sscan_full", "sscan_rest", "sscan_trailsp", "sscan_rune", "sscan_rune_last", "sscanf_w3", "sscanf_w5_short", "sscanf_w5_sp", "sscanf_w3_two", "sscanln_full", "sscanln_nl_rest", "sscanln_extra", "fscan_strreader_rest", "fscan_strreader_full", "fscan_bufio_rest", "fscanln_bufio", "err_custom", "err_custom_last", "err_int_after", "err_int_eof", "sscanf_v_rest", "sscanf_v_full", "sscanf_v_lit", "sscanf_s_rest"} {
		run(c)
	}
}

// Output:
// sscan_full -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// sscan_rest -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscan_trailsp -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscan_rune -> 1 <nil> 97
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscan_rune_last -> 1 <nil> 97
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscanf_w3 -> 1 <nil>abc
//   Width: 3 true
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscanf_w5_short -> 1 <nil>ab
//   Width: 5 true
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// sscanf_w5_sp -> 1 <nil>ab
//   Width: 5 true
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscanf_w3_two -> 1 expected space in input to match formatabc0
//   Width: 3 true
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscanln_full -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// sscanln_nl_rest -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// sscanln_extra -> 1 expected newlinetok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// fscan_strreader_rest -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// fscan_strreader_full -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// fscan_bufio_rest -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// fscanln_bufio -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// err_custom -> 0 custom fail
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// err_custom_last -> 0 custom fail
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// err_int_after -> 1 expected integertok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// err_int_eof -> 1 EOFtok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// sscanf_v_rest -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
// sscanf_v_full -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// sscanf_v_lit -> 1 unexpected EOFtok!
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: 0 0 EOF
//   SkipSpace: ok
//   Token: "" nil=false <nil>
//   TokenNoSkip: "" <nil>
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: 0 0 EOF
// sscanf_s_rest -> 1 <nil>tok
//   Width: 0 false
//   Read: 0 ScanState's Read should not be called. Use ReadRune
//   ReadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   SkipSpace: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   Token: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   TokenNoSkip: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   UnreadRune: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
//   ReadRune2: PANIC runtime.errorString iserr=true runtime error: invalid memory address or nil pointer dereference
