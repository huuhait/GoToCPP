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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	d := setup()
	defer cleanup()
	must(os.WriteFile(d+"/s", []byte("0123456789"), 0o644))
	f, err := os.Open(d + "/s")
	must(err)
	defer f.Close()
	buf := make([]byte, 3)
	for _, c := range []struct {
		off    int64
		whence int
	}{{2, io.SeekStart}, {1, io.SeekCurrent}, {-2, io.SeekEnd}, {0, io.SeekCurrent}, {20, io.SeekStart}, {-1, io.SeekStart}, {0, 7}} {
		pos, err := f.Seek(c.off, c.whence)
		fmt.Println(c.off, c.whence, "->", pos, e(err))
		if err == nil {
			n, err := f.Read(buf)
			fmt.Printf("  read %d %q %v\n", n, buf[:n], err)
		}
	}
}

// Output:
// 2 0 -> 2 <nil>
//   read 3 "234" <nil>
// 1 1 -> 6 <nil>
//   read 3 "678" <nil>
// -2 2 -> 8 <nil>
//   read 2 "89" <nil>
// 0 1 -> 10 <nil>
//   read 0 "" EOF
// 20 0 -> 20 <nil>
//   read 0 "" EOF
// -1 0 -> 0 seek $D/s: invalid argument
// 0 7 -> 0 seek $D/s: invalid argument
