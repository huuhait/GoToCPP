package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// The tests of gocpp fmt compare it with the real gofmt where they can:
// gocpp fmt on .gp files must print what gofmt prints on the same files
// named .go, with the same exit status. internal/format has the tests of
// the formatting itself.

// realGofmt returns the gofmt of the Go that runs the test
// ($(go env GOROOT)/bin/gofmt), or skips the test if there is none.
func realGofmt(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Skipf("go env GOROOT: %v", err)
	}
	name := filepath.Join(strings.TrimSpace(string(out)), "bin", "gofmt")
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, err := os.Stat(name); err != nil {
		t.Skipf("there is no real gofmt to compare with: %v", err)
	}
	return name
}

// A result is what a run of gofmt or gocpp fmt prints, and its exit status.
type result struct {
	code           int
	stdout, stderr string
}

// gofmtRun runs the real gofmt in the folder dir, with the standard input
// stdin.
func gofmtRun(t *testing.T, gofmt, dir, stdin string, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := exec.Command(gofmt, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	r := result{}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("gofmt: %v", err)
		}
		r.code = exit.ExitCode()
	}
	r.stdout, r.stderr = out.String(), errOut.String()
	return r
}

// gocppFmt runs "gocpp fmt" in the current folder, with the standard input
// stdin. It changes the names of the .gp files in the output to .go
// (goNames), so that the result can be compared with gofmt's for the same
// files named .go.
func gocppFmt(stdin string, args ...string) result {
	code, out, errOut := gocppStdin(stdin, append([]string{"fmt"}, args...)...)
	return result{code, goNames(out), goNames(errOut)}
}

// gpName is the end of a .gp file name in the output of gocpp fmt: in a
// list (-l), in a diff header (-d) or before the position of an error.
var gpName = regexp.MustCompile(`\.gp([\n: ]|\.orig)`)

// goNames changes the .gp file names in the output of gocpp fmt to .go.
func goNames(out string) string {
	return gpName.ReplaceAllString(out, ".go$1")
}

// gpArgs changes the .go file names in the arguments of gofmt to .gp, for
// gocpp fmt.
func gpArgs(args []string) []string {
	var gp []string
	for _, a := range args {
		if base, ok := strings.CutSuffix(a, ".go"); ok {
			a = base + ".gp"
		}
		gp = append(gp, a)
	}
	return gp
}

