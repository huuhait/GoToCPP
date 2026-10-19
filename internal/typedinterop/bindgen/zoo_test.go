package bindgen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The header zoo (tests/zoo): the C and C++ standard headers and the
// libraries of the zoo. A library that pkg-config does not know is skipped.
var zooHeaders = []struct {
	header   string
	pkg      string // the pkg-config package, if the header comes from a library
	golden   string // a name: its golden file is compared when the headers are the ones it was made from
	want     []string
	dontWant []string
}{
	{header: "<stdio.h>", golden: "stdio_h", want: []string{"func fopen(filename *C.char, modes *C.char) *FILE", "func printf(format *C.char, args ...any) C.int", "var stdin *FILE", "BUFSIZ", "EOF", "= -1", "type FILE = _IO_FILE"}},
	{header: "<stdlib.h>", golden: "stdlib_h", want: []string{"func malloc(size C.size_t) unsafe.Pointer", "func qsort(", "EXIT_FAILURE", "RAND_MAX"}},
	{header: "<string.h>", golden: "string_h", want: []string{"func strlen(s *C.char) C.size_t", "func memcpy(", "func strerror(errnum C.int) *C.char"}, dontWant: []string{"cstring"}},
	{header: "<math.h>", want: []string{"func sin(x C.double) C.double", "M_PI", "func hypot("}},
	{header: "<time.h>", golden: "time_h", want: []string{"func time(", "type tm struct {", "CLOCKS_PER_SEC", "tm_year"}},
	{header: "<errno.h>", golden: "errno_h", want: []string{"ENOENT", "EINVAL"}},
	{header: "<cstdio>", want: []string{"func fopen(", "BUFSIZ"}},
	{header: "<vector>"},
	{header: "<string>"},
	{header: "<map>"},
	{header: "<unordered_map>"},
	{header: "<memory>"},
	{header: "<algorithm>"},
	{header: "<filesystem>"},
	{header: "<chrono>"},
	{header: "<thread>"},
	{header: "<optional>"},
	{header: "<variant>"},
	{header: "<span>"},
	{header: "<regex>"},
	{header: "<sqlite3.h>", pkg: "sqlite3", golden: "sqlite3", want: []string{
		"package sqlite3", "func sqlite3_open(filename *C.char, ppDb **sqlite3) C.int", "SQLITE_OK", "= 0\n",
		"type sqlite3 struct{}", "type sqlite3_stmt struct{}", "func sqlite3_exec(", "var sqlite3_version [0]C.char",
		"type sqlite3_module struct {", "SQLITE_ROW", "= 100",
		"a pointer, not a Go constant: // SQLITE_STATIC, SQLITE_TRANSIENT"}},
	{header: "<zlib.h>", pkg: "zlib", golden: "zlib", want: []string{"package zlib", "func deflate(", "Z_OK", "type z_stream_s struct {", "type z_stream = z_stream_s"}},
	{header: "<SDL2/SDL.h>", pkg: "sdl2", want: []string{"package SDL", "func SDL_Init(flags Uint32) C.int", "SDL_INIT_VIDEO", "type SDL_Event struct {", "type SDL_Rect struct {", "func SDL_PollEvent(event *SDL_Event) C.int", "SDL_QUIT"}},
	{header: "<raylib.h>", pkg: "raylib", golden: "raylib", want: []string{
		"package raylib", "func InitWindow(width C.int, height C.int, title *C.char)", "type Color struct {\n\tr, g, b, a C.uchar\n}",
		"RAYWHITE = Color{245, 245, 245, 255}", "FLAG_VSYNC_HINT               ConfigFlags = 64", "KEY_A ", "type Texture2D = Texture",
		"type TraceLogCallback = func(C.int, *C.char, C.va_list)", "RAYLIB_VERSION_MAJOR"}},
}

func TestZoo(t *testing.T) {
	cxx := tsupport.Clang(t)
	for _, z := range zooHeaders {
		t.Run(strings.Trim(z.header, "<>"), func(t *testing.T) {
			t.Parallel()
			name := goIdentRaw(strings.TrimSuffix(strings.Trim(z.header, "<>"), ".h"))
			set := func(c *Config) { c.Package = "p_" + name }
			if z.pkg != "" {
				cflags, _, err := build.PkgConfig(context.Background(), headersDir(t), []string{z.pkg})
				if err != nil {
					t.Skipf("pkg-config %s: %v", z.pkg, err)
				}
				set = func(c *Config) { c.Flags = cflags; c.Package = "" }
			}
			res := generate(t, z.header, set)
			checkGP(t, z.header, res)
			squeeze := func(s string) string { return strings.Join(strings.Fields(s), " ") }
			for _, w := range z.want {
				if !strings.Contains(squeeze(string(res.Source)), squeeze(w)) {
					t.Errorf("the view lacks %q", w)
				}
			}
			for _, w := range z.dontWant {
				if strings.Contains(squeeze(string(res.Source)), squeeze(w)) {
					t.Errorf("the view has %q", w)
				}
			}
			if res.Stats.Funcs+res.Stats.Types+res.Stats.Consts+res.Stats.Vars == 0 {
				t.Error("an empty view")
			}
			if z.golden != "" {
				zooGolden(t, cxx, z.golden, res)
			}
			if z.golden != "" || z.pkg != "" {
				again := generate(t, z.header, set)
				if string(again.Source) != string(res.Source) {
					t.Errorf("two runs differ: %s", firstDiff(string(res.Source), string(again.Source)))
				}
			}
		})
	}
}

// zooGolden compares a view of a header of the system with its golden file,
// when the files of the view and the compiler are the ones that the golden
// file was made from (the key file says); else the golden file belongs to
// another machine and is left alone.
func zooGolden(t *testing.T, cxx build.CXX, name string, res *Result) {
	t.Helper()
	path := filepath.Join("testdata", "golden", "zoo", name+".gp")
	keyPath := path + ".key"
	key := hashFiles(t, cxx, res)
	if *update {
		if err := os.WriteFile(path, res.Source, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath, []byte(key+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("no key file (run with -update): %v", err)
	}
	if strings.TrimSpace(string(want)) != key {
		// Inside the dev shell the compiler and the headers are pinned: a golden file that no
		// longer matches them (after a flake.lock bump) must be renewed, not forgotten.
		if os.Getenv("IN_NIX_SHELL") != "" {
			t.Fatalf("%s: the header files or the compiler are not the ones of the golden file (key %s): run the test with -update", name, key[:12])
		}
		t.Skipf("%s: the header files or the compiler are not the ones of the golden file: not compared (run with -update to renew it)", name)
	}
	golden(t, filepath.Join("zoo", name), res)
}
