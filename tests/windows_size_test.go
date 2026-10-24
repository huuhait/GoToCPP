package tests

import (
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
)

// The size of a Windows program (job 15.size): gocpp build -os windows keeps the debug information
// (13 MB for a hello world, 63 MB for the Dear ImGui demo of Direct3D 11), -release leaves it out and
// -strip drops the COFF symbol table too (1.4 MB for a hello world with both, 3.3 MB for the demo). The
// test cross-builds one program in each mode, reads the sections and the symbol table with debug/pe, and,
// with GOCPP_WINDOWS_SSH=user@host, runs each program on the real PC (never Wine): they print the same,
// and a panic ends them with status 2 and Go's header, with function names in the trace while the symbol
// table is there. Without MinGW-w64 it skips; without GOCPP_WINDOWS_SSH it only reads the files.
//
// Job 15.fmtsize added two guards for what a small program is made of: -release -strip stays under
// winReleaseStripLimit (a hello world is 0.44 MB; libstdc++'s locale facets and floating-point
// to_chars, which one std::ostringstream and std::to_chars in the runtime pulled in, made it 1.4 MB),
// and the symbol table of the -release program names none of that code.

// winReleaseStripLimit is the size that the -release -strip program of winSizeProgram stays under.
// It is 448,000 bytes (1.4 MB before job 15.fmtsize): the limit has a third of room, so that a
// compiler update does not break it, and is far below the 1.4 MB of the locale machinery.
const winReleaseStripLimit = 600_000

// libstdcxxFragments are pieces of the mangled names of libstdc++'s locale and stream classes
// (std::locale, ios_base, basic_ios, basic_ostream, basic_istream, basic_streambuf,
// basic_ostringstream, time_get, money_get, num_get, num_put, ctype, codecvt, numpunct) and of
// its floating-point std::to_chars (the double, float and long double overloads): none may be in
// a hello world.
var libstdcxxFragments = []string{
	"St6locale", "St8ios_base", "St9basic_ios", "St13basic_ostream", "St13basic_istream",
	"St15basic_streambuf", "19basic_ostringstream", "8time_get", "9money_get", "7num_put",
	"7num_get", "St5ctype", "7codecvt", "8numpunct", "to_charsPcS_d", "to_charsPcS_f",
	"to_charsPcS_e",
}

const winSizeProgram = `package main

import "fmt"

func at(xs []int, i int) int { return xs[i] }

func main() {
	fmt.Println("size program")
	fmt.Println(at([]int{1, 2, 3}, 7))
}
`

