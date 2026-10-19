package rtfiles_test

import (
	"bytes"
	hexenc "encoding/hex"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// TestWindowsConsoleUTF8 checks the Windows OS layer's conversion of UTF-8 to UTF-16 for a console
// (src/os/windows/utf8.cpp, plain C++), built for this machine with each compiler, against Go's
// own writeConsole (internal/poll/fd_windows.go of Go 1.25, copied below): fixed cases (a character
// cut between two writes, bytes 0xC0, 0xC1 and 0xF5 to 0xFF at the end of a write, "\xe2\x82x",
// surrogate encodings, overlong forms) and 30000 random series of writes of valid and invalid bytes.
func TestWindowsConsoleUTF8(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	series := utf8Cases()
	var in, want bytes.Buffer
	for _, writes := range series {
		var hexes []string
		for _, w := range writes {
			hexes = append(hexes, hexenc.EncodeToString(w))
		}
		in.WriteString(strings.Join(hexes, "|") + "\n")
		units, tail := goWriteConsole(writes)
		for _, u := range units {
			fmt.Fprintf(&want, "%04x", u)
		}
		fmt.Fprintf(&want, ":%x\n", tail)
	}
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			dir := t.TempDir()
			bin := filepath.Join(dir, "utf8")
			args := append(c.flags([]string{"-O1"}), "-I", filepath.Join(rt, "include"),
				sourcePath(t, "testdata/winos/utf8.cpp"),
				filepath.Join(rt, "src", "os", "windows", "utf8.cpp"), "-o", bin)
			if err := c.compileCmd(prioRT, dir, nil, args); err != nil {
				t.Fatal(err)
			}
			out, stderr, code, err := commandStdin(dir, bin, in.Bytes())
			if err != nil || code != 0 || len(stderr) != 0 {
				t.Fatalf("exit %d %v\nstderr: %s", code, err, stderr)
			}
			if !bytes.Equal(out, want.Bytes()) {
				g, w := strings.Split(string(out), "\n"), strings.Split(want.String(), "\n")
				for i := 0; i < len(g) && i < len(w); i++ {
					if g[i] != w[i] {
						t.Fatalf("series %d, writes %x:\ngot  %s\nwant %s", i, series[i], g[i], w[i])
					}
				}
				t.Fatalf("output has %d lines, want %d", len(g), len(w))
			}
		})
	}
}

// goWriteConsole is what a Go program does with writes to a console: fd.writeConsole of Go 1.25
// (internal/poll/fd_windows.go), up to the call of WriteConsole; it returns the UTF-16 units that
// would be written and the bytes held at the end. (Copyright 2009 The Go Authors, BSD-style
// license.)
func goWriteConsole(writes [][]byte) (units []uint16, lastbits []byte) {
	for _, b := range writes {
		var runes []rune
		if len(lastbits) > 0 {
			b = append(append([]byte{}, lastbits...), b...)
			lastbits = nil
		}
		for len(b) >= utf8.UTFMax || utf8.FullRune(b) {
			r, l := utf8.DecodeRune(b)
			runes = append(runes, r)
			b = b[l:]
		}
		if len(b) > 0 {
			lastbits = append([]byte{}, b...)
		}
		units = append(units, utf16.Encode(runes)...)
	}
	return units, lastbits
}

func utf8Cases() [][][]byte {
	s := func(parts ...string) [][]byte {
		var w [][]byte
		for _, p := range parts {
			w = append(w, []byte(p))
		}
		return w
	}
	cases := [][][]byte{
		s(""), s("a"), s("hello\n"), s("\u00e9"), s("\u65e5\u672c"), s("\U0001F389"),
		s("a \xe6\x97", "\xa5 b"), s("\xe6", "\x97", "\xa5"), s("\xf0\x9f", "\x8e", "\x89"),
		s("\xc0"), s("\xc1"), s("\xf5"), s("\xff"), s("\x80"), s("a\xc0"), s("a\xf8"),
		s("\xe2\x82x"), s("\xe2\x82"), s("\xe2\x82", "x"), s("\xe2", "x"), s("\xe2\x28\xa1"),
		s("\xed\xa0\x80"), s("\xed\xa0"), s("\xf4\x90\x80\x80"), s("\xf4\x8f\xbf\xbf"), s("\xe0\x80\x80"),
		s("\xe0\x9f"), s("\xc0\x80"), s("\xf0\x80\x80\x80"), s("\xf0\x8f"), s("\xff\xfe"),
		s("\xf0\x9f\x8e"), s("\xf0\x9f\x8e", "\xff"), s("\xe2\x82\xac\xe2"),
		s(strings.Repeat("x", 5000)), s(strings.Repeat("\u65e5", 3000)), s(strings.Repeat("\U0001F389", 2500)),
		s(strings.Repeat("a", 2046) + "\U0001F389" + "b"), s(strings.Repeat("a", 2047) + "\U0001F389"),
	}
	r := rand.New(rand.NewSource(141))
	pool := [][]byte{
		[]byte("a"), []byte("z"), []byte("\n"), []byte("\u00e9"), []byte("\u65e5"), []byte("\U0001F389"),
		{0x80}, {0xbf}, {0xc0}, {0xc1}, {0xc2}, {0xdf}, {0xe0}, {0xe1}, {0xed}, {0xef}, {0xf0}, {0xf4}, {0xf5}, {0xf8}, {0xff},
		{0xa0}, {0x9f}, {0x90}, {0x8f},
	}
	for range 30000 {
		var writes [][]byte
		for range 1 + r.Intn(4) {
			var w []byte
			for range r.Intn(7) {
				w = append(w, pool[r.Intn(len(pool))]...)
			}
			writes = append(writes, w)
		}
		cases = append(cases, writes)
	}
	return cases
}
