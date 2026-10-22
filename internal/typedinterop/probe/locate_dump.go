package probe

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Reading the output of `#pragma clang __debug dump NAME` (step 12.13c2).
//
// The pragma prints "lookup results for NAME:" and then the AST dump of every declaration that
// an unqualified lookup of NAME finds at that place of the file: each overload, an inherited
// member, the members of an instantiated class template. That is the compiler's own answer to
// "what does this name mean here", and it has the declaration's file, line and column, which
// -ast-list and the JSON dump of a filtered name cannot give for a batch of names in one run.
// The text is read as a stream (the dump of a class template is large, its bodies are not
// wanted) and only the lines that later steps use are kept.

const (
	maxDumpNodes     = 8192    // retained lines of one lookup
	maxDumpLine      = 2048    // bytes of a retained line
	maxDumpGroups    = 4096    // lookups of one run
	maxDumpDiagBytes = 1 << 20 // kept of the compiler's own messages
)

// A dumpNode is one retained line of a dump, taken apart.
type dumpNode struct {
	depth int // 0 for the declaration itself, 1 for its children, ...
	hdr   nodeHeader
	text  string // the text of a line that has no range ("value: Int 65")
}

// A dumpGroup is the answer to one lookup.
type dumpGroup struct {
	name  string
	nodes []dumpNode

	// partial says that lines were dropped for the limit: the members of a record may be
	// incomplete. droppedTop says that a declaration of the answer itself was dropped, so the
	// set of declarations is not complete.
	partial    bool
	droppedTop bool
}

var (
	lookupHeaderRx = regexp.MustCompile(`^(?:lookup results for|Dumping) (.+):$`)
	topNodeRx      = regexp.MustCompile(`^[A-Z][A-Za-z0-9]* 0x[0-9a-f]+`)
)

// keptKinds are the nodes that the renderers read below a declaration. The children of any
// other node are not kept (a function body has literals that are not default arguments), but
// every line still moves the places that the dump leaves out (locState).
var keptKinds = map[string]bool{
	"FieldDecl": true, "CXXMethodDecl": true, "CXXConstructorDecl": true,
	"CXXDestructorDecl": true, "CXXConversionDecl": true, "ParmVarDecl": true,
	"TemplateTypeParmDecl": true, "NonTypeTemplateParmDecl": true,
	"TemplateTemplateParmDecl": true, "EnumConstantDecl": true, "ConstantExpr": true,
	"IntegerLiteral": true, "FloatingLiteral": true, "CXXBoolLiteralExpr": true,
	"StringLiteral": true, "CharacterLiteral": true, "CXXNullPtrLiteralExpr": true,
	"ImplicitCastExpr": true, "FullComment": true, "FunctionDecl": true, "CXXRecordDecl": true,
	"TypeAliasDecl": true, "VarDecl": true, "value:": true, "UsingDecl": true,
	"AccessSpecDecl": true, "UsingShadowDecl": true,
	"public": true, "protected": true, "private": true, "virtual": true, // base classes
}

// A dumpReader is an io.Writer that splits the compiler's standard error into lookups, node
// lines and everything else (the compiler's diagnostics). It never keeps more than its limits.
type dumpReader struct {
	groups  []dumpGroup
	diags   bytes.Buffer
	partial []byte
	cur     *dumpGroup // receives the node lines; nil after a line that is not a dump line
	keep    []bool     // whether the last node of each depth is kept
	at      []int      // the index in cur.nodes of the last node of each depth, when it is kept
	st      locState
	paths   func(string) string
	total   int64
	max     int64
	over    bool // more than max bytes were written: the run is not trustworthy
	cancel  func()
}

func newDumpReader(max int64, paths func(string) string, cancel func()) *dumpReader {
	return &dumpReader{max: max, paths: paths, cancel: cancel}
}

func (r *dumpReader) Write(p []byte) (int, error) {
	r.total += int64(len(p))
	if r.total > r.max {
		if !r.over && r.cancel != nil {
			r.cancel()
		}
		r.over = true
		return len(p), nil
	}
	data := p
	if len(r.partial) > 0 {
		data = append(r.partial, p...)
		r.partial = nil
	}
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break
		}
		r.line(strings.TrimRight(string(data[:i]), "\r"))
		data = data[i+1:]
	}
	if len(data) > 0 {
		if len(data) > 4*maxDumpLine {
			data = data[:4*maxDumpLine] // a longer unfinished line is dropped anyway
		}
		r.partial = append([]byte(nil), data...)
	}
	return len(p), nil
}

