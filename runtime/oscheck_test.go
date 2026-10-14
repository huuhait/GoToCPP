package rtfiles_test

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// The headers that runtime code outside src/os/ may include: the C++20 standard library and
// the standard C headers (in both spellings). Everything else, <unistd.h>, <sys/...>,
// <pthread.h>, <windows.h> or a library's header, belongs in the OS layer, src/os/<os>/
// (Phase 02 §2.6, ADR-021). A header added here must be standard C++ on Linux and Windows.
var stdHeaders = func() map[string]bool {
	m := make(map[string]bool)
	cxx := `algorithm any array atomic barrier bit bitset charconv chrono codecvt compare complex
		concepts condition_variable coroutine deque exception execution filesystem format
		forward_list fstream functional future initializer_list iomanip ios iosfwd iostream
		istream iterator latch limits list locale map memory memory_resource mutex new numbers
		numeric optional ostream queue random ranges ratio regex scoped_allocator semaphore set
		shared_mutex source_location span sstream stack stdexcept stop_token streambuf string
		string_view syncstream system_error thread tuple type_traits typeindex typeinfo
		unordered_map unordered_set utility valarray variant vector version`
	c := `assert ctype errno fenv float inttypes iso646 limits locale math setjmp signal stdalign
		stdarg stdbool stddef stdint stdio stdlib string time uchar wchar wctype`
	for _, h := range strings.Fields(cxx) {
		m[h] = true
	}
	for _, h := range strings.Fields(c) {
		m["c"+h] = true
		m[h+".h"] = true
	}
	return m
}()

// skipCheck reports whether the file p of runtime/ is left out of the OS check: the Go files
// and the two text files at the root. Every other file is checked, whatever its name, because
// C++ can #include a file of any name ("sneaky.inl", "helpers").
func skipCheck(p string) bool {
	return path.Ext(p) == ".go" || p == "README.md" || p == "compile_flags.txt"
}

// includeRx matches an #include line (after joining continuation lines and removing block
// comments), also with the digraph %: for #, and #include_next and #import.
var includeRx = regexp.MustCompile(`^\s*(?:#|%:)\s*(include|include_next|import)\b\s*(.*)$`)

var blockCommentRx = regexp.MustCompile(`/\*.*?\*/`)

// logicalLines returns the lines of text after joining each line that ends in a backslash
// with the next, as the preprocessor does, and the first line number of each.
func logicalLines(text string) (lines []string, nums []int) {
	phys := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := 0; i < len(phys); i++ {
		start, line := i+1, phys[i]
		for strings.HasSuffix(line, `\`) && i+1 < len(phys) {
			i++
			line = strings.TrimSuffix(line, `\`) + phys[i]
		}
		lines = append(lines, line)
		nums = append(nums, start)
	}
	return lines, nums
}

// checkIncludes returns a message for each #include in the file p (a path in fsys) that
// runtime code outside src/os/ may not have.
func checkIncludes(fsys fs.FS, p string) ([]string, error) {
	text, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, err
	}
	var bad []string
	lines, nums := logicalLines(string(text))
	for k, raw := range lines {
		m := includeRx.FindStringSubmatch(blockCommentRx.ReplaceAllString(raw, " "))
		if m == nil {
			continue
		}
		arg := strings.TrimSpace(m[2])
		where := fmt.Sprintf("runtime/%s:%d: #%s %s", p, nums[k], m[1], arg)
		switch {
		case strings.HasPrefix(arg, "<"):
			name, _, ok := strings.Cut(arg[1:], ">")
			if !ok || !(stdHeaders[name] || strings.HasPrefix(name, "gocpp/")) {
				bad = append(bad, where+": not a standard C++ header")
			}
		case strings.HasPrefix(arg, `"`):
			name, _, ok := strings.Cut(arg[1:], `"`)
			// A quoted include must name a runtime file outside src/os/ (and that file is
			// checked too).
			found := false
			for _, dir := range []string{path.Dir(p), "include"} {
				q := path.Join(dir, name)
				if _, err := fs.Stat(fsys, q); err == nil && !strings.HasPrefix(q, "src/os/") && !strings.HasPrefix(q, "..") {
					found = true
				}
			}
			if !ok || !found {
				bad = append(bad, where+": not a runtime file outside src/os/")
			}
		default:
			bad = append(bad, where+": a computed #include can't be checked")
		}
	}
	return bad, nil
}

