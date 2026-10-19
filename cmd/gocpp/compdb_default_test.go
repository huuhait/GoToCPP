package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The compile_commands.json that a debug build writes by itself (ADR-044, amended 2026-10-19):
// gocpp build, run and test write it in the default debug mode, a -release build only with
// -compdb, and -compdb=false turns it off.

var genLine = regexp.MustCompile(`(?m)^# gocpp emit > '?([^'\n]+)'?$`)

// genDirOf returns the folder of the generated C++ that a build with -x reported.
func genDirOf(t *testing.T, stderr string) string {
	t.Helper()
	m := genLine.FindStringSubmatch(stderr)
	if m == nil {
		t.Fatalf("no '# gocpp emit' line in:\n%.500s", stderr)
	}
	return m[1]
}

func compdbSetup(t *testing.T, tag string) string {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("GOCPP_GATE_PHASE", "06")
	return inDir(t, pkgsProject(tag, 3))
}

func TestBuildWritesCompDBInDebug(t *testing.T) {
	dir := compdbSetup(t, "dbg")
	for _, flags := range [][]string{{}, {"-debug"}} {
		os.Remove(filepath.Join(dir, "compile_commands.json"))
		args := append(append([]string{"build", "-x"}, flags...), "-o", "prog", ".")
		r := gocppRun("", false, args...)
		if r.code != 0 {
			t.Fatalf("gocpp %v = %+v", args, r)
		}
		mod := readCompDB(t, filepath.Join(dir, "compile_commands.json"))
		gen := readCompDB(t, filepath.Join(genDirOf(t, r.stderr), "compile_commands.json"))
		if len(mod) < 10 || len(gen) != len(mod) {
			t.Errorf("%v: %d entries in the module's file, %d in the generated folder", flags, len(mod), len(gen))
		}
		if !exists(filepath.Join(dir, "prog")) {
			t.Errorf("%v: no program", flags)
		}
	}
	// A build that is up to date is not stamped differently: it stays quiet and keeps the file.
	if r := gocppRun("", false, "build", "-o", "prog", "."); r.code != 0 || r.stderr != "" {
		t.Errorf("rebuild = %+v", r)
	}
	if len(readCompDB(t, filepath.Join(dir, "compile_commands.json"))) == 0 {
		t.Errorf("the file is gone after a rebuild")
	}
}

func TestBuildReleaseWritesCompDBOnlyWithFlag(t *testing.T) {
	dir := compdbSetup(t, "rel")
	r := gocppRun("", false, "build", "-x", "-release", "-o", "prog", ".")
	if r.code != 0 {
		t.Fatalf("gocpp build -release = %+v", r)
	}
	genDB := filepath.Join(genDirOf(t, r.stderr), "compile_commands.json")
	if exists(filepath.Join(dir, "compile_commands.json")) || exists(genDB) {
		t.Errorf("a -release build wrote compile_commands.json (module's: %v, generated folder's: %v)", exists(filepath.Join(dir, "compile_commands.json")), exists(genDB))
	}
	r = gocppRun("", false, "build", "-x", "-release", "-compdb", "-o", "prog", ".")
	if r.code != 0 {
		t.Fatalf("gocpp build -release -compdb = %+v", r)
	}
	for _, p := range []string{filepath.Join(dir, "compile_commands.json"), genDB} {
		entries := readCompDB(t, p)
		if len(entries) == 0 || !containsArg(entries[0].Arguments, "-O2") {
			t.Errorf("%s: no entries, or not the release flags: %+v", p, entries)
		}
	}
}

func containsArg(args []string, a string) bool {
	for _, x := range args {
		if x == a {
			return true
		}
	}
	return false
}

