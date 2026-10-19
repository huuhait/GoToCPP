// phase: 09

//go:build windows

package main

import (
	"fmt"
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

func main() {
	d := setup()
	defer cleanup()
	for _, n := range []string{"zeta", "alpha", "Mid", "beta", "a.b", "a_b", "10", "9", "é"} {
		must(os.WriteFile(d+"/"+n, nil, 0o644))
	}
	must(os.Mkdir(d+"/dir", 0o755))
	must(os.Symlink("alpha", d+"/lnk"))
	ents, err := os.ReadDir(d)
	fmt.Println(len(ents), e(err))
	for _, en := range ents {
		fmt.Println(en.Name(), en.IsDir(), en.Type(), en.Type().IsRegular())
	}
	last := ents[len(ents)-1]
	fi, err := last.Info()
	fmt.Println(fi.Name(), e(err))
	_, err = os.ReadDir(d + "/missing")
	fmt.Println(e(err))
	_, err = os.ReadDir(d + "/alpha")
	fmt.Println(e(err))
}

// Output:
// 11 <nil>
// 10 false ---------- true
// 9 false ---------- true
// Mid false ---------- true
// a.b false ---------- true
// a_b false ---------- true
// alpha false ---------- true
// beta false ---------- true
// dir true d--------- false
// lnk false L--------- false
// zeta false ---------- true
// é false ---------- true
// é <nil>
// open $D/missing: The system cannot find the file specified.
// readdir $D/alpha: The system cannot find the path specified.
