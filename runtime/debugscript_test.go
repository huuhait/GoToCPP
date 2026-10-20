package rtfiles_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	want := debugScriptInclude(py)
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

// debugScriptInclude renders the script as the lines of C string literals that the asm
// statement of debug_scripts.cpp takes: one ".ascii" for each line of the script. Each
// character that isn't printable ASCII, and the quote and the backslash, are octal escapes
// (the C compiler passes the backslash on, so the assembler reads \ooo).
func debugScriptInclude(py []byte) []byte {
	var b strings.Builder
	b.WriteString("// Generated from debug/gocpp-gdb.py by TestDebugScript (-update-debug-script). DO NOT EDIT.\n")
	for _, line := range strings.SplitAfter(string(py), "\n") {
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
	return []byte(b.String())
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