func TestBuildCompDBOff(t *testing.T) {
	dir := compdbSetup(t, "off")
	r := gocppRun("", false, "build", "-x", "-compdb=false", "-o", "prog", ".")
	if r.code != 0 {
		t.Fatalf("gocpp build -compdb=false = %+v", r)
	}
	if exists(filepath.Join(dir, "compile_commands.json")) || exists(filepath.Join(genDirOf(t, r.stderr), "compile_commands.json")) {
		t.Errorf("-compdb=false still wrote compile_commands.json")
	}
	// The build that wrote nothing is not "up to date" for the one that writes: the key differs.
	if r := gocppRun("", false, "build", "-o", "prog", "."); r.code != 0 || !exists(filepath.Join(dir, "compile_commands.json")) {
		t.Errorf("gocpp build after -compdb=false = %+v, file written: %v", r, exists(filepath.Join(dir, "compile_commands.json")))
	}
}

func TestBuildDryRunWritesNoCompDB(t *testing.T) {
	dir := compdbSetup(t, "dry")
	if r := gocppRun("", false, "build", "-n", "-o", "prog", "."); r.code != 0 {
		t.Fatalf("gocpp build -n = %+v", r)
	}
	if exists(filepath.Join(dir, "compile_commands.json")) {
		t.Errorf("gocpp build -n wrote compile_commands.json")
	}
}

// A module folder that can't be written does not fail the build: a warning, and the copy in the
// folder of the generated C++ is still written.
func TestBuildCompDBReadOnlyModule(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only folder can be written here")
	}
	dir := compdbSetup(t, "ro")
	out := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	r := gocppRun("", false, "build", "-x", "-o", filepath.Join(out, "prog"), ".")
	if r.code != 0 || !strings.Contains(r.stderr, "warning: compile_commands.json not written") {
		t.Fatalf("gocpp build in a read-only module = %+v", r)
	}
	if !exists(filepath.Join(out, "prog")) || !exists(filepath.Join(genDirOf(t, r.stderr), "compile_commands.json")) {
		t.Errorf("no program, or no file in the generated folder")
	}
	// -compdb asks for it: that is an error.
	os.Chmod(dir, 0o555)
	if r := gocppRun("", false, "build", "-compdb", "-o", filepath.Join(out, "prog2"), "."); r.code == 0 {
		t.Errorf("gocpp build -compdb in a read-only module succeeded: %+v", r)
	}
}

// gocpp run writes the copy in the folder of the generated C++ (it is kept) and none in the
// folder of the program: the file would point at commands of a run.
func TestRunWritesCompDBOnlyToGen(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	dir := inDir(t, map[string]string{"hello.gp": helloSrc})
	r := gocppRun("", true, "run", "-x", "hello.gp")
	if r.code != 0 {
		t.Fatalf("gocpp run = %+v", r)
	}
	if exists(filepath.Join(dir, "compile_commands.json")) {
		t.Errorf("gocpp run wrote compile_commands.json next to the program")
	}
	genDB := filepath.Join(genDirOf(t, r.stdout), "compile_commands.json")
	if len(readCompDB(t, genDB)) == 0 {
		t.Errorf("gocpp run wrote no compile_commands.json in the generated folder")
	}
	os.Remove(genDB)
	if r := gocppRun("", true, "run", "-compdb=false", "hello.gp"); r.code != 0 || exists(genDB) {
		t.Errorf("gocpp run -compdb=false = %+v (file written: %v)", r, exists(genDB))
	}
	// An explicit -compdb is honoured in run too: both places.
	if r := gocppRun("", true, "run", "-compdb", "hello.gp"); r.code != 0 || !exists(filepath.Join(dir, "compile_commands.json")) {
		t.Errorf("gocpp run -compdb = %+v", r)
	}
}

