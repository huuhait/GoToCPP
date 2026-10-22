package rtfiles_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var updateDebugScript = flag.Bool("update-debug-script", false, "rewrite src/os/gocpp_gdb.inc from debug/gocpp-gdb.py")

const (
	debugScriptPy  = "debug/gocpp-gdb.py"
	debugScriptInc = "src/os/gocpp_gdb.inc"
)

// TestDebugScript checks that src/os/gocpp_gdb.inc, the text that every program carries in
// its .debug_gdb_scripts section (src/os/linux/debug_scripts.cpp, src/os/windows/debug_scripts.cpp), is the script
// debug/gocpp-gdb.py. go test ./runtime -run TestDebugScript -update-debug-script rewrites it.
func TestDebugScript(t *testing.T) {
	py, err := os.ReadFile(debugScriptPy)
	if err != nil {
		t.Fatal(err)
	}
	want, err := debugScriptInclude(py)
	if err != nil {
		t.Fatalf("%s: %v", debugScriptPy, err)
	}
	if *updateDebugScript {
		if err := os.WriteFile(debugScriptInc, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(debugScriptInc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale (or edited): run go test ./runtime -run TestDebugScript -update-debug-script", debugScriptInc)
	}
}

// trimDebugScript drops the lines of the script that cost bytes in every program and that
// Python ignores: blank lines and lines of a comment only, outside the multi-line strings (the
// docstrings of the commands are gdb's help text, and stay). It follows the strings by their
// triple double quotes only, so it refuses a script that could fool it: a triple single quote anywhere, a
// backslash before a triple quote, a triple quote that opens a string after a comment sign or
// another quote on its line (it would be inside a comment or a one-quote string), and a string
// that is not closed. TestDebugScriptTrim checks that python reads the same program.
func trimDebugScript(py []byte) ([]byte, error) {
	var out []byte
	inString := false
	for n, line := range strings.SplitAfter(string(py), "\n") {
		if strings.Contains(line, `'''`) || strings.Contains(line, `\"""`) {
			return nil, fmt.Errorf("line %d: a triple single quote or an escaped triple quote: docstrings use triple double quotes only", n+1)
		}
		startsInString := inString
		rest := line
		for {
			at := strings.Index(rest, `"""`)
			if at < 0 {
				break
			}
			if !inString && strings.ContainsAny(rest[:at], "#'\"") {
				return nil, fmt.Errorf("line %d: a triple quote after a comment sign or a quote: the trimmer could take it for a string", n+1)
			}
			inString = !inString
			rest = rest[at+3:]
		}
		text := strings.TrimSpace(line)
		if !startsInString && (text == "" || strings.HasPrefix(text, "#")) {
			continue
		}
		out = append(out, line...)
	}
	if inString {
		return nil, fmt.Errorf("a triple-quoted string is not closed")
	}
	return out, nil
}

// debugScriptInclude renders the script (without its blank and comment lines: trimDebugScript) as
// the lines of C string literals that the asm statement of debug_scripts.cpp takes: one ".ascii"
// for each line of the script. Each character that isn't printable ASCII, and the quote and the
// backslash, are octal escapes (the C compiler passes the backslash on, so the assembler reads
// \ooo).
func debugScriptInclude(py []byte) ([]byte, error) {
	trimmed, err := trimDebugScript(py)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("// Generated from debug/gocpp-gdb.py by TestDebugScript (-update-debug-script). DO NOT EDIT.\n")
	for _, line := range strings.SplitAfter(string(trimmed), "\n") {
		if line == "" {
			continue
		}
		b.WriteString(`        ".ascii \"`)
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c >= 0x20 && c < 0x7f && c != '"' && c != '\\' {
				b.WriteByte(c)
			} else {
				fmt.Fprintf(&b, `\\%03o`, c)
			}
		}
		b.WriteString("\\\"\\n\"\n")
	}
	return []byte(b.String()), nil
}

// TestDebugScriptTrim checks that the trimmed script is the same Python program: the syntax trees
// (without line numbers) of the file and of the trimmed text are equal, so the docstrings and
// every string are as they were. It needs python3; without it the test is skipped, and with
// GOCPP_REQUIRE_GDB=1 (make ci) it fails.
func TestDebugScriptTrim(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		if os.Getenv("GOCPP_REQUIRE_GDB") == "1" {
			t.Fatalf("GOCPP_REQUIRE_GDB=1 needs python3: %v", err)
		}
		t.Skipf("python3 is not installed: %v", err)
	}
	full, err := os.ReadFile(debugScriptPy)
	if err != nil {
		t.Fatal(err)
	}
	trimmed, err := trimDebugScript(full)
	if err != nil {
		t.Fatalf("%s: %v", debugScriptPy, err)
	}
	if len(trimmed) >= len(full) {
		t.Fatalf("the trimmed script has %d bytes, the script %d: nothing was dropped", len(trimmed), len(full))
	}
	trimmedFile := filepath.Join(t.TempDir(), "trimmed.py")
	if err := os.WriteFile(trimmedFile, trimmed, 0o644); err != nil {
		t.Fatal(err)
	}
	const dump = "import ast, sys; print(ast.dump(ast.parse(open(sys.argv[1]).read())))"
	want, err := exec.Command(py, "-I", "-c", dump, debugScriptPy).Output()
	if err != nil {
		t.Fatalf("python3 reads %s: %v", debugScriptPy, err)
	}
	got, err := exec.Command(py, "-I", "-c", dump, trimmedFile).Output()
	if err != nil {
		t.Fatalf("python3 reads the trimmed script: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("the trimmed script is not the same program as debug/gocpp-gdb.py")
	}
}

// TestTrimDebugScriptKeepsStrings: a blank line or a # line inside a multi-line string is text.
func TestTrimDebugScriptKeepsStrings(t *testing.T) {
	src := "# header\n\nx = 1  # kept\ndef f():\n    \"\"\"Doc.\n\n# not a comment\n    \"\"\"\n\n    # gone\n    return 2\n"
	want := "x = 1  # kept\ndef f():\n    \"\"\"Doc.\n\n# not a comment\n    \"\"\"\n    return 2\n"
	got, err := trimDebugScript([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("trimDebugScript:\n%q\nwant\n%q", got, want)
	}
}

// TestTrimDebugScriptRefusesWhatItCannotFollow: scripts whose strings the trimmer could misread.
func TestTrimDebugScriptRefusesWhatItCannotFollow(t *testing.T) {
	tq := strings.Repeat("\"", 3)
	sq := strings.Repeat("'", 3)
	refused := map[string]string{
		"single quotes": "def f():\n    " + sq + "Doc.\n\n    " + sq + "\n",
		"in a string":   "x = 'a " + tq + " b'\n\n",
		"in a comment":  "x = 1  # a " + tq + " b\n",
		"escaped":       "x = \"a \\" + tq + " b\"\n",
		"comment first": "x = 1  # " + tq + "a\n\nb" + tq + "\n",
		"quote first":   "x = \"a\" + " + tq + "a\n\nb" + tq + "\n",
		"not closed":    "def f():\n    " + tq + "Doc.\n\n",
	}
	for name, src := range refused {
		if got, err := trimDebugScript([]byte(src)); err == nil {
			t.Errorf("%s: trimmed to %q, want an error", name, got)
		}
	}
	allowed := map[string]string{
		"one line":        "def f():\n    " + tq + "Doc." + tq + "\n\n    return 1\n",
		"raw":             "x = 1\nr" + tq + "a\n\nb" + tq + "\n",
		"assigned":        "x = f(" + tq + "a\n\nb" + tq + ".split())\n\n",
		"one line string": "x = re.compile(r" + tq + "^[\"'] a" + tq + ")\n",
		"quotes":          "def f():\n    " + tq + "Doc: \"x\" and 'y'.\n\n    More." + tq + "\n",
	}
	for name, src := range allowed {
		if _, err := trimDebugScript([]byte(src)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestDebugScriptUnits runs debug/test_gocpp_gdb.py: the tests of the part of the script that needs
// no gdb (names, Go text of strings and numbers, the reader of .gp sources). It needs python3;
// without it the test is skipped, and with GOCPP_REQUIRE_GDB=1 (make ci) it fails.
func TestDebugScriptUnits(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		if os.Getenv("GOCPP_REQUIRE_GDB") == "1" {
			t.Fatalf("GOCPP_REQUIRE_GDB=1 needs python3: %v", err)
		}
		t.Skipf("python3 is not installed: %v", err)
	}
	out, err := exec.Command(py, "-I", "debug/test_gocpp_gdb.py").CombinedOutput()
	if err != nil {
		t.Fatalf("python3 debug/test_gocpp_gdb.py: %v\n%s", err, out)
	}
}
