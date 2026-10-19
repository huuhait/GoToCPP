package rtfiles_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWindowsCommandLine checks the Windows command line splitter of the OS layer
// (src/os/windows/cmdline.cpp, plain C++), built for this machine with each compiler, against Go's
// own (os.commandLineToArgv of Go 1.25, copied below): a fixed list of hard cases and 20000
// random command lines of quotes, backslashes, spaces, tabs and non-ASCII text. The arguments of
// a gocpp program must be those of the same program built with Go (Phase 14, step 14.1).
func TestWindowsCommandLine(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	lines := cmdlineCases()
	var in bytes.Buffer
	var want bytes.Buffer
	for _, l := range lines {
		in.WriteString(l + "\n")
		args := goCommandLineToArgv(l)
		fmt.Fprintf(&want, "%d\n", len(args))
		for _, a := range args {
			fmt.Fprintf(&want, "%d:%s\n", len(a), a)
		}
	}
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			dir := t.TempDir()
			bin := filepath.Join(dir, "cmdline")
			args := append(c.flags([]string{"-O1"}), "-I", filepath.Join(rt, "include"),
				sourcePath(t, "testdata/winos/cmdline.cpp"),
				filepath.Join(rt, "src", "os", "windows", "cmdline.cpp"), "-o", bin)
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
						t.Fatalf("output differs from Go's at line %d (%d lines in all):\ngot  %q\nwant %q\nthe command line is one of the %d: look for it near the number of the line", i, len(w), g[i], w[i], len(lines))
					}
				}
				t.Fatalf("output has %d lines, want %d", len(g), len(w))
			}
		})
	}
}

// commandStdin runs bin in dir with input on its standard input.
func commandStdin(dir, bin string, input []byte) (stdout, stderr []byte, code int, err error) {
	gate.acquire(prioRun)
	defer gate.release()
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(input)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.Bytes(), errb.Bytes(), exit.ExitCode(), nil
	}
	return out.Bytes(), errb.Bytes(), 0, err
}

func cmdlineCases() []string {
	lines := []string{
		``, `a`, `a b  c`, "a\tb", `"a b"`, `a"b c"d`, `a\"b`, `a\\"b c"`, `a\\\"b`, `a\b`, `"a\\"`,
		`""`, `a "" b`, `"a""b"`, `""""`, `"""a"`, `"C:\Program Files\x.exe" arg`, `C:\x\y.exe /flag:"a b"`,
		`x  `, `  x`, `"unterminated arg`, `a\`, `a\\`, `\\\`, `"\"`, `"\\"`, `\"`, `\\"`, `"`, `""`,
		"h\u00e9llo \u65e5\u672c\u8a9e \U0001F389", `"h\u00e9llo w" x`, "a\t\tb \t c",
		`a" "b`, `a" b" c`, `"a b" "c d"`, `a"" b`, `"a" "b"`, `\\server\share\x.exe`,
	}
	r := rand.New(rand.NewSource(14))
	alphabet := []string{"a", "b", " ", " ", "\t", `"`, `"`, `\`, `\`, `\`, "\u00e9", "\u65e5", "\U0001F389", "-x", "="}
	for range 20000 {
		var b strings.Builder
		for range r.Intn(14) {
			b.WriteString(alphabet[r.Intn(len(alphabet))])
		}
		lines = append(lines, b.String())
	}
	return lines
}

// goCommandLineToArgv is os.commandLineToArgv of Go 1.25 (os/exec_windows.go): how a program
// built with Go splits the command line that Windows gives it. Copied with its readNextArg, for
// TestWindowsCommandLine. (Copyright 2011 The Go Authors, BSD-style license.)
func goCommandLineToArgv(cmd string) []string {
	var args []string
	for len(cmd) > 0 {
		if cmd[0] == ' ' || cmd[0] == '\t' {
			cmd = cmd[1:]
			continue
		}
		var arg []byte
		arg, cmd = goReadNextArg(cmd)
		args = append(args, string(arg))
	}
	return args
}

func goAppendBSBytes(b []byte, n int) []byte {
	for ; n > 0; n-- {
		b = append(b, '\\')
	}
	return b
}

func goReadNextArg(cmd string) (arg []byte, rest string) {
	var b []byte
	var inquote bool
	var nslash int
	for ; len(cmd) > 0; cmd = cmd[1:] {
		c := cmd[0]
		switch c {
		case ' ', '\t':
			if !inquote {
				return goAppendBSBytes(b, nslash), cmd[1:]
			}
		case '"':
			b = goAppendBSBytes(b, nslash/2)
			if nslash%2 == 0 {
				if inquote && len(cmd) > 1 && cmd[1] == '"' {
					b = append(b, c)
					cmd = cmd[1:]
				}
				inquote = !inquote
			} else {
				b = append(b, c)
			}
			nslash = 0
			continue
		case '\\':
			nslash++
			continue
		}
		b = goAppendBSBytes(b, nslash)
		nslash = 0
		b = append(b, c)
	}
	return goAppendBSBytes(b, nslash), ""
}