func TestWindowsSizeModes(t *testing.T) {
	// The builds use the MinGW-w64 g++ of PATH: GOCPP_CXX names the host's compiler in the test
	// matrix (gp-focused, make ci), which is no Windows compiler.
	if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil {
		t.Skip("no MinGW-w64 cross compiler (x86_64-w64-mingw32-g++) in PATH")
	}
	tmp := t.TempDir()
	gocpp := filepath.Join(tmp, "gocpp")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", gocpp, "./cmd/gocpp")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/gocpp: %v\n%s", err, out)
	}
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "main.gp"), []byte(winSizeProgram), 0o644); err != nil {
		t.Fatal(err)
	}

	modes := []struct {
		name   string
		flags  []string
		debug  bool   // has the sections of debug information (.debug_info and the like)
		symbol bool   // has the COFF symbol table
		frames string // "": the crash report is the message alone (-release without -trace: ADR-136)
	}{
		{name: "default", debug: true, symbol: true, frames: "main.main()"},
		{name: "release", flags: []string{"-release"}, symbol: true},
		{name: "strip", flags: []string{"-strip"}, frames: "?()"},
		{name: "release-strip", flags: []string{"-release", "-strip"}},
		// MinGW's LTO with -g1, and the trace from the PE file's .debug_* sections.
		{name: "release-trace", flags: []string{"-release", "-trace"}, debug: true, symbol: true, frames: "main.main()"},
	}
	size := map[string]int64{}
	exes := map[string]string{}
	for _, m := range modes {
		exe := filepath.Join(tmp, "size-"+m.name+".exe")
		exes[m.name] = exe
		args := append(append([]string{"build", "-os", "windows"}, m.flags...), "-o", exe, "main.gp")
		cmd := exec.CommandContext(t.Context(), gocpp, args...)
		cmd.Dir = src
		cmd.Env = append(withoutEnv(os.Environ(), "GOCPP_CXX", "GOCPP_CC"), "XDG_CACHE_HOME="+filepath.Join(tmp, "cache"), "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_GATE_PHASE=15")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gocpp %v: %v\n%s", args, err, out)
		}
		fi, err := os.Stat(exe)
		if err != nil {
			t.Fatal(err)
		}
		size[m.name] = fi.Size()

		f, err := pe.Open(exe)
		if err != nil {
			t.Fatal(err)
		}
		hasDebug := false
		for _, s := range f.Sections {
			// .debug_gdb_scripts holds the pretty printers of the runtime, which -release keeps.
			if strings.HasPrefix(s.Name, ".debug_") && s.Name != ".debug_gdb_scripts" {
				hasDebug = true
			}
		}
		symbols := len(f.COFFSymbols)
		var names []string
		for _, sym := range f.COFFSymbols {
			if name, err := sym.FullName(f.StringTable); err == nil {
				names = append(names, name)
			}
		}
		f.Close()
		if m.name == "release" {
			for _, name := range names {
				for _, frag := range libstdcxxFragments {
					if strings.Contains(name, frag) {
						t.Errorf("release: the program has the symbol %q: libstdc++'s locale and float code is back in every program (runtime/locale_test.go)", name)
					}
				}
			}
		}
		if hasDebug != m.debug {
			t.Errorf("%s: has debug information = %v, want %v", m.name, hasDebug, m.debug)
		}
		if (symbols > 0) != m.symbol {
			t.Errorf("%s: %d symbols in the COFF symbol table, want some = %v", m.name, symbols, m.symbol)
		}
	}
	if size["strip"]*4 > size["default"] || size["release"]*2 > size["default"] || size["release-strip"] >= size["release"] || size["release-strip"] >= size["strip"] {
		t.Errorf("sizes %v: each step should make the program smaller", size)
	}

	if size["release-strip"] > winReleaseStripLimit {
		t.Errorf("-release -strip is %d bytes, want at most %d", size["release-strip"], winReleaseStripLimit)
	}

	host := windowsHost()
	if host == "" {
		t.Log("GOCPP_WINDOWS_SSH is not set: the programs were not run")
		return
	}
	r, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is not reachable: %v", err)
	}
	t.Cleanup(r.Close)
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			res, err := r.Run(t.Context(), 2*time.Minute, exes[m.name], nil, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if res.TimedOut {
				t.Fatalf("it timed out\n%s\n%s", res.Stdout, res.Stderr)
			}
			out, errText := normNL(res.Stdout), normNL(res.Stderr)
			if res.Code != 2 || out != "size program\n" {
				t.Errorf("exit %d, stdout %q, stderr:\n%s", res.Code, out, errText)
			}
			const msg = "panic: runtime error: index out of range [7] with length 3\n"
			if m.frames == "" {
				if errText != msg {
					t.Errorf("the panic report is:\n%s\nwant only the message:\n%s", errText, msg)
				}
				return
			}
			if !strings.HasPrefix(errText, msg+"\ngoroutine 1 [running]:\n") {
				t.Errorf("the panic report starts with:\n%s", errText)
			}
			if !strings.Contains(errText, m.frames) {
				t.Errorf("the panic report lacks %q:\n%s", m.frames, errText)
			}
		})
	}
}

// withoutEnv returns env without the variables names.
func withoutEnv(env []string, names ...string) []string {
	var out []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.Contains(names, name) {
			out = append(out, kv)
		}
	}
	return out
}