// finish handles a last line without a newline.
func (r *dumpReader) finish() {
	if len(r.partial) > 0 {
		r.line(strings.TrimRight(string(r.partial), "\r"))
		r.partial = nil
	}
}

func (r *dumpReader) line(line string) {
	if m := lookupHeaderRx.FindStringSubmatch(line); m != nil {
		if len(r.groups) >= maxDumpGroups {
			r.over = true
			return
		}
		r.groups = append(r.groups, dumpGroup{name: strings.Clone(m[1])})
		r.cur = &r.groups[len(r.groups)-1]
		r.keep = r.keep[:0]
		r.at = r.at[:0]
		return
	}
	depth, text, ok := treeLine(line)
	if !ok || r.cur == nil {
		r.cur = nil
		if r.diags.Len() < maxDumpDiagBytes {
			r.diags.WriteString(line)
			r.diags.WriteByte('\n')
		}
		return
	}
	kind, _, _ := strings.Cut(text, " ")
	parentKept := depth == 0 || depth-1 < len(r.keep) && r.keep[depth-1]
	kept := parentKept && (depth == 0 || keptKinds[kind])
	if depth > len(r.keep) {
		return // a line whose parent is missing: the dump is not what this reader knows
	}
	r.keep = append(r.keep[:depth], kept)
	r.at = append(r.at[:depth], -1)
	if kind == "CompoundStmt" && depth > 0 && parentKept && r.at[depth-1] >= 0 {
		r.cur.nodes[r.at[depth-1]].hdr.body = true // (the statements below are not kept)
	}

	var node dumpNode
	node.depth = depth
	if kind == "value:" || depth > 0 && isBaseKind(kind) {
		node.text = text
		node.hdr.kind = kind
	} else if hdr, ok := parseNodeHeader(text, &r.st, depth == 0, r.paths); ok {
		node.hdr = hdr
	}
	if !kept {
		return
	}
	if len(r.cur.nodes) >= maxDumpNodes {
		r.cur.partial = true
		r.cur.droppedTop = r.cur.droppedTop || depth == 0
		return
	}
	if node.hdr.kind == "" && node.text == "" {
		node.hdr.kind = kind
	}
	r.at[depth] = len(r.cur.nodes)
	r.cur.nodes = append(r.cur.nodes, node)
}

// treeLine reads the tree prefix of a dump line ("| |-" and so on) and returns the depth and the
// text after it. A top-level node has no prefix and starts with its kind, an address next.
func treeLine(line string) (depth int, text string, ok bool) {
	if topNodeRx.MatchString(line) {
		return 0, line, true
	}
	i := 0
	for i+1 < len(line) && (line[i] == '|' || line[i] == ' ') && line[i+1] == ' ' {
		i += 2
	}
	if i+1 < len(line) && (line[i] == '|' || line[i] == '`') && line[i+1] == '-' {
		return i/2 + 1, line[i+2:], true
	}
	return 0, "", false
}

// A srcLoc is a place in the dump: the line and column are the compiler's (1-based, bytes).
type srcLoc struct {
	file  string // the resolved path; "" when it is not a file the run read
	line  int
	col   int
	valid bool // the dump printed a place (an implicit declaration has none)
}

// A locState is what the dump leaves out of a place that is the same as the one printed before
// it in the same top-level declaration: the file, then the line.
type locState struct {
	file     string // as printed
	resolved string
	line     int
}

var locTailRx = regexp.MustCompile(`:(\d+):(\d+)([,> ]|$)`)

