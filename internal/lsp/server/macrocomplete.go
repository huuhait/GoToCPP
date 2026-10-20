package server

import (
	"context"
	"fmt"
	"go/token"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// Completion after a # (step 11.13): the names that can follow it, #| and #util.|. The parser reads
// `#Assert` as a name with a # in it, which the normal completion (a scope lookup in the checked tree)
// does not know, so the macros have their own list, made from the text before the cursor and the macros
// of the package and of the packages the file imports (load.Package.Macros, which the summaries keep):
//
//	#|            the built-in macros (#file #line #func #text #embed #body #Derive), the macros of the
//	              package, the macros of the packages imported with a dot, and the imported packages
//	              that declare macros (they insert `util.`)
//	#util.|       the exported macros of the package that the file imports as util
//
// A macro inserts as a call with a placeholder for each parameter. A # inside a string or a comment
// completes nothing.

// builtinMacros are the built-in macros (README §2.2 of Phase 10) with what they do.
var builtinMacros = []struct{ name, detail, doc, snippet string }{
	{"Derive", "#Derive(String, Equal, Clone, JSON)", "Writes methods for the type below it (`gocpp expand` shows them).", "Derive(${1:Equal})"},
	{"file", "#file() string", "The file of the use (of the outermost use, inside a macro).", "file()"},
	{"line", "#line() int", "The line of the use.", "line()"},
	{"func", "#func() string", "The function the use is in.", "func()"},
	{"text", "#text(expr) string", "The source text of an expression, as written.", "text(${1:expr})"},
	{"embed", `#embed("file") string`, "The content of a file, read when the package is compiled.", `embed("${1:file}")`},
	{"body", "#body()", "In a wrap macro: runs the code of the function the macro stands on.", "body()"},
}

// macroCompletion answers the completion at p when the text before it is a # and the start of a name, or
// #pkg. and the start of a name. ok is false when it is not: the normal completion answers then.
func (s *Server) macroCompletion(ctx context.Context, e env, p protocol.CompletionParams) (list *protocol.CompletionList, ok bool, err error) {
	o := e.snapshot.Overlay(p.TextDocument.URI)
	if o == nil || o.Broken() {
		return nil, false, nil
	}
	text, mapper := o.Text(), o.Mapper()
	off := mapper.Offset(p.Position)
	start, qual, found := macroPrefix(text, off)
	if !found {
		return nil, false, nil
	}
	list = &protocol.CompletionList{IsIncomplete: true, Items: []protocol.CompletionItem{}}
	if inCommentOrString(text, off) {
		return nil, false, nil
	}
	q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, p.Position)
	if err != nil {
		return nil, true, err
	}
	// The end of the name under the cursor: it is replaced.
	end := off
	for end < len(text) {
		r, n := utf8.DecodeRuneInString(text[end:])
		if !isNameRune(r) {
			break
		}
		end += n
	}
	rng := protocol.Range{Start: mapper.Position(start), End: mapper.Position(end)}
	snippets := s.comp.snippets.Load()
	format := protocol.PlainTextTextFormat
	if snippets {
		format = protocol.SnippetTextFormat
	}
	prefix := strings.ToLower(text[start:off])
	paren := end < len(text) && text[end] == '('
	add := func(label string, kind protocol.CompletionItemKind, detail, docText, plain, snippet string) {
		call := strings.HasSuffix(plain, "(")
		if !strings.HasPrefix(strings.ToLower(label), prefix) {
			return
		}
		text := plain
		if snippets {
			text = snippet
		}
		if call && paren {
			text = label // (the parenthesis is there already: #Der|ive(Equal))
		}
		item := protocol.CompletionItem{
			Label:            label,
			Kind:             kind,
			Detail:           detail,
			TextEdit:         &protocol.Or_CompletionItem_textEdit{Value: protocol.TextEdit{NewText: text, Range: rng}},
			InsertTextFormat: &format,
			FilterText:       label,
			SortText:         fmt.Sprintf("%05d", len(list.Items)),
		}
		if docText != "" {
			item.Documentation = &protocol.Or_CompletionItem_documentation{Value: protocol.MarkupContent{Kind: protocol.PlainText, Value: docText}}
		}
		list.Items = append(list.Items, item)
	}
	addMacros := func(macros map[string]*load.MacroSig, exportedOnly bool) {
		names := make([]string, 0, len(macros))
		for n := range macros {
			if !exportedOnly || token.IsExported(n) {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		for _, n := range names {
			sig := macros[n]
			add(n, protocol.FunctionCompletion, sig.Sig, "", n+"(", macroSnippet(sig))
		}
	}
	if q == nil {
		if qual == "" {
			for _, b := range builtinMacros {
				add(b.name, protocol.KeywordCompletion, b.detail, b.doc, b.name+"(", b.snippet)
			}
		}
		return list, true, nil
	}
	if qual != "" {
		if dep := q.importedAs(qual); dep != nil {
			addMacros(dep.Macros, true)
		}
		return list, true, nil
	}
	addMacros(q.lp.Macros, false)
	for _, b := range builtinMacros {
		add(b.name, protocol.KeywordCompletion, b.detail, b.doc, b.name+"(", b.snippet)
	}
	if info := q.file.Info; info != nil {
		var pkgs []string
		for _, spec := range info.Packages {
			dep := q.lp.Imports[importPath(spec)]
			if dep == nil || len(dep.Macros) == 0 {
				continue
			}
			if spec.Name != nil && spec.Name.Name == "." {
				addMacros(dep.Macros, true)
				continue
			}
			name := dep.Name
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if name != "_" {
				pkgs = append(pkgs, name)
			}
		}
		sort.Strings(pkgs)
		for _, name := range pkgs {
			add(name, protocol.ModuleCompletion, "package "+name+": it declares macros", "", name+".", name+".")
		}
	}
	return list, true, nil
}

// importedAs returns the package that the file of q imports under the name name, or nil.
func (q *query) importedAs(name string) *load.Package {
	info := q.file.Info
	if info == nil {
		return nil
	}
	spec := info.PackageNames[name]
	if spec == nil {
		return nil
	}
	return q.lp.Imports[importPath(spec)]
}

// macroSnippet returns the text that inserts the call of a macro: its name and a placeholder for each
// parameter.
func macroSnippet(sig *load.MacroSig) string {
	var b strings.Builder
	b.WriteString(sig.Name)
	b.WriteByte('(')
	for i, p := range sig.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "${%d:%s}", i+1, p.Name)
	}
	b.WriteByte(')')
	return b.String()
}

