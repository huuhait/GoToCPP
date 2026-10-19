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
	f, err := os.Create(d + "/sparse")
	must(err)
	const size = 100 << 20
	fmt.Println(e(f.Truncate(size)))
	st, _ := f.Stat()
	fmt.Println(st.Size() == size)
	n, err := f.WriteAt([]byte("tail"), size-4)
	fmt.Println(n, e(err))
	off, err := f.Seek(0, io.SeekEnd)
	fmt.Println(off == size, e(err))
	buf := make([]byte, 8)
	n, err = f.ReadAt(buf, size-8)
	fmt.Println(n, e(err), buf)
	n, err = f.ReadAt(buf, size-4)
	fmt.Println(n, err == io.EOF, buf[:n])
	n, err = f.ReadAt(buf, size/2)
	fmt.Println(n, e(err), buf)
	var zeros int64
	f.Seek(0, 0)
	chunk := make([]byte, 1<<20)
	for {
		m, err := f.Read(chunk)
		for _, c := range chunk[:m] {
			if c == 0 {
				zeros++
			}
		}
		if err != nil {
			break
		}
	}
	fmt.Println(zeros == size-4)
	f.Close()
	fi, _ := os.Stat(d + "/sparse")
	fmt.Println(fi.Size() == size)
}

// Output:
// <nil>
// true
// 4 <nil>
// true <nil>
// 8 <nil> [0 0 0 0 116 97 105 108]
// 4 true [116 97 105 108]
// 8 <nil> [0 0 0 0 0 0 0 0]
// true
// true
