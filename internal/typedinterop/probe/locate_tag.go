package probe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/build"
)

// The body of a typedef'd tag (step 12.13e). In C, `typedef struct Vector2 { float x; float y; }
// Vector2;` is how every library names its structs, and the lookup of the name (locate.go) finds
// the typedef: the dump of a typedef has the name of its record and nothing of its fields, and no
// lookup of a pragma finds the record itself (C++ hides a tag that a typedef of the same name
// names). The compiler's filtered AST dump does: `-ast-dump -ast-dump-filter=NAME` prints the
// definition of every declaration whose name has NAME in it, with its fields, enumerators and
// comments. It names one filter, so a tag costs one run, which the runner caches like the others.
// The run belongs to the typedef (its range tells which unnamed tag is its own), and a tag whose
// run fails or does not find the definition keeps the plain typedef text.

const (
	maxBodyFields  = 40 // fields (also of nested records) or enumerators of a shown body
	maxBodyBytes   = 8 << 10
	maxTagRuns     = 16 // typedefs of one call that get a run
	minTagNameSize = 3  // a shorter name would match most of the headers
	maxFieldDocLen = 72 // a longer comment of a field is not shown after it
	bodyIndent     = "    "
)

var tagTypeRx = regexp.MustCompile(
	`^(struct|union|class|enum) ((?:[A-Za-z_][A-Za-z0-9_]*::)*[A-Za-z_][A-Za-z0-9_]*)$`)

// tagCanonRx matches the type with its typedef names resolved: `b::Thing` or `struct Thing`.
var tagCanonRx = regexp.MustCompile(
	`^(?:(?:struct|union|class|enum) )?((?:[A-Za-z_][A-Za-z0-9_]*::)*[A-Za-z_][A-Za-z0-9_]*)$`)

// typedefTag returns the tag that the typedef at h names, or nil when it names something else or
// the typedef's range is not one that can be compared (another file, a macro).
func typedefTag(h nodeHeader, qual string) *TagRef {
	m := tagTypeRx.FindStringSubmatch(h.typ)
	if m == nil || !h.begin.valid || !h.at.valid || h.begin.file == "" ||
		h.begin.file != h.at.file || h.end.valid && h.end.file != h.at.file {
		return nil
	}
	end := h.end
	if !end.valid {
		end = h.at
	}
	// The type as written may leave the namespaces out (`struct Thing` inside namespace b); the
	// type with its sugar removed names the tag in full (`b::Thing`).
	qualifiedTag := m[2]
	canon := tagCanonRx.FindStringSubmatch(h.canon)
	if canon != nil && strings.HasSuffix(canon[1], m[2]) {
		qualifiedTag = canon[1]
	}
	name := qualifiedTag
	if i := strings.LastIndex(name, "::"); i >= 0 {
		name = name[i+2:]
	}
	if len(name) < minTagNameSize {
		return nil
	}
	return &TagRef{
		Kind: m[1], Name: name, Qualified: qualifiedTag, Typedef: qualified(qual, h.name),
		Span: [4]int{h.begin.line, h.begin.col, end.line, end.col},
	}
}

// LocateBodies returns the declarations with the body of the tag in the signature of each typedef
// that names one: `typedef struct Vector2 { float x; float y; } Vector2;`. A declaration that is
// not a typedef of a tag, or whose body the compiler did not give exactly, is returned as it was.
// Like Locate it runs the compiler and must run off the request path; it fails only when the
// request is stopped.
func (rn *Runner) LocateBodies(ctx context.Context, cfg Config, includes []string,
	decls []Declaration,
) ([]Declaration, error) {
	if cfg.CXX.Family != build.Clang {
		return nil, ErrNoClang
	}
	out := slices.Clone(decls)
	type job struct {
		index int
		body  string
		anon  bool
		ok    bool
	}
	var jobs []*job
	for i, d := range decls {
		if d.Tag != nil && d.Kind == "type" && len(jobs) < maxTagRuns {
			jobs = append(jobs, &job{index: i})
		}
	}
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j.body, j.anon, j.ok = rn.tagBody(ctx, cfg, includes, decls[j.index])
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, j := range jobs {
		if j.ok {
			out[j.index].Signature = typedefWithBody(decls[j.index], j.body, j.anon)
		}
	}
	return out, nil
}

