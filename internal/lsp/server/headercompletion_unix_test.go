//go:build !windows

package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

func waitIndexMarker(t *testing.T, path string) []byte {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		if text, err := os.ReadFile(path); err == nil && len(text) > 0 {
			return text
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatal("fake compiler did not reach its blocking gate")
		}
	}
}

func TestHeaderIndexBlockedCompilerDoesNotBlockServer(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	gate := filepath.Join(dir, "gate")
	if err := syscall.Mkfifo(gate, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("INDEX_TEST_MARKER", marker)
	t.Setenv("INDEX_TEST_GATE", gate)
	compiler := filepath.Join(dir, "clang++")
	script := `#!/bin/sh
for arg do
  if [ "$arg" = --version ]; then
    printf 'clang version 18.0.0\n'
    exit 0
  fi
  if [ "$arg" = -MF ]; then
    printf '%s\n' "$$" > "$INDEX_TEST_MARKER"
    read answer < "$INDEX_TEST_GATE"
    exit 1
  fi
done
printf '#define __clang__ 1\n#define __clang_major__ 18\n#define __GNUC__ 4\n'
exit 0
`
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CLANG", compiler)
	foreign := "package foreign\nimport \"./api.hpp\"\nfunc f() { api.Unknown() }\n"
	plain := "package plain\nvar Value = 1\nfunc f() { _ = Value; Missing() }\n"
	root := writeTree(t, map[string]string{
		"gocpp.mod":       "module example.com/index\n",
		"foreign/main.gp": foreign,
		"foreign/api.hpp": "void Known();\n",
		"plain/main.gp":   plain,
	})
	s := newServer(nil, testConfig(), debounce.RealClock{})
	t.Cleanup(s.pipeline.Close)
	foreignURI := protocol.URIFromPath(filepath.Join(root, "foreign/main.gp"))
	if _, err := s.session.DidOpen(protocol.TextDocumentItem{
		URI: foreignURI, Version: 1, LanguageID: "gocpp", Text: foreign,
	}); err != nil {
		t.Fatal(err)
	}
	plainURI := protocol.URIFromPath(filepath.Join(root, "plain/main.gp"))
	if _, err := s.session.DidOpen(protocol.TextDocumentItem{
		URI: plainURI, Version: 1, LanguageID: "gocpp", Text: plain,
	}); err != nil {
		t.Fatal(err)
	}
	snap, release := s.session.Snapshot()
	_, err := snap.Package(context.Background(), filepath.Join(root, "foreign"))
	release()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(waitIndexMarker(t, marker))))
	if err != nil {
		t.Fatal(err)
	}
	blocked := func() {
		t.Helper()
		if err := syscall.Kill(pid, 0); err != nil {
			t.Fatalf("fake compiler was not still blocked: %v", err)
		}
	}
	// The index is already blocked when initialize runs, not merely started afterward.
	client := startServer(t, s, nil)
	c := &compClient{dclient: &dclient{client: client, root: root}, versions: make(map[string]int)}
	if resp := c.call("initialize", plainCompInit); resp.Error != nil {
		t.Fatal(resp.Error)
	}
	blocked()
	c.notify("initialized", "{}")
	s.diags.scheduleFile(plainURI, 0)
	position := protocol.NewMapper(plain).Position(strings.LastIndex(plain, "Value") + 2)
	paramsBytes, err := json.Marshal(protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: plainURI}, Position: position,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	params := string(paramsBytes)
	resp := c.call("textDocument/completion", params)
	var list protocol.CompletionList
	if err := json.Unmarshal(resp.Result, &list); err != nil || item(list.Items, "Value") == nil {
		t.Fatalf("ordinary completion = %s, %v", resp.Result, err)
	}
	blocked()
	if resp := c.call("textDocument/hover", params); resp.Error != nil || string(resp.Result) == "null" {
		t.Fatalf("ordinary hover = %+v", resp)
	}
	blocked()
	if got := c.next("plain/main.gp"); got.Version != 1 || len(got.Diagnostics) == 0 {
		t.Fatalf("ordinary diagnostics = %+v", got)
	}
	blocked()
	if resp := c.call("shutdown", "null"); resp.Error != nil {
		t.Fatal(resp.Error)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("shutdown did not join the compiler: %v", err)
	}
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}
