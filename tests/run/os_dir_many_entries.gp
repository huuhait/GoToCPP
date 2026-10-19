// phase: 09

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

func repeat(s string, n int) string {
	r := ""
	for i := 0; i < n; i++ {
		r += s
	}
	return r
}

func itoa(n int) string { return fmt.Sprint(n) }

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
	const n = 2500
	for i := 0; i < n; i++ {
		f, err := os.Create(fmt.Sprintf("%s/file-%05d", d, i))
		must(err)
		f.Close()
	}
	ents, err := os.ReadDir(d)
	fmt.Println(len(ents), e(err))
	sorted := true
	for i := range ents {
		if ents[i].Name() != fmt.Sprintf("file-%05d", i) {
			sorted = false
		}
	}
	fmt.Println(sorted)
	f, _ := os.Open(d)
	names, err := f.Readdirnames(-1)
	fmt.Println(len(names), e(err))
	f.Close()
	f, _ = os.Open(d)
	total := 0
	for {
		batch, err := f.Readdirnames(100)
		total += len(batch)
		if err != nil {
			fmt.Println(total, err)
			break
		}
	}
	f.Close()
	fmt.Println(e(os.RemoveAll(d)))
	_, err = os.Stat(d)
	fmt.Println(os.IsNotExist(err))
	os.Mkdir(d, 0o755)
}

// Output:
// 2500 <nil>
// true
// 2500 <nil>
// 2500 EOF
// <nil>
// true