// checkTree returns a message for each forbidden #include in the files of fsys (runtime/),
// and how many files it checked.
func checkTree(fsys fs.FS) ([]string, int, error) {
	var bad []string
	checked := 0
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipEntry(fsys, p, d) {
			return skipped(d)
		}
		if d.IsDir() {
			if p == "src/os" {
				return fs.SkipDir
			}
			return nil
		}
		if skipCheck(p) {
			return nil
		}
		checked++
		msgs, err := checkIncludes(fsys, p)
		bad = append(bad, msgs...)
		return err
	})
	return bad, checked, err
}

// TestNoOSHeadersOutsideOS checks the OS rule: in runtime/, only src/os/ may include OS
// headers. Everything else uses standard C++, so that the same code builds on Linux and
// Windows. It checks every file except the Go files, README.md and compile_flags.txt.
func TestNoOSHeadersOutsideOS(t *testing.T) {
	bad, checked, err := checkTree(os.DirFS("."))
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range bad {
		t.Errorf("%s\n\tOnly runtime/src/os/ may include OS headers (Phase 02 §2.6): call the OS through gocpp/os.hpp, or use standard C++.", msg)
	}
	if checked == 0 {
		t.Fatal("found no C++ files in runtime/")
	}
}

// TestCheckIncludes checks the checker itself on files with forbidden includes.
func TestCheckIncludes(t *testing.T) {
	files := map[string]string{
		"files.go":              "package rtfiles\n#include <unistd.h>\n",
		"README.md":             "#include <unistd.h>\n",
		"include/gocpp/ok.hpp":  "#include <vector>\n#include <cstdio>\n#include <stdint.h>\n#include <gocpp/x.hpp>\n#include \"ok2.hpp\"\n%:include <string>\n// #include <unistd.h>\n",
		"include/gocpp/ok2.hpp": "",
		"src/bad.cpp": "#include <unistd.h>\n  #  include <sys/mman.h>\n#include <pthread.h>\n#include <windows.h>\n" +
			"#include_next <fcntl.h>\n#include FOO\n#include \"os/linux/x.hpp\"\n#include <stacktrace>\n" +
			"%:include <sys/mman.h>\n%: include <dlfcn.h>\n#/**/include <poll.h>\n#\\\ninclude <sched.h>\n",
		// C++ can include a file of any name, so every file is checked.
		"include/gocpp/sneaky.inl": "#include <unistd.h>\n",
		"src/os_helpers":           "#include <sys/stat.h>\n",
		"test/data.txt":            "#include <windows.h>\n",
		"src/os/linux/x.hpp":       "#include <unistd.h>\n", // the OS layer may
	}
	fsys := fstest.MapFS{}
	for name, text := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(text)}
	}
	bad, err := checkIncludes(fsys, "include/gocpp/ok.hpp")
	if err != nil || len(bad) != 0 {
		t.Errorf("ok.hpp: %q, %v; want no problems", bad, err)
	}
	bad, err = checkIncludes(fsys, "src/bad.cpp")
	if err != nil || len(bad) != 12 {
		t.Errorf("bad.cpp: got %d problems, want 12 (%v):\n%s", len(bad), err, strings.Join(bad, "\n"))
	}
	bad, checked, err := checkTree(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if checked != 6 {
		t.Errorf("checkTree checked %d files, want 6 (all but files.go, README.md and src/os/)", checked)
	}
	for _, f := range []string{"include/gocpp/sneaky.inl:1:", "src/os_helpers:1:", "test/data.txt:1:", "src/bad.cpp:12:"} {
		if !slices.ContainsFunc(bad, func(m string) bool { return strings.Contains(m, "runtime/"+f) }) {
			t.Errorf("checkTree reports nothing for %s:\n%s", f, strings.Join(bad, "\n"))
		}
	}
	for _, m := range bad {
		if strings.HasPrefix(m, "runtime/src/os/") || strings.HasPrefix(m, "runtime/files.go") || strings.HasPrefix(m, "runtime/README.md") {
			t.Errorf("checkTree checked a file it must skip: %s", m)
		}
	}
}
