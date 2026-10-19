package build

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// The tests of the up-to-date check (stamp.go) that need no compiler: what the loader's file
// system records, how a file is judged (its stat, or its hash when its times are too new), what a
// stamp finds out about a folder, a header, a library and the program.

func writeAt(t *testing.T, path, text string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func TestListingHash(t *testing.T) {
	dir := t.TempDir()
	hash := func() string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		return listingHash(entries)
	}
	writeAt(t, filepath.Join(dir, "a.gp"), "package a\n", time.Hour)
	base := hash()
	// Files the loader does nothing with (the program that the build writes here, notes) do not change it.
	for _, name := range []string{"prog", "notes.txt", "a.out", "x.o"} {
		writeAt(t, filepath.Join(dir, name), "x", time.Hour)
		if got := hash(); got != base {
			t.Fatalf("a file %s changed the listing", name)
		}
	}
	// Source files, folders and links do.
	for _, name := range []string{"b.gp", "c.c", "d.cpp", "e.cc", "f.cxx", "g.h", "gocpp.mod", "gocpp.sum", "h.s"} {
		writeAt(t, filepath.Join(dir, name), "x", time.Hour)
		got := hash()
		if got == base {
			t.Errorf("a file %s did not change the listing", name)
		}
		os.Remove(filepath.Join(dir, name))
		if hash() != base {
			t.Fatalf("removing %s did not restore the listing", name)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if hash() == base {
		t.Error("a folder did not change the listing")
	}
}

func TestFSRecorder(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, filepath.Join(dir, "a.gp"), "package a\n", time.Hour)
	writeAt(t, filepath.Join(dir, "sub", "b.gp"), "package b\n", time.Hour)
	r := NewFSRecorder()
	if data, err := r.ReadFile(filepath.Join(dir, "a.gp")); err != nil || string(data) != "package a\n" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	if _, err := r.ReadFile(filepath.Join(dir, "missing.gp")); err == nil {
		t.Fatal("ReadFile of a missing file succeeded")
	}
	if entries, err := r.ReadDir(dir); err != nil || len(entries) != 2 {
		t.Fatalf("ReadDir = %v, %v", entries, err)
	}
	if _, err := r.ReadDir(filepath.Join(dir, "nodir")); err == nil {
		t.Fatal("ReadDir of a missing folder succeeded")
	}
	if fi, err := r.Stat(filepath.Join(dir, "sub")); err != nil || !fi.IsDir() {
		t.Fatalf("Stat = %v, %v", fi, err)
	}
	if _, err := r.Stat(filepath.Join(dir, "a.gp", "x")); err == nil { // a file is no folder
		t.Fatal("Stat below a file succeeded")
	}
	rc, err := r.Open(filepath.Join(dir, "sub", "b.gp"))
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if r.Bad() != "" {
		t.Fatalf("Bad = %q", r.Bad())
	}
	// A second look at the same name is not recorded twice.
	r.ReadFile(filepath.Join(dir, "a.gp"))
	n := r.Ops()
	r.ReadFile(filepath.Join(dir, "a.gp"))
	if r.Ops() != n {
		t.Errorf("a second read recorded again: %d ops, was %d", r.Ops(), n)
	}
	if got := r.FileHash(filepath.Join(dir, "a.gp")); got != hashHex([]byte("package a\n")) {
		t.Errorf("FileHash = %q", got)
	}
	// A file that reads differently the second time changed while the program was loaded.
	if err := os.WriteFile(filepath.Join(dir, "a.gp"), []byte("package changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.ReadFile(filepath.Join(dir, "a.gp"))
	if !strings.Contains(r.Bad(), "changed while it was loaded") {
		t.Errorf("Bad = %q after a file that changed between two reads", r.Bad())
	}
	// A read that fails for another reason than "does not exist" makes the load unstampable.
	r2 := NewFSRecorder()
	if _, err := r2.ReadFile(dir); err == nil { // a folder is not a file
		t.Fatal("ReadFile of a folder succeeded")
	}
	if r2.Bad() == "" {
		t.Error("a failed read did not make the load unstampable")
	}
}

// checkFile trusts a stat that matches and is old enough; a stat that does not match, or a record that is
// too new to trust, is decided by the hash.
func TestStampCheckFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.h")
	writeAt(t, p, "one\n", time.Hour)
	fi, _ := os.Stat(p)
	stat := statMemo().Stat
	var s Stamp

	rec := recOf(fi, time.Now())
	rec.Hash = hashHex([]byte("one\n"))
	// The change time is new (the file was just written and its times set back): racy, by the ctime.
	if !rec.Racy {
		t.Fatal("a file that was written a moment ago, with its times set back, is not racy: the change time must tell")
	}
	// Racy: decided by the hash, whatever the stat says.
	if !s.checkFile(p, &rec, stat) {
		t.Error("a racy record of an unchanged file is not accepted")
	}
	writeAt(t, p, "two\n", time.Hour) // the same size, the times set back: only the content differs
	if s.checkFile(p, &rec, statMemo().Stat) {
		t.Error("a racy record accepts a file with other content and the same stat")
	}

	// Not racy (the record says so): the stat decides, and the file is not read.
	fi, _ = os.Stat(p)
	trusted := recOf(fi, time.Now())
	trusted.Racy = false
	trusted.Hash = "not the hash of anything"
	if !s.checkFile(p, &trusted, statMemo().Stat) {
		t.Error("a record that matches the stat and is not racy is not trusted")
	}
	// A stat that does not match, and no hash: not accepted.
	other := trusted
	other.Chg++
	other.Hash = ""
	if s.checkFile(p, &other, statMemo().Stat) {
		t.Error("a record with another change time and no hash is accepted")
	}
	// A stat that does not match, but the content has the hash: accepted, and the record is renewed.
	same := trusted
	same.Chg++
	same.Hash = hashHex([]byte("two\n"))
	s.dirty = false
	if !s.checkFile(p, &same, statMemo().Stat) {
		t.Error("a touched file with the recorded content is not accepted")
	}
	if wrong := hashHex([]byte("three\n")); s.checkFile(p, &statRec{Size: fi.Size(), Mod: 1, Hash: wrong}, statMemo().Stat) {
		t.Error("a file with another hash is accepted")
	}
	// A missing file, and a folder in its place.
	if s.checkFile(filepath.Join(dir, "nope"), &trusted, statMemo().Stat) || s.checkFile(dir, &trusted, statMemo().Stat) {
		t.Error("a missing file, or a folder, is accepted for a file")
	}
}

// A stamp finds out what changed under the trace: a file, a folder's files, a file that appeared or
// disappeared, and the program.
func TestStampCheckTrace(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	writeAt(t, filepath.Join(pkg, "a.gp"), "package a\n", time.Hour)
	writeAt(t, filepath.Join(root, "gocpp.mod"), "module m\n", time.Hour)
	prog := filepath.Join(root, "prog")
	writeAt(t, prog, "binary", time.Hour)

	r := NewFSRecorder()
	r.ReadFile(filepath.Join(root, "gocpp.mod"))
	r.ReadFile(filepath.Join(pkg, "a.gp"))
	r.ReadDir(pkg)
	r.Stat(filepath.Join(root, "vendor"))        // does not exist
	r.ReadFile(filepath.Join(root, "gocpp.sum")) // does not exist
	rec := &Record{}
	rec.link = &LinkRec{}
	mk := func() *Stamp {
		s, why := NewStamp("key", r, nil, rec, []string{"cc"}, prog, nil)
		if s == nil {
			t.Fatalf("NewStamp: %s", why)
		}
		return s
	}
	check := func(s *Stamp) string { return s.Check(context.Background(), t.TempDir(), []string{"cc"}, nil) }

	if why := check(mk()); why != "" {
		t.Fatalf("a fresh stamp is not current: %s", why)
	}
	if why := mk().Check(context.Background(), t.TempDir(), []string{"other cc"}, nil); !strings.Contains(why, "compiler") {
		t.Errorf("another compiler: %q", why)
	}
	for _, tc := range []struct {
		name   string
		change func()
		undo   func()
		want   string
	}{
		{"source", func() { writeAt(t, filepath.Join(pkg, "a.gp"), "package b\n", time.Hour) },
			func() { writeAt(t, filepath.Join(pkg, "a.gp"), "package a\n", time.Hour) }, "a.gp changed"},
		{"module file", func() { writeAt(t, filepath.Join(root, "gocpp.mod"), "module n\n", time.Hour) },
			func() { writeAt(t, filepath.Join(root, "gocpp.mod"), "module m\n", time.Hour) }, "gocpp.mod changed"},
		{"new source in the folder", func() { writeAt(t, filepath.Join(pkg, "b.gp"), "package a\n", time.Hour) },
			func() { os.Remove(filepath.Join(pkg, "b.gp")) }, "the files of " + pkg + " changed"},
		{"new C file in the folder", func() { writeAt(t, filepath.Join(pkg, "c.c"), "", time.Hour) },
			func() { os.Remove(filepath.Join(pkg, "c.c")) }, "the files of " + pkg + " changed"},
		{"removed source", func() { os.Rename(filepath.Join(pkg, "a.gp"), filepath.Join(pkg, "a.gp.moved")) },
			func() { os.Rename(filepath.Join(pkg, "a.gp.moved"), filepath.Join(pkg, "a.gp")) }, "a.gp changed"},
		{"vendor appears", func() { os.Mkdir(filepath.Join(root, "vendor"), 0o755) },
			func() { os.Remove(filepath.Join(root, "vendor")) }, "vendor appeared"},
		{"gocpp.sum appears", func() { writeAt(t, filepath.Join(root, "gocpp.sum"), "x", time.Hour) },
			func() { os.Remove(filepath.Join(root, "gocpp.sum")) }, "gocpp.sum appeared"},
		{"the program is removed", func() { os.Rename(prog, prog+".moved") },
			func() { os.Rename(prog+".moved", prog) }, "prog is missing or was changed"},
		{"the program is changed", func() { writeAt(t, prog, "BINARY", time.Hour) },
			func() { writeAt(t, prog, "binary", time.Hour) }, "prog is missing or was changed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := mk()
			tc.change()
			why := check(s)
			tc.undo()
			if !strings.Contains(why, tc.want) {
				t.Errorf("reason %q, want %q", why, tc.want)
			}
			if why := check(mk()); why != "" {
				t.Errorf("after undoing the change the stamp is not current: %s", why)
			}
		})
	}
	// A file that is no source, and the program being written again with the same content, change nothing.
	writeAt(t, filepath.Join(pkg, "notes.txt"), "x", time.Hour)
	if why := check(mk()); why != "" {
		t.Errorf("a note in the folder: %s", why)
	}
}

// The headers that a compile read: a header that changed, a source that changed and a file that
// appears in front of a header (the Absent files) are found, and the stamp does not need the object
// cache to say so.
func TestStampCheckUnits(t *testing.T) {
	cache := t.TempDir()
	src := filepath.Join(t.TempDir(), "u.cpp")
	inc1, inc2 := t.TempDir(), t.TempDir()
	hdr := filepath.Join(inc2, "x.h")
	other := filepath.Join(inc2, "y.h")
	writeAt(t, src, "int u;\n", time.Hour)
	writeAt(t, hdr, "int h;\n", time.Hour)
	writeAt(t, other, "int o;\n", time.Hour)
	prog := filepath.Join(t.TempDir(), "prog")
	writeAt(t, prog, "b", time.Hour)

	drec, _, err := recordFile(hdr, time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	orec, _, err := recordFile(other, time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	_, srec, err := readRecorded(src)
	if err != nil {
		t.Fatal(err)
	}
	r := NewFSRecorder()
	rec := &Record{link: &LinkRec{}}
	rec.addUnit(UnitRec{Src: src, SrcRec: srec, deps: []depRec{drec}, Absent: []absentGroup{{Dirs: []string{inc1}, Rels: []string{"x.h"}}}})
	// A second unit reads the same header and one more: the stamp keeps each header once.
	rec.addUnit(UnitRec{Src: src + "2", SrcRec: srec, deps: []depRec{drec, orec}})
	mk := func() *Stamp {
		s, why := NewStamp("key", r, nil, rec, nil, prog, nil)
		if s == nil {
			t.Fatalf("NewStamp: %s", why)
		}
		return s
	}
	if s := mk(); len(s.Files) != 2 || len(s.Units[0].Deps) != 1 || len(s.Units[1].Deps) != 2 {
		t.Fatalf("the stamp has %d files and unit deps %v %v, want 2 files shared by the units", len(s.Files), s.Units[0].Deps, s.Units[1].Deps)
	}
	check := func(s *Stamp) string { return s.Check(context.Background(), cache, nil, nil) }
	// The source of the second unit is not a file: give both units the same file.
	rec.units[1].Src = src
	if why := check(mk()); why != "" {
		t.Fatalf("a fresh stamp is not current: %s", why)
	}
	writeAt(t, hdr, "int H;\n", time.Hour)
	if why := check(mk()); !strings.Contains(why, "the header "+hdr+" changed") {
		t.Errorf("a changed header: %q", why)
	}
	writeAt(t, hdr, "int h;\n", time.Hour)
	writeAt(t, other, "int O;\n", time.Hour)
	if why := check(mk()); !strings.Contains(why, "the header "+other+" changed") {
		t.Errorf("a header of the second unit only: %q", why)
	}
	writeAt(t, other, "int o;\n", time.Hour)
	if why := check(mk()); why != "" {
		t.Errorf("the headers restored: %q", why)
	}
	writeAt(t, filepath.Join(inc1, "x.h"), "int shadow;\n", time.Hour) // a header in front of the one that was read
	if why := check(mk()); !strings.Contains(why, "a file appeared in front of a header of "+src) {
		t.Errorf("a header that hides another: %q", why)
	}
	os.Remove(filepath.Join(inc1, "x.h"))
	writeAt(t, src, "int V;\n", time.Hour)
	if why := check(mk()); !strings.Contains(why, src+" changed") {
		t.Errorf("a changed source: %q", why)
	}
	writeAt(t, src, "int u;\n", time.Hour)
	// A header that was touched but has its content is accepted, and the stamp learns its new stat.
	s := mk()
	later := time.Now().Add(-30 * time.Minute)
	os.Chtimes(hdr, later, later)
	if why := check(s); why != "" {
		t.Errorf("a touched header: %q", why)
	}
}

func TestLinkFlagProblem(t *testing.T) {
	for _, tc := range []struct {
		flags []string
		bad   bool
	}{
		{nil, false},
		{[]string{"-lm", "-lpthread", "-L/opt/lib", "-L", "/opt/lib2", "-l", "dl"}, false},
		{[]string{"-pthread", "-static", "-rdynamic", "-s"}, false},
		{[]string{"-Wl,-rpath,/opt/lib", "-Wl,--as-needed", "-Wl,-z,relro", "-Wl,-rpath=/x", "-Wl,--gc-sections"}, false},
		{[]string{"/opt/lib/libfoo.a"}, false},
		{[]string{"-Wl,--version-script=v.map"}, true},
		{[]string{"-Wl,-Map=out.map"}, true},
		{[]string{"-Wl,--dynamic-list=x"}, true},
		{[]string{"-fuse-ld=lld"}, true},
		{[]string{"@args.txt"}, true},
		{[]string{"foo.o"}, true}, // a relative file: the work folder is not the user's
		{[]string{"-T", "script.ld"}, true},
		{[]string{"-lm", "-Wl,--version-script=v.map"}, true},
	} {
		if got := linkFlagProblem(tc.flags); (got != "") != tc.bad {
			t.Errorf("linkFlagProblem(%q) = %q, want a problem: %v", tc.flags, got, tc.bad)
		}
	}
}

func TestLinkTrace(t *testing.T) {
	work := t.TempDir()
	lib := filepath.Join(t.TempDir(), "libx.so")
	writeAt(t, lib, "x", time.Hour)
	writeAt(t, filepath.Join(work, "00-a.o"), "o", time.Hour)
	writeAt(t, filepath.Join(work, "00-b.o"), "o", time.Hour)
	arch := filepath.Join(t.TempDir(), "libz.a")
	writeAt(t, arch, "z", time.Hour)
	out := lib + "\n00-a.o\n" + filepath.Join(work, "00-b.o") + "\n/usr/bin/ld: warning: something is odd\n" + lib + "\n" + "undefined reference to `f'\n" +
		"(" + arch + ")member.o\n" + arch + "(other.o)\n(/no/such/lib.a)x.o\nprint(this)\n"
	rest, files := linkTrace([]byte(out), work)
	if len(files) != 2 || files[0] != lib || files[1] != arch {
		t.Errorf("files = %q, want the library and the archive once each (not the objects of the build, and the members are the archive)", files)
	}
	if want := "/usr/bin/ld: warning: something is odd\nundefined reference to `f'\n(/no/such/lib.a)x.o\nprint(this)\n"; string(rest) != want {
		t.Errorf("rest = %q, want %q", rest, want)
	}
}

// The libraries that the link read and the linker program are inputs: a changed one is found.
func TestStampCheckLink(t *testing.T) {
	dir := t.TempDir()
	lib, ld := filepath.Join(dir, "libx.so"), filepath.Join(dir, "ld")
	prog := filepath.Join(dir, "prog")
	for _, p := range []string{lib, ld, prog} {
		writeAt(t, p, "one", time.Hour)
	}
	mkLink := func(p, flag string) LinkFile {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return linkFile(flag, p, fi, time.Now())
	}
	r := NewFSRecorder()
	rec := &Record{}
	l := mkLink(ld, "ld")
	rec.link = &LinkRec{Files: []LinkFile{mkLink(lib, "")}, Linker: &l}
	mk := func() *Stamp {
		s, why := NewStamp("key", r, nil, rec, nil, prog, nil)
		if s == nil {
			t.Fatalf("NewStamp: %s", why)
		}
		return s
	}
	check := func(s *Stamp) string { return s.Check(context.Background(), t.TempDir(), nil, nil) }
	if why := check(mk()); why != "" {
		t.Fatalf("a fresh stamp is not current: %s", why)
	}
	writeAt(t, lib, "two", time.Hour)
	if why := check(mk()); !strings.Contains(why, "libx.so changed") {
		t.Errorf("a changed library: %q", why)
	}
	writeAt(t, lib, "one", time.Hour)
	writeAt(t, ld, "two", time.Hour)
	if why := check(mk()); !strings.Contains(why, "the linker "+ld+" changed") {
		t.Errorf("a changed linker: %q", why)
	}
	writeAt(t, ld, "one", time.Hour)
	os.Remove(lib)
	if why := check(mk()); !strings.Contains(why, "libx.so changed") {
		t.Errorf("a library that is gone: %q", why)
	}
}

// The answers of pkg-config are asked again, with the same arguments, and compared.
func TestStampCheckPkgConfig(t *testing.T) {
	prog := filepath.Join(t.TempDir(), "prog")
	writeAt(t, prog, "b", time.Hour)
	var calls []string
	answers := PkgConfigRec{Dir: "/d", Pkgs: []string{"x"}, CFlags: []string{"-DX"}, Libs: []string{"-lx"}}
	fake := func(ctx context.Context, dir string, pkgs []string) ([]string, []string, error) {
		calls = append(calls, dir+" "+strings.Join(pkgs, ","))
		if answers.Err != "" {
			return nil, nil, os.ErrInvalid
		}
		return answers.CFlags, answers.Libs, nil
	}
	var log PkgConfigLog
	wrapped := log.Wrap(fake)
	if _, _, err := wrapped(context.Background(), "/d", []string{"x"}); err != nil {
		t.Fatal(err)
	}
	rec := &Record{link: &LinkRec{}}
	s, why := NewStamp("key", NewFSRecorder(), &log, rec, nil, prog, nil)
	if s == nil {
		t.Fatalf("NewStamp: %s", why)
	}
	check := func() string { return s.Check(context.Background(), t.TempDir(), nil, fake) }
	calls = nil
	if why := check(); why != "" || len(calls) != 1 || calls[0] != "/d x" {
		t.Fatalf("a fresh stamp: %q, pkg-config runs %q", why, calls)
	}
	answers.CFlags = []string{"-DY"}
	if why := check(); !strings.Contains(why, "pkg-config x answers differently") {
		t.Errorf("other cflags: %q", why)
	}
	answers.CFlags = []string{"-DX"}
	answers.Libs = []string{"-lx", "-ly"}
	if why := check(); !strings.Contains(why, "answers differently") {
		t.Errorf("other libs: %q", why)
	}
	answers.Libs = []string{"-lx"}
	answers.Err = "x"
	if why := check(); !strings.Contains(why, "answers differently") {
		t.Errorf("an error: %q", why)
	}
}

// The compilers are part of the stamp, and so are the other things that make its identity.
func TestStampKey(t *testing.T) {
	a, b := StampKey("x", "y"), StampKey("xy", "")
	if a == b || a == StampKey("x", "y", "") || StampKey("x", "y") != a {
		t.Errorf("StampKey does not tell its parts apart: %s %s", a, b)
	}
	if got := CompilerID(CXX{Path: "/a/g++", Version: "g++ 15", Family: GCC}); got == CompilerID(CXX{Path: "/a/g++", Version: "g++ 16", Family: GCC}) ||
		got == CompilerID(CXX{Path: "/a/g++", Args: []string{"x"}, Version: "g++ 15", Family: GCC}) ||
		got == CompilerID(CXX{Path: "/b/g++", Version: "g++ 15", Family: GCC}) {
		t.Error("CompilerID does not tell compilers apart")
	}
}

func TestChangedPackages(t *testing.T) {
	old := []PkgSummary{{"a", "s1", "p1"}, {"b", "s2", "p2"}, {"gone", "s", "p"}}
	now := []PkgSummary{{"a", "s1x", "p1"}, {"b", "s2", "p2"}, {"new", "s", "p"}, {"c", "s3", "p3x"}}
	src, api := ChangedPackages(old, now)
	if want := "a new c"; strings.Join(src, " ") != want {
		t.Errorf("sources changed: %v, want %s", src, want)
	}
	if want := "new c"; strings.Join(api, " ") != want {
		t.Errorf("api changed: %v, want %s", api, want)
	}
	if s, a := ChangedPackages(now, now); len(s) != 0 || len(a) != 0 {
		t.Errorf("nothing changed: %v %v", s, a)
	}
}

// The identity of a compiler has the content of its program file: a file replaced by another of the
// same size, with its old modification time, is another compiler.
func TestCompilerIDContent(t *testing.T) {
	dir := t.TempDir()
	prog := fakeCXX(t, dir, "g++", "g++ (fake) 99.1.0", gccMacros...)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(prog, old, old)
	c, err := LookCXX(context.Background(), prog)
	if err != nil {
		t.Fatal(err)
	}
	before := CompilerID(c)
	data := mustRead(t, prog)
	changed := append([]byte(nil), data...)
	changed[len(changed)-3] ^= 1 // one byte of the script, in a comment-free tail: the size is the same
	if err := os.WriteFile(prog, changed, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(prog, old, old)
	if fi, _ := os.Stat(prog); fi.Size() != int64(len(data)) {
		t.Fatal("the size changed")
	}
	// The in-process answer of the detection still names the old file (same size and time): only the
	// content tells.
	if after := CompilerID(c); after == before {
		t.Errorf("the identity of a compiler that was replaced by a file of the same size and time is the same")
	}
}

// The permission bits are part of what the check keeps of a file: a chmod does not change the
// content, and is seen whether the record is trusted (the stat differs) or racy (the hash decides).
func TestStampCheckFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no permission bits")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "prog")
	writeAt(t, p, "program\n", time.Hour)
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	var s Stamp
	fi, _ := os.Stat(p)
	rec := recOf(fi, time.Now())
	rec.Hash = hashHex([]byte("program\n"))
	if !rec.Racy {
		t.Fatal("the record of a file that was just changed is not racy")
	}
	// The record was taken while racy; the stat is refreshed when the file is checked after the
	// racy window, and the hash stays (checkFile).
	if !s.checkFile(p, &rec, statMemo().Stat) {
		t.Fatal("an unchanged file is not accepted")
	}
	renewed := rec
	renewed.Racy = false
	if !s.checkFile(p, &renewed, statMemo().Stat) {
		t.Fatal("a trusted record of an unchanged file is not accepted")
	}
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if s.checkFile(p, &renewed, statMemo().Stat) {
		t.Error("a trusted record accepts a program that lost its execute bits")
	}
	racy := rec
	if s.checkFile(p, &racy, statMemo().Stat) {
		t.Error("a racy record accepts a program that lost its execute bits (the hash is the same)")
	}
	// Both with the ctime of the record, as when the change time did not move: the mode is compared by itself.
	fi, _ = os.Stat(p)
	same := recOf(fi, time.Now())
	same.Racy = true
	same.Hash = hashHex([]byte("program\n"))
	same.Mode = 0o755
	if s.checkFile(p, &same, statMemo().Stat) {
		t.Error("a record with the old mode and the same hash is accepted")
	}
}

// TrimOld and TrimStamps never go into a folder that is a link: what it points at is not the cache's.
func TestTrimNotThroughLinks(t *testing.T) {
	cache := t.TempDir()
	canary := t.TempDir()
	for _, name := range []string{"a.json", ".tmp-x"} {
		writeAt(t, filepath.Join(canary, name), "canary", 400*24*time.Hour)
	}
	for _, f := range []string{"stamp", "cxxid"} {
		if err := os.Symlink(canary, filepath.Join(cache, f)); err != nil {
			t.Skip(err)
		}
	}
	TrimStamps(cache)
	TrimOld(filepath.Join(cache, "stamp"), time.Hour, time.Now())
	for _, name := range []string{"a.json", ".tmp-x", ".trimmed"} {
		_, err := os.Lstat(filepath.Join(canary, name))
		if name == ".trimmed" && err == nil {
			t.Errorf("a marker was made in the folder that a link points at")
		}
		if name != ".trimmed" && err != nil {
			t.Errorf("%s of the folder that a link points at was removed", name)
		}
	}
}

// The temporary objects of a link with -flto (deleted before the trace is read) are no message.
func TestLinkTraceDropsTempObjects(t *testing.T) {
	tmp := t.TempDir()
	here := filepath.Join(tmp, "kept.o") // exists: not a temporary object that is gone, and in the work folder it is not recorded
	writeAt(t, here, "o", time.Hour)
	out := filepath.Join(tmp, "lto-llvm-976b28.o") + "\n" + filepath.Join(tmp, "ccAbC.ltrans0.ltrans.o") + "\n" +
		filepath.Join(tmp, "ccAbC.debug.temp.o") + "\n/other/dir/gone.o\n" + filepath.Join(tmp, "has space.o") + "\nmessage.o\n"
	rest, files := linkTrace([]byte(out), t.TempDir())
	if want := "/other/dir/gone.o\n" + filepath.Join(tmp, "has space.o") + "\nmessage.o\n"; string(rest) != want || len(files) != 0 {
		t.Errorf("rest = %q, files = %q; want only %q", rest, files, want)
	}
	if !tempObject(filepath.Join(tmp, "x.o")) || tempObject(here) || tempObject("rel.o") {
		t.Errorf("tempObject: a missing .o in the temporary folder only")
	}
}

func TestAutoLTO(t *testing.T) {
	in := []string{"-O2", "-flto", "-lm"}
	for _, c := range []struct {
		fam  Family
		want []string
	}{{GCC, []string{"-O2", "-flto=auto", "-lm"}}, {Clang, in}} {
		b := &Builder{CXX: CXX{Family: c.fam}}
		if got := b.autoLTO(in); !slices.Equal(got, c.want) {
			t.Errorf("%v: autoLTO = %q, want %q", c.fam, got, c.want)
		}
	}
	if in[1] != "-flto" {
		t.Errorf("autoLTO changed its argument: %q", in)
	}
}

// A c-archive writes a header next to the sources besides the library: a header is a file the
// loader looks at, so the listing of the folder that the build read changed during it, and both
// files are the build's own output (NewStamp gets the header as an extra output).
func TestNewStampSettlesListingWithExtraOutputs(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, filepath.Join(dir, "a.gp"), "package main\n", time.Hour)
	r := NewFSRecorder()
	rec := &Record{link: &LinkRec{}}
	if _, err := r.ReadDir(dir); err != nil {
		t.Fatal(err)
	}
	lib, hdr := filepath.Join(dir, "libu.a"), filepath.Join(dir, "libu.h")
	writeAt(t, lib, "lib", time.Hour)
	writeAt(t, hdr, "hdr", time.Hour)
	if s, why := NewStamp("key", r, nil, rec, nil, lib, nil); s != nil || !strings.Contains(why, "changed during the build") {
		t.Fatalf("without the header as an output: %v, %q", s, why)
	}
	s, why := NewStamp("key", r, nil, rec, nil, lib, nil, hdr)
	if s == nil {
		t.Fatalf("NewStamp: %s", why)
	}
	if why := s.AddOutputs([]string{hdr}); why != "" {
		t.Fatal(why)
	}
	if why := s.Check(context.Background(), t.TempDir(), nil, nil); why != "" {
		t.Errorf("the stamp is not current: %s", why)
	}
}
