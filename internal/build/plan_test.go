package build

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The tests of the plan of a build (plan.go), compile_commands.json (compdb.go) and the CMake
// export (cmake.go). The plans are made by hand: no compiler runs.

func testPlan() *Plan {
	p := &Plan{GenRoot: "/cache/gen/x", RuntimeDir: "/cache/rt/r", GOOS: "linux", CXX: CXX{Path: "/usr/bin/clang++", Family: Clang}}
	cxx := []string{"/usr/bin/clang++"}
	// Added out of order, with a duplicate, on purpose.
	p.add(Command{Kind: KindRuntime, Lang: "c++", Compiler: cxx, Args: []string{"-O0", "-I", "/cache/rt/r/include", "-c", "/cache/rt/r/src/rt.cpp", "-o", "/work/rt.o"}, Dir: "/cache/rt/r", File: "/cache/rt/r/src/rt.cpp"})
	p.add(Command{Kind: KindUnit, Pkg: "example.com/m", Lang: "c++", Compiler: cxx, Args: []string{"-std=c++20", "-DX=1", "-c", "-x", "c++", "/cache/gen/x/gpkg/m/m.gp.cpp", "-o", "/work/m.o"}, Dir: "/cache/gen/x", File: "/cache/gen/x/gpkg/m/m.gp.cpp"})
	p.add(Command{Kind: KindSource, Pkg: "example.com/m", Lang: "c", Compiler: []string{"/usr/bin/clang"}, Args: []string{"-std=c11", "-c", "-x", "c", "/proj/m/c.c", "-o", "/work/c.o"}, Dir: "/proj/m", File: "/proj/m/c.c"})
	p.add(Command{Kind: KindUnit, Pkg: "example.com/m", Lang: "c++", Compiler: cxx, Args: []string{"-std=c++20", "-DX=1", "-c", "-x", "c++", "/cache/gen/x/gpkg/m/m.gp.cpp", "-o", "/work/other.o"}, Dir: "/cache/gen/x", File: "/cache/gen/x/gpkg/m/m.gp.cpp"})
	p.Link = &Command{Kind: KindLink, Compiler: cxx, Pre: []string{"-O0", "-Wl,--gc-sections"}, Post: []string{"-lm", "-pthread"}, Output: "/work/prog"}
	return p
}

func TestCommandCompileArgs(t *testing.T) {
	c := Command{Args: []string{"-c", "a.cpp", "-o", "a.o", "-DX"}}
	if got, want := c.CompileArgs(), []string{"-c", "a.cpp", "-DX"}; !slices.Equal(got, want) {
		t.Errorf("CompileArgs = %q, want %q", got, want)
	}
	if got := (Command{Args: []string{"-o"}}).CompileArgs(); !slices.Equal(got, []string{"-o"}) {
		t.Errorf("a trailing -o is dropped: %q", got)
	}
}

func TestPlanSortedCompiles(t *testing.T) {
	got := testPlan().sortedCompiles()
	var files []string
	for _, c := range got {
		files = append(files, c.File)
	}
	want := []string{"/cache/gen/x/gpkg/m/m.gp.cpp", "/cache/rt/r/src/rt.cpp", "/proj/m/c.c"}
	if !slices.Equal(files, want) {
		t.Errorf("sortedCompiles files = %q, want %q (sorted, the duplicate once)", files, want)
	}
}

func TestCompDB(t *testing.T) {
	data, err := testPlan().CompDBJSON()
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, data)
	}
	if len(entries) != 3 {
		t.Fatalf("%d entries, want 3:\n%s", len(entries), data)
	}
	for _, e := range entries {
		// The fields of the clang JSON compilation database that gocpp writes.
		if len(e) != 3 || e["directory"] == nil || e["file"] == nil || e["arguments"] == nil {
			t.Errorf("entry %v: want directory, file and arguments only", e)
		}
	}
	first := entries[0]["arguments"].([]any)
	var args []string
	for _, a := range first {
		args = append(args, a.(string))
	}
	// The compiler first, no -o and no object file (in a work folder that is gone).
	if want := "/usr/bin/clang++ -std=c++20 -DX=1 -c -x c++ /cache/gen/x/gpkg/m/m.gp.cpp"; strings.Join(args, " ") != want {
		t.Errorf("arguments = %q, want %q", strings.Join(args, " "), want)
	}
	if entries[0]["directory"] != "/cache/gen/x" || entries[2]["directory"] != "/proj/m" {
		t.Errorf("directories: %v %v", entries[0]["directory"], entries[2]["directory"])
	}
	if strings.Contains(string(data), "/work/") {
		t.Errorf("the work folder is in the database:\n%s", data)
	}
	if !strings.HasSuffix(string(data), "]\n") || strings.Contains(string(data), `\u003c`) {
		t.Errorf("format of the file: %q", data)
	}
}

