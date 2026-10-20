// Command lspdiff compares the answers of two builds of "gocpp lsp" on the
// reference projects of the benchmarks (internal/lsp/bench): the same
// requests at many places of many files, before and after edits, and the
// diagnostics, must give the same answers. It is the check that a speed
// change of the language server changes no answer.
//
//	go run ./tools/lspdiff -a bin/gocpp.old -b bin/gocpp.new -projects small,medium
//
// It exits with status 1 when an answer differs. The answers are compared as
// JSON (the text of the project's folder is replaced by <root>); the
// diagnostics are compared as (severity, line, message).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

func main() { os.Exit(run()) }

type server struct {
	name string
	s    *bench.Session
}

var (
	binA      = flag.String("a", "", "the first gocpp binary")
	binB      = flag.String("b", "", "the second gocpp binary")
	projects  = flag.String("projects", "small,medium", "the projects: small, medium, large")
	perFile   = flag.Int("per-file", 40, "places to ask about in each file")
	nFiles    = flag.Int("files", 14, "how many files to open")
	nEdits    = flag.Int("edits", 24, "how many edits to make in the hot file, comparing after each")
	extraArgs = flag.String("args", "", "more arguments for both servers")
	env       = flag.String("env", "GOCPP_GATE_PHASE=15", "environment for both servers, comma-separated")
	verbose   = flag.Bool("v", false, "list the differences in full")
)

