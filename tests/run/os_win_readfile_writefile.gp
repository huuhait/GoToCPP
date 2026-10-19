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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	p := d + "/rw"
	fmt.Println(e(os.WriteFile(p, []byte("first contents"), 0o644)))
	fmt.Println(e(os.WriteFile(p, []byte("short"), 0o600)))
	b, err := os.ReadFile(p)
	fmt.Printf("%q %s\n", b, e(err))
	st, _ := os.Stat(p)
	fmt.Println(st.Mode())
	b, err = os.ReadFile(d + "/none")
	fmt.Println(b == nil, e(err))
	err = os.WriteFile(d+"/none/x", nil, 0o600)
	fmt.Println(e(err))
	big := make([]byte, 300000)
	for i := range big {
		big[i] = byte(i * 7)
	}
	must(os.WriteFile(p, big, 0o644))
	back, _ := os.ReadFile(p)
	same := len(back) == len(big)
	for i := range back {
		if back[i] != big[i] {
			same = false
		}
	}
	fmt.Println(len(back), same)
}

// Output:
// <nil>
// <nil>
// "short" <nil>
// -rw-rw-rw-
// true open $D/none: The system cannot find the file specified.
// open $D/none/x: The system cannot find the path specified.
// 300000 true
