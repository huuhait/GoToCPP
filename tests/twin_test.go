package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A test that says "// std-internal:" imports packages that only the standard library may import
// (internal/bytealg, ...), so real Go cannot build it as it is. For TestRunVsGo the harness builds
// it in a module with the same packages next to it: the .gp files of stdlib/internal/<name> copied
// as .go files, imported as difftest/internal/<name>. These packages are Go's own code, so real Go
// compiles them, with one exception: internal/bytealg's native.gp (C++ in gocpp) is replaced by
// testdata/twin/bytealg_native.go.txt, which calls bytes and strings. The output of the Go program
// is then what Go computes, and the test's // Output: block has to equal it.

var internalImport = regexp.MustCompile(`"((?:testing/)?internal/[a-z0-9]+)"`)

// twinPath is the folder of a twin in the module: the package itself, except that testing/internal/x becomes
// internal/testing_x (Go lets only the code under the parent of an internal folder import it, and the program is the root).
func twinPath(pkg string) string {
	return strings.Replace(pkg, "testing/internal/", "internal/testing_", 1)
}

// twinNative lists the packages of stdlib/internal that have natives: the file of the twin that
// replaces the one with the natives.
var twinNative = map[string]struct{ replaced, with string }{
	"internal/bytealg":          {"native.gp", "bytealg_native.go.txt"},
	"testing/internal/testdeps": {"native.gp", "testdeps_native.go.txt"},
}

// writeTwins copies the internal packages that src imports (and the ones those import) into dir,
// and returns src with the imports renamed to the module difftest.
func writeTwins(dir string, src []byte) ([]byte, error) {
	done := make(map[string]bool)
	var queue []string
	add := func(text []byte) {
		for _, m := range internalImport.FindAllSubmatch(text, -1) {
			if p := string(m[1]); !done[p] {
				done[p] = true
				queue = append(queue, p)
			}
		}
	}
	add(src)
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		from := filepath.Join(repoRoot, "stdlib", filepath.FromSlash(pkg))
		files, err := filepath.Glob(filepath.Join(from, "*.gp"))
		if err != nil || len(files) == 0 {
			return nil, fmt.Errorf("twin of %s: no .gp files in %s", pkg, from)
		}
		to := filepath.Join(dir, filepath.FromSlash(twinPath(pkg)))
		if err := os.MkdirAll(to, 0o755); err != nil {
			return nil, err
		}
		tw, hasTwin := twinNative[pkg]
		for _, f := range files {
			base := filepath.Base(f)
			if hasTwin && base == tw.replaced {
				continue
			}
			text, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			add(text)
			out := filepath.Join(to, strings.TrimSuffix(base, ".gp")+".go")
			if err := os.WriteFile(out, renameInternal(text), 0o644); err != nil {
				return nil, err
			}
		}
		if hasTwin {
			text, err := os.ReadFile(filepath.Join(repoRoot, "tests", "testdata", "twin", tw.with))
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(to, "twin_native.go"), text, 0o644); err != nil {
				return nil, err
			}
		}
	}
	return renameInternal(src), nil
}

// renameInternal makes the imports of internal packages the imports of the twins in the module.
func renameInternal(text []byte) []byte {
	return internalImport.ReplaceAllFunc(text, func(m []byte) []byte {
		return []byte(`"difftest/` + twinPath(string(m[1:len(m)-1])) + `"`)
	})
}
