package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gocpp bindgen over the real headers of raylib, sqlite3 and SDL2 (the exit line of Phase 12:
// "bindgen output for raylib.h, sqlite3.h and SDL.h is reviewed"; the review is in
// docs/guides/bindgen-review.md). Each library has one test. It runs the command the way a
// user does (-pkg-config <library> <header>) and holds the view to properties that do not move
// when the library is updated:
//
//   - the view has at least as many functions, types and constants as the header had when it was
//     reviewed (a floor, never an exact count: the libraries add declarations);
//   - known declarations have their exact signature, written as a whole line;
//   - the view is what gocpp fmt keeps and gocpp check accepts, and two runs write the same text;
//   - typed interop agrees with the view: a program that uses the library passes gocpp check in
//     typed mode with each compiler, and a wrong argument is rejected with the very type that the
//     view gives the parameter. Nothing is built or run, so no window or display is needed.
//
// A library that pkg-config does not know skips its test, with the reason.

// A libraryCase describes one library of the review.
type libraryCase struct {
	pkgConfig string // the pkg-config package
	header    string // the header as the program imports it
	pkg       string // the package of the view
	alias     string // how the typed program names the package

	// floors for the counts of the view
	minFuncs, minTypes, minConstants int

	// lines that the view has, each as a whole line (gofmt aligns the columns of a block to its
	// longest name, so the white space inside a line is not compared)
	lines []string

	// text that the view has, with the white space of a line break and an alignment squeezed to
	// one space (a struct, a comment above a declaration)
	blocks []string

	// text that the view must not have
	absent []string

	// a program that uses the library and passes gocpp check -interop=typed
	program string

	// wrong calls, each one an argument of a function of lines
	mismatches []mismatch
}

// A mismatch is a call with one wrong argument. The typed checker must reject it, naming the type
// that the view's signature (a member of libraryCase.lines) gives to that parameter.
type mismatch struct {
	signature string // the line of the view
	param     int    // the index of the parameter
	args      string // the arguments, with the wrong one in place
}

// Signatures that a case uses twice, as lines of the view.
const (
	sqliteOpenV2 = "func sqlite3_open_v2(filename *C.char, ppDb **sqlite3, " +
		"flags C.int, zVfs *C.char) C.int"
	sdlCreateWindow = "func SDL_CreateWindow(title *C.char, x C.int, y C.int, " +
		"w C.int, h C.int, flags Uint32) *SDL_Window"
)

func TestBindgenRaylib(t *testing.T) {
	checkLibraryView(t, libraryCase{
		pkgConfig: "raylib",
		header:    "<raylib.h>",
		pkg:       "raylib",
		alias:     "rl",
		minFuncs:  590, minTypes: 60, minConstants: 300,
		lines: []string{
			"func InitWindow(width C.int, height C.int, title *C.char)",
			"func WindowShouldClose() C.bool",
			"func IsKeyPressed(key C.int) C.bool",
			"func GetMousePosition() Vector2",
			"func DrawCircleV(center Vector2, radius C.float, color Color)",
			"func DrawText(text *C.char, posX C.int, posY C.int, fontSize C.int, color Color)",
			"func LoadTexture(fileName *C.char) Texture2D",
			"func SetConfigFlags(flags C.uint)",
			// a C variadic function
			"func TextFormat(text *C.char, args ...any) *C.char",
			// a parameter named like a Go keyword gets a trailing underscore
			"func LoadFontData(fileData *C.uchar, dataSize C.int, fontSize C.int, " +
				"codepoints *C.int, codepointCount C.int, type_ C.int, " +
				"glyphCount *C.int) *GlyphInfo",
			// pointers to pointers
			"func LoadTextLines(text *C.char, count *C.int) **C.char",
			// typedefs are aliases; a function pointer is a func type, va_list is C.va_list
			"type Texture2D = Texture",
			"type Quaternion = Vector4",
			"type TraceLogCallback = func(C.int, *C.char, C.va_list)",
			"type ConfigFlags C.uint",
			// the color macros are struct-valued, so they are variables
			"\tRAYWHITE   = Color{245, 245, 245, 255}",
			"\tFLAG_VSYNC_HINT               ConfigFlags = 64",
			"\tKEY_A                         KeyboardKey = 65",
			// an incomplete type of the header
			"type rAudioBuffer struct{}",
		},
		blocks: []string{
			"type Color struct { r, g, b, a C.uchar }",
			"type Vector2 struct { x, y C.float }",
			"type Image struct { data unsafe.Pointer width, height, mipmaps, format C.int }",
			"type BoneInfo struct { name [32]C.char parent C.int }",
			// a macro that only renames a function is not a constant
			"not a constant: // GetMouseRay",
		},
		absent: []string{"/nix/store", "func CLITERAL"},
		program: `//gocpp:pkg-config raylib
package main

import rl "<raylib.h>"

func main() {
	rl.SetConfigFlags(rl.FLAG_WINDOW_HIDDEN)
	rl.InitWindow(320, 200, "view check")
	var ball rl.Vector2
	ball.x = 10
	var tint rl.Color = rl.MAROON
	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE)
		rl.DrawCircleV(ball, 12, tint)
		rl.DrawText(rl.TextFormat("frame %d", C.int(3)), 10, 10, 20, rl.DARKGRAY)
		rl.EndDrawing()
	}
	rl.CloseWindow()
}
`,
		mismatches: []mismatch{
			{"func InitWindow(width C.int, height C.int, title *C.char)", 0, `"wide", 450, "t"`},
			{"func InitWindow(width C.int, height C.int, title *C.char)", 2, `800, 450, 7`},
			{"func IsKeyPressed(key C.int) C.bool", 0, `"a"`},
			{"func LoadTexture(fileName *C.char) Texture2D", 0, `9`},
		},
	})
}

func TestBindgenSQLite3(t *testing.T) {
	checkLibraryView(t, libraryCase{
		pkgConfig: "sqlite3",
		header:    "<sqlite3.h>",
		pkg:       "sqlite3",
		alias:     "sqlite3",
		minFuncs:  250, minTypes: 35, minConstants: 450,
		lines: []string{
			"func sqlite3_open(filename *C.char, ppDb **sqlite3) C.int",
			sqliteOpenV2,
			"func sqlite3_close(*sqlite3) C.int",
			"func sqlite3_prepare_v2(db *sqlite3, zSql *C.char, nByte C.int, " +
				"ppStmt **sqlite3_stmt, pzTail **C.char) C.int",
			"func sqlite3_step(*sqlite3_stmt) C.int",
			// parameters that the header leaves unnamed are unnamed or "_"
			"func sqlite3_busy_timeout(_ *sqlite3, ms C.int) C.int",
			"func sqlite3_column_text(_ *sqlite3_stmt, iCol C.int) *C.uchar",
			"func sqlite3_last_insert_rowid(*sqlite3) sqlite3_int64",
			// the C variadic functions
			"func sqlite3_config(C.int, ...any) C.int",
			"func sqlite3_db_config(_ *sqlite3, op C.int, args ...any) C.int",
			"func sqlite3_mprintf(*C.char, ...any) *C.char",
			// the callback of sqlite3_exec is a func type, and a destructor too
			"func sqlite3_exec(_ *sqlite3, sql *C.char, " +
				"callback func(unsafe.Pointer, C.int, **C.char, **C.char) C.int, " +
				"_ unsafe.Pointer, errmsg **C.char) C.int",
			"func sqlite3_bind_blob(_ *sqlite3_stmt, _ C.int, _ unsafe.Pointer, n C.int, " +
				"_ func(unsafe.Pointer)) C.int",
			// a function that returns a function pointer is a func that returns a func
			"\txDlSym                         func(*sqlite3_vfs, unsafe.Pointer, *C.char) func()",
			// typedefs
			"type sqlite3_int64 = sqlite_int64",
			"type sqlite_int64 = C.longlong",
			"type sqlite3_destructor_type = func(unsafe.Pointer)",
			// the types that the header only declares are incomplete
			"type sqlite3 struct{}",
			"type sqlite3_stmt struct{}",
			// an unsized extern array
			"var sqlite3_version [0]C.char",
			"var sqlite3_temp_directory *C.char",
			// the result codes
			"\tSQLITE_OK                             = 0",
			"\tSQLITE_ROW                            = 100",
			"\tSQLITE_DONE                           = 101",
		},
		blocks: []string{
			// the types declared inside sqlite3_index_info (the struct is forward declared by
			// a typedef first) have their fields
			"aConstraint *sqlite3_index_info_sqlite3_index_constraint",
			"// struct sqlite3_index_info::sqlite3_index_constraint " +
				"type sqlite3_index_info_sqlite3_index_constraint struct { " +
				"iColumn C.int op, usable C.uchar iTermOffset C.int }",
			// a struct of function pointers
			"type sqlite3_mem_methods struct { xMalloc func(C.int) unsafe.Pointer",
			// a pointer macro cannot be a Go constant: it is listed
			"a pointer, not a Go constant: // SQLITE_STATIC, SQLITE_TRANSIENT",
		},
		absent: []string{
			"/nix/store",
			// the stand-ins that were written before the nested types were found
			"// C++: sqlite3_index_constraint",
			"type sqlite3_index_constraint struct{}",
			// the session and pre-update functions belong to builds with
			// SQLITE_ENABLE_SESSION and SQLITE_ENABLE_PREUPDATE_HOOK
			"func sqlite3session_create",
			"func sqlite3_preupdate_hook",
		},
		program: `//gocpp:pkg-config sqlite3
package main

import "<sqlite3.h>"

func main() {
	var db *sqlite3.sqlite3
	var stmt *sqlite3.sqlite3_stmt
	var errmsg *C.char
	if sqlite3.sqlite3_open(":memory:", &db) != sqlite3.SQLITE_OK {
		return
	}
	sqlite3.sqlite3_exec(db, "select 1", nil, nil, &errmsg)
	sqlite3.sqlite3_prepare_v2(db, "select 1", -1, &stmt, nil)
	if sqlite3.sqlite3_step(stmt) == sqlite3.SQLITE_ROW {
		_ = sqlite3.sqlite3_column_int(stmt, 0)
	}
	var info sqlite3.sqlite3_index_info
	_ = info.nConstraint
	sqlite3.sqlite3_finalize(stmt)
	sqlite3.sqlite3_close(db)
}
`,
		mismatches: []mismatch{
			{"func sqlite3_open(filename *C.char, ppDb **sqlite3) C.int", 0, `5, nil`},
			{"func sqlite3_busy_timeout(_ *sqlite3, ms C.int) C.int", 1, `nil, "slow"`},
			{sqliteOpenV2, 3, `nil, nil, 0, 1`},
		},
	})
}

func TestBindgenSDL2(t *testing.T) {
	checkLibraryView(t, libraryCase{
		pkgConfig: "sdl2",
		header:    "<SDL2/SDL.h>",
		pkg:       "SDL",
		alias:     "SDL",
		minFuncs:  800, minTypes: 150, minConstants: 1000,
		lines: []string{
			"func SDL_Init(flags Uint32) C.int",
			"func SDL_Quit()",
			"func SDL_GetError() *C.char",
			sdlCreateWindow,
			"func SDL_PollEvent(event *SDL_Event) C.int",
			"func SDL_Delay(ms Uint32)",
			"func SDL_SetRenderDrawColor(renderer *SDL_Renderer, " +
				"r Uint8, g Uint8, b Uint8, a Uint8) C.int",
			"func SDL_RenderFillRect(renderer *SDL_Renderer, rect *SDL_Rect) C.int",
			// C variadic functions, and their va_list twins
			"func SDL_snprintf(text *C.char, maxlen C.size_t, fmt *C.char, args ...any) C.int",
			"func SDL_vsnprintf(text *C.char, maxlen C.size_t, fmt *C.char, ap C.va_list) C.int",
			// an out parameter that is a pointer to a pointer
			"func SDL_CreateWindowAndRenderer(width C.int, height C.int, window_flags Uint32, " +
				"window **SDL_Window, renderer **SDL_Renderer) C.int",
			"func SDL_LockTexture(texture *SDL_Texture, rect *SDL_Rect, pixels *unsafe.Pointer, " +
				"pitch *C.int) C.int",
			// the fixed-width types are Go's, SDL's names for them are aliases
			"type Uint8 = uint8",
			"type Uint32 = uint32",
			"type SDL_AudioFormat = Uint16",
			// callbacks
			"type SDL_AudioCallback = func(unsafe.Pointer, *Uint8, C.int)",
			"type SDL_main_func = func(C.int, **C.char) C.int",
			// an enum is an integer type with typed constants, a negative one is signed
			"type SDL_bool C.uint",
			"type SDL_JoystickPowerLevel C.int",
			"\tSDL_JOYSTICK_POWER_UNKNOWN SDL_JoystickPowerLevel = -1",
			"\tSDL_QUIT                                       SDL_EventType = 256",
			// macros that are plain numbers and strings
			"\tSDL_MIX_MAXVOLUME                                 = 128",
			"\tSDL_HINT_RENDER_DRIVER                            = \"SDL_RENDER_DRIVER\"",
			// incomplete types
			"type SDL_Window struct{}",
			"type SDL_Renderer struct{}",
		},
		blocks: []string{
			"type SDL_Rect struct { x, y, w, h C.int }",
			// a union is a struct with a comment, and a field named like a Go keyword has a
			// trailing underscore
			"// union: the fields share their storage " +
				"type SDL_Event struct { type_ Uint32 common SDL_CommonEvent",
			"type SDL_CommonEvent struct { type_, timestamp Uint32 }",
			// anonymous unions and structs inside a struct keep their shape
			"hidden struct { stdio struct { autoclose SDL_bool fp *FILE } // unnamed struct type",
			// a packed struct is flagged
			"// layout: packed type SDL_AudioCVT struct",
			// a declaration that the header marks deprecated
			"// Deprecated: the header marks it deprecated. func SDL_GetRevisionNumber() C.int",
			// a macro that renames a function is not a constant
			"a pointer, not a Go constant: // SDL_BlitScaled, SDL_BlitSurface",
		},
		absent: []string{"/nix/store"},
		program: `//gocpp:pkg-config sdl2
package main

import "<SDL2/SDL.h>"

func main() {
	if SDL.SDL_Init(SDL.SDL_INIT_VIDEO) != 0 {
		return
	}
	win := SDL.SDL_CreateWindow("view check", SDL.SDL_WINDOWPOS_CENTERED,
		SDL.SDL_WINDOWPOS_CENTERED, 320, 200, SDL.SDL_WINDOW_HIDDEN)
	var ev SDL.SDL_Event
	for SDL.SDL_PollEvent(&ev) != 0 {
		if ev.type == SDL.SDL_QUIT {
			break
		}
	}
	box := SDL.SDL_Rect{10, 20, 100, 50}
	box.x += 5
	SDL.SDL_DestroyWindow(win)
	SDL.SDL_Quit()
}
`,
		mismatches: []mismatch{
			{sdlCreateWindow, 0, `7, 0, 0, 0, 0, 0`},
			{sdlCreateWindow, 3, `nil, 0, 0, "wide", 0, 0`},
		},
	})
}

// checkLibraryView runs the checks of a libraryCase.
func checkLibraryView(t *testing.T, lib libraryCase) {
	typedEnv(t)
	if why := missingLibs([]string{lib.pkgConfig}); why != "" {
		t.Skip(why)
	}
	dir := t.TempDir()
	t.Chdir(dir)

	view := generateLibraryView(t, lib)
	t.Run("view", func(t *testing.T) {
		checkViewContent(t, lib, view)
	})
	t.Run("repeatable", func(t *testing.T) {
		if again := generateLibraryView(t, lib); again != view {
			t.Errorf("two runs write different views")
		}
	})
	t.Run("gocpp fmt and gocpp check", func(t *testing.T) {
		checkViewFile(t, lib, view)
	})
	t.Run("typed interop agrees", func(t *testing.T) {
		checkTypedAgreement(t, lib, view)
	})
}

// generateLibraryView runs gocpp bindgen on the header and returns the view. The run has no
// warnings on standard error.
func generateLibraryView(t *testing.T, lib libraryCase) string {
	t.Helper()
	r := gocppRun("", false, "bindgen", "-pkg-config", lib.pkgConfig, lib.header)
	if r.code == 3 {
		t.Skip("no clang++: bindgen needs it")
	}
	if r.code != 0 {
		t.Fatalf("gocpp bindgen %s: exit %d\n%s", lib.header, r.code, r.stderr)
	}
	if r.stderr != "" {
		t.Errorf("gocpp bindgen %s wrote to standard error:\n%s", lib.header, r.stderr)
	}
	return r.stdout
}

var (
	funcLine  = regexp.MustCompile(`(?m)^func `)
	typeLine  = regexp.MustCompile(`(?m)^type `)
	constLine = regexp.MustCompile(`^\t\w+( +\w+)? += `)
)

// countConstants counts the entries of the const blocks of the view and of its var block of
// struct values (the color macros).
func countConstants(view string) int {
	count := 0
	inBlock := false
	for _, line := range strings.Split(view, "\n") {
		switch {
		case line == "const (":
			inBlock = true
		case line == ")":
			inBlock = false
		case inBlock && constLine.MatchString(line):
			count++
		}
	}
	return count
}

func checkViewContent(t *testing.T, lib libraryCase, view string) {
	t.Helper()
	prefix := "// Code generated by gocpp bindgen from " + lib.header +
		". DO NOT EDIT.\n\npackage " + lib.pkg + "\n"
	if !strings.HasPrefix(view, prefix) {
		t.Errorf("the view starts with\n%s\nwant\n%s", firstLines(view, 4), prefix)
	}

	// floors
	funcs := len(funcLine.FindAllString(view, -1))
	types := len(typeLine.FindAllString(view, -1))
	constants := countConstants(view)
	t.Logf("%d functions, %d types, %d constants", funcs, types, constants)
	if funcs < lib.minFuncs {
		t.Errorf("%d functions, want at least %d", funcs, lib.minFuncs)
	}
	if types < lib.minTypes {
		t.Errorf("%d types, want at least %d", types, lib.minTypes)
	}
	if constants < lib.minConstants {
		t.Errorf("%d constants, want at least %d", constants, lib.minConstants)
	}

	// exact declarations, as whole lines
	whole := map[string]bool{}
	for _, line := range strings.Split(view, "\n") {
		whole[strings.Join(strings.Fields(line), " ")] = true
	}
	for _, want := range lib.lines {
		if !whole[strings.Join(strings.Fields(want), " ")] {
			t.Errorf("the view has no line %q", want)
		}
	}

	squeezed := strings.Join(strings.Fields(view), " ")
	for _, want := range lib.blocks {
		if !strings.Contains(squeezed, strings.Join(strings.Fields(want), " ")) {
			t.Errorf("the view lacks %q", want)
		}
	}
	for _, text := range lib.absent {
		if strings.Contains(view, text) {
			t.Errorf("the view has %q", text)
		}
	}
}

func firstLines(text string, n int) string {
	lines := strings.SplitN(text, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// checkViewFile writes the view and asks gocpp fmt and gocpp check about it.
func checkViewFile(t *testing.T, lib libraryCase, view string) {
	t.Helper()
	folder := filepath.Join(t.TempDir(), "view")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "view.gp")
	if err := os.WriteFile(file, []byte(view), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := gocppRun("", false, "fmt", "-l", file); r.code != 0 || r.stdout != "" {
		t.Errorf("gocpp fmt -l: exit %d, files %q, %s", r.code, r.stdout, r.stderr)
	}
	if r := gocppRun("", false, "check", folder); r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("gocpp check: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
}

// checkTypedAgreement checks a program that uses the library in typed mode with each compiler
// (it must pass), then the calls with a wrong argument with clang++ (it must fail, and the
// message names the type of the view).
func checkTypedAgreement(t *testing.T, lib libraryCase, view string) {
	t.Helper()
	inDir(t, map[string]string{"use.gp": lib.program})
	t.Setenv("GOCPP_INTEROP", "typed")
	needClang(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		r := gocppRun("", true, "check", "-interop=typed", "use.gp")
		if r.code != 0 || r.stdout != "" {
			t.Fatalf("typed check with %s: exit %d\n%s", c.name, r.code, r.stdout)
		}
	})

	var calls strings.Builder
	var want []string
	for _, m := range lib.mismatches {
		name := signatureName(t, m.signature)
		kind := parameterType(t, m.signature, m.param)
		fmt.Fprintf(&calls, "\t%s.%s(%s)\n", lib.alias, name, m.args)
		want = append(want, "as "+kind+" value in argument to "+lib.alias+"."+name)
	}
	source := strings.Replace(lib.program, "func main() {\n", "func main() {\n"+calls.String(), 1)
	if err := os.WriteFile("wrong.gp", []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CXX", "clang++")
	r := gocppRun("", true, "check", "-interop=typed", "wrong.gp")
	if r.code == 0 {
		t.Fatalf("typed check accepted the wrong calls:\n%s", calls.String())
	}
	for _, text := range want {
		if !strings.Contains(r.stdout, text) {
			t.Errorf("typed check does not say %q:\n%s", text, r.stdout)
		}
	}
}

// signatureName returns the name of the function of a "func name(...)" line.
func signatureName(t *testing.T, signature string) string {
	t.Helper()
	rest, ok := strings.CutPrefix(signature, "func ")
	if !ok {
		t.Fatalf("not a function line: %q", signature)
	}
	name, _, ok := strings.Cut(rest, "(")
	if !ok {
		t.Fatalf("not a function line: %q", signature)
	}
	return name
}

// parameterType returns the type of parameter index of a "func name(params) results" line: the
// part after the name, or the whole parameter when it has no name.
func parameterType(t *testing.T, signature string, index int) string {
	t.Helper()
	open := strings.Index(signature, "(")
	depth := 0
	start := open + 1
	var params []string
	for at := open; at < len(signature); at++ {
		switch signature[at] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
			if depth == 0 {
				params = append(params, signature[start:at])
				at = len(signature)
			}
		case ',':
			if depth == 1 {
				params = append(params, signature[start:at])
				start = at + 1
			}
		}
	}
	if index >= len(params) {
		t.Fatalf("%q has no parameter %d", signature, index)
	}
	param := strings.TrimSpace(params[index])
	if name, kind, ok := strings.Cut(param, " "); ok && !strings.ContainsAny(name, "*[(.") {
		return strings.TrimSpace(kind)
	}
	return param
}
