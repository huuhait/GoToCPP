package icheck

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// The zoo of the checker (testdata/zoo): programs the checker must reject, with the
// exact messages (the .golden files), and programs it must accept. The same
// programs are built by the C++ compilers in cmd/gocpp (TestTypedCheckZoo): the
// rejected ones do not compile, the accepted ones do, with clang++ and g++.

func zooPath(t testing.TB) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "zoo"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// pkgConfigLibs lists the libraries named by the //gocpp:pkg-config lines of a
// program.
func pkgConfigLibs(t testing.TB, dir, name string) []string {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	var libs []string
	for _, l := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(l, "//gocpp:pkg-config "); ok {
			libs = append(libs, strings.Fields(rest)...)
		}
	}
	return libs
}

func needLibs(t *testing.T, dir, name string) {
	t.Helper()
	for _, lib := range pkgConfigLibs(t, dir, name) {
		if err := exec.Command("pkg-config", "--exists", lib).Run(); err != nil {
			t.Skipf("pkg-config does not know %s (the dev shell has it)", lib)
		}
	}
}

// runCheck checks the program name of the zoo and runs the checker on it.
func runCheck(t *testing.T, dir, name string, fam build.Family) *Result {
	t.Helper()
	clang := tsupport.Clang(t)
	needLibs(t, dir, name)
	p, fset := tsupport.CheckDir(t, dir, name)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("%s does not pass the opaque check: %v", name, errs[0])
	}
	res, err := Check(context.Background(), p, fset, Options{
		Analyze: typedinterop.Options{Clang: clang, Runner: new(probe.Runner)},
		Build:   fam,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Skipped != "" {
		t.Fatalf("%s was skipped: %s", name, res.Skipped)
	}
	return res
}

// format writes the diagnostics of a result in the form of the golden files:
// the position (file name only), the message, and the related lines that are in
// the zoo's own files (those in system headers depend on the library's version).
func format(dir string, ds []diag.Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		sev := ""
		if d.Severity == diag.Warning {
			sev = "warning: "
		}
		fmt.Fprintf(&b, "%s:%d:%d: %s%s\n", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Pos.Column, sev, d.Msg)
		for _, r := range d.Related {
			switch {
			case r.Pos.Filename == "":
				fmt.Fprintf(&b, "\t%s\n", r.Msg)
			case strings.HasPrefix(r.Pos.Filename, dir) || strings.HasPrefix(r.Pos.Filename, "./"):
				fmt.Fprintf(&b, "\t%s:%d:%d: %s\n", filepath.Base(r.Pos.Filename), r.Pos.Line, r.Pos.Column, r.Msg)
			}
		}
	}
	return b.String()
}

func zooFiles(t testing.TB, prefix string) []string {
	m, err := filepath.Glob(filepath.Join(zooPath(t), prefix+"*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(m)
	var out []string
	for _, f := range m {
		out = append(out, filepath.Base(f))
	}
	return out
}

// TestRejectZoo: the checker rejects each program of the zoo with these messages
// (clang++ builds: errors; g++ builds: the same messages as warnings).
func TestRejectZoo(t *testing.T) {
	dir := zooPath(t)
	names := zooFiles(t, "reject_")
	if len(names) < 15 {
		t.Fatalf("only %d reject programs", len(names))
	}
	total := 0
	for _, name := range names {
		t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
			t.Parallel()
			res := runCheck(t, dir, name, build.Clang)
			got := format(dir, res.Diags)
			golden := filepath.Join(dir, strings.TrimSuffix(name, ".gp")+".golden")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("no golden file (run with -update): %v", err)
			}
			if got != string(want) {
				t.Errorf("diagnostics of %s:\n%s\nwant:\n%s", name, got, want)
			}
			if res.Errors() != len(res.Diags) || len(res.Diags) == 0 {
				t.Errorf("%d diagnostics, %d errors", len(res.Diags), res.Errors())
			}
			// With g++ the same messages are warnings.
			wres := runCheck(t, dir, name, build.GCC)
			if wres.Errors() != 0 || len(wres.Diags) != len(res.Diags) {
				t.Errorf("g++: %d diagnostics, %d errors; want %d warnings", len(wres.Diags), wres.Errors(), len(res.Diags))
			}
			for i := range wres.Diags {
				if i < len(res.Diags) && wres.Diags[i].Msg != res.Diags[i].Msg {
					t.Errorf("g++ message %q differs from %q", wres.Diags[i].Msg, res.Diags[i].Msg)
				}
			}
		})
	}
	_ = total
}

// TestAcceptZoo: nothing is reported for the programs of the zoo that the C++
// compilers accept, and the checker looked at their expressions.
func TestAcceptZoo(t *testing.T) {
	dir := zooPath(t)
	names := zooFiles(t, "accept_")
	if len(names) < 5 {
		t.Fatalf("only %d accept programs", len(names))
	}
	for _, name := range names {
		t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
			t.Parallel()
			res := runCheck(t, dir, name, build.Clang)
			if len(res.Diags) != 0 {
				t.Errorf("diagnostics:\n%s", format(dir, res.Diags))
			}
			if res.Stats.Compiled < 8 {
				t.Errorf("only %d probes compiled: the program is not much of a test (%+v)", res.Stats.Compiled, res.Stats)
			}
		})
	}
}

// TestGoRuleZoo pins the one place where typed mode rejects what opaque mode builds: Go's own
// typing rules reject a void C++ call used as a value (assignment to blank, a switch tag, an
// argument, a declaration), and the checker gives the same error in every context, also for a
// member call. A change of this rule must be deliberate (ADR-079).
func TestGoRuleZoo(t *testing.T) {
	dir := zooPath(t)
	for _, name := range zooFiles(t, "gorule_") {
		t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
			res := runCheck(t, dir, name, build.Clang)
			got := format(dir, res.Diags)
			golden := filepath.Join(dir, strings.TrimSuffix(name, ".gp")+".golden")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("no golden file (run with -update): %v", err)
			}
			if got != string(want) {
				t.Errorf("diagnostics of %s:\n%s\nwant:\n%s", name, got, want)
			}
		})
	}
}
