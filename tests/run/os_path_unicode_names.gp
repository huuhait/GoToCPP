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
	names := []string{"héllo", "日本語", "emoji-\U0001F600", "sp ace", "tab\there", "new\nline", "-dash", ".hidden", "a\\b", "q\"uote"}
	for _, n := range names {
		fmt.Println(e(os.WriteFile(d+"/"+n, []byte(n), 0o644)))
	}
	ents, _ := os.ReadDir(d)
	for _, en := range ents {
		b, _ := os.ReadFile(d + "/" + en.Name())
		fmt.Printf("%q %q\n", en.Name(), b)
	}
	_, err := os.Stat(d + "/日本")
	fmt.Println(norm(err.Error()))
	fmt.Println(e(os.Remove(d + "/new\nline")))
}

// Output:
// <nil>
// <nil>
// <nil>
// <nil>
// <nil>
// <nil>
// <nil>
// <nil>
// <nil>
// <nil>
// "-dash" "-dash"
// ".hidden" ".hidden"
// "a\\b" "a\\b"
// "emoji-😀" "emoji-😀"
// "héllo" "héllo"
// "new\nline" "new\nline"
// "q\"uote" "q\"uote"
// "sp ace" "sp ace"
// "tab\there" "tab\there"
// "日本語" "日本語"
// stat $D/日本: no such file or directory
// <nil>