// The copy that a debug build writes into the folder of the generated files is an output of the
// build for the up-to-date check (ADR-036): a deleted copy is written again. The module's copy is
// not (it holds the commands of the program that was built last): it is no change of the package,
// so the next build is up to date, and a deleted one is not written again by a build that is
// current. -compdb=false is another build.
func TestBuildCompDBUpToDate(t *testing.T) {
	dir := compdbSetup(t, "utd")
	cacheOn(t)
	t.Setenv("GOCPP_NOUPTODATE", "")
	ageTree(t, dir)
	db := filepath.Join(dir, "compile_commands.json")
	var genDB string
	build := func(extra ...string) string {
		t.Helper()
		r := gocppRun("", false, append(append([]string{"build", "-x"}, extra...), "-o", "prog", ".")...)
		if r.code != 0 {
			t.Fatalf("gocpp build %v = %+v", extra, r)
		}
		if genDB == "" {
			genDB = filepath.Join(genDirOf(t, r.stderr), "compile_commands.json")
		}
		return r.stderr
	}
	if out := build(); !strings.Contains(out, "# not up to date") || !exists(db) || !exists(genDB) {
		t.Fatalf("first build: file written %v, trace:\n%.300s", exists(db), out)
	}
	if out := build(); !strings.Contains(out, "# up to date") {
		t.Errorf("the second build is not up to date:\n%.600s", out)
	}
	os.Remove(genDB)
	if out := build(); !strings.Contains(out, "# not up to date") || !exists(genDB) {
		t.Errorf("after deleting the generated folder's file: written again %v, trace:\n%.600s", exists(genDB), out)
	}
	os.Remove(db)
	if out := build(); !strings.Contains(out, "# up to date") || exists(db) {
		t.Errorf("a deleted module copy is no reason to build (written again: %v):\n%.600s", exists(db), out)
	}
}

// -compdb=false is in the key of the check: the build that wrote nothing is not taken for the one
// that writes.
func TestBuildCompDBOffThenDefault(t *testing.T) {
	dir := compdbSetup(t, "offutd")
	cacheOn(t)
	t.Setenv("GOCPP_NOUPTODATE", "")
	ageTree(t, dir)
	db := filepath.Join(dir, "compile_commands.json")
	if r := gocppRun("", false, "build", "-x", "-compdb=false", "-o", "prog", "."); r.code != 0 || exists(db) {
		t.Fatalf("gocpp build -compdb=false = %+v (file written: %v)", r, exists(db))
	}
	if r := gocppRun("", false, "build", "-x", "-compdb=false", "-o", "prog", "."); !strings.Contains(r.stderr, "# up to date") {
		t.Errorf("the same build is not up to date:\n%.400s", r.stderr)
	}
	r := gocppRun("", false, "build", "-x", "-o", "prog", ".")
	if r.code != 0 || strings.Contains(r.stderr, "# up to date") || !exists(db) {
		t.Errorf("gocpp build after -compdb=false = %+v, file written: %v", r, exists(db))
	}
}

// Two programs of one module: the module's copy holds the commands of the one that was built last,
// and builds that alternate stay up to date.
func TestBuildCompDBTwoPrograms(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	prog := func(msg string) string {
		return "package main\n\nfunc main() { println(" + msg + ") }\n"
	}
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/two\n", "a/main.gp": prog(`"a"`), "b/main.gp": prog(`"b"`)})
	cacheOn(t)
	t.Setenv("GOCPP_NOUPTODATE", "")
	ageTree(t, dir)
	build := func(name string) string {
		t.Helper()
		r := gocppRun("", false, "build", "-x", "-o", name, "./"+name[1:])
		if r.code != 0 {
			t.Fatalf("gocpp build %s = %+v", name, r)
		}
		return r.stderr
	}
	build("pa")
	build("pb")
	for _, name := range []string{"pa", "pb", "pa", "pb"} {
		if out := build(name); !strings.Contains(out, "# up to date") {
			t.Errorf("build %s is not up to date after the other program was built:\n%.600s", name, out)
		}
	}
	db := readCompDB(t, filepath.Join(dir, "compile_commands.json"))
	if len(db) == 0 {
		t.Errorf("no entries in the module's copy")
	}
}

