package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The tests of the trusted flags of the user (userflags.go, Phase 06 step 6.12): the layers, the
// modes, and the compiler plugins that the flags load.

func TestLayeredOrder(t *testing.T) {
	env := EnvFlags{CFlags: []string{"-DENV_C"}, CXXFlags: []string{"-DENV_CXX"}, LDFlags: []string{"-lenv"}}
	got := env.Layered([]string{"-DMODE"}, []string{"-lmode"}, []string{"-DFLAG_C"}, []string{"-DFLAG_CXX"}, []string{"-lflag"})
	// The mode's come first, then the environment's, then the build flags': a later flag wins.
	if want := []string{"-DMODE", "-DENV_C", "-DFLAG_C"}; !slices.Equal(got.CFlags, want) {
		t.Errorf("CFlags = %q, want %q", got.CFlags, want)
	}
	if want := []string{"-DENV_CXX", "-DFLAG_CXX"}; !slices.Equal(got.CXXFlags, want) {
		t.Errorf("CXXFlags = %q, want %q", got.CXXFlags, want)
	}
	if want := []string{"-lmode", "-lenv", "-lflag"}; !slices.Equal(got.LDFlags, want) {
		t.Errorf("LDFlags = %q, want %q", got.LDFlags, want)
	}
	// C flags come before C++ flags in every C++ compile.
	if want := []string{"-DMODE", "-DENV_C", "-DFLAG_C", "-DENV_CXX", "-DFLAG_CXX"}; !slices.Equal(got.CXXCompile(), want) {
		t.Errorf("CXXCompile = %q, want %q", got.CXXCompile(), want)
	}
}

func TestLayeredDoesNotShare(t *testing.T) {
	env := EnvFlags{CXXFlags: make([]string, 1, 10)}
	a := env.Layered(nil, nil, nil, []string{"-Da"}, nil)
	b := env.Layered(nil, nil, nil, []string{"-Db"}, nil)
	if a.CXXFlags[1] != "-Da" || b.CXXFlags[1] != "-Db" {
		t.Errorf("two layers share an array: %q %q", a.CXXFlags, b.CXXFlags)
	}
}

func TestModeFlags(t *testing.T) {
	for _, c := range []struct {
		name    string
		mode    Profile
		asan    bool
		goos    string
		compile []string
		link    []string
		err     string
	}{
		{name: "none"},
		{name: "debug", mode: Profile{Debug: true}},
		{name: "release", mode: Profile{Release: true}, compile: []string{"-O2", "-flto"}, link: []string{"-O2", "-flto"}},
		{name: "race", mode: Profile{Race: true}, goos: "linux", compile: []string{"-fsanitize=thread", "-fno-omit-frame-pointer"}, link: []string{"-fsanitize=thread"}},
		{name: "race on darwin", mode: Profile{Race: true}, goos: "darwin", compile: []string{"-fsanitize=thread", "-fno-omit-frame-pointer"}, link: []string{"-fsanitize=thread"}},
		{name: "debug and release", mode: Profile{Debug: true, Release: true}, err: "-debug and -release"},
		{name: "race and asan", mode: Profile{Race: true}, asan: true, err: "-race and -asan"},
		{name: "race and release", mode: Profile{Race: true, Release: true}, err: "-race and -release"},
		{name: "race on windows", mode: Profile{Race: true}, goos: "windows", err: "-race is not supported for windows"},
		{name: "release with asan", mode: Profile{Release: true}, asan: true, compile: []string{"-O2", "-flto"}, link: []string{"-O2", "-flto"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			compile, link, err := ProfileFlags(c.mode, c.asan, c.goos)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("ProfileFlags = %v, want an error with %q", err, c.err)
				}
				return
			}
			if err != nil || !slices.Equal(compile, c.compile) || !slices.Equal(link, c.link) {
				t.Errorf("ProfileFlags = %q %q %v, want %q %q", compile, link, err, c.compile, c.link)
			}
		})
	}
}

func TestIsPluginFlag(t *testing.T) {
	for f, want := range map[string]bool{
		"-fplugin=/x/p.so": true, "-fplugin-arg-p-key=v": true, "-fpass-plugin=/x/p.so": true,
		"-fplugin": false, "-O2": false, "-fpass-plugin": false, "-Xclang": false, "-DX=-fplugin=a": false,
		"-fno-plugin=x": false, "-fpass-plugins=x": false,
	} {
		if got := IsPluginFlag(f); got != want {
			t.Errorf("IsPluginFlag(%q) = %v, want %v", f, got, want)
		}
	}
}

