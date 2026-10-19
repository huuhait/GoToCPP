package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/winpath"
)

// The tests of the Windows build driver (Phase 14 step 14.2) that run on every system: the response
// file for a command that is too long, the quoting of its lines, the manifest object, the flags of
// DLL builds. What needs Windows itself (job objects, LockFileEx, MoveFileEx, the real compilers) is
// tested on the real PC: tests/windows_native_test.go and the *_windows_test.go files here.

// gnuTokens splits a response file the way GCC does (libiberty's buildargv): white space separates
// words, single and double quotes group, a backslash makes the next character literal.
func gnuTokens(s string) []string {
	var words []string
	var cur strings.Builder
	have := false
	var quote rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '\\' && i+1 < len(rs):
			i++
			cur.WriteRune(rs[i])
			have = true
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteRune(c)
			}
		case c == '"' || c == '\'':
			quote, have = c, true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if have {
				words = append(words, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteRune(c)
			have = true
		}
	}
	if have {
		words = append(words, cur.String())
	}
	return words
}

// winTokens splits a response file the way LLVM's Windows tokenizer does (CommandLineToArgvW's rules): a
// backslash is literal unless it comes before a double quote, and only double quotes group.
func winTokens(s string) []string {
	var words []string
	var cur strings.Builder
	have, in := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			n := 0
			for i < len(s) && s[i] == '\\' {
				n++
				i++
			}
			if i < len(s) && s[i] == '"' {
				cur.WriteString(strings.Repeat(`\`, n/2))
				if n%2 == 1 {
					cur.WriteByte('"')
				} else {
					in = !in
				}
			} else {
				cur.WriteString(strings.Repeat(`\`, n))
				i--
			}
			have = true
		case c == '"':
			in, have = !in, true
		case (c == ' ' || c == '\t' || c == '\n' || c == '\r') && !in:
			if have {
				words = append(words, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteByte(c)
			have = true
		}
	}
	if have {
		words = append(words, cur.String())
	}
	return words
}

func TestRspQuoteWindows(t *testing.T) {
	// Arguments of a Windows build: a drive letter, spaces, quotes in a definition, an empty word.
	args := []string{
		"-c", `C:\Users\Some One\proj\a b.cpp`, `-IC:\Program Files\x\include`, `-DNAME="Joe Doe"`, `-DQ='x'`, "", "-o", `D:\out dir\a.o`,
	}
	// A Windows target writes the separators as slashes first (both parsers read them the same).
	want := []string{
		"-c", "C:/Users/Some One/proj/a b.cpp", "-IC:/Program Files/x/include", `-DNAME="Joe Doe"`, `-DQ='x'`, "", "-o", "D:/out dir/a.o",
	}
	var text strings.Builder
	for _, a := range args {
		text.WriteString(rspQuote(a, true) + "\n")
	}
	for name, got := range map[string][]string{"gcc": gnuTokens(text.String()), "clang (Windows rules)": winTokens(text.String())} {
		if !slices.Equal(got, want) {
			t.Errorf("%s reads\n%q\nwant\n%q\nfrom\n%s", name, got, want, text.String())
		}
	}
	// A system with backslash-free paths: the arguments are written as they are.
	text.Reset()
	for _, a := range []string{"/home/me/a b.cpp", `-DS="x"`, "plain"} {
		text.WriteString(rspQuote(a, false) + "\n")
	}
	if got, want := gnuTokens(text.String()), []string{"/home/me/a b.cpp", `-DS="x"`, "plain"}; !slices.Equal(got, want) {
		t.Errorf("non-Windows round trip: %q", got)
	}
}

func TestFitCommand(t *testing.T) {
	old := compileLimit
	compileLimit = 100
	t.Cleanup(func() { compileLimit = old })
	work := t.TempDir()
	b := &Builder{GOOS: "windows", Work: work}
	cxx := CXX{Path: `C:\llvm\bin\x86_64-w64-mingw32-clang++.exe`, Args: []string{"-std=c++20"}}

	// A short command is unchanged and leaves no file.
	short := []string{"-c", "a.cpp"}
	got, done, err := b.fitCommand(cxx, short)
	if err != nil || !slices.Equal(got, short) {
		t.Fatalf("short command = %q, %v", got, err)
	}
	done()
	if entries, _ := os.ReadDir(work); len(entries) != 0 {
		t.Errorf("a short command left %d files", len(entries))
	}

	// A long one goes into a response file, whose words are the arguments.
	var long []string
	for i := 0; i < 20; i++ {
		long = append(long, `-IC:\very long include folder\number `+string(rune('a'+i)))
	}
	long = append(long, "-c", "a.cpp")
	got, done, err = b.fitCommand(cxx, long)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "@") || filepath.Dir(got[0][1:]) != work {
		t.Fatalf("long command = %q, want one @file in the work folder", got)
	}
	data, err := os.ReadFile(got[0][1:])
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, a := range long {
		want = append(want, strings.ReplaceAll(a, `\`, "/"))
	}
	if toks := gnuTokens(string(data)); !slices.Equal(toks, want) {
		t.Errorf("response file words = %q, want %q", toks, want)
	}
	// Two long commands have two files, and done removes its own.
	got2, done2, _ := b.fitCommand(cxx, long)
	if got2[0] == got[0] {
		t.Error("two commands share a response file")
	}
	done()
	if _, err := os.Stat(got[0][1:]); !os.IsNotExist(err) {
		t.Error("done did not remove the response file")
	}
	if _, err := os.Stat(got2[0][1:]); err != nil {
		t.Error("done removed the other command's file")
	}
	done2()
	// No work folder: nothing to write into, the command stays.
	b0 := &Builder{GOOS: "windows"}
	if got, _, _ := b0.fitCommand(cxx, long); !slices.Equal(got, long) {
		t.Error("a builder without a work folder changed the command")
	}
}

func TestCommandLength(t *testing.T) {
	c := CXX{Path: "/b/g++", Args: []string{"-x"}}
	if got, want := commandLength(c, []string{"ab", "c"}), len("/b/g++")+1+len("-x")+1+len("ab")+1+len("c"); got != want {
		t.Errorf("commandLength = %d, want %d", got, want)
	}
	// A program with a space is quoted for CreateProcess.
	if got, want := commandLength(CXX{Path: `C:\Program Files\x\g++.exe`}, nil), len(`"C:\Program Files\x\g++.exe"`); got != want {
		t.Errorf("commandLength with a space = %d, want %d", got, want)
	}
}

// refEscapeArg is syscall.EscapeArg of Go 1.25 (appendEscapeArg), which exists on Windows only: the
// reference for escapedLen.
func refEscapeArg(s string) string {
	if len(s) == 0 {
		return `""`
	}
	needsBackslash, hasSpace := false, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			needsBackslash = true
		case ' ', '\t':
			hasSpace = true
		}
	}
	if !needsBackslash && !hasSpace {
		return s
	}
	if !needsBackslash {
		return `"` + s + `"`
	}
	var b []byte
	if hasSpace {
		b = append(b, '"')
	}
	slashes := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		default:
			slashes = 0
		case '\\':
			slashes++
		case '"':
			for ; slashes > 0; slashes-- {
				b = append(b, '\\')
			}
			b = append(b, '\\')
		}
		b = append(b, c)
	}
	if hasSpace {
		for ; slashes > 0; slashes-- {
			b = append(b, '\\')
		}
		b = append(b, '"')
	}
	return string(b)
}

// escapedLen is the length of the argument as os/exec writes it for CreateProcess.
func TestEscapedLen(t *testing.T) {
	for _, s := range []string{"", "a", "a b", " ", "\t", `"`, `""`, `a"b`, `a b"c`, `\`, `\\`, `a\`, `a b\`, `a b\\`, `a\"b`, `a b\"c`, `a b\\"c`, `C:\Program Files\x\`, `-DX="a b"`, `-DX=\"a\\"`, "x\ty"} {
		if got, want := escapedLen(s), len(refEscapeArg(s)); got != want {
			t.Errorf("escapedLen(%q) = %d, want %d (%s)", s, got, want, refEscapeArg(s))
		}
	}
}

// The boundary: a command of exactly compileLimit characters (as CreateProcess counts them) runs on the command line; one
// more goes into a response file. Many short arguments with spaces pass 32,767 once quoted though their raw
// length is far below it, and the default limit leaves room for that.
func TestFitCommandBoundary(t *testing.T) {
	old := compileLimit
	compileLimit = windowsCompileLimit // the limit of a Windows host, whatever this system's is
	t.Cleanup(func() { compileLimit = old })
	work := t.TempDir()
	b := &Builder{GOOS: "windows", Work: work}
	cxx := CXX{Path: `C:\llvm\g++.exe`}
	// filler returns arguments that bring the command to exactly n characters.
	filler := func(n int) []string {
		args := []string{"-c", "a.cpp"}
		pad := n - commandLength(cxx, args) - 1 // the space before the padding argument
		if pad < 1 {
			t.Fatalf("limit %d is too small", n)
		}
		return append(args, strings.Repeat("x", pad))
	}
	for _, c := range []struct {
		n    int
		wrap bool
	}{{compileLimit - 1, false}, {compileLimit, false}, {compileLimit + 1, true}} {
		args := filler(c.n)
		if got := commandLength(cxx, args); got != c.n {
			t.Fatalf("test arguments have length %d, want %d", got, c.n)
		}
		got, done, err := b.fitCommand(cxx, args)
		if err != nil {
			t.Fatal(err)
		}
		done()
		if wrapped := len(got) == 1 && strings.HasPrefix(got[0], "@"); wrapped != c.wrap {
			t.Errorf("a command of %d characters (limit %d): wrapped = %v, want %v", c.n, compileLimit, wrapped, c.wrap)
		}
	}
	// 7000 arguments of "a b" are 28,000 bytes with their separators, and 42,000 when each is quoted for CreateProcess: past the
	// 32,767 of the system while the raw bytes are not past the limit.
	var many []string
	for i := 0; i < 7000; i++ {
		many = append(many, "a b")
	}
	raw := 0
	for _, a := range many {
		raw += len(a) + 1
	}
	if raw >= compileLimit || commandLength(cxx, many) <= compileLimit {
		t.Fatalf("the test arguments: raw %d, quoted %d, limit %d", raw, commandLength(cxx, many), compileLimit)
	}
	got, done, _ := b.fitCommand(cxx, many)
	done()
	if len(got) != 1 || !strings.HasPrefix(got[0], "@") {
		t.Error("quoted arguments past the limit were not put into a response file")
	}
	// CreateProcess takes 32,767 characters: the limit is below it with room for the quoting of the program.
	if windowsCompileLimit >= 32767-1000 || windowsCompileLimit < 20000 {
		t.Errorf("windowsCompileLimit = %d, want 20,000 to 31,767", windowsCompileLimit)
	}
}

func TestModeFlagsPerOS(t *testing.T) {
	for _, c := range []struct {
		mode        Mode
		goos        string
		fam         Family
		compile     []string
		linkContain []string
		linkAbsent  []string
	}{
		{ModeCShared, "linux", GCC, []string{"-fPIC", "-fvisibility=hidden", "-fvisibility-inlines-hidden", "-fno-gnu-unique"}, []string{"-shared", "-Wl,-soname,libx.so", "-Wl,--version-script=v.map"}, nil},
		// A DLL: no -fPIC (all code is position independent), no GNU unique symbols, no soname, no version script; the import library.
		{ModeCShared, "windows", GCC, []string{"-fvisibility=hidden", "-fvisibility-inlines-hidden"}, []string{"-shared", "-Wl,--out-implib,x.dll.a"}, []string{"-Wl,-soname,libx.so", "-Wl,--version-script=v.map"}},
		{ModeCArchive, "windows", Clang, nil, nil, nil},
		{ModeCPPLib, "windows", GCC, nil, nil, nil},
		{ModeCArchive, "linux", GCC, []string{"-fPIC"}, nil, nil},
	} {
		if got := c.mode.CompileFlagsFor(c.goos, c.fam, false); !slices.Equal(got, c.compile) {
			t.Errorf("%s on %s: compile flags = %v, want %v", c.mode, c.goos, got, c.compile)
		}
		link := c.mode.LinkFlagsFor(c.goos, "libx.so", "v.map", "x.dll.a")
		for _, w := range c.linkContain {
			if !slices.Contains(link, w) {
				t.Errorf("%s on %s: link flags %v lack %s", c.mode, c.goos, link, w)
			}
		}
		for _, w := range c.linkAbsent {
			if slices.Contains(link, w) {
				t.Errorf("%s on %s: link flags %v have %s", c.mode, c.goos, link, w)
			}
		}
	}
	// The old entry points are the Linux ones.
	if got := ModeCShared.CompileFlags(GCC, false); !slices.Contains(got, "-fPIC") {
		t.Errorf("CompileFlags = %v", got)
	}
}

func TestManifestObject(t *testing.T) {
	data := manifestObject(manifestXML)
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		t.Errorf("machine = %#x", f.Machine)
	}
	if len(f.Sections) != 2 || f.Sections[0].Name != ".rsrc$01" || f.Sections[1].Name != ".rsrc$02" {
		t.Fatalf("sections = %v", f.Sections)
	}
	tree, _ := f.Sections[0].Data()
	body, _ := f.Sections[1].Data()
	// The directory tree: RT_MANIFEST (24), name 1, language 0, then the data entry with the size of the XML.
	le := func(b []byte, off int) uint32 {
		return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
	}
	if le(tree, 16) != 24 || le(tree, 16+4) != 0x80000000|24 || le(tree, 40) != 1 || le(tree, 44) != 0x80000000|48 || le(tree, 64) != 0 || le(tree, 68) != 72 {
		t.Errorf("resource tree wrong: % x", tree)
	}
	if size := le(tree, 76); int(size) != len(manifestXML) {
		t.Errorf("data entry size = %d, want %d", size, len(manifestXML))
	}
	// One relocation, IMAGE_REL_AMD64_ADDR32NB at the data entry, against the symbol of .rsrc$02.
	if len(f.Sections[0].Relocs) != 1 {
		t.Fatalf("relocations = %v", f.Sections[0].Relocs)
	}
	r := f.Sections[0].Relocs[0]
	if sym := f.COFFSymbols[r.SymbolTableIndex]; r.VirtualAddress != 72 || r.Type != 3 || strings.TrimRight(string(sym.Name[:]), "\x00") != ".rsrc$02" {
		t.Errorf("relocation = %+v", r)
	}
	if got := strings.TrimRight(string(body), "\x00"); got != manifestXML {
		t.Errorf("the data is not the manifest: %q", got)
	}
	// The XML is well formed and says what the ADR says it does.
	var doc struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal([]byte(manifestXML), &doc); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{">true</longPathAware>", ">UTF-8</activeCodePage>", `level="asInvoker"`, "{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"} {
		if !strings.Contains(manifestXML, want) {
			t.Errorf("the manifest lacks %s", want)
		}
	}
	// Deterministic: the same bytes every time.
	if !bytes.Equal(data, manifestObject(manifestXML)) {
		t.Error("the object is not deterministic")
	}
}

func TestManifestFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "manifest")
	p, err := manifestFile(dir, "m.o", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err == nil {
		t.Error("a dry run wrote the file")
	}
	p1, err := manifestFile(dir, "m.o", true)
	if err != nil || p1 != p {
		t.Fatalf("manifestFile = %q, %v; want %q", p1, err, p)
	}
	st1, _ := os.Stat(p1)
	p2, _ := manifestFile(dir, "m.o", true)
	st2, _ := os.Stat(p2)
	if p2 != p1 || !st2.ModTime().Equal(st1.ModTime()) {
		t.Error("the second call wrote the file again")
	}
	// A damaged file is replaced.
	os.WriteFile(p1, []byte("junk"), 0o644)
	if _, err := manifestFile(dir, "m.o", true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p1); !bytes.Equal(got, manifestObject(manifestXML)) {
		t.Error("a damaged manifest object was kept")
	}
}

func TestManifestLink(t *testing.T) {
	cache := t.TempDir()
	// clang: an object file on the link line.
	obj, flag, warn, err := manifestLink(Clang, cache, "", "", true)
	if err != nil || flag != "" || warn != "" || filepath.Base(obj) != "manifest.o" {
		t.Fatalf("clang: %q %q %q %v", obj, flag, warn, err)
	}
	if got, _ := os.ReadFile(obj); !bytes.Equal(got, manifestObject(manifestXML)) {
		t.Error("the object is not the manifest")
	}
	manifestIn := func(flag string) []byte {
		dir := filepath.FromSlash(strings.TrimSuffix(strings.TrimPrefix(flag, "-B"), "/"))
		got, _ := os.ReadFile(filepath.Join(dir, "default-manifest.o"))
		return got
	}
	// GCC: -B and a folder that holds default-manifest.o, which the driver links instead of its own. The folder is
	// absolute and has no space and no non-ASCII character (a -B prefix is expanded into spec text that is split at spaces and
	// read as ANSI): the cache when it is such a folder.
	obj, flag, warn, err = manifestLink(GCC, cache, "", "", true)
	if err != nil || obj != "" || warn != "" || !strings.HasPrefix(flag, "-B") || !strings.HasSuffix(flag, "/") || !plainFolder(filepath.FromSlash(strings.TrimPrefix(flag, "-B"))) {
		t.Fatalf("gcc: %q %q %q %v", obj, flag, warn, err)
	}
	if !bytes.Equal(manifestIn(flag), manifestObject(manifestXML)) {
		t.Errorf("default-manifest.o in %s is not the manifest", flag)
	}

	// A cache with a space and a Japanese name: the short name of the folder when there is one...
	spaced := filepath.Join(t.TempDir(), "John Smith \u65e5\u672c", "gocpp")
	short := t.TempDir()
	fakeWindowsPaths(t, nil)
	shortOf := map[string]string{}
	winpath.ShortFunc = func(p string) (string, bool) { s, ok := shortOf[p]; return s, ok }
	winpath.SafeFunc = func(s string) bool { return !strings.ContainsAny(s, " \u65e5") }
	sum := sha256.Sum256(manifestObject(manifestXML))
	manifestDir := filepath.Join(spaced, "manifest", hex.EncodeToString(sum[:6]))
	shortOf[manifestDir] = filepath.Join(short, "MANIFE~1")
	os.MkdirAll(shortOf[manifestDir], 0o755) // the short name names the same folder
	_, flag, warn, err = manifestLink(GCC, spaced, "", "", true)
	if err != nil || warn != "" || flag != "-B"+filepath.ToSlash(shortOf[manifestDir])+"/" {
		t.Fatalf("short name: %q %q %v", flag, warn, err)
	}
	if !bytes.Equal(manifestIn(flag), manifestObject(manifestXML)) {
		t.Error("the manifest is not in the short folder")
	}

	// ...else a private folder below ProgramData (made by winpath.PrivateFolder, replaced here by a recorder)...
	delete(shortOf, manifestDir)
	programData := t.TempDir()
	var asked [][]string
	oldPrivate := privateFolder
	privateFolder = func(base string, parts ...string) (string, error) {
		asked = append(asked, append([]string{base}, parts...))
		dir := filepath.Join(append([]string{base, "gocpp-paths-S-1-5-21-1"}, parts...)...)
		return dir, os.MkdirAll(dir, 0o755)
	}
	t.Cleanup(func() { privateFolder = oldPrivate })
	_, flag, warn, err = manifestLink(GCC, spaced, "", programData, true)
	wantDir := filepath.Join(programData, "gocpp-paths-S-1-5-21-1", "manifest-"+hex.EncodeToString(sum[:6]))
	if err != nil || warn != "" || flag != "-B"+filepath.ToSlash(wantDir)+"/" {
		t.Fatalf("ProgramData: %q (want %q) %q %v", flag, wantDir, warn, err)
	}
	if len(asked) != 1 || asked[0][0] != programData || asked[0][1] != "manifest-"+hex.EncodeToString(sum[:6]) {
		t.Errorf("the private folder was asked for as %v", asked)
	}
	if !bytes.Equal(manifestIn(flag), manifestObject(manifestXML)) {
		t.Error("the manifest is not below ProgramData")
	}
	// A folder that is refused (it is somebody else's, or open to others) is not used, and the warning says why.
	privateFolder = func(string, ...string) (string, error) { return "", errors.New("owned by S-1-5-21-9: refused") }
	_, flag, warn, err = manifestLink(GCC, spaced, "", programData, true)
	if err != nil || flag != "" || !strings.Contains(warn, "built without it") || !strings.Contains(warn, "owned by S-1-5-21-9: refused") {
		t.Fatalf("refused private folder: %q %q %v", flag, warn, err)
	}
	// A dry run does not make folders in a shared place.
	asked = nil
	privateFolder = func(string, ...string) (string, error) {
		asked = append(asked, nil)
		return "", errors.New("must not be called")
	}
	if _, _, _, err := manifestLink(GCC, spaced, "", programData, false); err != nil || len(asked) != 0 {
		t.Errorf("dry run asked for a private folder: %v %v", asked, err)
	}

	// The order of the plain candidates: the cache first, then the work folder.
	cachePlain := filepath.Join(t.TempDir(), "plaincache")
	workPlain := filepath.Join(t.TempDir(), "plainwork")
	_, flag, _, _ = manifestLink(GCC, cachePlain, workPlain, "", true)
	if !strings.Contains(flag, "plaincache") {
		t.Errorf("both plain: the flag is %q, want the cache", flag)
	}
	_, flag, _, _ = manifestLink(GCC, spaced, workPlain, "", true)
	if !strings.Contains(flag, "plainwork") {
		t.Errorf("cache with a space, work plain: the flag is %q, want the work folder", flag)
	}

	// ...else no manifest and a warning that says so, and no flag (not a flag that breaks the link).
	_, flag, warn, err = manifestLink(GCC, spaced, "", "", true)
	if err != nil || flag != "" || !strings.Contains(warn, "built without it") {
		t.Fatalf("nothing usable: %q %q %v", flag, warn, err)
	}
	// A dry run writes nothing and does not warn.
	dry := filepath.Join(t.TempDir(), "dry")
	if _, flag, warn, _ := manifestLink(GCC, dry, "", "", false); warn != "" || flag == "" {
		t.Errorf("dry run: %q %q", flag, warn)
	}
	if _, err := os.Stat(dry); err == nil {
		t.Error("a dry run wrote files")
	}
}

func TestPlainFolder(t *testing.T) {
	for p, want := range map[string]bool{
		"/tmp/gocpp/manifest/abc": true, `C:\Users\JRGEN~1\AppData\Local\gocpp`: true, "relative/dir": false, "/tmp/a b": false, "/tmp/\u65e5": false,
		`/tmp/it's`: false, `/tmp/100%`: false, "/tmp/\t": false,
	} {
		if p[0] == 'C' && filepath.Separator == '/' {
			continue // a drive path is absolute only on Windows
		}
		if got := plainFolder(p); got != want {
			t.Errorf("plainFolder(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestWindowsHostCompilerGetsTheTargetFlags(t *testing.T) {
	// On a Windows host the compiler is a target compiler like any other (FindCXXFor sets GOOS): its
	// flags are the static link, the API level and the stack.
	c := CXX{Family: GCC, Path: `C:\mingw\bin\g++.exe`, GOOS: "windows", GOARCH: "amd64"}
	if got := targetFlags(c); !slices.Contains(got, "-D_WIN32_WINNT=0x0A00") {
		t.Errorf("targetFlags = %v", got)
	}
	if got := LinkFlags(c); !slices.Contains(got, "-static") {
		t.Errorf("LinkFlags = %v", got)
	}
}

// fakeWindowsPaths makes this a Windows where "X" is outside the code page and the short names are a table (winpath's
// hooks), for the tests of the paths in compiler arguments.
func fakeWindowsPaths(t *testing.T, shorts map[string]string) {
	t.Helper()
	oldE, oldS, oldH := winpath.Enabled, winpath.SafeFunc, winpath.ShortFunc
	winpath.Enabled = true
	winpath.SafeFunc = func(s string) bool { return !strings.Contains(s, "X") }
	winpath.ShortFunc = func(p string) (string, bool) { s, ok := shorts[p]; return s, ok }
	t.Cleanup(func() { winpath.Enabled, winpath.SafeFunc, winpath.ShortFunc = oldE, oldS, oldH })
}

func TestCompilerArg(t *testing.T) {
	fakeWindowsPaths(t, map[string]string{
		`C:\Users\JXrgen\proj`:     `C:\Users\JRGEN~1\proj`,
		`C:\Users\JXrgen\proj\a.c`: `C:\Users\JRGEN~1\proj\A.C`,
	})
	const short = `C:\Users\JRGEN~1\proj`
	for _, c := range []struct{ in, want string }{
		{`-c`, `-c`},
		{`C:\Users\x\a.c`, `C:\Users\x\a.c`},
		{`C:\Users\JXrgen\proj\a.c`, `C:\Users\JRGEN~1\proj\A.C`},
		{`-IC:\Users\JXrgen\proj`, `-I` + short},
		{`-iquote`, `-iquote`},
		{`@C:\Users\JXrgen\proj\link.rsp`, `@` + short + `\link.rsp`},
		{`-ffile-prefix-map=C:\Users\JXrgen\proj=$WORK`, `-ffile-prefix-map=` + short + `=$WORK`},
		{`-o`, `-o`},
		{`-DXNAME=1`, `-DXNAME=1`}, // not a path: left alone
		{`-LC:\a;C:\Users\JXrgen\proj`, `-LC:\a;` + short},
	} {
		if got := CompilerArg(c.in); got != c.want {
			t.Errorf("CompilerArg(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if !CompilerSafe("prog.exe") || CompilerSafe("Xprog.exe") {
		t.Error("CompilerSafe")
	}
	// CXX.Command maps the arguments, and so do the response files.
	c := CXX{Path: "/b/g++", Args: []string{"-std=c++20"}, Family: GCC, Major: 13} // GCC before 14 reads the ANSI code page
	cmd := c.Command(context.Background(), "-c", `C:\Users\JXrgen\proj\a.c`)
	if got := cmd.Args[len(cmd.Args)-1]; got != `C:\Users\JRGEN~1\proj\A.C` {
		t.Errorf("the compiler got %q", got)
	}
	// GCC 14 and clang read their command line as Unicode: nothing to map.
	for _, u := range []CXX{{Path: "/b/g++", Family: GCC, Major: 14}, {Path: "/b/g++", Family: GCC, Major: 16}, {Path: "/b/clang++", Family: Clang, Major: 21}} {
		cmd := u.Command(context.Background(), "-c", `C:\Users\JXrgen\proj\a.c`)
		if got := cmd.Args[len(cmd.Args)-1]; got != `C:\Users\JXrgen\proj\a.c` {
			t.Errorf("%v family %v major %d got %q", u.Path, u.Family, u.Major, got)
		}
	}
	if (CXX{}).readsUTF8Arguments() {
		t.Error("a compiler of unknown version was trusted with UTF-8 arguments")
	}
	b := &Builder{GOOS: "windows", Work: t.TempDir()}
	old := compileLimit
	compileLimit = 10
	t.Cleanup(func() { compileLimit = old })
	args, done, err := b.fitCommand(c, []string{`-IC:\Users\JXrgen\proj`, "-c", `C:\Users\JXrgen\proj\a.c`})
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	data, _ := os.ReadFile(args[0][1:])
	if strings.Contains(string(data), "X") || !strings.Contains(string(data), "JRGEN~1") {
		t.Errorf("the response file has paths the compiler cannot read:\n%s", data)
	}
	// The object of a source whose name the compiler cannot read has a name that it can.
	if got := safeObjName("Xjp.cpp", 3); got != "gpsrc3.cpp.o" {
		t.Errorf("safeObjName = %q", got)
	}
	if got := safeObjName("a.cpp", 3); got != "a.cpp.o" {
		t.Errorf("safeObjName = %q", got)
	}
}