// writeFiles writes the files into the folder dir: each name maps to its
// contents. A file whose name ends in .go is written twice: once as x.go
// for gofmt, and once as x.gp for gocpp fmt. Other files are written once.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		names := []string{name}
		if base, ok := strings.CutSuffix(name, ".go"); ok {
			names = append(names, base+".gp")
		}
		for _, n := range names {
			path := filepath.Join(dir, filepath.FromSlash(n))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// fmtTree is a folder of Go inputs. Every .go file is also there as a .gp
// file (writeFiles).
var fmtTree = map[string]string{
	"good.go":            "package p\n\nfunc f() {}\n",
	"bad.go":             "package  p\nfunc f( ) {\nx:=1\n_ = x\n}\n",
	"syntax.go":          "package p\n\nfunc f( {\n",
	".hidden.go":         "package  p\n",
	"sub/nested.go":      "package  q\nimport (\n\"os\"\n\"fmt\"\n)\nvar _, _ = os.Args, fmt.Sprint\n",
	"testdata/td.go":     "package  t\n",
	".dot/indot.go":      "package  d\n",
	"other.txt":          "not  Go\n",
	"sub/deeper/last.go": "package q // ok\n",
}

// TestFmtLikeGofmt runs gocpp fmt and gofmt with the same flags and paths
// on the same files, and they must print the same and exit with the same
// status: folders (walked), files, a syntax error, a missing file,
// absolute paths, -l and -d. TestFmtWrite checks -w.
func TestFmtLikeGofmt(t *testing.T) {
	gofmt := realGofmt(t)
	dir := t.TempDir()
	writeFiles(t, dir, fmtTree)
	t.Chdir(dir)
	for _, args := range [][]string{
		{"."},
		{"-l", "."},
		{"-d", "."},
		{"-l", "-d", "."},
		{"-l", "sub"},
		{"-l", "./sub/"},
		{"-d", "bad.go", "good.go", ".hidden.go", "other.txt"},
		{"bad.go", "good.go"},
		{"-l", "syntax.go", "bad.go"},
		{"-l", "missing.go", "bad.go"},
		{"-l", dir},
		{"-d", filepath.Join(dir, "bad.go")},
		// Flags end at the first path, so "-w" here is a path.
		{"-l", "bad.go", "-w"},
		{"-l", "--", "bad.go"},
	} {
		want := gofmtRun(t, gofmt, dir, "", args...)
		if got := gocppFmt("", gpArgs(args)...); got != want {
			t.Errorf("gocpp fmt %s:\n%s\ngofmt %s:\n%s", strings.Join(gpArgs(args), " "), show(got), strings.Join(args, " "), show(want))
		}
	}
}

// TestFmtUnreadableFolder checks a folder that can't be read during the
// walk: like gofmt, gocpp fmt reports it and stops walking that path, so
// the files after it are not formatted.
func TestFmtUnreadableFolder(t *testing.T) {
	if os.Getuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("needs a folder that the test can't read")
	}
	gofmt := realGofmt(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.go": "package  a\n", "b/x.go": "package  b\n", "c.go": "package  c\n"})
	if err := os.Chmod(filepath.Join(dir, "b"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dir, "b"), 0o755) // so that t.TempDir can remove it
	t.Chdir(dir)
	for _, args := range [][]string{{"-l", "."}, {"-l", ".", "c.go"}} {
		want := gofmtRun(t, gofmt, dir, "", args...)
		if got := gocppFmt("", gpArgs(args)...); got != want || got.code != 2 {
			t.Errorf("gocpp fmt %s:\n%s\ngofmt %s:\n%s", strings.Join(gpArgs(args), " "), show(got), strings.Join(args, " "), show(want))
		}
	}
}

// show returns r as text for a failure message.
func show(r result) string {
	return "exit status " + strconv.Itoa(r.code) + "\n--- stdout:\n" + r.stdout + "--- stderr:\n" + r.stderr
}

// TestFmtWrite checks -w: gocpp fmt and gofmt write back the same files,
// print the same and exit with the same status, and the files that need
// no change or that have errors stay as they are.
func TestFmtWrite(t *testing.T) {
	gofmt := realGofmt(t)
	for _, args := range [][]string{{"-w", "."}, {"-l", "-w", "."}, {"-w", "-d", "bad.go"}} {
		goDir, gpDir := t.TempDir(), t.TempDir()
		writeFiles(t, goDir, fmtTree)
		writeFiles(t, gpDir, fmtTree)
		want := gofmtRun(t, gofmt, goDir, "", args...)
		t.Chdir(gpDir)
		if got := gocppFmt("", gpArgs(args)...); got != want {
			t.Errorf("gocpp fmt %s:\n%s\ngofmt %s:\n%s", strings.Join(gpArgs(args), " "), show(got), strings.Join(args, " "), show(want))
		}
		for name, src := range fmtTree {
			base, ok := strings.CutSuffix(name, ".go")
			if !ok {
				continue
			}
			goFile := readTestFile(t, filepath.Join(goDir, filepath.FromSlash(name)))
			gpFile := readTestFile(t, filepath.Join(gpDir, filepath.FromSlash(base+".gp")))
			if gpFile != goFile {
				t.Errorf("after %v, %s.gp holds\n%s\nwant (like gofmt)\n%s", args, base, gpFile, goFile)
			}
			// gocpp fmt changes no .go file, and gofmt no .gp file.
			if other := readTestFile(t, filepath.Join(gpDir, filepath.FromSlash(name))); other != src {
				t.Errorf("after gocpp fmt %v, %s changed", args, name)
			}
			if other := readTestFile(t, filepath.Join(goDir, filepath.FromSlash(base+".gp"))); other != src {
				t.Errorf("after gofmt %v, %s.gp changed", args, base)
			}
		}
	}
}

func readTestFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestFmtWriteMode checks that -w keeps the mode of a file, and that a file
// that can't be written gives the same error as in gofmt and stays as it
// is.
func TestFmtWriteMode(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFiles(t, dir, map[string]string{"x.go": fmtTree["bad.go"]})
	if err := os.Chmod("x.gp", 0o751); err != nil {
		t.Fatal(err)
	}
	if got := gocppFmt("", "-w", "x.gp"); got != (result{}) {
		t.Fatalf("gocpp fmt -w x.gp:\n%s\nwant no output and exit status 0", show(got))
	}
	info, err := os.Stat("x.gp")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o751 {
		t.Errorf("after gocpp fmt -w, x.gp has the mode %v, want -rwxr-x--x", info.Mode().Perm())
	}
	if got, want := readTestFile(t, "x.gp"), "package p\n\nfunc f() {\n\tx := 1\n\t_ = x\n}\n"; got != want {
		t.Errorf("after gocpp fmt -w, x.gp holds\n%s\nwant\n%s", got, want)
	}

	if os.Getuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	gofmt := realGofmt(t)
	writeFiles(t, dir, map[string]string{"ro.go": fmtTree["bad.go"]})
	for _, name := range []string{"ro.go", "ro.gp"} {
		if err := os.Chmod(name, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	want := gofmtRun(t, gofmt, dir, "", "-w", "ro.go")
	if got := gocppFmt("", "-w", "ro.gp"); got != want || got.code != 2 {
		t.Errorf("gocpp fmt -w ro.gp:\n%s\ngofmt -w ro.go:\n%s", show(got), show(want))
	}
	if got := readTestFile(t, "ro.gp"); got != fmtTree["bad.go"] {
		t.Errorf("gocpp fmt -w changed the read-only file:\n%s", got)
	}
}

// TestFmtWriteSymlink checks -w on a file that the folder walk reaches
// through a symbolic link, when the formatted text is shorter. The walk
// describes the link, not the file, and gofmt -w takes the link's size and
// mode: the end of the old text stays in the file. gocpp fmt uses the size
// of what it read and the mode of the file (the note at the top of
// fmt.go). What it prints and its exit status stay gofmt's, also for a
// link that points nowhere.
func TestFmtWriteSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need special rights on Windows")
	}
	gofmt := realGofmt(t)
	dir := t.TempDir()
	t.Chdir(dir)
	const src = "package    p\n\n\n\n\n\n\nvar    x    =    1    \n"
	const want = "package p\n\nvar x = 1\n"
	writeFiles(t, dir, map[string]string{"real/a.go": src})
	for _, name := range []string{"real/a.go", "real/a.gp"} {
		if err := os.Chmod(name, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, link := range [][2]string{
		{"../real/a.go", "walk/l.go"}, {"../real/a.gp", "walk/l.gp"},
		{"../real/missing.go", "dangling/d.go"}, {"../real/missing.gp", "dangling/d.gp"},
	} {
		if err := os.MkdirAll(filepath.Dir(link[1]), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(link[0], link[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"-l", "-d", "walk"}, {"-w", "dangling"}, {"-l", "-w", "walk"}} {
		want := gofmtRun(t, gofmt, dir, "", args...)
		if got := gocppFmt("", args...); got != want {
			t.Errorf("gocpp fmt %s:\n%s\ngofmt %s:\n%s", strings.Join(args, " "), show(got), strings.Join(args, " "), show(want))
		}
	}
	if got := readTestFile(t, "real/a.gp"); got != want {
		t.Errorf("after gocpp fmt -w walk, real/a.gp holds\n%q\nwant\n%q", got, want)
	}
	if info, err := os.Stat("real/a.gp"); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Errorf("after gocpp fmt -w walk, real/a.gp has the mode %v, want -rw-------", info.Mode().Perm())
	}
	if info, err := os.Lstat("walk/l.gp"); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("after gocpp fmt -w walk, walk/l.gp is not a symbolic link any more (error %v)", err)
	}
	// No backup file stays.
	for _, d := range []string{"real", "walk"} {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Errorf("after gocpp fmt -w walk, the folder %s holds %d files, want 2", d, len(entries))
		}
	}
}

// TestWritePerm checks the mode that -w gives to the backup of a file: the
// mode of the file, also when the folder walk reaches it through a
// symbolic link, which has the mode 0777.
func TestWritePerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need special rights on Windows")
	}
	dir := t.TempDir()
	file, link := filepath.Join(dir, "a.gp"), filepath.Join(dir, "l.gp")
	if err := os.WriteFile(file, []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.gp", link); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{file, link} {
		info, err := os.Lstat(name) // what the folder walk gives
		if err != nil {
			t.Fatal(err)
		}
		if perm, err := writePerm(name, info); perm != 0o640 || err != nil {
			t.Errorf("writePerm(%s) = %v, %v; want -rw-r-----", filepath.Base(name), perm, err)
		}
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writePerm(link, info); !os.IsNotExist(err) {
		t.Errorf("writePerm of a link that points nowhere: %v, want a not-exist error", err)
	}
}

// TestFmtStdin checks gocpp fmt with no paths: it formats standard input,
// which may be a part of a file, like gofmt.
func TestFmtStdin(t *testing.T) {
	gofmt := realGofmt(t)
	dir := t.TempDir()
	for _, stdin := range []string{
		"package p\n",
		"package  p\nfunc f( ) {}\n",
		"func f( ) {}\nvar x=1\n",
		"\n\tx:=1\n\tif x>0{ y()}\n\n",
		"a+b *c",
		"",
		"package p\n\nfunc f( {\n",
		"x :=",
	} {
		for _, args := range [][]string{{}, {"-l"}, {"-d"}, {"-l", "-d"}, {"-w"}} {
			want := gofmtRun(t, gofmt, dir, stdin, args...)
			if got := gocppFmt(stdin, args...); got != want {
				t.Errorf("gocpp fmt %s with the input %q:\n%s\ngofmt:\n%s", strings.Join(args, " "), stdin, show(got), show(want))
			}
		}
	}
}

// TestFmtGocpp checks what is gocpp's own in gocpp fmt: an import group with
// headers keeps its order, gocpp syntax works, and a file with //gocpp:
// directive errors is left as it is, with the errors on stderr and exit
// status 2, like a file with syntax errors.
func TestFmtGocpp(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	const headers = "package  p\n\nimport (\n\t\"os\"\n\t\"fmt\"\n\n\t\"<winsock2.h>\"\n\t\"<windows.h>\"\n)\n\nvar m=std.map[std.string,int]{}\n"
	const broken = "//gocpp:Link ws2_32\npackage  p\n"
	files := map[string]string{"headers.gp": headers, "broken.gp": broken}
	for name, src := range files {
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const errs = `broken.gp:1:1: unknown //gocpp: directive "Link"; did you mean //gocpp:link?` + "\n"
	formatted := "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\n\t\"<winsock2.h>\"\n\t\"<windows.h>\"\n)\n\nvar m = std.map[std.string, int]{}\n"
	for _, tc := range []struct {
		args []string
		want result
	}{
		{[]string{"headers.gp"}, result{0, formatted, ""}},
		{[]string{"-d", "headers.gp"}, result{0, "diff headers.gp.orig headers.gp\n" +
			"--- headers.gp.orig\n+++ headers.gp\n" +
			"@@ -1,11 +1,11 @@\n" +
			"-package  p\n+package p\n \n import (\n-\t\"os\"\n \t\"fmt\"\n+\t\"os\"\n \n \t\"<winsock2.h>\"\n \t\"<windows.h>\"\n )\n \n-var m=std.map[std.string,int]{}\n+var m = std.map[std.string, int]{}\n", ""}},
		{[]string{"broken.gp"}, result{2, "", errs}},
		{[]string{"-l", "."}, result{2, "headers.gp\n", errs}},
		{[]string{"-d", "broken.gp"}, result{2, "", errs}},
	} {
		code, out, errOut := gocpp(append([]string{"fmt"}, tc.args...)...)
		if got := (result{code, out, errOut}); got != tc.want {
			t.Errorf("gocpp fmt %s:\n%s\nwant:\n%s", strings.Join(tc.args, " "), show(got), show(tc.want))
		}
	}

	// -w writes the good file and leaves the broken one as it is.
	code, out, errOut := gocpp("fmt", "-w", ".")
	if got, want := (result{code, out, errOut}), (result{2, "", errs}); got != want {
		t.Errorf("gocpp fmt -w .:\n%s\nwant:\n%s", show(got), show(want))
	}
	if got := readTestFile(t, "headers.gp"); got != formatted {
		t.Errorf("after gocpp fmt -w, headers.gp holds\n%s\nwant\n%s", got, formatted)
	}
	if got := readTestFile(t, "broken.gp"); got != broken {
		t.Errorf("gocpp fmt -w changed broken.gp, which has a directive error:\n%s", got)
	}

	// Standard input.
	code, out, errOut = gocppStdin(broken, "fmt")
	if got, want := (result{code, out, errOut}), (result{2, "", strings.Replace(errs, "broken.gp", "<standard input>", 1)}); got != want {
		t.Errorf("gocpp fmt with the input %q:\n%s\nwant:\n%s", broken, show(got), show(want))
	}
}

// TestFmtUsage checks -h and a wrong flag. Like gofmt, -h prints the usage
// to stderr and exits with status 0, and a wrong flag also prints the
// error and exits with status 2.
func TestFmtUsage(t *testing.T) {
	code, out, errOut := gocpp("fmt", "-h")
	if code != 0 || out != "" || !strings.HasPrefix(errOut, "usage: gocpp fmt [-l] [-w] [-d] [path ...]\n") {
		t.Errorf("gocpp fmt -h: exit status %d, stdout %q, stderr:\n%s\nwant exit status 0 and the usage on stderr", code, out, errOut)
	}
	for _, flag := range []string{"-d", "-l", "-w"} {
		if !strings.Contains(errOut, "\n  "+flag+"\t") {
			t.Errorf("the usage doesn't explain the flag %s:\n%s", flag, errOut)
		}
	}
	usage := errOut
	code, out, errOut = gocpp("fmt", "-x", "a.gp")
	if code != 2 || out != "" || errOut != "flag provided but not defined: -x\n"+usage {
		t.Errorf("gocpp fmt -x: exit status %d, stdout %q, stderr:\n%s\nwant exit status 2, the error and the usage", code, out, errOut)
	}
	// The lines of the usage fit in 80 columns.
	for line := range strings.Lines(usage) {
		if len(strings.ReplaceAll(strings.TrimSuffix(line, "\n"), "\t", "        ")) > 80 {
			t.Errorf("a line of the usage is wider than 80 columns: %q", line)
		}
	}
}