// The automatic write never replaces a file that gocpp did not write, nor a symbolic link; it
// replaces its own file, as long as nobody changed it.
func TestBuildCompDBForeignFile(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	files := map[string]string{"gocpp.mod": "module example.com/m\n"}
	for _, p := range []string{"a", "b", "c", "d"} {
		files[p+"/"+p+".gp"] = "package " + p + "\n\nfunc F() int { return 1 }\n"
	}
	// The programs main1..main4 import more packages, so that their databases have more entries.
	mainOf := func(n int) {
		imports, body := "", ""
		for _, p := range []string{"a", "b", "c", "d"}[:n] {
			imports += "\t\"example.com/m/" + p + "\"\n"
			body += "\t_ = " + p + ".F()\n"
		}
		files["main.gp"] = "package main\n\nimport (\n" + imports + ")\n\nfunc main() {\n" + body + "}\n"
	}
	mainOf(1)
	dir := inDir(t, files)
	setMain := func(n int) {
		mainOf(n)
		if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(files["main.gp"]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	db := filepath.Join(dir, "compile_commands.json")
	build := func(extra ...string) result {
		t.Helper()
		r := gocppRun("", false, append(append([]string{"build"}, extra...), "-o", "prog", ".")...)
		if r.code != 0 {
			t.Fatalf("gocpp build %v = %+v", extra, r)
		}
		return r
	}
	const note = "was not written by gocpp; not touching it"
	read := func() string {
		b, _ := os.ReadFile(db)
		return string(b)
	}
	// A file that a person made.
	if err := os.WriteFile(db, []byte("hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := build(); !strings.Contains(r.stderr, note) || read() != "hand\n" {
		t.Errorf("a hand-made file: stderr %q, content %q", r.stderr, read())
	}
	// -compdb replaces it, and is gocpp's own file from then on.
	if r := build("-compdb"); r.stderr != "" || read() == "hand\n" {
		t.Errorf("-compdb over a hand-made file: stderr %q, content %.20q", r.stderr, read())
	}
	// Another build of the program (other flags: other commands) replaces the file gocpp wrote.
	before := read()
	setMain(2)
	if r := build(); r.stderr != "" || read() == before {
		t.Errorf("replacing gocpp's own file: stderr %q, changed %v", r.stderr, read() != before)
	}
	// ... but not once somebody edited it.
	edited := read() + "\n"
	if err := os.WriteFile(db, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	setMain(3)
	if r := build(); !strings.Contains(r.stderr, note) || read() != edited {
		t.Errorf("an edited file: stderr %q, kept %v", r.stderr, read() == edited)
	}
	// A symbolic link (CMake and Bear setups link the file into a build folder) stays, and so does
	// the file it points to.
	if runtime.GOOS == "windows" {
		return
	}
	real := filepath.Join(t.TempDir(), "real_cc.json")
	if err := os.WriteFile(real, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(db)
	if err := os.Symlink(real, db); err != nil {
		t.Fatal(err)
	}
	setMain(4)
	if r := build(); !strings.Contains(r.stderr, note) {
		t.Errorf("a symbolic link: stderr %q", r.stderr)
	}
	if fi, err := os.Lstat(db); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symbolic link was replaced")
	}
	if b, _ := os.ReadFile(real); string(b) != "[]\n" {
		t.Errorf("the file behind the link was written: %.30q", b)
	}
	// -compdb replaces the link by a file and never writes through it: the file behind it stays.
	if r := build("-compdb"); r.stderr != "" {
		t.Errorf("-compdb over a link: %q", r.stderr)
	}
	if fi, err := os.Lstat(db); err != nil || !fi.Mode().IsRegular() || len(readCompDB(t, db)) == 0 {
		t.Errorf("-compdb did not replace the symbolic link by the database")
	}
	if b, _ := os.ReadFile(real); string(b) != "[]\n" {
		t.Errorf("-compdb wrote through the link: %.30q", b)
	}
}

// No write goes through a symbolic link, whatever asks for it: a repository can ship
// compile_commands.json -> ~/.bashrc. The link is replaced by a file; the file behind it is as it
// was, and a dangling link creates nothing at its target.
func TestCompDBNeverWritesThroughLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links")
	}
	dir := compdbSetup(t, "link")
	db := filepath.Join(dir, "compile_commands.json")
	other := t.TempDir()
	precious := filepath.Join(other, "bashrc")
	const keep = "export PATH=precious\n"
	dangling := filepath.Join(other, "created-by-link")
	link := func(target string) {
		t.Helper()
		os.Remove(db)
		if err := os.Symlink(target, db); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name string
		args []string
	}{
		{"build -compdb", []string{"build", "-compdb", "-o", "prog", "."}},
		{"export compdb", []string{"export", "compdb", "."}},
	} {
		for _, target := range []string{precious, dangling} {
			if err := os.WriteFile(precious, []byte(keep), 0o640); err != nil {
				t.Fatal(err)
			}
			link(target)
			if r := gocppRun("", false, c.args...); r.code != 0 {
				t.Fatalf("gocpp %v = %+v", c.args, r)
			}
			if fi, err := os.Lstat(db); err != nil || !fi.Mode().IsRegular() || len(readCompDB(t, db)) == 0 {
				t.Errorf("%s over a link to %s: the link was not replaced by the database", c.name, filepath.Base(target))
			}
			if b, _ := os.ReadFile(precious); string(b) != keep {
				t.Errorf("%s over a link to %s: the precious file was written: %.30q", c.name, filepath.Base(target), b)
			}
			if fi, _ := os.Stat(precious); fi.Mode().Perm() != 0o640 {
				t.Errorf("%s: the mode of the precious file changed", c.name)
			}
			if exists(dangling) {
				t.Errorf("%s: a file was created at the target of a dangling link", c.name)
			}
		}
	}
	// export compdb -o link: the same, the link is replaced by the file that was named.
	for _, target := range []string{precious, dangling} {
		if err := os.WriteFile(precious, []byte(keep), 0o640); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "out.json")
		if err := os.Symlink(target, out); err != nil {
			t.Fatal(err)
		}
		r := gocppRun("", false, "export", "compdb", "-o", out, ".")
		if r.code != 0 || strings.TrimSpace(r.stdout) != out {
			t.Fatalf("gocpp export compdb -o link = %+v", r)
		}
		if fi, err := os.Lstat(out); err != nil || !fi.Mode().IsRegular() || len(readCompDB(t, out)) == 0 {
			t.Errorf("export compdb -o link to %s: the link was not replaced by the database", filepath.Base(target))
		}
		if b, _ := os.ReadFile(precious); string(b) != keep || exists(dangling) {
			t.Errorf("export compdb -o link to %s wrote through the link", filepath.Base(target))
		}
	}
	// export compdb -o file: exactly that path, new or not.
	for _, pre := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "db", "cc.json")
		if pre {
			os.MkdirAll(filepath.Dir(out), 0o755)
			os.WriteFile(out, []byte("old\n"), 0o644)
		}
		if r := gocppRun("", false, "export", "compdb", "-o", out, "."); r.code != 0 || strings.TrimSpace(r.stdout) != out || len(readCompDB(t, out)) == 0 {
			t.Errorf("export compdb -o %s (existing: %v) = %+v", out, pre, r)
		}
	}
}

