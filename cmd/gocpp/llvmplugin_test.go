package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The LLVM side of step 6.12/6.14 with the real tools, not a fake compiler: a pass plugin that counts
// the functions of a module, loaded into clang through `gocpp build -cxxflags=-fpass-plugin=...` (it
// sees every compile of the build, and counts what `-emit-llvm` writes), and `opt` / `llvm-dis`
// reading the IR that `-emit-llvm` writes. The plugin is built from testdata/llvmplugin/count.cpp with
// clang++ and the LLVM headers of the dev shell (flake.nix sets GOCPP_LLVM_BIN and GOCPP_LLVM_DEV;
// outside it the tools are looked up on PATH, and a test without them skips and says what is missing).
// g++ and its refusal of -emit-llvm are TestEmitLLVMGCCError's.

// llvmTools returns the folder of the LLVM tools (opt, llvm-dis) and the dev output of LLVM
// (include/llvm/Passes/PassPlugin.h), or skips the test.
func llvmTools(t *testing.T) (bin, dev string) {
	t.Helper()
	bin, dev = os.Getenv("GOCPP_LLVM_BIN"), os.Getenv("GOCPP_LLVM_DEV")
	if bin == "" {
		if p, err := exec.LookPath("opt"); err == nil {
			bin = filepath.Dir(p)
		}
	}
	if dev == "" {
		if p, err := exec.LookPath("llvm-config"); err == nil {
			if out, err := exec.Command(p, "--prefix").Output(); err == nil {
				dev = strings.TrimSpace(string(out))
			}
		}
	}
	switch {
	case bin == "" || !exists(filepath.Join(bin, "opt")) || !exists(filepath.Join(bin, "llvm-dis")):
		t.Skip("no LLVM tools (opt, llvm-dis): enter the dev shell (flake.nix sets GOCPP_LLVM_BIN) or put them on PATH")
	case dev == "" || !exists(filepath.Join(dev, "include", "llvm", "Passes", "PassPlugin.h")):
		t.Skip("no LLVM headers (llvm/Passes/PassPlugin.h): enter the dev shell (flake.nix sets GOCPP_LLVM_DEV) or install the dev output of LLVM")
	}
	return bin, dev
}

// buildCountPlugin builds testdata/llvmplugin/count.cpp with the clang++ of the test (LLVM is built
// without RTTI, so the plugin is too) and returns the .so. Call it before inDir: the source is found
// relative to the package folder.
func buildCountPlugin(t *testing.T, dev string) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "llvmplugin", "count.cpp"))
	if err != nil || !exists(src) {
		t.Fatalf("no plugin source: %v", err)
	}
	cx, err := build.LookCXX(context.Background(), os.Getenv("GOCPP_CXX"))
	if err != nil || cx.Family != build.Clang {
		t.Fatalf("the plugin needs clang++: %+v, %v", cx, err)
	}
	so := filepath.Join(t.TempDir(), "count.so")
	args := []string{"-std=c++17", "-shared", "-fPIC", "-fno-rtti", "-O1", "-I", filepath.Join(dev, "include"), src, "-o", so}
	if out, err := exec.Command(cx.Path, args...).CombinedOutput(); err != nil {
		t.Fatalf("clang++ %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return so
}

// llvmPaths walks the IR folder of a build and returns its files with the extension ext, by the path
// below gen/gpkg/ ("example.com/m/a/a.gp.ll").
func irFiles(t *testing.T, ir, ext string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	prefix := filepath.Join(ir, "gen", "gpkg") + string(filepath.Separator)
	filepath.WalkDir(ir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ext) {
			files[filepath.ToSlash(strings.TrimPrefix(p, prefix))] = p
		}
		return nil
	})
	return files
}

// parseCounts reads the file of the plugin: the counts of each source file (one per compile that
// loaded the plugin).
func parseCounts(t *testing.T, path string) map[string][]int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the plugin wrote no counts: %v", err)
	}
	counts := make(map[string][]int)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		file, n, ok := strings.Cut(line, "\t")
		v, err := strconv.Atoi(n)
		if !ok || err != nil {
			t.Fatalf("bad line of the plugin's file: %q", line)
		}
		counts[file] = append(counts[file], v)
	}
	return counts
}

// defines counts the functions that an IR file defines, the lines "define ..." of the text format.
func defines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return definesIn(string(data))
}

func definesIn(text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "define ") {
			n++
		}
	}
	return n
}

