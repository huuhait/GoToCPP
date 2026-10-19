package fuzzkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// RepoRoot returns the folder of the repository (found from the place of this file).
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// maxSeed is the size of the biggest seed: the engine mutates every input it gets, and a big
// one only makes every run slow.
const maxSeed = 24 << 10

var (
	seedsMu    sync.Mutex
	seedsCache = map[string][][]byte{}
)

// Seeds returns the contents of the files under the folders (relative to the repository) with
// the extensions exts, sorted by path, without the files bigger than 24 KB and the folders called
// testdata that hold broken inputs on purpose only when withTestdata is false.
func Seeds(folders []string, exts ...string) [][]byte {
	key := strings.Join(folders, ",") + "|" + strings.Join(exts, ",")
	seedsMu.Lock()
	defer seedsMu.Unlock()
	if s, ok := seedsCache[key]; ok {
		return s
	}
	var out [][]byte
	root := RepoRoot()
	for _, f := range folders {
		filepath.WalkDir(filepath.Join(root, f), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !slices.Contains(exts, filepath.Ext(p)) {
				return nil
			}
			if info, err := d.Info(); err != nil || info.Size() > maxSeed {
				return nil
			}
			if b, err := os.ReadFile(p); err == nil {
				out = append(out, b)
			}
			return nil
		})
	}
	seedsCache[key] = out
	return out
}

// GPSeeds returns the .gp files of the test programs, the examples and the standard library.
func GPSeeds() [][]byte {
	return Seeds([]string{"tests", "examples", "stdlib"}, ".gp")
}

// ProgramSeeds returns the .gp files of tests/ and examples/ that are whole programs (a main
// package): the seeds of the targets that go on to emit the C++.
func ProgramSeeds() [][]byte {
	var out [][]byte
	for _, s := range Seeds([]string{"tests", "examples"}, ".gp") {
		if strings.Contains(string(s), "package main") {
			out = append(out, s)
		}
	}
	return out
}
