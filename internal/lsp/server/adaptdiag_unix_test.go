//go:build !windows

package server

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// A compiler that does not answer holds the analysis, never the diagnostics: the package is
// published at once without the refusal, other requests are answered, and the refusal arrives when
// the compiler does.
func TestAdaptCompilerBlockedDoesNotBlockDiagnostics(t *testing.T) {
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
	t.Setenv("ADAPT_TEST_MARKER", marker)
	t.Setenv("ADAPT_TEST_GATE", gate)
	t.Setenv("ADAPT_TEST_PID", strconv.Itoa(os.Getpid())) // the gate's wait ends with the test
	t.Setenv("ADAPT_TEST_CLANG", clang.Path)
	compiler := filepath.Join(dir, "clang++")
	const script = `#!/bin/sh
last=""
for arg do last="$arg"; done
if [ -f "$last" ] && grep -q "gp_probe_" "$last" 2>/dev/null; then
  echo started > "$ADAPT_TEST_MARKER"
  while [ ! -f "$ADAPT_TEST_GATE" ]; do
    kill -0 "$ADAPT_TEST_PID" 2>/dev/null || exit 3
    sleep 0.02
  done
fi
exec "$ADAPT_TEST_CLANG" "$@"
`
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CLANG", compiler)
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/adapt\n",
		"p/main.gp": adaptProgram("200"),
		"p/api.hpp": "inline void take_char(char c) {}\n",
	})
	cfg := testConfig()
	cfg.CXX = func(ctx context.Context) (build.CXX, error) { return build.FindCXXFor(ctx, cfg.GOOS, cfg.GOARCH) }
	s := newServer(nil, cfg, debounce.RealClock{})
	t.Cleanup(s.pipeline.Close)
	c := &dclient{client: startServer(t, s, nil), root: root}
	c.call("initialize", diagInitParams)
	c.notify("initialized", "{}")
	start := time.Now()
	c.open("p/main.gp", adaptProgram("200"))
	first := c.next("p/main.gp")
	if hasMessage(first, adaptOverflowMsg) {
		t.Fatalf("the refusal came before the compiler answered: %s", first.raw)
	}
	t.Logf("first publish after %v", time.Since(start))
	for deadline := time.Now().Add(hangLimit); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(marker); err == nil {
			break
		}
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the analysis never reached the compiler; the log:\n%s", c.logs.String())
	}
	// The compiler is held: the server answers.
	if resp := c.call("textDocument/documentSymbol",
		`{"textDocument":{"uri":"`+string(c.uri("p/main.gp"))+`"}}`); resp.Error != nil {
		t.Fatalf("documentSymbol while the compiler is held: %+v", resp.Error)
	}
	if c.publishWith("p/main.gp", 300*time.Millisecond, func(p *publish) bool { return hasMessage(p, adaptOverflowMsg) }) != nil {
		t.Fatal("the refusal came while the compiler was held")
	}
	if err := os.WriteFile(gate, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if c.publishWith("p/main.gp", hangLimit, func(p *publish) bool { return hasMessage(p, adaptOverflowMsg) }) == nil {
		t.Fatalf("no refusal after the compiler answered; the log:\n%s", c.logs.String())
	}
}
