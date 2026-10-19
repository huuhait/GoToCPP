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
	must(os.WriteFile(d+"/orig", []byte("shared"), 0o644))
	fmt.Println(e(os.Link(d+"/orig", d+"/hard")))
	a, _ := os.Stat(d + "/orig")
	b, _ := os.Stat(d + "/hard")
	fmt.Println(os.SameFile(a, b), a.Size(), b.Size())
	f, _ := os.OpenFile(d+"/hard", os.O_WRONLY|os.O_APPEND, 0)
	f.WriteString("+more")
	f.Close()
	data, _ := os.ReadFile(d + "/orig")
	fmt.Println(string(data))
	fmt.Println(e(os.Remove(d + "/orig")))
	data, _ = os.ReadFile(d + "/hard")
	fmt.Println(string(data))
	fmt.Println(e(os.Link(d+"/hard", d+"/hard")))
	fmt.Println(e(os.Link(d+"/none", d+"/n2")))
	must(os.Mkdir(d+"/dir", 0o755))
	fmt.Println(e(os.Link(d+"/dir", d+"/dirlink")))
}

// Output:
// <nil>
// true 6 6
// shared+more
// <nil>
// shared+more
// link $D/hard $D/hard: file exists
// link $D/none $D/n2: no such file or directory
// link $D/dir $D/dirlink: operation not permitted
