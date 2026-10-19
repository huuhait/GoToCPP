// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

var tmpDir string

func setup() string {
	d, err := os.MkdirTemp("", "gpwin")
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
	fmt.Println(string(os.PathSeparator), string(os.PathListSeparator), os.DevNull)
	fmt.Println(os.IsPathSeparator('\\'), os.IsPathSeparator('/'), os.IsPathSeparator(':'))
	// Both separators work; a name with a trailing dot or space is the name without it.
	must(os.Mkdir(d+`/a`, 0o777))
	must(os.WriteFile(d+`\a/f.txt`, []byte("1"), 0o666))
	b, err := os.ReadFile(d + `/a\f.txt`)
	fmt.Println(string(b), err)
	b, err = os.ReadFile(d + `\a\f.txt.`)
	fmt.Println(string(b), err)
	b, err = os.ReadFile(d + `\a\.\f.txt`)
	fmt.Println(string(b), err)
	b, err = os.ReadFile(d + `\a\..\a\f.txt`)
	fmt.Println(string(b), err)
	// Names are case-insensitive.
	b, err = os.ReadFile(d + `\A\F.TXT`)
	fmt.Println(string(b), err)
	fi, _ := os.Stat(d + `\A\F.TXT`)
	fmt.Println(fi.Name())
	// Reserved device names.
	_, err = os.Open("NUL")
	fmt.Println(err)
	_, err = os.Stat("NUL")
	fmt.Println(err)
	_, err = os.Open(`\\.\NUL`)
	fmt.Println(err)
	// Characters that Windows does not allow in a name.
	for _, name := range []string{"a*b", "a?b", "a<b", "a|b", `a"b`, "a:b"} {
		_, err := os.Create(d + `\` + name)
		fmt.Println(name, e(err))
	}
	// Absolute paths and the drive.
	wd, _ := os.Getwd()
	fmt.Println(len(wd) > 2 && wd[1] == ':' && wd[2] == '\\')
	fmt.Println(os.Chdir(`C:\`))
	wd, _ = os.Getwd()
	fmt.Println(wd)
	fmt.Println(e(os.Chdir(d + `\a\f.txt`)))
	fmt.Println(e(os.Chdir(d + `\missing`)))
}

// Output:
// \ ; NUL
// true true false
// 1 <nil>
// 1 <nil>
// 1 <nil>
// 1 <nil>
// 1 <nil>
// F.TXT
// <nil>
// <nil>
// <nil>
// a*b open $D\a*b: The filename, directory name, or volume label syntax is incorrect.
// a?b open $D\a?b: The filename, directory name, or volume label syntax is incorrect.
// a<b open $D\a<b: The filename, directory name, or volume label syntax is incorrect.
// a|b open $D\a|b: The filename, directory name, or volume label syntax is incorrect.
// a"b open $D\a"b: The filename, directory name, or volume label syntax is incorrect.
// a:b <nil>
// true
// <nil>
// C:\
// chdir $D\a\f.txt: The directory name is invalid.
// chdir $D\missing: The system cannot find the file specified.
