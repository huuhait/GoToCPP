package typedinterop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// mappedPrograms are the programs of tests/interop whose mapped results are goldens.
var mappedPrograms = []string{
	"cxx_containers", "cookbook_std_containers", "c_operators", "cmath_funcs", "stdio_calls", "string_ops",
	"map_ops", "template_or_index", "range_containers", "header_namespace", "pointer_chains", "macro_collisions",
	"c_unions", "c_bitfields", "cxx_inheritance", "cxx_features", "helper_header", "cookbook_sqlite", "cookbook_zlib",
}

// dumpMapped is the text form of the mapped results of a report: each
// reference's key, the probe's type and the foreign view.
func dumpMapped(rep *Report) string {
	var b strings.Builder
	for _, ref := range rep.Set.Refs {
		r := rep.Table.Result(ref.ID)
		fmt.Fprintf(&b, "%s\n    %s\n    => %s\n", ref.Key, describe(r), rep.Values[ref.ID])
	}
	if rep.Decls != nil {
		var names []string
		for k := range rep.Decls.Records {
			names = append(names, "record "+k)
		}
		for k := range rep.Decls.Enums {
			names = append(names, "enum "+k)
		}
		for k := range rep.Decls.Funcs {
			names = append(names, "func "+k)
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "\ndeclarations: %s\n", strings.Join(names, ", "))
		var errs []string
		for k, v := range rep.Decls.Errors {
			errs = append(errs, k+": "+v)
		}
		sort.Strings(errs)
		for _, e := range errs {
			fmt.Fprintf(&b, "no answer: %.110s\n", e)
		}
	}
	return b.String()
}

func analyzeDecls(t testing.TB, root, name string, rn *probe.Runner) *Report {
	t.Helper()
	p, fset := tsupport.CheckDir(t, root, name+".gp")
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Skipf("the program doesn't check here (a missing library?): %v", errs[0])
	}
	rep, err := Analyze(context.Background(), p, fset, Options{Clang: tsupport.Clang(t), Runner: rn, Decls: true, DeclLimits: probe.DeclConfig{Parallel: 4}})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return rep
}

func mappedGoldenDir() string { return filepath.Join("testdata", "mapped") }

// The mapped results of nineteen programs of tests/interop are goldens: for each
// reference its probe result and its foreign view, then the declarations that
// were extracted. A change of the mapper, the extraction or the probe shows up as a diff
// to review (run with -update).
func TestMappedGoldens(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "tests", "interop"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range mappedPrograms {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rep := analyzeDecls(t, root, name, new(probe.Runner))
			got := dumpMapped(rep)
			path := filepath.Join(mappedGoldenDir(), name+".txt")
			if *update {
				if err := os.MkdirAll(mappedGoldenDir(), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if string(want) != got {
				t.Errorf("the mapped results changed (run with -update and review the diff)\n%s", firstDiff(string(want), got))
			}
		})
	}
}

// opaqueReasons are the reasons that a probed result may have no foreign view
// (everything else is a gap of the mapper, and fails the sweep).
var opaqueReasons = []string{
	"pointer to member", "a type with no name", "has no gocpp view", "pointer to a type with no name", "array of a type with no name",
	"a dependent type", "<",
}

func okReason(r string) bool {
	for _, p := range opaqueReasons {
		if strings.Contains(r, p) {
			return true
		}
	}
	return false
}

// sweepPrograms are the programs for the invariants: the header zoo and the
// goldens' programs.
func sweepPrograms(t testing.TB) (root string, names []string) {
	zoo := zooDir(t)
	var all []string
	for _, n := range zooPrograms(t) {
		all = append(all, "zoo:"+strings.TrimSuffix(n, ".gp"))
	}
	for _, n := range mappedPrograms {
		all = append(all, "interop:"+n)
	}
	return zoo, all
}

// For every program that opaque mode accepts, the typed view agrees with it:
//   - the mapper reads every type clang printed (ctype), and a result is opaque only for the
//     listed reasons;
//   - no value goes where its type cannot go (Compat): the sites of a program that compiles
//     in opaque mode have a Go target that the C++ type converts to, so a "no" would be a
//     typed-mode error that the opaque build does not have;
//   - the extraction has an answer for every class and enum the results name, or says why not.
func TestMappedInvariants(t *testing.T) {
	_, names := sweepPrograms(t)
	interop, _ := filepath.Abs(filepath.Join("..", "..", "tests", "interop"))
	for _, n := range names {
		kind, name, _ := strings.Cut(n, ":")
		root := zooDir(t)
		if kind == "interop" {
			root = interop
		}
		t.Run(n, func(t *testing.T) {
			t.Parallel()
			rep := analyzeDecls(t, root, name, new(probe.Runner))
			var checked, targets, opaque int
			for _, ref := range rep.Set.Refs {
				r := rep.Table.Result(ref.ID)
				v := rep.Values[ref.ID]
				if !r.OK {
					continue
				}
				checked++
				if v.Kind == ftype.Opaque {
					opaque++
					if !okReason(v.Reason) {
						t.Errorf("%s: %q is opaque for a reason that is a gap: %s", name, r.Type, v.Reason)
					}
					continue
				}
				for _, s := range ref.Sites {
					if s.Target == "" {
						continue
					}
					targets++
					if got := v.Compat(s.Target); got == ftype.No {
						t.Errorf("%s:%d: a %s (%s) cannot go to %s (%s), but opaque mode accepts it", s.File, s.Line, v, r.Type, s.Target, s.Ctx)
					}
				}
			}
			if rep.Decls != nil {
				for k, e := range rep.Decls.Errors {
					if strings.Contains(e, "a name that cannot be used") || strings.Contains(e, "larger than the limit") || strings.Contains(e, "no type named") ||
						strings.Contains(e, "unknown type name") || strings.Contains(e, "undeclared") || strings.Contains(e, "not a class") || strings.Contains(e, "no template named") ||
						strings.Contains(e, "is not a class, namespace") || strings.Contains(e, "no type") {
						continue
					}
					t.Errorf("%s: no answer for %s: %s", name, k, e)
				}
			}
			t.Logf("%s: %d refs, %d probed, %d opaque, %d targets checked, %d declaration runs", name, len(rep.Set.Refs), checked, opaque, targets, rep.Decls.Runs)
		})
	}
}

// Two analyses of the same program give the same answer, whatever the order of the goroutines.
func TestMappedDeterministic(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", "..", "tests", "interop"))
	for _, name := range []string{"cxx_inheritance", "header_namespace", "c_unions"} {
		t.Run(name, func(t *testing.T) {
			a := dumpMapped(analyzeDecls(t, root, name, new(probe.Runner)))
			b := dumpMapped(analyzeDecls(t, root, name, new(probe.Runner)))
			if a != b {
				t.Errorf("two runs differ:\n%s", firstDiff(a, b))
			}
		})
	}
}
