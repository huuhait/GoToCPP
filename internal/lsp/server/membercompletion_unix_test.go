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

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func TestMemberCompilerBlockedDoesNotBlockServer(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	clang, err := probe.FindClang(context.Background())
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	gate := filepath.Join(dir, "gate")
	if err := syscall.Mkfifo(gate, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEMBER_TEST_MARKER", marker)
	t.Setenv("MEMBER_TEST_GATE", gate)
	t.Setenv("MEMBER_TEST_CLANG", clang.Path)
	compiler := filepath.Join(dir, "clang++")
	const script = `#!/bin/sh
last=""
for arg do last="$arg"; done
if [ -f "$last" ]; then
  while IFS= read -r line; do
    case "$line" in
      *"gocpp_member_query()"*)
        printf '%s\n' "$$" > "$MEMBER_TEST_MARKER"
        read answer < "$MEMBER_TEST_GATE"
        exit 1
        ;;
    esac
  done < "$last"
fi
exec "$MEMBER_TEST_CLANG" "$@"
`
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CLANG", compiler)
	foreign := "package foreign\nimport \"./api.hpp\"\nfunc f() { var x api.Widget; _ = x.field }\n"
	plain := "package plain\nvar Value = 1\nfunc f() { _ = Value; Missing() }\n"
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/members\n", "foreign/main.gp": foreign,
		"foreign/api.hpp": "struct Widget { int field; };\n", "plain/main.gp": plain,
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
	_, err = snap.Package(context.Background(), filepath.Join(root, "foreign"))
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
			t.Fatalf("member compiler is not blocked: %v", err)
		}
	}
	client := startServer(t, s, nil)
	c := &compClient{dclient: &dclient{client: client, root: root}, versions: make(map[string]int)}
	if response := c.call("initialize", plainCompInit); response.Error != nil {
		t.Fatal(response.Error)
	}
	blocked()
	c.notify("initialized", "{}")
	s.diags.scheduleFile(plainURI, 0)
	result := memberCompletions(t, c, "plain/main.gp", plain, "_ = Va")
	if item(result.Items, "Value") == nil {
		t.Fatal("ordinary completion lost its result")
	}
	blocked()
	params, _ := json.Marshal(protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: plainURI},
		Position: protocol.NewMapper(plain).Position(strings.LastIndex(plain, "Value") + 2)}})
	if response := c.call("textDocument/hover", string(params)); response.Error != nil || string(response.Result) == "null" {
		t.Fatalf("ordinary hover = %+v", response)
	}
	blocked()
	if diagnostics := c.next("plain/main.gp"); diagnostics.Version != 1 || len(diagnostics.Diagnostics) == 0 {
		t.Fatalf("ordinary diagnostics = %+v", diagnostics)
	}
	blocked()
	if response := c.call("shutdown", "null"); response.Error != nil {
		t.Fatal(response.Error)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("shutdown did not join member compiler: %v", err)
	}
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}