// typedefWithBody writes the typedef with the body of its tag. A tag that has no name of its own,
// or the typedef's own name, is written in the one declaration the header has; a tag with another
// name is shown after the typedef.
func typedefWithBody(d Declaration, body string, anonymous bool) string {
	if anonymous || d.Tag.Name == d.Tag.Typedef ||
		strings.HasSuffix(d.Tag.Typedef, "::"+d.Tag.Name) {
		return "typedef " + body + " " + d.Tag.Typedef + ";"
	}
	return d.Signature + "\n\n" + body + ";"
}

// tagVersion is the cache version of the run of one typedef.
func tagVersion(d Declaration) string {
	return fmt.Sprintf("%s-tag/%s/%s/%v", locateKeyVersion, d.Tag.Qualified, d.File, d.Tag.Span)
}

// tagSource is the probe file of a tag: the includes of the program, and nothing else (the dump
// of the filter is of the declarations of the headers).
func tagSource(includes []string) (string, error) {
	text, _, err := locateSource(includes, nil, nil)
	if err != nil {
		return "", err
	}
	end := strings.Index(text, "#pragma clang __debug dump gocpp_loc_end")
	if end < 0 {
		return "", errors.New("the probe file has no end")
	}
	return text[:end], nil
}

// tagBody runs the filtered dump for the tag of d and returns the body text.
func (rn *Runner) tagBody(ctx context.Context, cfg Config, includes []string,
	d Declaration,
) (body string, anonymous, ok bool) {
	text, err := tagSource(includes)
	if err != nil {
		return "", false, false
	}
	key := "t\x00" + d.Tag.Name
	produce := func(set *sourceSet, answers map[string]roundAnswer) ([]*auxOut, error) {
		return rn.produceTag(ctx, cfg, text, set, d, key, answers)
	}
	answers, _, err := rn.cachedRound(ctx, cfg, includes, tagVersion(d), text, produce)
	if err != nil {
		return "", false, false
	}
	answer := answers[key]
	if answer.Failed || len(answer.Decls) != 1 || answer.Decls[0].Signature == "" {
		return "", false, false
	}
	return answer.Decls[0].Signature, answer.Decls[0].Type == "anonymous", true
}

// filteredDump runs the compiler on the text with the AST dump of the declarations whose
// qualified name has filter in it, and returns what it printed.
func (rn *Runner) filteredDump(ctx context.Context, cfg Config, text string, set *sourceSet,
	filter string,
) (*dumpReader, *auxOut, error) {
	reader := newDumpReader(maxLocateDump, set.resolve, nil)
	mode := []string{"-fsyntax-only", "-fparse-all-comments",
		"-fretain-comments-from-system-headers", "-ferror-limit=0", "-fno-caret-diagnostics",
		"-Xclang", "-ast-dump", "-Xclang", "-ast-dump-filter=" + filter}
	out, err := runAuxTo(ctx, cfg, mode, text, func(string) io.Writer { return reader })
	if err != nil {
		return nil, nil, err
	}
	reader.finish()
	if reader.over || out.timedOut {
		return nil, nil, errors.New("the filtered dump was too large or took too long")
	}
	if out.exitErr != nil {
		return nil, nil, fmt.Errorf("the headers have an error: %v", out.exitErr)
	}
	return reader, out, nil
}

func (rn *Runner) produceTag(ctx context.Context, cfg Config, text string, set *sourceSet,
	d Declaration, key string, answers map[string]roundAnswer,
) ([]*auxOut, error) {
	reader, out, err := rn.filteredDump(ctx, cfg, text, set, d.Tag.Qualified)
	if err != nil {
		return nil, err
	}
	answer := roundAnswer{Failed: true}
	r := &renderer{set: set}
	for _, group := range reader.groups {
		if group.droppedTop {
			continue
		}
		t := newTree(group.nodes)
		if len(t.tops) != 1 {
			continue
		}
		top := t.tops[0]
		matched, anonymous := tagMatches(group.name, t.nodes[top].hdr, d)
		if !matched {
			continue
		}
		body, rendered := r.tagBody(t, top, anonymous)
		if !rendered || len(body) > maxBodyBytes {
			break
		}
		kind := "named"
		if anonymous {
			kind = "anonymous"
		}
		answer = roundAnswer{Decls: []Declaration{{Kind: "type", Signature: body, Type: kind}}}
		break
	}
	answers[key] = answer
	return []*auxOut{out}, nil
}

