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
	f, _ := os.Create(d + "/shared")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				f.Write([]byte("0123456789"))
			}
		}(i)
	}
	wg.Wait()
	st, _ := f.Stat()
	fmt.Println(st.Size())
	off, _ := f.Seek(0, 1)
	fmt.Println(off)
	f.Close()
	b, _ := os.ReadFile(d + "/shared")
	ok := true
	for i, c := range b {
		if c != byte('0'+i%10) {
			ok = false
		}
	}
	fmt.Println(len(b), ok)
}

// Output:
// 10000
// 10000
// 10000 true
