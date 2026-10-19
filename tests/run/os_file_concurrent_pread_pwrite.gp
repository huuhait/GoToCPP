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
	f, err := os.OpenFile(d+"/c", os.O_RDWR|os.O_CREATE, 0o644)
	must(err)
	const workers = 16
	const rec = 512
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			buf := make([]byte, rec)
			for i := range buf {
				buf[i] = byte(w)
			}
			for round := 0; round < 50; round++ {
				if n, err := f.WriteAt(buf, int64(w*rec)); n != rec || err != nil {
					panic(fmt.Sprint(n, err))
				}
			}
		}(w)
	}
	wg.Wait()
	bad := 0
	for w := 0; w < workers; w++ {
		var rwg sync.WaitGroup
		got := make([]byte, rec)
		rwg.Add(1)
		go func() {
			defer rwg.Done()
			f.ReadAt(got, int64(w*rec))
		}()
		rwg.Wait()
		for _, c := range got {
			if c != byte(w) {
				bad++
			}
		}
	}
	st, _ := f.Stat()
	fmt.Println(st.Size() == workers*rec, bad)
	f.Close()
}

// Output:
// true 0
