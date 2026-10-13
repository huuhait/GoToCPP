package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/version"
)

// planned is every planned command with the phase that builds it, in the
// order "gocpp help" lists them (from job 00-0.6 of the plan).
var planned = []struct {
	name  string
	phase int
}{
	{"build", 2}, {"run", 2}, {"test", 9}, {"check", 1}, {"emit", 2},
	{"expand", 10}, {"fmt", 1}, {"lsp", 11}, {"vet", 13}, {"doc", 13},
	{"debug", 13}, {"explain", 13}, {"env", 6}, {"list", 6}, {"clean", 6},
	{"get", 15}, {"bindgen", 12}, {"export", 6}, {"version", 0},
}

// gocpp runs the CLI with args and returns its exit code and output.
func gocpp(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestCommandTable(t *testing.T) {
	if len(commands) != len(planned) {
		t.Fatalf("got %d commands, want %d", len(commands), len(planned))
	}
	for i, want := range planned {
		c := commands[i]
		if c.name != want.name || c.phase != want.phase {
			t.Errorf("command %d = %s (Phase %02d), want %s (Phase %02d)",
				i, c.name, c.phase, want.name, want.phase)
		}
		if c.summary == "" || strings.Contains(c.summary, "\n") {
			t.Errorf("command %s: summary %q, want one non-empty line", c.name, c.summary)
		}
	}
}

func TestVersion(t *testing.T) {
	code, out, errOut := gocpp("version")
	if code != 0 || errOut != "" {
		t.Fatalf("gocpp version: exit %d, stderr %q; want exit 0 and no stderr", code, errOut)
	}
	if want := version.String() + "\n"; out != want {
		t.Errorf("gocpp version printed %q, want %q", out, want)
	}
	// For example "gocpp dev (phase 00) go1.25.7 linux/amd64".
	if re := regexp.MustCompile(`^gocpp \S+ \(phase \d\d\) .+ \S+/\S+\n$`); !re.MatchString(out) {
		t.Errorf("gocpp version printed %q, want it to match %s", out, re)
	}
}

func TestVersionArgs(t *testing.T) {
	code, out, errOut := gocpp("version", "extra")
	if code != 2 || out != "" || errOut != "usage: gocpp version\n" {
		t.Errorf("gocpp version extra: exit %d, stdout %q, stderr %q; want exit 2 and the usage on stderr",
			code, out, errOut)
	}
}

func TestHelp(t *testing.T) {
	for _, arg := range []string{"help", "-h", "-help", "--help"} {
		code, out, errOut := gocpp(arg)
		if code != 0 || errOut != "" {
			t.Errorf("gocpp %s: exit %d, stderr %q; want exit 0 and no stderr", arg, code, errOut)
			continue
		}
		checkUsage(t, "gocpp "+arg, out)
	}
}

func TestHelpArgs(t *testing.T) {
	code, out, errOut := gocpp("help", "build")
	if code != 2 || out != "" || errOut != "usage: gocpp help\n" {
		t.Errorf("gocpp help build: exit %d, stdout %q, stderr %q; want exit 2 and the usage on stderr",
			code, out, errOut)
	}
}

func TestNoArgs(t *testing.T) {
	// Like "go" with no arguments: the usage goes to stderr, exit 2.
	code, out, errOut := gocpp()
	if code != 2 || out != "" {
		t.Fatalf("gocpp: exit %d, stdout %q; want exit 2 and no stdout", code, out)
	}
	checkUsage(t, "gocpp", errOut)
}

// checkUsage checks that the usage lists every planned command on one line,
// in order, and marks the ones that are not built yet with their phase.
func checkUsage(t *testing.T, what, usage string) {
	t.Helper()
	_, list, ok := strings.Cut(usage, "The commands are:\n\n")
	if !ok {
		t.Fatalf("%s: no command list in the usage:\n%s", what, usage)
	}
	list, _, _ = strings.Cut(list, "\n\n")
	lines := strings.Split(list, "\n")
	if len(lines) != len(planned) {
		t.Fatalf("%s: %d command lines, want %d:\n%s", what, len(lines), len(planned), list)
	}
	for i, want := range planned {
		line := lines[i]
		if f := strings.Fields(line); !strings.HasPrefix(line, "\t") || len(f) < 2 || f[0] != want.name {
			t.Errorf("%s: command line %d is %q, want a line for %q", what, i, line, want.name)
			continue
		}
		mark := fmt.Sprintf("(Phase %02d)", want.phase)
		if lookup(want.name).run == nil {
			if !strings.HasSuffix(line, mark) {
				t.Errorf("%s: line %q does not end with %s", what, line, mark)
			}
		} else if strings.Contains(line, "(Phase") {
			t.Errorf("%s: line %q has a phase mark, but %s is built", what, line, want.name)
		}
		if len(strings.ReplaceAll(line, "\t", "        ")) > 80 {
			t.Errorf("%s: line %q is wider than 80 columns", what, line)
		}
	}
}

func TestNotAvailable(t *testing.T) {
	for _, p := range planned {
		if lookup(p.name).run != nil {
			continue
		}
		code, out, errOut := gocpp(p.name, "some", "args")
		want := fmt.Sprintf("gocpp %s: not available yet (Phase %02d)\n", p.name, p.phase)
		if code != 2 || out != "" || errOut != want {
			t.Errorf("gocpp %s: exit %d, stdout %q, stderr %q; want exit 2 and stderr %q",
				p.name, code, out, errOut, want)
		}
	}
}

func TestBuildNotAvailable(t *testing.T) {
	// The example from the job ticket, spelled out. Phase 02 builds "build".
	if lookup("build").run != nil {
		t.Skip("build is built")
	}
	code, out, errOut := gocpp("build")
	if code != 2 || out != "" || errOut != "gocpp build: not available yet (Phase 02)\n" {
		t.Errorf("gocpp build: exit %d, stdout %q, stderr %q; want exit 2 and the message on stderr",
			code, out, errOut)
	}
}

func TestUnknownCommand(t *testing.T) {
	for _, name := range []string{"frob", "Build", "-x", ""} {
		code, out, errOut := gocpp(name)
		want := "gocpp " + name + ": unknown command\nRun 'gocpp help' for usage.\n"
		if code != 2 || out != "" || errOut != want {
			t.Errorf("gocpp %q: exit %d, stdout %q, stderr %q; want exit 2 and stderr %q",
				name, code, out, errOut, want)
		}
	}
}