// A real pass plugin, loaded through -cxxflags, counts the functions of every translation unit; its
// numbers are those of the IR that -emit-llvm wrote for the same unit.
func TestPassPluginCountsFunctions(t *testing.T) {
	clangOnly(t)
	_, dev := llvmTools(t)
	so := buildCountPlugin(t, dev)
	t.Setenv("GOCPP_GATE_PHASE", "06")
	counts := filepath.Join(t.TempDir(), "counts.txt")
	t.Setenv("GOCPP_PLUGIN_COUNT_FILE", counts)
	dir := inDir(t, pkgsProject("plugin", 3))
	r := gocppRun("", false, "build", "-emit-llvm", "-cxxflags=-fpass-plugin="+so, "-o", "prog", ".")
	if r.code != 0 || strings.Contains(r.stderr, "warning") {
		t.Fatalf("gocpp build -cxxflags=-fpass-plugin=... = %+v", r)
	}
	if out, err := exec.Command(filepath.Join(dir, "prog")).Output(); err != nil || string(out) != "8 6 plugin\n" {
		t.Errorf("prog = %q, %v", out, err)
	}
	got := parseCounts(t, counts)
	ll := irFiles(t, filepath.Join(dir, "prog.llvm"), ".ll")
	units := 0
	for file, ns := range got {
		_, rel, ok := strings.Cut(filepath.ToSlash(file), "/gpkg/")
		if !ok {
			continue // the runtime's units: the flags reach them too (below)
		}
		units++
		// The object compile and the IR compile load the plugin once each, and see the same module.
		if len(ns) != 2 || ns[0] != ns[1] || ns[0] == 0 {
			t.Errorf("%s: counts %v, want the same positive number twice (object compile, IR compile)", rel, ns)
			continue
		}
		irPath, ok := ll[strings.TrimSuffix(rel, ".cpp")+".ll"]
		if !ok {
			t.Errorf("%s: no IR file among %v", rel, slices.Sorted(mapsKeys(ll)))
			continue
		}
		if want := defines(t, irPath); want != ns[0] {
			t.Errorf("%s: the plugin counted %d functions, the IR file defines %d", rel, ns[0], want)
		}
	}
	if units != 4 { // a, b, main and pkg_init of the module
		t.Errorf("the plugin saw %d generated units, want 4: %v", units, got)
	}
	if len(got) <= units {
		t.Errorf("the plugin saw no runtime unit: the flag must reach every compile: %v", got)
	}
}

func mapsKeys[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// `opt` accepts what -emit-llvm writes, as text and as bitcode, per translation unit: the verifier
// passes, llvm-dis reads the bitcode, and the same plugin loaded into opt counts what the compile
// counted. A truncated file is refused (the check does check something).
func TestEmitLLVMOptAccepts(t *testing.T) {
	clangOnly(t)
	bin, dev := llvmTools(t)
	so := buildCountPlugin(t, dev)
	t.Setenv("GOCPP_GATE_PHASE", "06")
	opt := filepath.Join(bin, "opt")
	for _, format := range []string{"ll", "bc"} {
		t.Run(format, func(t *testing.T) {
			dir := inDir(t, pkgsProject("opt", 3))
			r := gocppRun("", false, "build", "-emit-llvm="+format, "-o", "prog", ".")
			if r.code != 0 {
				t.Fatalf("gocpp build -emit-llvm=%s = %+v", format, r)
			}
			ir := irFiles(t, filepath.Join(dir, "prog.llvm"), "."+format)
			for _, want := range []string{"example.com/m/a/a.gp." + format, "example.com/m/b/b.gp." + format, "example.com/m/main.gp." + format, "example.com/m/pkg_init." + format} {
				if _, ok := ir[want]; !ok {
					t.Fatalf("no %s among %v", want, slices.Sorted(mapsKeys(ir)))
				}
			}
			for rel, path := range ir {
				if out, err := exec.Command(opt, "-passes=verify", "-disable-output", path).CombinedOutput(); err != nil || len(out) != 0 {
					t.Errorf("opt -passes=verify %s: %v\n%s", rel, err, out)
				}
				// llvm-dis reads the bitcode; the text is what the plugin's count is compared with.
				want := 0
				if format == "bc" {
					dis, err := exec.Command(filepath.Join(bin, "llvm-dis"), "-o", "-", path).Output()
					if err != nil || !strings.Contains(string(dis), "target triple") {
						t.Errorf("llvm-dis %s: %v\n%.200s", rel, err, dis)
					}
					want = definesIn(string(dis))
				} else {
					want = defines(t, path)
				}
				// The plugin as a pass of opt: it counts the function definitions of this module.
				countFile := filepath.Join(t.TempDir(), "count.txt")
				cmd := exec.Command(opt, "-load-pass-plugin="+so, "-passes=gocpp-count-functions", "-disable-output", path)
				cmd.Env = append(os.Environ(), "GOCPP_PLUGIN_COUNT_FILE="+countFile)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("opt -load-pass-plugin %s: %v\n%s", rel, err, out)
					continue
				}
				for file, ns := range parseCounts(t, countFile) {
					if len(ns) != 1 || ns[0] != want || want == 0 {
						t.Errorf("opt with the plugin on %s (module %s): counted %v, the IR defines %d", rel, file, ns, want)
					}
				}
			}
			// Not vacuous: opt refuses a broken file.
			path := ir["example.com/m/a/a.gp."+format]
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			broken := filepath.Join(t.TempDir(), "broken."+format)
			cut := data[:len(data)/2]
			if format == "ll" {
				cut = append(cut, []byte("\ndefine i32 @broken( {\n")...)
			}
			if err := os.WriteFile(broken, cut, 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(opt, "-passes=verify", "-disable-output", broken).CombinedOutput(); err == nil {
				t.Errorf("opt accepted a broken %s file: %s", format, out)
			}
		})
	}
}