func TestDropPluginFlags(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"none", "-O2 -DX", "-O2 -DX"},
		{"gcc plugin", "-O2 -fplugin=/p.so -fplugin-arg-p-a=1 -DX", "-O2 -DX"},
		{"clang pass plugin", "-fpass-plugin=/p.so -O2", "-O2"},
		{"xclang load", "-Xclang -load -Xclang /p.so -Xclang -add-plugin -Xclang name -O1", "-O1"},
		{"xclang plugin arg", "-Xclang -plugin -Xclang name -Xclang -plugin-arg-name -Xclang v -DX", "-DX"},
		{"other xclang stays", "-Xclang -fno-foo -DX", "-Xclang -fno-foo -DX"},
		{"xclang at the end", "-DX -Xclang", "-DX -Xclang"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(DropPluginFlags(strings.Fields(c.in)), " ")
			if got != c.want {
				t.Errorf("DropPluginFlags(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDropPluginFlagsKeepsInput(t *testing.T) {
	in := []string{"-fplugin=/p.so", "-O2"}
	DropPluginFlags(in)
	if in[0] != "-fplugin=/p.so" || in[1] != "-O2" {
		t.Errorf("DropPluginFlags changed its argument: %q", in)
	}
}

func TestPluginFiles(t *testing.T) {
	flags := strings.Fields("-O2 -fplugin=/a.so -fpass-plugin=/b.so -Xclang -load -Xclang /c.so -fplugin-arg-a-x=1 -fplugin=name")
	if got, want := PluginFiles(flags), []string{"/a.so", "/b.so", "/c.so", "name"}; !slices.Equal(got, want) {
		t.Errorf("PluginFiles = %q, want %q", got, want)
	}
	if got := PluginFiles(strings.Fields("-O2 -DX")); len(got) != 0 {
		t.Errorf("PluginFiles(no plugins) = %q", got)
	}
}

func TestAbsPluginPaths(t *testing.T) {
	dir := filepath.FromSlash("/proj")
	in := []string{"-fplugin=lib/p.so", "-fpass-plugin=q.so", "-fpass-plugin=" + filepath.FromSlash("/abs/r.so"), "-fplugin=gccname", "-O2"}
	got := AbsPluginPaths(in, dir)
	want := []string{"-fplugin=" + filepath.Join(dir, "lib", "p.so"), "-fpass-plugin=q.so", "-fpass-plugin=" + filepath.FromSlash("/abs/r.so"), "-fplugin=gccname", "-O2"}
	// A relative path with a folder is made absolute; a bare name is the compiler's to look up.
	if !slices.Equal(got, want) {
		t.Errorf("AbsPluginPaths = %q, want %q", got, want)
	}
	if in[0] != "-fplugin=lib/p.so" {
		t.Errorf("AbsPluginPaths changed its argument")
	}
	same := []string{"-O2"}
	if got := AbsPluginPaths(same, dir); &got[0] != &same[0] {
		t.Errorf("AbsPluginPaths copies a list that has nothing to change")
	}
}

func TestPluginHashAndSalt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "p.so")
	if got := PluginHash(p); got != "missing" {
		t.Errorf("PluginHash(missing file) = %q", got)
	}
	if got := PluginHash(dir); got != "missing" {
		t.Errorf("PluginHash(folder) = %q", got)
	}
	if err := os.WriteFile(p, []byte("one"), 0o755); err != nil {
		t.Fatal(err)
	}
	h1 := PluginHash(p)
	if len(h1) != 64 || h1 == "missing" {
		t.Fatalf("PluginHash = %q", h1)
	}
	// A new content, same name: the hash changes (the size differs, so the memo is not hit).
	if err := os.WriteFile(p, []byte("three"), 0o755); err != nil {
		t.Fatal(err)
	}
	if h2 := PluginHash(p); h2 == h1 {
		t.Errorf("PluginHash did not change with the content")
	}
	salt := PluginSalt([]string{"-fpass-plugin=" + p, "-O2"})
	if len(salt) != 3 || salt[0] != "plugin" || salt[1] != p || salt[2] != PluginHash(p) {
		t.Errorf("PluginSalt = %q", salt)
	}
	if PluginSalt([]string{"-O2"}) != nil {
		t.Errorf("PluginSalt without a plugin is not nil")
	}
}