// parseLoc reads one place from the start of s and returns the rest. paths decides which file
// names are files of this run: the text of a place is "file:line:col", and a file name may have
// colons, spaces and commas, so the end of the name is the end that names a known file.
func (st *locState) parseLoc(s string, paths func(string) string) (srcLoc, string, bool) {
	switch {
	case strings.HasPrefix(s, "<invalid sloc>"):
		return srcLoc{}, s[len("<invalid sloc>"):], true
	case strings.HasPrefix(s, "col:"):
		col, rest, ok := leadingInt(s[len("col:"):])
		if !ok {
			return srcLoc{}, s, false
		}
		return srcLoc{file: st.resolved, line: st.line, col: col, valid: true}, rest, true
	case strings.HasPrefix(s, "line:"):
		line, rest, ok := leadingInt(s[len("line:"):])
		if !ok || !strings.HasPrefix(rest, ":") {
			return srcLoc{}, s, false
		}
		col, rest, ok := leadingInt(rest[1:])
		if !ok {
			return srcLoc{}, s, false
		}
		st.line = line
		return srcLoc{file: st.resolved, line: line, col: col, valid: true}, rest, true
	}
	var chosen []int
	var resolved string
	for _, m := range locTailRx.FindAllStringSubmatchIndex(s, -1) {
		if chosen == nil {
			chosen = m // when no file matches, the first end is the best guess
		}
		if path := paths(s[:m[0]]); path != "" {
			chosen, resolved = m, path
			break
		}
	}
	if chosen == nil {
		return srcLoc{}, s, false
	}
	line, _ := strconv.Atoi(s[chosen[2]:chosen[3]])
	col, _ := strconv.Atoi(s[chosen[4]:chosen[5]])
	st.file, st.resolved, st.line = s[:chosen[0]], resolved, line
	return srcLoc{file: resolved, line: line, col: col, valid: true}, s[chosen[5]:], true
}

func leadingInt(s string) (int, string, bool) {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, s, false
	}
	n, err := strconv.Atoi(s[:end])
	return n, s[end:], err == nil
}

// A nodeHeader is the first line of a declaration or of a member of a dump, taken apart.
type nodeHeader struct {
	kind  string
	prev  bool // a redeclaration: the first declaration is somewhere else
	begin srcLoc
	end   srcLoc
	at    srcLoc   // where the name is
	pre   []string // the words between the place and the name: referenced, constexpr, struct, ...
	name  string   // "" for an unnamed parameter
	typ   string   // the type as the compiler prints it, with typedef names
	canon string   // the type with the top-level typedef names resolved, or ""
	post  []string // the words after the type: static, virtual, cinit, ...
	rest  string   // the text after the place of the name
	// definition says that the node is the definition of a class (the dump prints "definition").
	definition bool
	invalid    bool // the declaration has no place (implicit, built in)
	// body says that a function has a body: the dump has a CompoundStmt below it.
	body bool
}

// parseNodeRange reads the kind, the redeclaration words and the range of a node line, and
// returns the text after the range. A top-level node starts the state of the places again (the
// compiler dumps each declaration with a fresh printer).
func parseNodeRange(text string, st *locState, top bool, paths func(string) string,
) (nodeHeader, string, bool) {
	var h nodeHeader
	if top {
		*st = locState{}
	}
	kind, rest, ok := strings.Cut(text, " ")
	if !ok {
		return h, "", false
	}
	h.kind = kind
	// The address, and the words that tell where the first declaration or the parent is.
	for {
		word, tail, _ := strings.Cut(rest, " ")
		if strings.HasPrefix(word, "0x") {
			rest = tail
			continue
		}
		if word == "prev" || word == "parent" {
			h.prev = h.prev || word == "prev"
			_, tail, _ = strings.Cut(tail, " ")
			rest = tail
			continue
		}
		break
	}
	if strings.HasPrefix(rest, "<<invalid sloc>>") {
		h.invalid = true
		return h, strings.TrimPrefix(rest[len("<<invalid sloc>>"):], " "), true
	}
	if !strings.HasPrefix(rest, "<") {
		return h, "", false
	}
	var okLoc bool
	if h.begin, rest, okLoc = st.parseLoc(rest[1:], paths); !okLoc {
		return h, "", false
	}
	if strings.HasPrefix(rest, ", ") {
		if h.end, rest, okLoc = st.parseLoc(rest[2:], paths); !okLoc {
			return h, "", false
		}
	}
	if !strings.HasPrefix(rest, ">") {
		return h, "", false
	}
	return h, strings.TrimPrefix(rest[1:], " "), true
}

