package icheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The early-error inventory of Phase 12 (step 12.12, exit criterion "at least 30 early-error tests
// give clear gocpp-style messages before the C++ compile"). The cases are the reject programs of the
// zoo and their golden files, which TestRejectZoo compares with the checker's output and
// TestTypedCheckZoo (cmd/gocpp) compares with the lines where the C++ compilers fail. This test
// needs no compiler: it counts the cases and checks the words of the messages, so a golden file that
// is regenerated with -update cannot lose cases or let clang's own words through unnoticed.

// diagnosticRE is the first line of a diagnostic in a golden file ("name.gp:line:col: message").
var diagnosticRE = regexp.MustCompile(`^(\S+\.gp):(\d+):(\d+): (.*)$`)

// goldenDiagnostics returns the first lines of the diagnostics of all reject_* golden files, with
// the related lines (the tab-indented ones) that follow each.
func goldenDiagnostics(t *testing.T) (messages map[string][]string, related []string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "zoo", "reject_*.golden"))
	if err != nil {
		t.Fatal(err)
	}
	messages = map[string][]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
			if strings.HasPrefix(line, "\t") {
				related = append(related, strings.TrimSpace(line))
				continue
			}
			m := diagnosticRE.FindStringSubmatch(line)
			if m == nil {
				t.Errorf("%s: not a diagnostic line: %q", f, line)
				continue
			}
			messages[m[1]] = append(messages[m[1]], m[4])
		}
	}
	return messages, related
}

func TestEarlyErrorInventory(t *testing.T) {
	messages, _ := goldenDiagnostics(t)
	programs := len(messages)
	diagnostics := 0
	for _, ms := range messages {
		diagnostics += len(ms)
	}
	// An early-error test is one diagnostic at one line of a reject program (each is a statement of
	// its own, and TestTypedCheckZoo checks that the C++ build fails at that line).
	if diagnostics < 30 {
		t.Errorf("%d early errors in the golden files, want at least 30", diagnostics)
	}
	if programs < 20 {
		t.Errorf("%d reject programs have diagnostics, want at least 20", programs)
	}
	// Every reject program has a golden file with at least one error, and the other way round.
	progs, err := filepath.Glob(filepath.Join("testdata", "zoo", "reject_*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range progs {
		if len(messages[filepath.Base(p)]) == 0 {
			t.Errorf("%s has no diagnostic in its golden file", p)
		}
	}
	t.Logf("%d reject programs, %d early errors", programs, diagnostics)
}

// A message is in Go's words: the compiler's own spellings (quoted names, "aka", "candidate",
// the probe's helper names) are translated or left out, and the only place where C++ scope syntax
// shows is the name of a header declaration that is missing.
func TestEarlyErrorWords(t *testing.T) {
	messages, related := goldenDiagnostics(t)
	banned := []string{
		"declval", "gp_probe", "gocpp_probe", "gocpp_probe_internal", "(aka ",
		"candidate function", "candidate template", "lvalue of type", "rvalue of type",
		"no viable", "expected expression", "clang",
	}
	cppScope := regexp.MustCompile(`\w::\w`)
	check := func(where, msg string) {
		for _, w := range banned {
			if strings.Contains(msg, w) {
				t.Errorf("%s: %q has clang's own word %q", where, msg, w)
			}
		}
	}
	for name, ms := range messages {
		for _, msg := range ms {
			check(name, msg)
			missingName := strings.Contains(msg, "the imported headers declare no ::")
			if cppScope.MatchString(msg) && !missingName {
				t.Errorf("%s: %q shows C++ scope syntax", name, msg)
			}
		}
	}
	for _, msg := range related {
		check("a related line", msg)
	}
}
