// phase: 09

package main

import (
	"fmt"
	"os"
	"sync"
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
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, err := os.OpenFile(d+"/log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			must(err)
			defer f.Close()
			for i := 0; i < 100; i++ {
				f.WriteString("0123456789\n")
			}
		}()
	}
	wg.Wait()
	b, _ := os.ReadFile(d + "/log")
	ok := len(b) == 8*100*11
	for i := 0; i < len(b); i += 11 {
		if string(b[i:i+11]) != "0123456789\n" {
			ok = false
		}
	}
	fmt.Println(len(b), ok)
}

// Output:
// 8800 true