// The automatic write leaves a symbolic link alone even when the file behind it has exactly the
// content that would be written.
func TestBuildCompDBLinkWithSameContent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links")
	}
	dir := compdbSetup(t, "samelink")
	db := filepath.Join(dir, "compile_commands.json")
	if r := gocppRun("", false, "build", "-o", "prog", "."); r.code != 0 {
		t.Fatalf("gocpp build = %+v", r)
	}
	same, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(t.TempDir(), "real.json")
	if err := os.WriteFile(real, same, 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(db)
	if err := os.Symlink(real, db); err != nil {
		t.Fatal(err)
	}
	r := gocppRun("", false, "build", "-o", "prog", ".")
	if r.code != 0 || !strings.Contains(r.stderr, "not touching it") {
		t.Errorf("a link to the same content: %+v", r)
	}
	if fi, err := os.Lstat(db); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symbolic link was replaced")
	}
}

// The record of what gocpp wrote is found through a folder that is a link: a build from there sees
// its own file, not a foreign one.
func TestBuildCompDBLinkedModuleFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links")
	}
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	files := pkgsProject("linked", 3)
	real := t.TempDir()
	writeFiles(t, real, files)
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(real, linked); err != nil {
		t.Fatal(err)
	}
	t.Chdir(linked)
	if r := gocppRun("", false, "build", "-o", "prog", "."); r.code != 0 || r.stderr != "" {
		t.Fatalf("gocpp build from the linked folder = %+v", r)
	}
	db := filepath.Join(real, "compile_commands.json")
	before, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	// Another build, from the folder itself, of another program: other commands.
	t.Chdir(real)
	small := "package main\n\nimport \"example.com/m/a\"\n\nfunc main() { println(a.Tag()) }\n"
	if err := os.WriteFile(filepath.Join(real, "main.gp"), []byte(small), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gocppRun("", false, "build", "-o", "prog", ".")
	if r.code != 0 {
		t.Fatalf("gocpp build = %+v", r)
	}
	if strings.Contains(r.stderr, "not touching it") {
		t.Errorf("gocpp's own file looks foreign from another path to the same folder:\n%s", r.stderr)
	}
	if after, _ := os.ReadFile(db); string(after) == string(before) {
		t.Errorf("the file was not replaced")
	}
}

