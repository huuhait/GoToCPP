package server

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/cache/filecache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
)

// TestDiskWarmRestart: server A opens a file and shuts down; server B,
// with the same cache folder, publishes the same diagnostics for it, and
// checks only the file's package: its imports (lib, fmt and the standard
// packages under fmt) come from the summaries on disk.
func TestDiskWarmRestart(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/m/lib\"\n)\n\nfunc main() {\n\tvar s string = lib.F()\n\tfmt.Println(s)\n}\n",
		"lib/lib.gp": "package lib\n\nimport \"strings\"\n\nfunc F() int { return len(strings.Repeat(\"a\", 2)) }\n",
	}
	root, dir := writeTree(t, files), t.TempDir()
	run := func() (diags string, checks, hits int64) {
		t.Helper()
		disk, err := filecache.Open(filecache.Options{Dir: dir, Version: "gocpp-test"})
		if err != nil {
			t.Fatal(err)
		}
		defer disk.Close()
		cfg := testConfig()
		cfg.Disk = disk
		c := &dclient{client: startServer(t, newServer(nil, cfg, debounce.RealClock{}), nil), root: root}
		c.call("initialize", diagInitParams)
		c.notify("initialized", "{}")
		c.open("main.gp", files["main.gp"])
		diags = format(c.next("main.gp"))
		_, _, checks = c.s.pipeline.Stats()
		hits, _, _ = c.s.pipeline.DiskStats()
		wantResult(t, c.call("shutdown", ""), "null")
		c.notify("exit", "")
		if code := c.wait(); code != 0 {
			t.Errorf("exit code %d", code)
		}
		return diags, checks, hits
	}
	const want = "9:16-9:23 error: cannot use lib.F() (value of type int) as string value in variable declaration\n"
	diagsA, checksA, hitsA := run()
	if diagsA != want || checksA < 4 || hitsA != 0 {
		t.Fatalf("server A: %d checks, %d from disk, diagnostics:\n%s", checksA, hitsA, diagsA)
	}
	diagsB, checksB, hitsB := run()
	if diagsB != diagsA {
		t.Errorf("server B:\n%s\nserver A:\n%s", diagsB, diagsA)
	}
	if checksB != 1 || hitsB < 3 {
		t.Errorf("server B: %d checks (want 1: main), %d summaries from disk (A made %d checks)", checksB, hitsB, checksA)
	}
}

// TestOpenDiskEnv: a relative GOCPP_LSP_CACHE is refused (it would be
// relative to the editor's working folder), and the server logs why and
// runs without the cache; "off" turns it off without a word.
func TestOpenDiskEnv(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	t.Setenv(filecache.EnvDir, "relative/cache")
	if c := openDisk(logger); c != nil {
		c.Close()
		t.Error("a relative folder was opened")
	}
	if !strings.Contains(buf.String(), "must be an absolute folder") {
		t.Errorf("the log: %q", buf.String())
	}
	buf.Reset()
	t.Setenv(filecache.EnvDir, "off")
	if c := openDisk(logger); c != nil || buf.Len() != 0 {
		t.Errorf("off: %v, log %q", c, buf.String())
	}
	dir := t.TempDir()
	t.Setenv(filecache.EnvDir, dir)
	c := openDisk(logger)
	if c == nil || c.Dir() != dir {
		t.Fatalf("an absolute folder: %v, log %q", c, buf.String())
	}
	c.Close()
}