// parseNodeHeader reads the first line of a declaration or of a member: the range, the place of
// the name, the words, the name and the type. A node without the place of a name (an
// expression) has only its range, and rest says what follows it.
func parseNodeHeader(text string, st *locState, top bool, paths func(string) string) (nodeHeader, bool) {
	h, rest, ok := parseNodeRange(text, st, top, paths)
	if !ok {
		return h, false
	}
	h.rest = rest
	if !looksLikeLoc(rest) {
		return h, true
	}
	var okLoc bool
	if h.at, rest, okLoc = st.parseLoc(rest, paths); !okLoc {
		return h, false
	}
	rest = strings.TrimPrefix(rest, " ")
	h.rest = rest
	if !h.at.valid {
		h.invalid = true
	}
	// Words, then the name; the type is quoted.
	pre, afterType := rest, ""
	if i := strings.IndexByte(rest, '\''); i >= 0 {
		pre = rest[:i]
		h.typ, h.canon, afterType = readQuotedTypes(rest[i:])
	}
	words := strings.Fields(pre)
	if len(words) > 0 && words[len(words)-1] == "definition" {
		words = words[:len(words)-1]
		h.definition = true
	}
	// A template parameter prints "typename depth 0 index 1 Name": the name, when it has one,
	// follows the numbers, and an unnamed parameter ends with them.
	if at := slices.Index(words, "depth"); at >= 0 && len(words) >= at+4 && words[at+2] == "index" {
		_, _, depthOK := leadingInt(words[at+1])
		_, _, indexOK := leadingInt(words[at+3])
		if depthOK && indexOK {
			tail := words[at+4:]
			words = words[:at:at]
			for len(tail) > 0 && isDumpFlag(tail[0]) {
				words = append(words, tail[0]) // "..." of a parameter pack
				tail = tail[1:]
			}
			if len(tail) > 0 {
				h.name = tail[len(tail)-1]
			}
			h.pre = words
			h.post = strings.Fields(afterType)
			return h, true
		}
	}
	if len(words) > 0 && !isDumpFlag(words[len(words)-1]) {
		h.name = words[len(words)-1]
		words = words[:len(words)-1]
	}
	h.pre = words
	h.post = strings.Fields(afterType)
	return h, true
}

// looksLikeLoc reports whether s starts with a place: col:, line:, an invalid place, or the
// start of a file name.
func looksLikeLoc(s string) bool {
	switch {
	case strings.HasPrefix(s, "col:"), strings.HasPrefix(s, "line:"), strings.HasPrefix(s, "<invalid sloc>"):
		return true
	case strings.HasPrefix(s, "<"), strings.HasPrefix(s, "/"), strings.HasPrefix(s, `\`):
		return locTailRx.MatchString(s)
	case len(s) > 2 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') && isLetter(s[0]):
		return locTailRx.MatchString(s) // a drive path
	}
	return false
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// isDumpFlag are the words that the dump prints before the name and that are no name (a
// parameter without a name has only these before its type).
func isDumpFlag(word string) bool {
	switch word {
	case "referenced", "used", "invalid", "implicit", "constexpr", "cinit", "callinit",
		"listinit", "parenlistinit", "...":
		return true
	}
	return false
}

// readQuotedTypes reads 'type' or 'type':'canonical' from the start of s and returns the
// types and the text after them.
func readQuotedTypes(s string) (typ, canon, rest string) {
	typ, rest, ok := readQuoted(s)
	if !ok {
		return "", "", ""
	}
	if strings.HasPrefix(rest, ":'") {
		if c, tail, ok := readQuoted(rest[1:]); ok {
			return typ, c, tail
		}
	}
	return typ, "", rest
}

func readQuoted(s string) (string, string, bool) {
	if !strings.HasPrefix(s, "'") {
		return "", s, false
	}
	end := strings.IndexByte(s[1:], '\'')
	if end < 0 {
		return "", s, false
	}
	return s[1 : 1+end], s[2+end:], true
}

func isBaseKind(kind string) bool {
	switch kind {
	case "public", "protected", "private", "virtual":
		return true
	}
	return false
}
