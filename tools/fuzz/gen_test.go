package fuzz

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/tools/fuzz/fuzzkit"
	"github.com/huuhait/gocpp/tools/fuzz/gen"
)

// The programs of the generator (tools/fuzz/gen): valid Go that uses the types, closures, defer,
// generics, channels and goroutines of the language, as seeds of the fuzz targets and as a
// target of their own.

// genSeeds returns a few generated programs, the seeds of the targets.
func genSeeds() [][]byte {
	var out [][]byte
	for seed := uint64(1); seed <= 8; seed++ {
		out = append(out, gen.FromSeed(seed, gen.Default()))
	}
	return out
}

// FuzzGen: every program the generator makes from the bytes the engine gives is a valid
// program: the checker accepts it (a refusal is a bug of the generator or of the checker, and a
// finding either way), and the rest is FuzzEmit's checks.
func FuzzGen(f *testing.F) {
	for seed := uint64(1); seed <= 8; seed++ {
		f.Add([]byte(strconv.FormatUint(seed*7919, 10)))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}
		var src []byte
		fuzzkit.Guard("FuzzGen: the generator", func() { src = gen.FromBytes(data, gen.Default()) })
		genChecks(t, "FuzzGen", src)
	})
}

// genCheckProblem checks that the generated program src checks cleanly in gocpp (the generator's
// programs are valid Go), and returns what is wrong, or "". A known gap of the checker is no
// problem.
func genCheckProblem(src []byte, what string) string {
	var r *fuzzkit.Result
	fuzzkit.Guard(what, func() { r = fuzzkit.Run(src, false) })
	if !r.Checked && len(r.Errors) > 0 && knownCheckGap(r.Errors) == "" {
		return "the generated program does not check:\n" + strings.Join(r.Errors, "\n")
	}
	return ""
}

// genChecks makes the checks of FuzzEmit, and also requires that the program checks cleanly.
func genChecks(t *testing.T, what string, src []byte) {
	t.Helper()
	if msg := genCheckProblem(src, what); msg != "" {
		t.Fatalf("%s\n--- program\n%s", msg, src)
	}
	emitChecks(t, what, src)
}

// TestSeededBugs: a defect put into a generated program, in each of the new constructs, is
// reported by the check that FuzzGen makes (a generator whose programs were not checked, or a
// check that accepted anything, would let it through). The program is found among the first
// seeds by a pattern of the construct, and the edit makes it invalid.
func TestSeededBugs(t *testing.T) {
	bugs := []struct {
		name, pattern, from, to string
	}{
		{"forwarding with an extra argument", `\bf\d+\(f\d+\(`, "(f", "(1, f"},
		{"forwarding into the wrong parameters", `\bf\d+\(f\d+\([^()]*\)\)`, "(f", "(\"x\", f"},
		{"iota constant of a string", `= iota`, "= iota", `= iota + "x"`},
		{"shift by a float", `<< \(uint\(`, "<< (uint(", "<< (float64("},
		{"goto to a label that is not there", `goto G\d+`, "goto G", "goto Gnone"},
		{"three-index slice with a string index", `\w\[\d*:0:1\]`, ":0:", ":\"a\":"},
		{"method expression of a method that does not exist", `(?m):= (\(\*)?[TNI]\d+\)?\.m\d+$`, ").m", ").nonesuch"},
		{"rune literal of a string", `'a'`, "'a'", `"a" + 1`},
	}
	for _, b := range bugs {
		rx := regexp.MustCompile(b.pattern)
		done := false
		for seed := uint64(1); seed <= 400 && !done; seed++ {
			src := gen.FromSeed(seed, gen.Default())
			loc := rx.FindIndex(src)
			if loc == nil {
				continue
			}
			text := string(src[loc[0]:loc[1]])
			if !strings.Contains(text, b.from) {
				continue
			}
			bad := append(append(append([]byte{}, src[:loc[0]]...), strings.Replace(text, b.from, b.to, 1)...), src[loc[1]:]...)
			if genCheckProblem(src, "TestSeededBugs") != "" {
				t.Fatalf("%s: seed %d does not check before the bug is put in", b.name, seed)
			}
			if genCheckProblem(bad, "TestSeededBugs") == "" {
				t.Errorf("%s: seed %d: the bug is not reported:\n%s", b.name, seed, text)
			}
			done = true
		}
		if !done {
			t.Errorf("%s: no program of the first 400 seeds has the construct (pattern %s)", b.name, b.pattern)
		}
	}
}

// TestGenerated sweeps programs of the generator through check, lower and emit, as an ordinary
// test (GOCPP_GEN_N programs; 60 by default).
func TestGenerated(t *testing.T) {
	n := 60
	if v, err := strconv.Atoi(os.Getenv("GOCPP_GEN_N")); err == nil {
		n = v
	}
	skipped := 0
	for seed := uint64(1); seed <= uint64(n); seed++ {
		src := gen.FromSeed(seed, gen.Default())
		ok := t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			genChecks(t, "TestGenerated", src)
		})
		if !ok && testing.Short() {
			return
		}
	}
	_ = skipped
}