// macroPrefix looks at the text before off. If it ends with a # and the start of a name (#, #As), it returns
// the offset where the name starts and an empty qualifier; if it ends with #pkg. and the start of a name, the
// offset where the name after the dot starts and the package name. found is false for any other text.
func macroPrefix(src string, off int) (start int, qual string, found bool) {
	if off > len(src) {
		return 0, "", false
	}
	i := nameStart(src, off)
	if i > 0 && src[i-1] == '#' {
		return i, "", true
	}
	if i > 0 && src[i-1] == '.' {
		j := nameStart(src, i-1)
		if j < i-1 && j > 0 && src[j-1] == '#' {
			return i, src[j : i-1], true
		}
	}
	return 0, "", false
}

// nameStart returns the offset where the run of name characters that ends at off starts.
func nameStart(src string, off int) int {
	i := off
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(src[:i])
		if !isNameRune(r) {
			break
		}
		i -= n
	}
	return i
}

// inCommentOrString reports whether the offset off of the text is inside a comment, a string literal (a raw string
// of several lines too) or a character literal: a # there is text. It scans the text from its start, as the
// compiler does, so a backquote inside a string or a quote inside a comment mean nothing.
func inCommentOrString(src string, off int) bool {
	b := []byte(src)
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(b))
	var sc scanner.Scanner
	sc.Init(file, b, func(token.Position, string) {}, scanner.ScanComments)
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			return false
		}
		start := file.Offset(pos)
		if start >= off {
			return false // the tokens are in order: none of the rest can hold the offset
		}
		var end int
		switch {
		case tok == token.COMMENT && strings.HasPrefix(lit, "//"):
			end = start + strings.IndexByte(src[start:]+"\n", '\n')
			if end > start && src[end-1] == '\r' {
				end--
			}
		case tok == token.COMMENT:
			end = len(src)
			if k := strings.Index(src[start+2:], "*/"); k >= 0 {
				end = start + 2 + k + 2
			}
		case tok == token.STRING && strings.HasPrefix(lit, "`"):
			end = len(src)
			if k := strings.IndexByte(src[start+1:], '`'); k >= 0 {
				end = start + 1 + k + 1
			}
		case tok == token.STRING || tok == token.CHAR:
			end = start + len(lit)
		default:
			continue
		}
		// inside: after the first character, up to the end (a `//` comment has no closing, so the cursor at its end is
		// inside; a string or a block comment ends at its closing quote or star-slash, after which the cursor is outside)
		if off > start && (off < end || off == end && !closed(src, start, end)) {
			return true
		}
	}
}

// closed reports whether the string or character literal at src[start:end] has its closing quote.
func closed(src string, start, end int) bool {
	return end-start >= 2 && src[end-1] == src[start]
}
