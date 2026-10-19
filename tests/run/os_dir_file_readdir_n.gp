// phase: 09

package main

import (
	"fmt"
	"io"
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
	for i := 0; i < 7; i++ {
		must(os.WriteFile(fmt.Sprintf("%s/f%d", d, i), nil, 0o644))
	}
	f, err := os.Open(d)
	must(err)
	var all []string
	for {
		ents, err := f.ReadDir(3)
		fmt.Println(len(ents), err == io.EOF, e(err))
		for _, en := range ents {
			all = append(all, en.Name())
		}
		if err != nil {
			break
		}
	}
	sortStrings(all)
	fmt.Println(all)
	ents, err := f.ReadDir(2)
	fmt.Println(len(ents), err == io.EOF)
	ents, err = f.ReadDir(-1)
	fmt.Println(len(ents), e(err))
	f.Close()
}

// Output:
// 3 false <nil>
// 3 false <nil>
// 1 false <nil>
// 0 true EOF
// [f0 f1 f2 f3 f4 f5 f6]
// 0 true
// 0 <nil>