// The warning that the file could not be written is shown at most once a day for a file.
func TestCompdbWarnOncePerDay(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	var out strings.Builder
	d := &driver{name: "build", stderr: &out}
	path := filepath.Join(t.TempDir(), "compile_commands.json")
	warn := func() int {
		out.Reset()
		d.warnCompdb(path, errors.New("read-only"))
		return strings.Count(out.String(), "warning: compile_commands.json not written")
	}
	if n := warn(); n != 1 {
		t.Fatalf("first warning: %d lines", n)
	}
	if n := warn(); n != 0 {
		t.Errorf("the second warning within a day: %d lines", n)
	}
	d.warnCompdb(filepath.Join(t.TempDir(), "compile_commands.json"), errors.New("x")) // another file: its own
	marker := compdbWarnMarker(filepath.Join(cache, "gocpp"), path)
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	if n := warn(); n != 1 {
		t.Errorf("a warning after a day: %d lines", n)
	}
}

// Without a usable cache folder the build is in a work folder that is removed: no compile_commands.json
// is written (the commands would name files that are gone), and one that is there stays.
func TestBuildCompDBNoCache(t *testing.T) {
	dir := compdbSetup(t, "nocache")
	db := filepath.Join(dir, "compile_commands.json")
	if r := gocppRun("", false, "build", "-o", "prog", "."); r.code != 0 {
		t.Fatalf("gocpp build = %+v", r)
	}
	good, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "prog", "no-such-folder")) // prog is a file: the folder can't be made
	r := gocppRun("", false, "build", "-o", "prog2", ".")
	if r.code != 0 || !exists(filepath.Join(dir, "prog2")) || r.stderr != "" {
		t.Fatalf("gocpp build without a cache = %+v, want no output", r)
	}
	if now, _ := os.ReadFile(db); string(now) != string(good) {
		t.Errorf("the module's compile_commands.json was replaced by one that names the work folder")
	}
	// An explicit -compdb says that it writes nothing.
	r = gocppRun("", false, "build", "-compdb", "-o", "prog2", ".")
	if r.code != 0 || !strings.Contains(r.stderr, "compile_commands.json not written") {
		t.Errorf("gocpp build -compdb without a cache = %+v", r)
	}
	if now, _ := os.ReadFile(db); string(now) != string(good) {
		t.Errorf("-compdb without a cache replaced the file")
	}
}

// A program of single .gp files has no module: gocpp build writes no copy next to it.
func TestBuildCompDBNoModule(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	dir := inDir(t, map[string]string{"hello.gp": helloSrc})
	r := gocppRun("", false, "build", "-x", "-o", "hello", "hello.gp")
	if r.code != 0 {
		t.Fatalf("gocpp build hello.gp = %+v", r)
	}
	if exists(filepath.Join(dir, "compile_commands.json")) {
		t.Errorf("gocpp build of a single file wrote compile_commands.json next to it")
	}
	if !exists(filepath.Join(genDirOf(t, r.stderr), "compile_commands.json")) {
		t.Errorf("no copy in the folder of the generated files")
	}
}