func TestPluginStamp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "p.so")
	flags := []string{"-fplugin=" + p}
	if got := PluginStamp(flags); len(got) != 1 || !strings.HasSuffix(got[0], " missing") {
		t.Errorf("PluginStamp(missing) = %q", got)
	}
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := PluginStamp(flags)
	if err := os.WriteFile(p, []byte("xy"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := PluginStamp(flags)
	if len(a) != 1 || len(b) != 1 || a[0] == b[0] || strings.HasSuffix(a[0], "missing") {
		t.Errorf("PluginStamp does not follow the file: %q %q", a, b)
	}
}

func TestProbeFlagsDropPlugins(t *testing.T) {
	env := EnvFlags{CXXFlags: []string{"-fpass-plugin=/p.so", "-DX=1"}}
	got := ProbeFlags(env, PackageFlags{})
	for _, f := range got {
		if IsPluginFlag(f) {
			t.Errorf("ProbeFlags has the plugin flag %q: %q", f, got)
		}
	}
	if !slices.Contains(got, "-DX=1") {
		t.Errorf("ProbeFlags lost -DX=1: %q", got)
	}
	if got := ProbeFlags(EnvFlags{CXXFlags: []string{"-fplugin=/p.so"}}, PackageFlags{}); got != nil {
		t.Errorf("ProbeFlags with only a plugin = %q, want nil (the probe's own flags)", got)
	}
}

func TestLinkFlagProblemModeFlags(t *testing.T) {
	// The flags of -race and -release read no file that the up-to-date check should follow.
	for _, f := range [][]string{{"-fsanitize=thread"}, {"-fsanitize=address"}, {"-O2", "-flto"}, {"-flto=thin"}} {
		if why := linkFlagProblem(f); why != "" {
			t.Errorf("linkFlagProblem(%q) = %q", f, why)
		}
	}
	if why := linkFlagProblem([]string{"-fuse-ld=gold"}); why == "" {
		t.Errorf("linkFlagProblem(-fuse-ld=gold) = none: another linker reads other files")
	}
}

func TestFlagFiles(t *testing.T) {
	got := FlagFiles([]string{"-DX", "@r.txt", "-include", "h.h", "-imacros", "m.h", "-include", "@", "-include"})
	if want := []string{"r.txt", "h.h", "m.h", "@"}; !slices.Equal(got, want) {
		t.Errorf("FlagFiles = %q, want %q", got, want)
	}
	if got := FlagFiles([]string{"-fplugin=x.so", "-O2"}); got != nil {
		t.Errorf("FlagFiles(plugin, -O2) = %q, want none", got)
	}
}

// The salt of a cache key has the content of the plugins and of the files the flags name, not
// their time.
func TestPluginSaltFollowsContent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.dat")
	salt := func(content string) []string {
		t.Helper()
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return PluginSalt([]string{"-fpass-plugin=" + file, "@" + file, "-include", file})
	}
	a, b, a2 := salt("AAAA"), salt("BBBB"), salt("AAAA")
	if len(a) != 9 || slices.Equal(a, b) || !slices.Equal(a, a2) {
		t.Errorf("salts: %q, %q, %q: want 9 entries that follow the content", a, b, a2)
	}
	if got := PluginSalt([]string{"-O2", "-DX"}); got != nil {
		t.Errorf("PluginSalt without files = %q", got)
	}
}

func TestAbsPluginPathsFiles(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"r.txt", "h.h"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in := []string{"@r.txt", "@nothere.txt", "-include", "h.h", "-include", "stdio.h", "-imacros", "/abs.h", "-fplugin=sub/x.so", "-fplugin=name"}
	want := []string{"@" + filepath.Join(dir, "r.txt"), "@nothere.txt", "-include", filepath.Join(dir, "h.h"), "-include", "stdio.h", "-imacros", "/abs.h", "-fplugin=" + filepath.Join(dir, "sub/x.so"), "-fplugin=name"}
	if got := AbsPluginPaths(in, dir); !slices.Equal(got, want) {
		t.Errorf("AbsPluginPaths:\n got %q\nwant %q", got, want)
	}
	if in[0] != "@r.txt" {
		t.Errorf("AbsPluginPaths changed its argument: %q", in)
	}
}

func TestAbsUserPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "libzz.a"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	in := []string{"-Iinc", "-I", "inc2", "-I/abs", "-I=sysroot", "-isystem", "sys", "-iquote", "q", "-idirafter", "after",
		"-L.", "-L", "lib", "-lzz", "--sysroot=root", "-Wl,-Map,x.map", "-Wl,-Map=y.map", "-Wl,-rpath,rel", "-Wl,--gc-sections",
		"libzz.a", "nothere.a", "-DX=1", "-O2"}
	j := func(n string) string { return filepath.Join(dir, n) }
	want := []string{"-I" + j("inc"), "-I", j("inc2"), "-I/abs", "-I=sysroot", "-isystem", j("sys"), "-iquote", j("q"), "-idirafter", j("after"),
		"-L" + j("."), "-L", j("lib"), "-lzz", "--sysroot=" + j("root"), "-Wl,-Map," + j("x.map"), "-Wl,-Map=" + j("y.map"), "-Wl,-rpath,rel", "-Wl,--gc-sections",
		j("libzz.a"), "nothere.a", "-DX=1", "-O2"}
	orig := slices.Clone(in)
	if got := AbsUserPaths(in, dir); !slices.Equal(got, want) {
		t.Errorf("AbsUserPaths:\n got %q\nwant %q", got, want)
	}
	if !slices.Equal(in, orig) {
		t.Errorf("AbsUserPaths changed its argument: %q", in)
	}
	if got := AbsUserPaths([]string{"-O2", "-DX"}, dir); !slices.Equal(got, []string{"-O2", "-DX"}) {
		t.Errorf("flags with no path = %q", got)
	}
}
