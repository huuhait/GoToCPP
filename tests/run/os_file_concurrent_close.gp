// phase: 09

package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
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
	must(os.WriteFile(d+"/c", []byte("0123456789"), 0o644))
	f, _ := os.Open(d + "/c")
	var wg sync.WaitGroup
	var ok, closed, other atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 4)
			for j := 0; j < 200; j++ {
				_, err := f.ReadAt(buf, 0)
				switch {
				case err == nil:
					ok.Add(1)
				case err.(*os.PathError).Err == os.ErrClosed:
					closed.Add(1)
				default:
					other.Add(1)
				}
			}
		}()
	}
	var cwg sync.WaitGroup
	var closeOK, closeErr atomic.Int32
	for i := 0; i < 5; i++ {
		cwg.Add(1)
		go func() {
			defer cwg.Done()
			if f.Close() == nil {
				closeOK.Add(1)
			} else {
				closeErr.Add(1)
			}
		}()
	}
	wg.Wait()
	cwg.Wait()
	fmt.Println(ok.Load()+closed.Load() == 4000, other.Load(), closeOK.Load(), closeErr.Load())
}

// Output:
// true 0 1 4
