package main

import (
	"fmt"
	"go/build/constraint"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// A Header holds the lines at the top of a test program (the comment lines before the package
// clause) that matter to a sanitizer run. tests/README.md describes them; the harness of the
// tests package parses the same lines (tests/format.go), with the checks of a test suite. This
// parser reads what it needs and ignores the rest: a bad line is the harness's error to report.
type Header struct {
	Phase       int  // // phase: NN (0 when absent)
	ExitCode    int  // // ExitCode: N, the exit status the program must have (default 0)
	Cycle       bool // // lsan: cycle: the program leaks a reference cycle on purpose (D1)
	Pending     bool // // pending: ...: waits for a step that is not built yet
	NeedsLibs   bool // // pkg-config: ...: needs C libraries (the interop tests)
	NeedsScreen bool // // display: required
	StdInternal bool // // std-internal: ...: imports the standard library's internal packages
	Canary      Kind // // canary: KIND: a program that is wrong on purpose; the run must report KIND
	Build       constraint.Expr
	BuildLine   string
}

// ParseHeader reads the header of the program source src.
func ParseHeader(src string) (*Header, error) {
	h := &Header{}
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break // the package clause
		}
		switch {
		case strings.HasPrefix(line, "//go:build"):
			expr, err := constraint.Parse(line)
			if err != nil {
				return nil, fmt.Errorf("bad //go:build line: %v", err)
			}
			h.Build, h.BuildLine = expr, line
		case strings.HasPrefix(line, "// phase:"):
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "// phase:")))
			if err != nil {
				return nil, fmt.Errorf("bad %q line", line)
			}
			h.Phase = n
		case strings.HasPrefix(line, "// ExitCode:"):
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "// ExitCode:")))
			if err != nil {
				return nil, fmt.Errorf("bad %q line", line)
			}
			h.ExitCode = n
		case strings.TrimSpace(line) == "// lsan: cycle":
			h.Cycle = true
		case strings.HasPrefix(line, "// pending:"):
			h.Pending = true
		case strings.HasPrefix(line, "// pkg-config:"):
			h.NeedsLibs = true
		case strings.HasPrefix(line, "// display:"):
			h.NeedsScreen = true
		case strings.HasPrefix(line, "// canary:"):
			k := Kind(strings.TrimSpace(strings.TrimPrefix(line, "// canary:")))
			switch k {
			case ASan, UBSan, LSan, TSan:
				h.Canary = k
			default:
				return nil, fmt.Errorf("bad %q line: want asan, ubsan, lsan or tsan", line)
			}
		case strings.HasPrefix(line, "// std-internal:"):
			h.StdInternal = true
		}
	}
	return h, nil
}

// ReadHeader reads the header of the file path.
func ReadHeader(path string) (*Header, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	h, err := ParseHeader(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	return h, nil
}

// goMinor is the newest Go release tag (go1.N) that holds, as in the harness of the tests package.
const goMinor = 25

// targetTag reports whether the build tag holds for the machine that runs the programs.
func targetTag(tag string) bool {
	switch {
	case tag == runtime.GOOS, tag == runtime.GOARCH:
		return true
	case tag == "unix":
		return runtime.GOOS != "windows" && runtime.GOOS != "plan9" && runtime.GOOS != "js" && runtime.GOOS != "wasip1"
	}
	if minor, ok := strings.CutPrefix(tag, "go1."); ok {
		n, err := strconv.Atoi(minor)
		return err == nil && n >= 1 && n <= goMinor && strconv.Itoa(n) == minor
	}
	return false
}

// Skip returns why a sanitizer run does not run a program with this header, or "". phase is the
// highest phase to run; pending says to run the programs that wait for a step too.
func (h *Header) Skip(phase int, pending bool) string {
	switch {
	case h.Build != nil && !h.Build.Eval(targetTag):
		return "build-constraint"
	case h.Phase > phase:
		return "phase"
	case h.Pending && !pending:
		return "pending"
	case h.NeedsLibs:
		return "needs-libs"
	case h.NeedsScreen:
		return "needs-display"
	}
	return ""
}
