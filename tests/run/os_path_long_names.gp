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
	name255 := repeat("n", 255)
	name256 := repeat("n", 256)
	fmt.Println(e(os.WriteFile(d+"/"+name255, []byte("ok"), 0o644)))
	err := os.WriteFile(d+"/"+name256, []byte("no"), 0o644)
	fmt.Println(len(err.Error()) > 256, err.(*os.PathError).Err)
	_, err = os.Stat(d + "/" + name256)
	fmt.Println(err.(*os.PathError).Err)
	fmt.Println(e(os.Mkdir(d+"/"+name255+"d", 0o755)) != "<nil>")
	ents, _ := os.ReadDir(d)
	fmt.Println(len(ents), len(ents[0].Name()))
	deep := d
	for i := 0; i < 45; i++ {
		deep += "/" + repeat("x", 100)
	}
	err = os.MkdirAll(deep, 0o755)
	fmt.Println(err.(*os.PathError).Err)
}

// Output:
// <nil>
// true file name too long
// file name too long
// true
// 1 255
// file name too long
