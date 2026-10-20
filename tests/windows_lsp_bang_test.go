package tests

import (
	"net/url"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// TestWindowsLSPBang: the features for `!` on a real Windows PC (step 11.13, ADR-099): hover and inlay hints on a CRLF file
// with an emoji before the call (UTF-16 columns), and the code action's command, which writes the expansion of the file
// to a folder of the server's under the PC's temp folder; the folder is there while the server runs, and gone after a
// shutdown. GOCPP_WINDOWS_SSH=user@host turns it on.
func TestWindowsLSPBang(t *testing.T) {
	n := nativeRemote(t)
	w := startWindowsLSP(t, n, lspOpts{rootURI: func(root string) string { return fileURI(root, true) }})
	src := "package bang\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc size(p string) (int64, error) {\n\tf := os.Open!(p)\n\tfmt.Println(\"😀\", f.Name()); st := f.Stat!()\n\treturn st.Size(), nil\n}\n"
	text := strings.ReplaceAll(src, "\n", "\r\n")
	w.ctl(ctlOp{Op: "mkdir", Path: w.root + `\bang`}, w.write("bang/b.gp", src))
	uri := fileURI(w.root+`\bang\b.gp`, true)
	mark := w.c.Seq()
	w.open(uri, text, 1)
	if p := w.publish(uri, mark, nil); !noErrors(p) {
		t.Fatalf("diagnostics of a good file: %+v", p)
	}

	// hover on the name of the call, and on its !
	line, col := column(text, "Open!")
	for _, c := range []int{col + 1, col + len("Open")} {
		var h protocol.Hover
		w.call("textDocument/hover", w.at(uri, line, c), &h)
		for _, want := range []string{"`size` returns it", "f, must_e1 := os.Open(p)"} {
			if !strings.Contains(h.Contents.Value, want) {
				t.Errorf("hover at column %d: %q does not contain %q", c, h.Contents.Value, want)
			}
		}
	}

	// hints: the second ! is after an emoji on its line
	var hints []protocol.InlayHint
	w.call("textDocument/inlayHint", map[string]any{"textDocument": map[string]any{"uri": uri},
		"range": protocol.Range{End: protocol.Position{Line: 13}}}, &hints)
	line2, col2 := column(text, "Stat!()")
	wantCol := col2 + len("Stat!()")
	found := 0
	for _, h := range hints {
		if len(h.Label) == 1 && h.Label[0].Value == "→ return err" {
			found++
			if h.Position.Line == uint32(line2) && h.Position.Character != uint32(wantCol) {
				t.Errorf("the hint after f.Stat!() is at column %d, want %d (UTF-16)", h.Position.Character, wantCol)
			}
		}
	}
	if found != 2 {
		t.Errorf("%d hints of the ! calls, want 2: %+v", found, hints)
	}

	// the command
	var er struct {
		URI       string
		Text      string
		Selection *protocol.Range
	}
	w.call("workspace/executeCommand", map[string]any{"command": "gocpp.showExpansion",
		"arguments": []any{map[string]any{"uri": uri, "range": protocol.Range{Start: protocol.Position{Line: uint32(line), Character: uint32(col)}, End: protocol.Position{Line: uint32(line), Character: uint32(col)}}}}}, &er)
	if !strings.Contains(er.Text, "f, must_e1 := os.Open(p)\r\n") || !strings.Contains(er.Text, "st, must_e") || er.Selection == nil {
		t.Fatalf("the expansion: %q (selection %+v)", er.Text, er.Selection)
	}
	u, err := url.Parse(er.URI)
	if err != nil || !strings.HasSuffix(u.Path, "/b.gpexpand") {
		t.Fatalf("the file of the expansion: %q (%v)", er.URI, err)
	}
	file := strings.ReplaceAll(strings.TrimPrefix(u.Path, "/"), "/", `\`)
	dir := file[:strings.LastIndex(file, `\`)]
	listing := func() (string, int) {
		res := n.cmd(t, "", nil, `winlspctl.exe `+w.ctlScript(ctlOp{Op: "list", Path: dir}))
		return normNL(res.Stdout), res.Code
	}
	if out, code := listing(); code != 0 || strings.TrimSpace(out) != `"b.gpexpand"` {
		t.Errorf("the folder %s while the server runs: exit %d, %q", dir, code, out)
	}

	// shutdown: the folder goes
	w.call("shutdown", nil, nil)
	if err := w.c.Notify("exit", nil); err != nil {
		t.Fatal(err)
	}
	if code, err := w.p.Wait(w.ctx()); err != nil || code != 0 {
		t.Errorf("gocpp lsp after exit: %d (%v); stderr:\n%s", code, err, w.p.Stderr())
	}
	if out, code := listing(); code == 0 {
		t.Errorf("the folder %s is still there after the end of the server: %q", dir, out)
	}
}
