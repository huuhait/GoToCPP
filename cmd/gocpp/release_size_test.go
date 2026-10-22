package main

import (
	"debug/elf"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The size of a program (job 15.size): a build keeps its debug information (it is what the debuggers
// need), -release leaves it out, and -strip drops the symbol table too. These tests build a real
// program with each compiler, read its sections with debug/elf and run it.

// sizeProgram panics after it prints, so that one build shows both the output and the trace.
const sizeProgram = `package main

import "fmt"

func at(xs []int, i int) int { return xs[i] }

func main() {
	fmt.Println("size program")
	fmt.Println(at([]int{1, 2, 3}, 7))
}
`

// debugSections returns the names of the sections of the ELF file path that hold debug information,
// and whether it has a symbol table.
func debugSections(t *testing.T, path string) (names []string, symtab bool) {
	t.Helper()
	f, err := elf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, s := range f.Sections {
		if strings.HasPrefix(s.Name, ".debug_") {
			names = append(names, s.Name)
		}
		if s.Name == ".symtab" {
			symtab = true
		}
	}
	return names, symtab
}

// releaseStripLimit is the size that the -release -strip program stays under: it is 233,000 bytes
// (225,000 with g++ before job 15.fmtsize's float digits, which cost 4 KB here), with room for a
// compiler update.
const releaseStripLimit = 300_000

// fileSize returns the size of the file path.
func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Size()
}

// withoutGDBScripts returns names without .debug_gdb_scripts: the section of the pretty printers is
// not debug information, it is in the runtime of every program, and only -strip removes it.
func withoutGDBScripts(names []string) []string {
	var out []string
	for _, n := range names {
		if n != ".debug_gdb_scripts" {
			out = append(out, n)
		}
	}
	return out
}

func TestBuildSizeModes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the sections are read with debug/elf")
	}
	forEachCXX(t, func(t *testing.T, c testCXX) {
		cacheOn(t)
		t.Setenv("GOCPP_CFLAGS", "")
		t.Setenv("GOCPP_CXXFLAGS", "")
		t.Setenv("GOCPP_LDFLAGS", "")
		inDir(t, map[string]string{"main.gp": sizeProgram})
		size := map[string]int64{}
		for _, m := range []struct {
			name      string
			flags     []string
			debug     bool // the program has debug information (the sections other than the gdb script)
			symtab    bool
			gdbScript bool // the section of the pretty printers is still there
			frames    string
		}{
			{name: "default", debug: true, symtab: true, gdbScript: true, frames: "main.main()"},
			{name: "release", flags: []string{"-release"}, symtab: true, gdbScript: true, frames: "main.main()"},
			{name: "strip", flags: []string{"-strip"}, frames: "?()"},
			{name: "release strip", flags: []string{"-release", "-strip"}, frames: "?()"},
			{
				name:  "release with debug information",
				flags: []string{"-release", "-cflags=-g", "-ldflags=-g"}, debug: true, symtab: true, gdbScript: true,
				frames: "main.main()",
			},
		} {
			t.Run(m.name, func(t *testing.T) {
				out := t.TempDir() + "/prog"
				r := gocppRun("", false, append(append([]string{"build"}, m.flags...), "-o", out, "main.gp")...)
				if r.code != 0 || r.stderr != "" {
					t.Fatalf("gocpp build %v = %+v", m.flags, r)
				}
				size[m.name] = fileSize(t, out)
				names, symtab := debugSections(t, out)
				if got := len(withoutGDBScripts(names)) > 0; got != m.debug {
					t.Errorf("debug sections %q: has debug information = %v, want %v", names, got, m.debug)
				}
				if got := slices.Contains(names, ".debug_gdb_scripts"); got != m.gdbScript {
					t.Errorf("sections %q: the gdb script is there = %v, want %v", names, got, m.gdbScript)
				}
				if symtab != m.symtab {
					t.Errorf("symbol table = %v, want %v", symtab, m.symtab)
				}

				// The program still runs; a panic still ends it with Go's status and header, with the
				// names of the functions only while the symbol table is there.
				cmd := exec.Command(out)
				var stderr strings.Builder
				cmd.Stderr = &stderr
				stdout, err := cmd.Output()
				if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 2 {
					t.Fatalf("the program ended with %v, want exit status 2", err)
				}
				if string(stdout) != "size program\n" {
					t.Errorf("the program printed %q", stdout)
				}
				text := stderr.String()
				if !strings.HasPrefix(text, "panic: runtime error: index out of range [7] with length 3\n\ngoroutine 1 [running]:\n") {
					t.Errorf("the panic report starts with:\n%s", text)
				}
				if !strings.Contains(text, m.frames) {
					t.Errorf("the panic report lacks %q:\n%s", m.frames, text)
				}
			})
		}

		// Each step makes the program much smaller. The factors are far below what is measured
		// (a hello world: 5.5 MB, 585 KB with -strip, 375 KB with -release, 224 KB with both), so
		// that the compiler and the machine do not matter.
		for _, c := range []struct {
			small, big string
			factor     int64
		}{
			{"strip", "default", 4}, {"release", "default", 4}, {"release strip", "release", 1},
		} {
			if size[c.small]*c.factor > size[c.big] {
				t.Errorf("%s is %d bytes and %s is %d: want it at most 1/%d of it", c.small, size[c.small], c.big, size[c.big], c.factor)
			}
		}
		if size["release strip"] > releaseStripLimit {
			t.Errorf("-release -strip is %d bytes, want at most %d", size["release strip"], releaseStripLimit)
		}
		if size["release strip"] >= size["release"] || size["release strip"] >= size["strip"] {
			t.Errorf("sizes %v: -release -strip is not the smallest", size)
		}
	})
}