// recordKeyword returns the keyword of a record node and whether it has a name: the dump of an
// unnamed record prints "struct definition", where the word before the name is the keyword.
func recordKeyword(h nodeHeader) (keyword string, named bool) {
	for _, word := range h.pre {
		if word == "struct" || word == "union" || word == "class" {
			keyword = word
		}
	}
	if keyword != "" {
		return keyword, h.name != ""
	}
	switch h.name {
	case "struct", "union", "class":
		return h.name, false
	}
	return "", false
}

// tagMatches says whether the top node of a group (headed by the qualified name group) is the
// definition of the tag that d names, and whether the tag has no name of its own.
func tagMatches(group string, h nodeHeader, d Declaration) (matched, anonymous bool) {
	if slices.Contains(h.pre, "implicit") || h.invalid {
		return false, false
	}
	var named bool
	switch h.kind {
	case "CXXRecordDecl":
		if !h.definition {
			return false, false
		}
		var keyword string
		keyword, named = recordKeyword(h)
		if keyword != d.Tag.Kind {
			return false, false
		}
	case "EnumDecl":
		if d.Tag.Kind != "enum" {
			return false, false
		}
		named = h.name != ""
	default:
		return false, false
	}
	if named {
		// The filter lists every declaration whose name has the tag's name in it, in every
		// namespace: the heading of the group is the qualified name of the one at hand.
		return h.name == d.Tag.Name && group == d.Tag.Qualified, false
	}
	inside := h.begin.valid && h.begin.file == d.File &&
		before(d.Tag.Span[0], d.Tag.Span[1], h.begin.line, h.begin.col) &&
		before(h.begin.line, h.begin.col, d.Tag.Span[2], d.Tag.Span[3])
	return inside, inside
}

// before reports whether the place (line1, col1) is not after (line2, col2).
func before(line1, col1, line2, col2 int) bool {
	return line1 < line2 || line1 == line2 && col1 <= col2
}

// tagBody writes the definition of the tag at i without a trailing semicolon.
func (r *renderer) tagBody(t *tree, i int, anonymous bool) (string, bool) {
	h := t.nodes[i].hdr
	budget := maxBodyFields
	if h.kind == "EnumDecl" {
		return enumBody(t, i, anonymous), true
	}
	body, ok := r.recordBody(t, i, 0, &budget)
	return body, ok
}

// recordBody writes a record: its head, its bases and its public data members, one per line. A
// nested record without a name is written where its member is, a named one is not (the member's
// type names it). The count is shared by the nested records.
func (r *renderer) recordBody(t *tree, i, depth int, budget *int) (string, bool) {
	h := t.nodes[i].hdr
	keyword, named := recordKeyword(h)
	if keyword == "" {
		return "", false
	}
	head := keyword
	if named {
		head += " " + h.name
	}
	var bases []string
	for _, k := range t.kids[i] {
		if access, typ, ok := baseOf(t.nodes[k]); ok {
			bases = append(bases, access+typ)
		}
	}
	if len(bases) > 0 {
		head += " : " + strings.Join(bases, ", ")
	}
	indent := strings.Repeat(bodyIndent, depth+1)
	public := keyword != "class"
	var lines []string
	var pending []string // the bodies of unnamed nested records that no member has taken yet
	omitted := false
	for _, k := range t.kids[i] {
		m := t.nodes[k].hdr
		switch m.kind {
		case "AccessSpecDecl":
			public = m.name == "public"
		case "CXXRecordDecl":
			if slices.Contains(m.pre, "implicit") || !m.definition || !public {
				continue
			}
			if _, named := recordKeyword(m); named {
				continue
			}
			if nestedBody, ok := r.recordBody(t, k, depth+1, budget); ok {
				pending = append(pending, nestedBody)
			} else {
				omitted = true
			}
		case "FieldDecl":
			if !public {
				continue
			}
			if *budget <= 0 {
				omitted = true
				continue
			}
			line, rest, ok := r.fieldInBody(t, k, pending)
			pending = rest
			if !ok {
				omitted = true
				continue
			}
			*budget--
			lines = append(lines, indent+line)
		}
	}
	if omitted {
		lines = append(lines, indent+"// ...")
	}
	closing := strings.Repeat(bodyIndent, depth) + "}"
	if len(lines) == 0 {
		return head + " {}", true
	}
	return head + " {\n" + strings.Join(lines, "\n") + "\n" + closing, true
}

