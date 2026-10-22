//go:build unix

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// TestLSPUnreadableHeaderOneDiagnostic: the language server says once that a local header without
// permission can't be read (the compiler's message), not also that the parse-as-C check failed.
func TestLSPUnreadableHeaderOneDiagnostic(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"gocpp.mod": "module example.com/noperm\n",
		"main.gp":   "package main\n\nimport \"./noperm.h\"\n\nfunc main() {}\n",
		"noperm.h":  "int noperm(void);\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	header := filepath.Join(dir, "noperm.h")
	if err := os.Chmod(header, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(header, 0o644) })
	if f, err := os.Open(header); err == nil {
		f.Close()
		t.Skip("the file can be read without permission (root?)")
	}
	uri := string(protocol.URIFromPath(filepath.Join(dir, "main.gp")))
	// g++: the first diagnostics with an error are the final ones (clang++ publishes the probe's
	// answer before the check's).
	p := startLSPEnv(t, []string{"GOCPP_CXX=g++", "GOCPP_LSP_CACHE=off", "XDG_CACHE_HOME=" + t.TempDir()})
	p.send(lspInitialize)
	p.read()
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"gocpp","version":1,"text":%q}}}`,
		uri, files["main.gp"]))
	for {
		body, err := p.out.Read()
		if err != nil {
			t.Fatalf("reading from gocpp lsp: %v; stderr:\n%s", err, p.stderr.String())
		}
		text := string(body)
		if !strings.Contains(text, `"method":"textDocument/publishDiagnostics"`) || strings.Contains(text, `"diagnostics":[]`) {
			continue
		}
		if !strings.Contains(text, "can't be read") || !strings.Contains(text, "Permission denied") {
			t.Errorf("diagnostics %s; want the compiler's \"can't be read\" message", text)
		}
		if strings.Contains(text, "cannot check whether header") {
			t.Errorf("diagnostics %s; want no parse-as-C error", text)
		}
		// Stop the server the proper way: a kill while the probe's compiler runs leaves its work
		// folder in the TMPDIR of the tests.
		p.send(lspShutdown)
		p.send(lspExit)
		p.cmd.Wait()
		return
	}
}
