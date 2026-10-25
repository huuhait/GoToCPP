package rtfiles_test

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	rtfiles "github.com/huuhait/gocpp/runtime"
)

// skipEntry reports whether the walks of runtime/ in these tests leave out the entry p: a name
// that starts with "." (an editor's or a tool's file: an Emacs lock .#rt.cpp, which is a
// symbolic link that points nowhere, or clangd's .cache/; //go:embed leaves them out too),
// and a symbolic link that points nowhere. For a folder, the walk then skips the whole folder.
func skipEntry(fsys fs.FS, p string, d fs.DirEntry) bool {
	if p != "." && strings.HasPrefix(d.Name(), ".") {
		return true
	}
	if d.Type()&fs.ModeSymlink != 0 {
		if _, err := fs.Stat(fsys, p); err != nil {
			return true
		}
	}
	return false
}

// skipped returns what a WalkDir function returns for an entry that skipEntry leaves out.
func skipped(d fs.DirEntry) error {
	if d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

// TestFSHasEveryFile checks that the embedded FS holds exactly the files of include/ and src/
// on disk. A file that the //go:embed directory patterns skip (a name that starts with "."
// "_") fails here until someone renames it or decides it doesn't ship. Names that start with
// "." are editor and tool files, which this test and //go:embed both leave out.
func TestFSHasEveryFile(t *testing.T) {
	var want []string
	for _, dir := range []string{"include", "src"} {
		disk := os.DirFS(".")
		err := fs.WalkDir(disk, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if skipEntry(disk, p, d) {
				return skipped(d)
			}
			if d.IsDir() {
				return nil
			}
			want = append(want, p)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	err := fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		got = append(got, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the embedded FS holds\n\t%s\nwant the files of include/ and src/\n\t%s\n(files.go embeds the folders include and src; a name that starts with _ is skipped)",
			strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
	for _, p := range []string{"include/gocpp/rt.hpp", "include/gocpp/nofortify.h"} {
		if _, err := fs.Stat(rtfiles.FS(), p); err != nil {
			t.Errorf("the embedded FS has no %s: %v", p, err)
		}
	}
}

func TestSources(t *testing.T) {
	linux := rtfiles.Sources("linux")
	if !slices.Contains(linux, "src/rt.cpp") || !slices.Contains(linux, "src/os/linux/os.cpp") {
		t.Errorf(`Sources("linux") = %q, want src/rt.cpp and src/os/linux/os.cpp among them`, linux)
	}
	if !slices.IsSorted(linux) {
		t.Errorf(`Sources("linux") = %q is not sorted`, linux)
	}
	for _, p := range linux {
		if _, err := fs.Stat(rtfiles.FS(), p); err != nil {
			t.Errorf(`Sources("linux") has %s, which is not in FS: %v`, p, err)
		}
	}
	// An OS without an OS layer gets only the portable files.
	for _, p := range rtfiles.Sources("plan9") {
		if strings.HasPrefix(p, "src/os/") {
			t.Errorf(`Sources("plan9") has %s`, p)
		}
	}
}

// TestGCSources checks the GC engine's sources (job 16.1c): they are the .cpp files under
// src/gc/, they are in FS, they are sorted, and they are in no program that the ARC engine
// builds (Sources), because a .gp program must link no gp::gc symbol.
func TestGCSources(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		srcs := rtfiles.GCSources(goos)
		for _, want := range []string{"src/gc/arith.cpp", "src/gc/heap_bdwgc.cpp", "src/gc/heap_malloc.cpp", "src/gc/print.cpp", "src/gc/string.cpp"} {
			if !slices.Contains(srcs, want) {
				t.Errorf("GCSources(%s) = %q lacks %s", goos, srcs, want)
			}
		}
		if !slices.IsSorted(srcs) {
			t.Errorf("GCSources(%s) = %q is not sorted", goos, srcs)
		}
		for _, p := range srcs {
			if !strings.HasPrefix(p, "src/gc/") {
				t.Errorf("GCSources(%s) has %s, which is not under src/gc/", goos, p)
			}
			if _, err := fs.Stat(rtfiles.FS(), p); err != nil {
				t.Errorf("GCSources(%s) has %s, which is not in FS: %v", goos, p, err)
			}
			if slices.Contains(rtfiles.Sources(goos), p) {
				t.Errorf("Sources(%s) has %s: a .gp program would link the GC engine's runtime", goos, p)
			}
		}
	}
}

// TestTestingSources checks the two lists: the files of package testing are not among the sources of
// a program, and they are the sources of the programs that import testing.
func TestTestingSources(t *testing.T) {
	// Every OS layer has its part of the natives of package testing (Windows had none until step
	// 14.9: a test binary did not link there).
	for _, goos := range []string{"linux", "windows"} {
		var testingFiles []string
		for _, p := range rtfiles.TestingSources(goos) {
			if !strings.HasPrefix(path.Base(p), "testing") {
				t.Errorf("TestingSources has %s", p)
			}
			testingFiles = append(testingFiles, p)
		}
		for _, want := range []string{"src/testing.cpp", "src/testing_alloc.cpp", "src/os/" + goos + "/testing_os.cpp"} {
			if !slices.Contains(testingFiles, want) {
				t.Errorf("TestingSources(%s) = %q lacks %s", goos, testingFiles, want)
			}
		}
		for _, p := range rtfiles.Sources(goos) {
			if strings.HasPrefix(path.Base(p), "testing") {
				t.Errorf("Sources has %s: a program that does not import testing would be linked with it", p)
			}
		}
	}
}

// paramTypes reduces a parameter list to its types: "gp::string name, int64_t start" becomes
// "gp::string,int64_t" (the names may differ between the header and a definition).
func paramTypes(params string) string {
	var out []string
	for _, p := range strings.Split(params, ",") {
		f := strings.Fields(p)
		if len(f) > 1 {
			f = f[:len(f)-1]
		}
		out = append(out, strings.Join(f, " "))
	}
	return strings.Join(out, ",")
}

// TestTestingNativesDefined checks that every function that gocpp/testing.hpp declares in
// gp::testing has a definition with the same parameter types in the sources that a test binary is
// linked with, for every OS layer: a function that exists only for Linux makes `gocpp test` fail
// to link on Windows (undefined reference to gp::testing::cpu_name, make_temp_dir, set_env: step
// 14.9), and so does an overload with other parameters.
func TestTestingNativesDefined(t *testing.T) {
	header, err := fs.ReadFile(rtfiles.FS(), "include/gocpp/testing.hpp")
	if err != nil {
		t.Fatal(err)
	}
	decl := regexp.MustCompile(`(?m)^(?:\[\[noreturn\]\] )?[\w:<>, ]+? \**(\w+)\(([^;{]*)\)\s*(?:noexcept)?;`)
	body := string(header[strings.Index(string(header), "namespace gp::testing {"):])
	type fn struct{ name, params string }
	var fns []fn
	for _, m := range decl.FindAllStringSubmatch(body, -1) {
		fns = append(fns, fn{m[1], paramTypes(m[2])})
	}
	if len(fns) < 30 {
		t.Fatalf("found only %d declarations in testing.hpp: %v", len(fns), fns)
	}
	for _, goos := range []string{"linux", "windows"} {
		var src strings.Builder
		for _, p := range rtfiles.TestingSources(goos) {
			data, err := fs.ReadFile(rtfiles.FS(), p)
			if err != nil {
				t.Fatal(err)
			}
			src.Write(data)
		}
		for _, f := range fns {
			// a definition: a line that starts with a type, names the function and ends in { (the
			// parameters may span lines)
			def := regexp.MustCompile(`(?m)^[A-Za-z_\[][^;\n]*\b` + f.name + `\(([^;{]*)\)\s*(?:noexcept)?\s*\{`)
			found := false
			for _, m := range def.FindAllStringSubmatch(src.String(), -1) {
				if paramTypes(m[1]) == f.params {
					found = true
				}
			}
			if !found {
				t.Errorf("gp::testing::%s(%s) is declared in testing.hpp, but no source of TestingSources(%q) defines it with these parameters: a test binary does not link", f.name, f.params, goos)
			}
		}
	}
}