func run() int {
	flag.Parse()
	if *binA == "" || *binB == "" {
		fmt.Fprintln(os.Stderr, "lspdiff: -a and -b are needed")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	failed := false
	for _, name := range strings.Split(*projects, ",") {
		spec, ok := bench.SpecByName(strings.TrimSpace(name))
		if !ok {
			fmt.Fprintf(os.Stderr, "lspdiff: no project %q\n", name)
			return 2
		}
		diffs, compared, totals, err := compareProject(ctx, spec)
		fmt.Printf("%s: replies a=%d successful/%d RPC refusals/%d failed; "+
			"b=%d successful/%d RPC refusals/%d failed\n", name,
			totals[0].Success, totals[0].Refused, totals[0].Failed,
			totals[1].Success, totals[1].Refused, totals[1].Failed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lspdiff: %s: %v\n", name, err)
			return 2
		}
		fmt.Printf("%s: %d answers compared, %d differ\n", name, compared, len(diffs))
		for i, d := range diffs {
			if i >= 20 && !*verbose {
				fmt.Printf("  ... and %d more\n", len(diffs)-i)
				break
			}
			fmt.Println("  " + d)
		}
		failed = failed || len(diffs) > 0
	}
	if failed {
		return 1
	}
	return 0
}

type replyTotals struct{ Success, Refused, Failed int }

type rawCaller interface {
	CallRaw(context.Context, string, any) (json.RawMessage, error)
}

type requestReply struct {
	value           string
	refused, failed bool
}

// compareReplies requires an actual result or explicit semantic refusal from
// each side. Equal EOFs, timeouts, cancellations or missing results prove nothing.
func compareReplies(
	ctx context.Context,
	clients [2]rawCaller,
	method string,
	params [2]any,
	norm func([]byte) string,
) (out [2]requestReply, err error) {
	var failures []error
	for i, client := range clients {
		raw, callErr := client.CallRaw(ctx, method, params[i])
		if callErr != nil {
			if rpc := semanticRefusal(callErr); rpc != nil {
				data, _ := json.Marshal(rpc)
				out[i] = requestReply{value: "RPC refusal: " + canonical(norm(data)), refused: true}
				continue
			}
			out[i].failed = true
			failures = append(failures,
				fmt.Errorf("server %c %s: request failed: %w", 'a'+i, method, callErr))
			continue
		}
		if !json.Valid(raw) {
			out[i].failed = true
			failures = append(failures,
				fmt.Errorf("server %c %s: missing or invalid JSON result", 'a'+i, method))
			continue
		}
		out[i].value = canonical(norm(raw))
	}
	return out, errors.Join(failures...)
}

func semanticRefusal(err error) *protocol.ResponseError {
	var rpc *protocol.ResponseError
	if !errors.As(err, &rpc) {
		return nil
	}
	switch rpc.Code {
	case int64(protocol.RequestCancelled), int64(protocol.ServerCancelled),
		int64(protocol.ContentModified):
		return nil
	}
	return rpc
}

func compareProject(
	ctx context.Context,
	spec bench.Spec,
) (diffs []string, compared int, totals [2]replyTotals, err error) {
	proj := bench.Generate(spec)
	dir, err := os.MkdirTemp("", "lspdiff-")
	if err != nil {
		return nil, 0, totals, err
	}
	defer os.RemoveAll(dir)
	tree := filepath.Join(dir, "tree")
	if err := proj.Write(tree, ".gp"); err != nil {
		return nil, 0, totals, err
	}
	var servers []*server
	for i, bin := range []string{*binA, *binB} {
		abs, err := filepath.Abs(bin)
		if err != nil {
			return nil, 0, totals, err
		}
		t := bench.Target{Name: "gocpp", Ext: ".gp", Command: append([]string{abs, "lsp"}, strings.Fields(*extraArgs)...), Env: strings.Split(*env, ",")}
		s, err := bench.StartSession(ctx, time.Minute, proj, t, tree, filepath.Join(dir, fmt.Sprintf("cache%d", i)))
		if err != nil {
			return nil, 0, totals, err
		}
		defer s.P.Kill()
		servers = append(servers, &server{name: string('a' + rune(i)), s: s})
	}
	root := regexp.MustCompile(regexp.QuoteMeta(tree))
	rootURI := regexp.MustCompile(regexp.QuoteMeta(string(protocol.URIFromPath(tree))))
	norm := func(b []byte) string {
		b = rootURI.ReplaceAll(b, []byte("<root>"))
		return string(root.ReplaceAll(b, []byte("<root>")))
	}
	report := func(what string, a, b string) {
		compared++
		if a != b {
			diffs = append(diffs, formatDifference(what, a, b, *verbose))
		}
	}
	// ask sends the same request to both and compares only received answers.
	var requestErr error
	ask := func(what, method string, params func(s *bench.Session) any) {
		if requestErr != nil {
			return
		}
		out, callErr := compareReplies(ctx, [2]rawCaller{servers[0].s, servers[1].s}, method,
			[2]any{params(servers[0].s), params(servers[1].s)}, norm)
		for i, reply := range out {
			switch {
			case reply.failed:
				totals[i].Failed++
			case reply.refused:
				totals[i].Refused++
			default:
				totals[i].Success++
			}
		}
		if callErr != nil {
			requestErr = fmt.Errorf("%s: %w", what, callErr)
			return
		}
		report(what, out[0].value, out[1].value)
	}
	// Open the files: the hot and dependent ones first, then others.
	files := []string{proj.Hot, proj.Dep, proj.Dependent, proj.Core, proj.Big, proj.APIFile, proj.APIUser}
	for _, f := range proj.SourceFiles(false) {
		if len(files) >= *nFiles {
			break
		}
		if !slices.Contains(files, f) && !strings.HasPrefix(f, "cpp") {
			files = append(files, f)
		}
	}
	for _, f := range files {
		for _, sv := range servers {
			if err := sv.s.OpenAndWait(ctx, f); err != nil {
				return diffs, compared, totals, fmt.Errorf("%s: open %s: %w", sv.name, f, err)
			}
		}
	}
	time.Sleep(500 * time.Millisecond)
	diagnostics := func(when string) {
		for _, f := range files {
			var out [2]string
			for i, sv := range servers {
				out[i] = diagText(sv.s, f)
			}
			report("diagnostics of "+f+" "+when, out[0], out[1])
		}
	}
	diagnostics("after the open")

	ident := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	for _, f := range files {
		text := servers[0].s.Doc(f).Text
		locs := ident.FindAllStringIndex(text, -1)
		step := max(1, len(locs) / *perFile)
		uri := string(servers[0].s.Doc(f).URI)
		for k := 0; k < len(locs); k += step {
			off := locs[k][0] + (locs[k][1]-locs[k][0])/2
			at := func(s *bench.Session) any { return s.Params(f, off) }
			where := fmt.Sprintf("%s@%d (%s)", f, off, text[locs[k][0]:locs[k][1]])
			ask("hover "+where, "textDocument/hover", at)
			ask("definition "+where, "textDocument/definition", at)
			ask("typeDefinition "+where, "textDocument/typeDefinition", at)
			ask("highlight "+where, "textDocument/documentHighlight", at)
			ask("references "+where, "textDocument/references", func(s *bench.Session) any {
				return map[string]any{"textDocument": map[string]any{"uri": uri}, "position": protocolPos(s, f, off), "context": map[string]any{"includeDeclaration": true}}
			})
			ask("prepareRename "+where, "textDocument/prepareRename", at)
			ask("rename "+where, "textDocument/rename", func(s *bench.Session) any {
				return map[string]any{"textDocument": map[string]any{"uri": uri}, "position": protocolPos(s, f, off), "newName": "ZqRenamed"}
			})
			ask("completion "+where, "textDocument/completion", func(s *bench.Session) any { return s.Params(f, locs[k][1]) })
			ask("signatureHelp "+where, "textDocument/signatureHelp", func(s *bench.Session) any { return s.Params(f, min(locs[k][1]+1, len(text))) })
		}
		doc := func(s *bench.Session) any {
			return map[string]any{"textDocument": map[string]any{"uri": uri}}
		}
		ask("documentSymbol "+f, "textDocument/documentSymbol", doc)
		ask("semanticTokens "+f, "textDocument/semanticTokens/full", doc)
		ask("formatting "+f, "textDocument/formatting", func(s *bench.Session) any {
			return map[string]any{"textDocument": map[string]any{"uri": uri}, "options": map[string]any{"tabSize": 8, "insertSpaces": false}}
		})
		ask("documentLink "+f, "textDocument/documentLink", doc)
		ask("codeAction "+f, "textDocument/codeAction", func(s *bench.Session) any {
			return map[string]any{"textDocument": map[string]any{"uri": uri}, "range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 1, "character": 0}}, "context": map[string]any{"diagnostics": []any{}}}
		})
		ask("inlayHint "+f, "textDocument/inlayHint", func(s *bench.Session) any {
			return map[string]any{"textDocument": map[string]any{"uri": uri}, "range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 100000, "character": 0}}}
		})
		if requestErr != nil {
			return diffs, compared, totals, requestErr
		}
	}
	for _, q := range append(slices.Clone(proj.Queries), "", "a", "Zq", "New", "get") {
		ask("workspace/symbol "+q, "workspace/symbol", func(*bench.Session) any { return map[string]any{"query": q} })
	}

	if requestErr != nil {
		return diffs, compared, totals, requestErr
	}

	// Edits in the hot file: each is applied to both servers, and the diagnostics and a few
	// answers are compared after it.
	hot := proj.Hot
	base := servers[0].s.Doc(hot).Text
	anchor := strings.Index(base, "\tsum := 0\n")
	if anchor < 0 {
		return diffs, compared, totals, fmt.Errorf("no anchor in %s", hot)
	}
	valueAt := anchor + len("\tsum := ")
	cur := "0"
	for i := 0; i < *nEdits; i++ {
		var next string
		switch i % 4 {
		case 0:
			next = fmt.Sprintf("undefinedName%d", i)
		case 1:
			next = fmt.Sprintf("%d", 1000+i)
		case 2:
			next = fmt.Sprintf("%d +", i) // a syntax error
		default:
			next = fmt.Sprintf("%d", 2000+i)
		}
		for _, sv := range servers {
			seq := sv.s.C.Seq()
			t0, err := sv.s.Edit(hot, valueAt, valueAt+len(cur), next)
			if err != nil {
				return diffs, compared, totals, err
			}
			ver := sv.s.Doc(hot).Version
			if _, _, err := sv.s.Wait(ctx, hot, seq, t0, func(p *bench.Publish) bool { return p.Version >= ver }); err != nil {
				return diffs, compared, totals, fmt.Errorf("%s: edit %d: %w", sv.name, i, err)
			}
		}
		cur = next
		time.Sleep(150 * time.Millisecond)
		diagnostics(fmt.Sprintf("after edit %d (%s)", i, next))
		for _, off := range []int{anchor + 3, valueAt, valueAt + 1, anchor - 40} {
			at := func(s *bench.Session) any { return s.Params(hot, off) }
			ask(fmt.Sprintf("hover after edit %d@%d", i, off), "textDocument/hover", at)
			ask(fmt.Sprintf("definition after edit %d@%d", i, off), "textDocument/definition", at)
			ask(fmt.Sprintf("completion after edit %d@%d", i, off), "textDocument/completion", at)
		}
		ask(fmt.Sprintf("semanticTokens after edit %d", i), "textDocument/semanticTokens/full", func(s *bench.Session) any {
			return map[string]any{"textDocument": map[string]any{"uri": string(s.Doc(hot).URI)}}
		})
		if requestErr != nil {
			return diffs, compared, totals, requestErr
		}
		if i%6 == 5 {
			// An API change in a package that others import, saved: the importers' diagnostics follow.
			api := proj.APIFile
			sig := "func Label(s string"
			for _, sv := range servers {
				off := strings.Index(sv.s.Doc(api).Text, sig)
				if off < 0 {
					return diffs, compared, totals, fmt.Errorf("%s: missing API-change anchor", sv.name)
				}
				seq := sv.s.C.Seq()
				if _, err := sv.s.Edit(api, off+len(sig), off+len(sig), ", n int"); err != nil {
					return diffs, compared, totals, err
				}
				if err := sv.s.Save(api); err != nil {
					return diffs, compared, totals, err
				}
				if _, _, err := sv.s.Wait(ctx, proj.APIUser, seq, time.Now(), func(p *bench.Publish) bool { return p.Errors() > 0 }); err != nil {
					return diffs, compared, totals, fmt.Errorf("%s: API change at edit %d: %w", sv.name, i, err)
				}
			}
			time.Sleep(200 * time.Millisecond)
			diagnostics(fmt.Sprintf("after the API change at edit %d", i))
			for _, sv := range servers {
				off := strings.Index(sv.s.Doc(api).Text, sig+", n int")
				if off < 0 {
					return diffs, compared, totals, fmt.Errorf("%s: missing API-revert anchor", sv.name)
				}
				seq := sv.s.C.Seq()
				if _, err := sv.s.Edit(api, off+len(sig), off+len(sig)+len(", n int"), ""); err != nil {
					return diffs, compared, totals, err
				}
				if err := sv.s.Save(api); err != nil {
					return diffs, compared, totals, err
				}
				if _, _, err := sv.s.Wait(ctx, proj.APIUser, seq, time.Now(), func(p *bench.Publish) bool { return p.Errors() == 0 }); err != nil {
					return diffs, compared, totals, fmt.Errorf("%s: API revert at edit %d: %w", sv.name, i, err)
				}
			}
			time.Sleep(200 * time.Millisecond)
			diagnostics(fmt.Sprintf("after the API change is undone at edit %d", i))
		}
	}
	for _, sv := range servers {
		sv.s.P.Stop()
	}
	return diffs, compared, totals, nil
}

// formatDifference keeps verbose evidence lossless: a difference in a long
// completion list can occur well after its shared prefix.
func formatDifference(what, a, b string, full bool) string {
	if !full {
		if len(a) > 300 {
			a = a[:300] + "..."
		}
		if len(b) > 300 {
			b = b[:300] + "..."
		}
	}
	return fmt.Sprintf("%s:\n    a: %s\n    b: %s", what, a, b)
}

func protocolPos(s *bench.Session, rel string, off int) protocol.Position {
	return s.Pos(rel, off)
}

// diagText is the last diagnostics of the file as text.
func diagText(s *bench.Session, rel string) string {
	p := s.C.LastPublish(s.Doc(rel).URI)
	if p == nil {
		return "(none)"
	}
	var b strings.Builder
	for _, d := range p.Diags {
		fmt.Fprintf(&b, "%d:%d:%s\n", d.Severity, d.Line, d.Message)
	}
	return b.String()
}

// canonical re-encodes JSON with sorted keys, so that two equal answers read the same.
func canonical(s string) string {
	var v any
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return s
	}
	out, err := json.Marshal(v)
	if err != nil {
		return s
	}
	return string(out)
}
