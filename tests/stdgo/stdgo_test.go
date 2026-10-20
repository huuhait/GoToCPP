// Package stdgo checks the tests of Go's standard packages that gocpp runs in the packages it ships
// (step 9.10): the converted test files in this folder, made by
// tools/portstd/gotests/convert_std.py from the pinned tarball of Go, the manifests that say what
// happened to every test function, and the counts that tools/stdgocount computes from them (the
// counting rule of the Phase 09 README, section 2.8).
//
// The converted files and the manifests are committed. TestConvertedTestsAreCurrent makes them
// again in a temporary folder and compares; it needs python3 and the pinned tarball in the cache of
// tools/portstd (it skips without them). TestSmallPackagesPass builds gocpp and runs the tests of a
// few small packages (it is skipped with -short).
package stdgo

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/stdcount"
)

var repoRoot string

// regenerate is the converter that makes the files of this folder.
const regenerate = "tools/portstd/gotests/convert_std.py"

func TestMain(m *testing.M) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot = filepath.Join(filepath.Dir(file), "..", "..")
	// The tests never use the real caches or the network.
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// manifestPackages returns the import paths that have a manifest, sorted.
func manifestPackages(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot, "tests", "stdgo", "manifest", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no manifests (%v)", err)
	}
	var pkgs []string
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		pkgs = append(pkgs, strings.ReplaceAll(name, "__", "/"))
	}
	sort.Strings(pkgs)
	return pkgs
}

func TestEverySpecificationHasManifest(t *testing.T) {
	specDir := filepath.Join(repoRoot, "tools", "portstd", "gotests", "std")
	specs, err := filepath.Glob(filepath.Join(specDir, "*.py"))
	if err != nil || len(specs) == 0 {
		t.Fatalf("no specifications (%v)", err)
	}
	var want []string
	for _, spec := range specs {
		name := strings.TrimSuffix(filepath.Base(spec), ".py")
		want = append(want, strings.ReplaceAll(name, "__", "/"))
	}
	sort.Strings(want)

	got := manifestPackages(t)

	if !slices.Equal(got, want) {
		t.Fatalf("manifests = %v; specifications = %v", got, want)
	}
}

func TestManifestsGiveReasonForEveryExclusion(t *testing.T) {
	for _, pkg := range manifestPackages(t) {
		t.Run(pkg, func(t *testing.T) {
			m, err := stdcount.LoadManifest(filepath.Join(repoRoot, "tests", "stdgo"), pkg)
			if err != nil {
				t.Fatal(err)
			}
			for file, entry := range m.Files {
				if entry.Status != "run" && entry.Reason == "" {
					t.Errorf("file %s is left out without a reason", file)
				}
			}
			for _, test := range m.Tests {
				if _, ok := m.Files[test.File]; !ok {
					t.Errorf("%s names the file %s, which the manifest does not list",
						test.Name, test.File)
				}
				if test.Status != "run" && test.Reason == "" {
					t.Errorf("%s is left out without a reason", test.Name)
				}
			}
		})
	}
}

func TestManifestsExpectOnlyFunctionsThatRun(t *testing.T) {
	classes := []string{"divergence", "bug", "unsupported", "flaky"}
	for _, pkg := range manifestPackages(t) {
		t.Run(pkg, func(t *testing.T) {
			m, err := stdcount.LoadManifest(filepath.Join(repoRoot, "tests", "stdgo"), pkg)
			if err != nil {
				t.Fatal(err)
			}
			running := map[string]bool{}
			for _, test := range m.Tests {
				if test.Status == "run" && m.Files[test.File].Status == "run" {
					running[test.Name] = true
				}
			}
			for name, expect := range m.Expected {
				if !running[name] {
					t.Errorf("the expectation for %s names a function that does not run", name)
				}
				if !slices.Contains(classes, expect.Class) {
					t.Errorf("the expectation for %s has the class %q", name, expect.Class)
				}
				if expect.Reason == "" {
					t.Errorf("the expectation for %s has no reason", name)
				}
			}
		})
	}
}

func TestManifestsListConvertedFiles(t *testing.T) {
	for _, pkg := range manifestPackages(t) {
		t.Run(pkg, func(t *testing.T) {
			m, err := stdcount.LoadManifest(filepath.Join(repoRoot, "tests", "stdgo"), pkg)
			if err != nil {
				t.Fatal(err)
			}
			for file, entry := range m.Files {
				if entry.Status != "run" {
					continue
				}
				convertedName := strings.TrimSuffix(file, ".go") + ".gp"
				converted := filepath.Join(repoRoot, "tests", "stdgo", filepath.FromSlash(pkg),
					convertedName)
				if _, err := os.Stat(converted); err != nil {
					t.Errorf("%s is taken but its converted file is missing: %v", file, err)
				}
			}
		})
	}
}

// treeFiles returns the files below root (relative, slash separated) except the Go files of this
// test.
func treeFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.Ext(rel) == ".go" && filepath.Dir(rel) == "." {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestConvertedTestsAreCurrent(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not in the PATH")
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, _ := os.UserHomeDir()
		cache = filepath.Join(home, ".cache")
	}
	if _, err := os.Stat(filepath.Join(cache, "gocpp", "portstd")); err != nil {
		t.Skip("the pinned tarball of Go is not in the cache of tools/portstd")
	}
	pkgs := manifestPackages(t)
	out := t.TempDir()
	script := filepath.FromSlash(regenerate)
	command := exec.Command(python, append([]string{"-I", script}, pkgs...)...)
	command.Dir = repoRoot
	command.Env = append(os.Environ(), "GOCPP_TEST_STDGO_OUT="+out)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("convert_std.py: %v\n%s", err, output)
	}

	want := treeFiles(t, out)
	got := treeFiles(t, filepath.Join(repoRoot, "tests", "stdgo"))

	for name, data := range want {
		have, ok := got[name]
		switch {
		case !ok:
			t.Errorf("%s is missing: run python3 %s", name, regenerate)
		case !bytes.Equal(have, data):
			t.Errorf("%s differs from what convert_std.py makes: run python3 %s", name, regenerate)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s is not made by convert_std.py any more: delete it", name)
		}
	}
}

// smallPackages are the packages whose tests build and run fastest.
var smallPackages = []string{"cmp", "path"}

func TestSmallPackagesPass(t *testing.T) {
	if testing.Short() {
		t.Skip("builds gocpp and the test binaries: skipped with -short")
	}
	work := t.TempDir()
	gocpp := filepath.Join(work, "gocpp")
	build := exec.Command("go", "build", "-o", gocpp, "./cmd/gocpp")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/gocpp: %v\n%s", err, out)
	}
	for _, pkg := range smallPackages {
		t.Run(pkg, func(t *testing.T) {
			stdgo := filepath.Join(repoRoot, "tests", "stdgo")
			m, err := stdcount.LoadManifest(stdgo, pkg)
			if err != nil {
				t.Fatal(err)
			}
			options := stdcount.RunOptions{
				Gocpp:   gocpp,
				StdGo:   stdgo,
				Work:    filepath.Join(work, pkg),
				Timeout: 5 * time.Minute,
			}

			results, log, err := stdcount.Run(context.Background(), options, m)
			if err != nil {
				t.Fatal(err)
			}

			count := stdcount.Tally(m, results)
			for _, problem := range count.Problems {
				t.Error(problem)
			}
			if count.Percent() < 90 {
				t.Errorf("%.1f %% of %d selected functions pass, under the 90 %% budget\n%s",
					count.Percent(), count.Selected, log)
			}
			if count.Selected == 0 {
				t.Errorf("%s has no selected function", pkg)
			}
		})
	}
}
