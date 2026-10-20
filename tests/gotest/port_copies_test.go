package gotest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// portCopies are the fixtures that run Go's own tests of a ported package in a copy of the port (tools/portstd/gotests): the folder of
// testdata and the import path of the package.
var portCopies = map[string]string{
	"container_heap_internal":       "container/heap",
	"container_list_internal":       "container/list",
	"container_ring_internal":       "container/ring",
	"encoding_base64_internal":      "encoding/base64",
	"encoding_binary_internal":      "encoding/binary",
	"encoding_hex_internal":         "encoding/hex",
	"encoding_json_internal":        "encoding/json",
	"hash_crc32_internal":           "hash/crc32",
	"hash_fnv_internal":             "hash/fnv",
	"internal_chacha8rand_internal": "internal/chacha8rand",
	"log_internal":                  "log",
	"math_rand_internal":            "math/rand",
	"math_rand_v2_internal":         "math/rand/v2",
	"path_filepath_internal":        "path/filepath",
	"text_tabwriter_internal":       "text/tabwriter",
}

// TestPortCopies checks that the copy of each ported package in its fixture (the package that Go's internal tests, which name unexported
// functions and fields, run in) is the port in stdlib/, file for file, and that the other files of the fixture are the converted tests
// and the shim packages of tools/portstd/gotests.
func TestPortCopies(t *testing.T) {
	for name, pkg := range portCopies {
		t.Run(name, func(t *testing.T) {
			fix := filepath.Join(repoRoot, "tests", "gotest", "testdata", name)
			port := filepath.Join(repoRoot, "stdlib", filepath.FromSlash(pkg))
			var pp struct { // the changes of the copy that convert.py makes (imports of internal/ packages) and the other ports it copies
				Patches [][2]string `json:"patches"`
				Extra   [][2]string `json:"extra"`
			}
			if data, err := os.ReadFile(filepath.Join(fix, "portpatches.json")); err == nil {
				if err := json.Unmarshal(data, &pp); err != nil {
					t.Fatal(err)
				}
			}
			patches := pp.Patches
			for _, ex := range pp.Extra {
				checkCopy(t, filepath.Join(repoRoot, "stdlib", filepath.FromSlash(ex[0])), filepath.Join(fix, filepath.FromSlash(ex[1])), patches)
			}
			checkCopy(t, port, fix, patches)
			checkShims(t, fix, name)
			ents, err := os.ReadDir(fix)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range ents {
				n := e.Name()
				if _, err := os.Stat(filepath.Join(port, n)); err == nil || e.IsDir() {
					continue
				}
				if !strings.HasSuffix(n, "_test.gp") && n != "args" && n != "gocpp.mod" && n != "portpatches.json" {
					t.Errorf("%s is neither a file of the port nor a converted test", n)
				}
			}
		})
	}
}

// checkCopy checks that the .gp files of the port folder are in the copy, byte for byte after the patches of the copy.
func checkCopy(t *testing.T, port, copyDir string, patches [][2]string) {
	t.Helper()
	ports, err := filepath.Glob(filepath.Join(port, "*.gp"))
	if err != nil || len(ports) == 0 {
		t.Fatalf("no port files in %s (%v)", port, err)
	}
	for _, p := range ports {
		fn := filepath.Base(p)
		want, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, pp := range patches {
			want = []byte(strings.ReplaceAll(string(want), pp[0], pp[1]))
		}
		got, err := os.ReadFile(filepath.Join(copyDir, fn))
		if err != nil {
			t.Errorf("%s: %v (run python3 tools/portstd/gotests/convert.py)", fn, err)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("%s differs from %s/%s: run python3 tools/portstd/gotests/convert.py", filepath.Join(copyDir, fn), port, fn)
		}
	}
}

// checkShims checks that the shim packages in a fixture (tools/portstd/gotests/shim: _shared/NAME for the ones a package names in SHARED, and a
// folder with the name of the package for its own) are what the generator has: a file of the fixture at the path of a shim file has its text.
func checkShims(t *testing.T, fix, name string) {
	t.Helper()
	root := filepath.Join(repoRoot, "tools", "portstd", "gotests", "shim")
	key := strings.ReplaceAll(portCopies[name], "/", "__")
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		folder, dest, ok := strings.Cut(rel, string(filepath.Separator))
		if !ok || folder != "_shared" && folder != key {
			return nil // a shim of another package
		}
		got, err := os.ReadFile(filepath.Join(fix, dest))
		if err != nil {
			return nil // this fixture does not use that shim
		}
		want, _ := os.ReadFile(p)
		if string(got) != string(want) {
			t.Errorf("%s differs from %s: run python3 tools/portstd/gotests/convert.py %s", filepath.Join(fix, dest), p, portCopies[name])
		}
		return nil
	})
}