func TestCompDBDeterministic(t *testing.T) {
	a, _ := testPlan().CompDBJSON()
	p := testPlan()
	slices.Reverse(p.Compiles)
	b, _ := p.CompDBJSON()
	if string(a) != string(b) {
		t.Errorf("the order of the actions changes the file")
	}
}

func TestCompDBEmpty(t *testing.T) {
	if _, err := (&Plan{}).CompDBJSON(); err == nil {
		t.Errorf("an empty plan has a database")
	}
}

// TestCompDBDirectoryDefault: a command without a folder of its own gets the folder of its file.
// The plan's paths are the host's (filepath.Join of absolute paths: C:\a\b\x.c on Windows), and
// so is the folder.
func TestCompDBDirectoryDefault(t *testing.T) {
	file, err := filepath.Abs(filepath.FromSlash("/a/b/x.c"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(file, string(filepath.Separator)+"x.c") // /a/b, or C:\a\b
	p := &Plan{}
	p.add(Command{Kind: KindSource, Compiler: []string{"cc"}, Args: []string{"-c", file}, File: file})
	if got := p.CompDB(); len(got) != 1 || got[0].Directory != want || got[0].File != file {
		t.Errorf("CompDB = %+v, want the folder %s of the file %s", got, want, file)
	}
}

func TestWriteCompDB(t *testing.T) {
	dir := t.TempDir()
	p := testPlan()
	// A folder gets compile_commands.json.
	path, err := WriteCompDB(p, dir)
	if err != nil || path != filepath.Join(dir, "compile_commands.json") {
		t.Fatalf("WriteCompDB(folder) = %q, %v", path, err)
	}
	st1, _ := os.Stat(path)
	// The same content is not written again (a watching tool is not woken up).
	if err := os.Chtimes(path, st1.ModTime().Add(-1e9*3600), st1.ModTime().Add(-1e9*3600)); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Stat(path)
	if _, err := WriteCompDB(p, dir); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.Stat(path); !now.ModTime().Equal(old.ModTime()) {
		t.Errorf("a file with the same content was written again")
	}
	// A new content replaces it, and no temporary file stays.
	p.add(Command{Kind: KindSource, Compiler: []string{"cc"}, Args: []string{"-c", "/z.c"}, File: "/z.c"})
	if _, err := WriteCompDB(p, dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "/z.c") {
		t.Errorf("the new entry is not in the file")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("the folder has %d entries, want only the database", len(entries))
	}
	// A file name in a folder that does not exist yet.
	if path, err := WriteCompDB(p, filepath.Join(dir, "sub", "db.json")); err != nil || !exists(path) {
		t.Errorf("WriteCompDB(new file) = %q, %v", path, err)
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestIRArgs(t *testing.T) {
	args := []string{"-std=c++20", "-c", "-x", "c++", "a.cpp", "-o", "a.o", "-DX"}
	if got, want := irArgs(args, false, "out.ll"), []string{"-std=c++20", "-S", "-emit-llvm", "-x", "c++", "a.cpp", "-DX", "-o", "out.ll"}; !slices.Equal(got, want) {
		t.Errorf("irArgs(ll) = %q, want %q", got, want)
	}
	if got, want := irArgs(args, true, "out.bc"), []string{"-std=c++20", "-c", "-emit-llvm", "-x", "c++", "a.cpp", "-DX", "-o", "out.bc"}; !slices.Equal(got, want) {
		t.Errorf("irArgs(bc) = %q, want %q", got, want)
	}
}

func TestIRPath(t *testing.T) {
	b := &Builder{LLVMDir: filepath.FromSlash("/o/prog.llvm"), EmitLLVM: "ll"}
	c := Command{IR: "gen/gpkg/a/a.gp"}
	if got, want := b.irPath(c), filepath.FromSlash("/o/prog.llvm/gen/gpkg/a/a.gp.ll"); got != want {
		t.Errorf("irPath = %q, want %q", got, want)
	}
	b.EmitLLVM = "bc"
	if got := b.irPath(c); !strings.HasSuffix(got, ".gp.bc") {
		t.Errorf("irPath(bc) = %q", got)
	}
}

func TestIRName(t *testing.T) {
	b := &Builder{GenRoot: filepath.FromSlash("/cache/gen/x")}
	if got, want := b.irName(filepath.FromSlash("/cache/gen/x/gpkg/m/m.gp.cpp")), "gen/gpkg/m/m.gp"; got != want {
		t.Errorf("irName = %q, want %q", got, want)
	}
	if got, want := b.irName(filepath.FromSlash("/elsewhere/z.cpp")), "gen/z"; got != want {
		t.Errorf("irName(outside) = %q, want %q", got, want)
	}
}

func TestEmitIRNeedsEmitLLVM(t *testing.T) {
	// Without -emit-llvm, and for a command that has no IR name, nothing runs.
	b := &Builder{}
	if err := b.emitIR(context.TODO(), CXX{}, "x", Command{IR: "a"}); err != nil {
		t.Errorf("emitIR without EmitLLVM = %v", err)
	}
	b.EmitLLVM = "ll"
	if err := b.emitIR(context.TODO(), CXX{}, "x", Command{}); err != nil {
		t.Errorf("emitIR without an IR name = %v", err)
	}
}

func TestPrepareIRDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "prog.llvm")
	// A new folder is made and marked.
	if err := PrepareIRDir(dir); err != nil || !exists(filepath.Join(dir, IRMarker)) {
		t.Fatalf("PrepareIRDir(new) = %v", err)
	}
	// An older output of gocpp is emptied.
	old := filepath.Join(dir, "gen", "old.ll")
	os.MkdirAll(filepath.Dir(old), 0o755)
	os.WriteFile(old, []byte("x"), 0o644)
	if err := PrepareIRDir(dir); err != nil || exists(old) || !exists(filepath.Join(dir, IRMarker)) {
		t.Errorf("PrepareIRDir(again) = %v, old file exists: %v", err, exists(old))
	}
	// A folder of somebody else is not touched.
	foreign := filepath.Join(root, "mine")
	os.MkdirAll(foreign, 0o755)
	os.WriteFile(filepath.Join(foreign, "precious"), []byte("x"), 0o644)
	if err := PrepareIRDir(foreign); err == nil || !strings.Contains(err.Error(), "not a folder of LLVM IR files") {
		t.Errorf("PrepareIRDir(foreign) = %v", err)
	}
	if !exists(filepath.Join(foreign, "precious")) {
		t.Errorf("the foreign folder lost a file")
	}
	// An empty existing folder is fine.
	empty := filepath.Join(root, "empty")
	os.MkdirAll(empty, 0o755)
	if err := PrepareIRDir(empty); err != nil {
		t.Errorf("PrepareIRDir(empty) = %v", err)
	}
}

func TestCMakeLists(t *testing.T) {
	text, err := testPlan().CMakeLists("prog", "v0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, CMakeMarker) {
		t.Errorf("no marker in the first line:\n%s", text)
	}
	for _, want := range []string{
		"project(prog LANGUAGES CXX C)",
		`"${GOCPP_EXPORT_DIR}/gen/gpkg/m/m.gp.cpp"`,
		`"${GOCPP_EXPORT_DIR}/runtime/src/rt.cpp"`,
		`"/proj/m/c.c"`,
		"CMAKE_CXX_COMPILER_ID MATCHES \"Clang\"",
		"add_executable(prog",
		"-DX=1",
		"target_link_libraries(prog PRIVATE",
		`"-lm"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("CMakeLists.txt lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "/cache/gen/x") || strings.Contains(text, "/cache/rt/r") || strings.Contains(text, "/work/") {
		t.Errorf("a folder of the build is in the file:\n%s", text)
	}
	// The link: --gc-sections before the objects, libraries after.
	if i, j := strings.Index(text, "target_link_options"), strings.Index(text, "target_link_libraries"); i < 0 || j < i {
		t.Errorf("link options must come before the libraries")
	}
}

func TestCMakeListsGCC(t *testing.T) {
	p := testPlan()
	p.CXX.Family = GCC
	text, err := p.CMakeLists("prog", "v0")
	if err != nil || !strings.Contains(text, `CMAKE_CXX_COMPILER_ID STREQUAL "GNU"`) {
		t.Errorf("CMakeLists(gcc) = %v:\n%s", err, text)
	}
}

func TestCMakeListsGroupsByFlags(t *testing.T) {
	text, err := testPlan().CMakeLists("prog", "v0")
	if err != nil {
		t.Fatal(err)
	}
	// Three files with three different flag lists: three groups.
	if n := strings.Count(text, "PROPERTIES COMPILE_FLAGS"); n != 3 {
		t.Errorf("%d groups, want 3:\n%s", n, text)
	}
	p := testPlan()
	p.add(Command{Kind: KindUnit, Lang: "c++", Compiler: []string{"/usr/bin/clang++"}, Args: []string{"-std=c++20", "-DX=1", "-c", "-x", "c++", "/cache/gen/x/gpkg/n/n.gp.cpp"}, File: "/cache/gen/x/gpkg/n/n.gp.cpp"})
	text, _ = p.CMakeLists("prog", "v0")
	if n := strings.Count(text, "PROPERTIES COMPILE_FLAGS"); n != 3 {
		t.Errorf("a file with the flags of another shares its group: %d groups", n)
	}
}

func TestCMakeListsEscapes(t *testing.T) {
	p := testPlan()
	p.Compiles[1].Args = []string{"-DMSG=\"a b\"", "-DV=$HOME", "-c", "-x", "c++", p.Compiles[1].File}
	text, err := p.CMakeLists("prog", "v0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `\$HOME`) || !strings.Contains(text, `\"`) {
		t.Errorf("$ and \" are not escaped:\n%s", text)
	}
}

func TestCMakeListsErrors(t *testing.T) {
	p := testPlan()
	p.Compiles[1].Args = []string{"-DX=a;b", "-c", p.Compiles[1].File}
	if _, err := p.CMakeLists("prog", "v0"); err == nil || !strings.Contains(err.Error(), "';'") {
		t.Errorf("a ';' in a flag: %v", err)
	}
	if _, err := testPlan().CMakeLists("a\"b", "v0"); err == nil {
		t.Errorf("a program name with a quote was accepted")
	}
	if _, err := testPlan().CMakeLists("", "v0"); err == nil {
		t.Errorf("an empty program name was accepted")
	}
	if _, err := (&Plan{}).CMakeLists("prog", "v0"); err == nil {
		t.Errorf("an empty plan was exported")
	}
	q := testPlan()
	q.Link.Post = []string{"-lx;y"}
	if _, err := q.CMakeLists("prog", "v0"); err == nil {
		t.Errorf("a ';' in a link flag was accepted")
	}
}

func TestExportCMakeFolders(t *testing.T) {
	gen := t.TempDir()
	os.MkdirAll(filepath.Join(gen, "gpkg", "m"), 0o755)
	os.WriteFile(filepath.Join(gen, "gpkg", "m", "m.gp.cpp"), []byte("// gen\n"), 0o644)
	os.WriteFile(filepath.Join(gen, ".tmp-x"), []byte("tmp"), 0o644)
	p := testPlan()
	p.GenRoot = gen
	p.Compiles = nil
	p.add(Command{Kind: KindUnit, Lang: "c++", Compiler: []string{"/usr/bin/clang++"}, Args: []string{"-c", filepath.Join(gen, "gpkg", "m", "m.gp.cpp")}, File: filepath.Join(gen, "gpkg", "m", "m.gp.cpp")})

	// A folder that is not gocpp's and not empty is refused.
	foreign := t.TempDir()
	os.WriteFile(filepath.Join(foreign, "keep"), []byte("x"), 0o644)
	if _, err := ExportCMake(p, foreign, "prog", "v0"); err == nil || !exists(filepath.Join(foreign, "keep")) {
		t.Fatalf("ExportCMake(foreign) = %v", err)
	}
	// A new folder is written; the runtime is extracted; temporary files are not copied.
	out := filepath.Join(t.TempDir(), "out")
	list, err := ExportCMake(p, out, "prog", "v0")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{list, filepath.Join(out, "gen", "gpkg", "m", "m.gp.cpp"), filepath.Join(out, "runtime", "include")} {
		if !exists(f) {
			t.Errorf("%s was not written", f)
		}
	}
	if exists(filepath.Join(out, "gen", ".tmp-x")) {
		t.Errorf("a temporary file was copied")
	}
	// Again: the folder is gocpp's, so it is replaced, and a file that is no longer generated goes.
	stale := filepath.Join(out, "gen", "stale.cpp")
	os.WriteFile(stale, []byte("x"), 0o644)
	if _, err := ExportCMake(p, out, "prog", "v0"); err != nil || exists(stale) {
		t.Errorf("ExportCMake(again) = %v, stale file exists: %v", err, exists(stale))
	}
}

// TestRuntimeArgsRace checks that a -race build compiles the files of the race reporter and of the
// code that names the frames without ThreadSanitizer, and every other file with it.
func TestRuntimeArgsRace(t *testing.T) {
	flags := []string{"-fsanitize=thread", "-DGOCPP_RACE_MODE=1"}
	for rel, want := range map[string]bool{
		"src/os/linux/race.cpp":  true,
		"src/trace.cpp":          true,
		"src/os/linux/trace.cpp": true,
		"src/frames.cpp":         true,
		"src/chan.cpp":           false,
		"src/goroutine.cpp":      false,
		"src/sync.cpp":           false,
	} {
		args := runtimeArgs("/rt", rel, flags, "o.o")
		if got := slices.Contains(args, "-fno-sanitize=thread"); got != want {
			t.Errorf("runtimeArgs(%s) has -fno-sanitize=thread: %v, want %v: %q", rel, got, want, args)
		}
		if !slices.Contains(args, "-fsanitize=thread") {
			t.Errorf("runtimeArgs(%s) lost -fsanitize=thread: %q", rel, args)
		}
	}
	// Without ThreadSanitizer nothing changes.
	if args := runtimeArgs("/rt", "src/trace.cpp", []string{"-g"}, "o.o"); slices.Contains(args, "-fno-sanitize=thread") {
		t.Errorf("runtimeArgs without -race has -fno-sanitize=thread: %q", args)
	}
}