// fieldInBody writes the member at k and returns what is left of the pending nested bodies. A
// member of an unnamed record type takes the oldest pending body; the member that the compiler
// adds for an anonymous union or struct has no name and is written as the body alone.
func (r *renderer) fieldInBody(t *tree, k int, pending []string,
) (line string, rest []string, ok bool) {
	m := t.nodes[k].hdr
	unnamedType := strings.Contains(m.typ, "(unnamed ") || strings.Contains(m.typ, "(anonymous ")
	switch {
	case m.name == "" && unnamedType:
		if len(pending) == 0 {
			return "", pending, false
		}
		return pending[0] + ";", pending[1:], true
	case unnamedType:
		if len(pending) == 0 {
			return "", pending, false
		}
		return pending[0] + " " + m.name + ";", pending[1:], true
	case m.name == "":
		return "", pending, false
	}
	text := declare(m.typ, m.name)
	if width, found := bitWidth(t, k); found {
		text += " : " + width
	}
	text += ";"
	if doc := r.fieldComment(t, k); doc != "" {
		text += " // " + doc
	}
	return text, pending, true
}

// bitWidth returns the width of a bit-field: its first child is the constant the compiler
// evaluated.
func bitWidth(t *tree, k int) (string, bool) {
	for _, c := range t.children(k, "ConstantExpr") {
		for j := c + 1; j < len(t.nodes) && t.nodes[j].depth > t.nodes[c].depth; j++ {
			if value, ok := strings.CutPrefix(t.nodes[j].text, "value: Int "); ok && value != "" {
				return value, true
			}
		}
	}
	return "", false
}

// fieldComment returns the comment of a member when it is one short line: a comment after the
// member on its line, or a comment of a documentation tool. A plain comment above is the heading of
// a group of members in most headers and is not the member's. A comment after the line of several
// members (`int a; int b; // text`, `int x, y; // text`) may belong to any of them, so it is shown
// for none (a documentation comment is attached by the compiler to one member and stays).
func (r *renderer) fieldComment(t *tree, k int) string {
	m := t.nodes[k].hdr
	doc := ""
	if !sharesLine(t, k) {
		doc = r.set.trailingComment(m.end, m.at)
	}
	if doc == "" {
		if c := commentOf(t, k); c != nil && c.hdr.begin.valid && c.hdr.begin.file != "" {
			_, doc = r.set.commentText(c.hdr.begin, c.hdr.end)
		}
	}
	if doc == "" || strings.Contains(doc, "\n") || len(doc) > maxFieldDocLen {
		return ""
	}
	return doc
}

// sharesLine says that another member of the tree ends on the line where the member at k ends.
func sharesLine(t *tree, k int) bool {
	m := t.nodes[k].hdr
	if !m.end.valid || m.end.file == "" {
		return false
	}
	for i, n := range t.nodes {
		if i != k && n.hdr.kind == "FieldDecl" && n.hdr.end.valid && n.hdr.end.file == m.end.file &&
			n.hdr.end.line == m.end.line {
			return true
		}
	}
	return false
}

// enumBody writes an enum with its enumerators and their values.
func enumBody(t *tree, i int, anonymous bool) string {
	h := t.nodes[i].hdr
	head := "enum"
	for _, word := range h.pre {
		if word == "class" || word == "struct" {
			head += " " + word
		}
	}
	if !anonymous && h.name != "" {
		head += " " + h.name
	}
	if h.typ != "" {
		head += " : " + h.typ
	}
	var items []string
	omitted := false
	for _, k := range t.children(i, "EnumConstantDecl") {
		if len(items) >= maxBodyFields {
			omitted = true
			break
		}
		item := t.nodes[k].hdr.name
		if value, ok := valueBelow(t, k); ok {
			item += " = " + value
		}
		items = append(items, bodyIndent+item+",")
	}
	if omitted {
		items = append(items, bodyIndent+"// ...")
	}
	if len(items) == 0 {
		return head + " {}"
	}
	return head + " {\n" + strings.Join(items, "\n") + "\n}"
}
