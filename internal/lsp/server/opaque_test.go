package server

import (
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/cache/filecache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
)

// The C/C++ types of an imported package (job 11.6, part A): hover and
// signature help show them as the source wrote them (*SDL.SDL_Window), not
// as "invalid type", from the side table of the summaries (step 11.3b),
// whether the package is checked from source (its own document), comes
// from its summary in memory, or from the disk cache.

// opaqueFiles is a module whose package lib has C types in its API, which
// main uses.
var opaqueFiles = map[string]string{
	"gocpp.mod": "module example.com/app\n",
	"main.gp": `package main

import "example.com/app/lib"

func main() {
	w := lib.Open(lib.Default)
	var h lib.Handle
	a := lib.App{N: 1}
	_, _, _ = w, h, a.Win
	lib.Use(lib.Default, 2)
	_ = lib.Pick[int](1, lib.Default)
	a.Run(nil)
	_ = lib.V2
	lib.Paren(nil)
}
`,
	"lib/lib.gp": `package lib

import "<SDL.h>"

type Handle SDL.Uint32

type App struct {
	Win *SDL.SDL_Window
	N   int
}

func (a *App) Run(r *SDL.SDL_Renderer) (*SDL.SDL_Window, error) { return nil, nil }

var Default *SDL.SDL_Window

func Open(w *SDL.SDL_Window) *SDL.SDL_Renderer { return nil }

func Pick[T any](x T, w *SDL.SDL_Window) *SDL.SDL_Renderer { return nil }

func Use(w *SDL.SDL_Window, n int) {}

func Make(w *SDL.SDL_Window, a, b int) (*App, *SDL.SDL_Window) { return nil, nil }

var V2 = struct{ W *SDL.SDL_Window }{}

func Paren(w (*SDL.SDL_Window)) {}
`,
}

// opaqueHovers are hovers in main.gp of opaqueFiles.
var opaqueHovers = []hoverCase{
	{"lib.Op|en", "func lib.Open(w *SDL.SDL_Window) *SDL.SDL_Renderer", "Open"},
	{"lib.Def|ault)", "var lib.Default *SDL.SDL_Window", "Default"},
	{"lib.Han|dle", "type Handle SDL.Uint32", "Handle"},
	{"a.W|in", "field Win *SDL.SDL_Window", "Win"},
	{"lib.U|se", "func lib.Use(w *SDL.SDL_Window, n int)", "Use"},
	{"lib.Pi|ck", "func lib.Pick[T any](x T, w *SDL.SDL_Window) *SDL.SDL_Renderer", "Pick"},
	{"a.R|un", "func (a *lib.App) Run(r *SDL.SDL_Renderer) (*SDL.SDL_Window, error)", "Run"},
	{"App{|N", "field N int", "N"},
	// No type expression of its own; parentheses around a type.
	{"lib.V|2", "var lib.V2 struct{W *SDL.SDL_Window}", "V2"},
	{"lib.Pa|ren", "func lib.Paren(w *SDL.SDL_Window)", "Paren"},
}

// opaqueSignatures are signature helps in main.gp of opaqueFiles.
var opaqueSignatures = map[string]string{
	"lib.Open(|lib": "Open(w *SDL.SDL_Window) *SDL.SDL_Renderer",
	"lib.Use(|lib":  "Use(w *SDL.SDL_Window, n int)",
	"a.Run(|nil":    "Run(r *SDL.SDL_Renderer) (*SDL.SDL_Window, error)",
	"Pick[int](|1":  "Pick(x int, w *SDL.SDL_Window) *SDL.SDL_Renderer",
}

// checkOpaque checks the hovers and signature helps of main.gp.
func checkOpaque(t *testing.T, c *dclient) {
	t.Helper()
	src := opaqueFiles["main.gp"]
	for _, tc := range opaqueHovers {
		if text, rng := c.hover("main.gp", src, tc.mark); text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q:\n%s\n(range %q), want\n%s", tc.mark, text, rng, tc.text)
		}
	}
	for mark, want := range opaqueSignatures {
		if label, _, _ := c.signature(src, mark); label != want {
			t.Errorf("signature help at %q: %q, want %q", mark, label, want)
		}
	}
}

// TestHoverOpaqueImport: in memory (lib's summary from its check), and in
// lib itself (from source), where an unnamed result is no "invalid type"
// either.
func TestHoverOpaqueImport(t *testing.T) {
	c := startNav(t, testConfig(), opaqueFiles, plainInit)
	c.open("main.gp", opaqueFiles["main.gp"])
	checkOpaque(t, c)
	lib := opaqueFiles["lib/lib.gp"]
	c.open("lib/lib.gp", lib)
	for _, tc := range []hoverCase{
		{"func Op|en", "func Open(w *SDL.SDL_Window) *SDL.SDL_Renderer", "Open"},
		{"func (a *App) R|un", "func (a *App) Run(r *SDL.SDL_Renderer) (*SDL.SDL_Window, error)", "Run"},
		{"func M|ake", "func Make(w *SDL.SDL_Window, a int, b int) (*App, *SDL.SDL_Window)", "Make"},
		{"var Def|ault", "var Default *SDL.SDL_Window", "Default"},
		{"var V|2", "var V2 struct{W *SDL.SDL_Window}", "V2"},
		{"func Pa|ren", "func Paren(w *SDL.SDL_Window)", "Paren"},
	} {
		if text, rng := c.hover("lib/lib.gp", lib, tc.mark); text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q in lib:\n%s\n(range %q), want\n%s", tc.mark, text, rng, tc.text)
		}
	}
}

// TestHoverOpaqueDisk: a second server, whose lib comes from the disk
// cache, shows the same.
//
// lib imports a header, so its first check may go on before the compiler
// lookup (in the background) has answered: that check is provisional, and
// its summary is not written. lib is written when it is checked again
// after the answer (the server rechecks main, which is stale then). So the
// first server waits for that write before it shuts down; a shutdown
// before it would leave the disk cache empty (under load, the recheck
// comes late).
func TestHoverOpaqueDisk(t *testing.T) {
	root, dir := writeTree(t, opaqueFiles), t.TempDir()
	run := func(waitWrite bool) (hits int64) {
		t.Helper()
		disk, err := filecache.Open(filecache.Options{Dir: dir, Version: "gocpp-test"})
		if err != nil {
			t.Fatal(err)
		}
		defer disk.Close()
		cfg := testConfig()
		cfg.Disk = disk
		c := &dclient{client: startServer(t, newServer(nil, cfg, debounce.RealClock{}), nil), root: root}
		c.call("initialize", plainInit)
		c.notify("initialized", "{}")
		c.open("main.gp", opaqueFiles["main.gp"])
		checkOpaque(t, c)
		hits, _, _ = c.s.pipeline.DiskStats()
		for deadline := time.Now().Add(hangLimit); waitWrite; time.Sleep(time.Millisecond) {
			if _, _, writes := c.s.pipeline.DiskStats(); writes > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the first server wrote no summary to disk")
			}
		}
		c.call("shutdown", "")
		c.notify("exit", "")
		c.wait()
		return hits
	}
	run(true)
	if hits := run(false); hits == 0 {
		t.Error("the second server read no summary from disk")
	}
}
