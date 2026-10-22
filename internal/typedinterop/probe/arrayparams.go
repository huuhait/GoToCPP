package probe

import (
	"cmp"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// Parameters that are written as arrays (ADR-130, "arrays").
//
// A parameter written `float col[3]` has the type `float *` in C and C++, and every dump of the
// compiler says so: the declaration cannot be told from `float *col` by its type. But the callee
// reads or writes several elements, and the address of one Go variable is the address of one. So
// the lowering must know how a parameter was written, and the only place that says is the source:
// the places (offsets) in the dump, and the text of the file at them. A pointer parameter ends at
// its name (or, with a default argument, at the default, after "= ") and has a `*` before the name;
// an array parameter has the `[3]` after its name, or is a typedef of an array (`typedef float
// vec3[3]; void scale(vec3 v)`: the compiler turns it into `float *` and keeps no trace of the
// typedef), which is a pointer parameter without a `*` in its text. A typedef of a pointer
// (`LPDWORD p`) keeps its name in the type, and that tells it from the typedef of an array.

// A paramPlace is where the pieces of one parameter are in the source, in one scale (a file offset
// in the JSON dump, line and column together in the text dump).
type paramPlace struct {
	named   bool
	at      int // the name; for an unnamed parameter the place the compiler gives it
	nameLen int
	end     int // the last token of the parameter, the end of the default argument if there is one

	hasDefault   bool
	defaultKnown bool // defaultAt is the start of the default argument
	defaultAt    int

	typeText    string // the source between the start of the parameter and its name
	typeKnown   bool   // typeText was read from the file
	pointerName bool   // the type is the name of a typedef of a pointer (LPDWORD)
	notPointer  bool   // the type is not a pointer: no array
}

// array reports whether the parameter is written as an array. It is conservative: whatever it
// cannot tell (a default argument it cannot place, spaces it does not expect, a text it could not
// read) is an array.
func (p paramPlace) array() bool {
	if p.notPointer {
		return false // float s: an array parameter has the type of a pointer
	}
	if p.pointerName {
		return false // LPDWORD p: a pointer, with a default argument or without a name too
	}
	if p.suffixArray() {
		return true
	}
	return !p.typeKnown || !strings.Contains(withoutComments(p.typeText), "*")
}

// suffixArray reports whether the places say that something follows the name of the parameter
// that is not a default argument of a pointer: `[3]`.
func (p paramPlace) suffixArray() bool {
	switch {
	case !p.named:
		// The compiler puts an unnamed parameter's place at its declarator suffix, or after it.
		return p.hasDefault || p.end >= p.at
	case !p.hasDefault:
		return p.end != p.at // (a macro's places are the spelling's: they are not in this order)
	case !p.defaultKnown:
		return true
	}
	gap := p.defaultAt - (p.at + p.nameLen)
	return gap < 1 || gap > 3 // `p = nullptr`, `p=0`: a pointer; `v[2] = 0` has more between
}

// place reads the place of a ParmVarDecl of the JSON dump: ok is false when a place is a macro's
// or is missing.
func (n *dnode) place() (paramPlace, bool) {
	if n.Loc == nil || n.Loc.Offset == nil || n.Range == nil || n.Range.End.Offset == nil {
		return paramPlace{}, false
	}
	p := paramPlace{
		named:      n.Name != "",
		at:         *n.Loc.Offset,
		nameLen:    n.Loc.TokLen,
		end:        *n.Range.End.Offset,
		hasDefault: n.Init != "",
		typeText:   n.typeText,
		typeKnown:  n.typeKnown,
	}
	if n.Type != nil {
		p.pointerName = pointerTypedef(n.Type.QualType, n.Type.DesugaredQualType)
		p.notPointer = !strings.Contains(cmp.Or(n.Type.DesugaredQualType, n.Type.QualType), "*")
	}
	if p.hasDefault {
		for i := range n.Inner {
			if r := n.Inner[i].Range; r != nil && r.Begin.Offset != nil {
				p.defaultAt = *r.Begin.Offset
				p.defaultKnown = true
				break
			}
		}
	}
	return p, true
}

// spelledAsArray reports whether the ParmVarDecl is written as an array, or it is not known.
func (n *dnode) spelledAsArray() bool {
	if n.Type != nil && !strings.Contains(cmp.Or(n.Type.DesugaredQualType, n.Type.QualType), "*") {
		return false // float s: an array parameter has the type of a pointer
	}
	p, ok := n.place()
	return !ok || p.array()
}

// pointerTypedef reports whether the type of a parameter, as the compiler prints it with its
// typedef names, is the name of a typedef of a pointer (`LPDWORD`, `const IPtr`) and not a pointer
// type (`int *`, `I *`): a parameter written with the typedef of an array has no typedef name left,
// because the compiler decays it.
func pointerTypedef(written, canonical string) bool {
	return canonical != "" && endsWithStar(canonical) && !endsWithStar(written)
}

// endsWithStar reports whether the type text ends with a `*`, with or without qualifiers after it.
func endsWithStar(typ string) bool {
	typ = strings.TrimSpace(typ)
	for {
		before := typ
		for _, qualifier := range []string{"const", "volatile", "restrict", "__restrict"} {
			rest, found := strings.CutSuffix(typ, qualifier)
			if found && (rest == "" || !isIdentByte(rest[len(rest)-1])) {
				typ = strings.TrimSpace(rest)
			}
		}
		if typ == before {
			break
		}
	}
	return strings.HasSuffix(typ, "*")
}

// withoutComments removes the comments of a piece of source (a `*` in a comment is not a pointer).
func withoutComments(text string) string {
	for {
		start := strings.Index(text, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(text[start+2:], "*/")
		if end < 0 {
			text = text[:start]
			break
		}
		text = text[:start] + " " + text[start+2+end+2:]
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if cut, _, found := strings.Cut(line, "//"); found {
			line = cut
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------- the text of the source

// sourceFiles reads the text of the files that a dump names: the files of the run only (its
// dependencies), as the compiler spelled them.
type sourceFiles struct {
	set *sourceSet
}

func newSourceFiles(dir string, deps []string) *sourceFiles {
	return &sourceFiles{set: newSourceSet(dir, deps)}
}

// text returns the text of the file, when it is a file of the run that could be read.
func (s *sourceFiles) text(name string) (*sourceFile, bool) {
	path := s.set.resolve(name)
	if path == "" {
		return nil, false
	}
	f := s.set.file(path)
	return f, f.ok
}

// slice returns the text from offset from to offset to of the file.
func (s *sourceFiles) slice(name string, from, to int) (string, bool) {
	f, ok := s.text(name)
	if !ok || from < 0 || to < from || to > len(f.text) {
		return "", false
	}
	return f.text[from:to], true
}

// offset returns the offset of a line and column (both 1-based, the column in bytes) of the file.
func (s *sourceFiles) offset(name string, line, col int) (int, bool) {
	f, ok := s.text(name)
	if !ok || line < 1 || line > len(f.offsets) || col < 1 {
		return 0, false
	}
	return f.offsets[line-1] + col - 1, true
}

// ---------------------------------------------------------------- methods (the JSON dump)

// annotateParams gives every ParmVarDecl under n the text of its type as it is written. The
// compiler leaves the file out of a place when the place before it (in the order the dump is
// written) is in the same file, so the files are followed through the whole declaration.
func (n *dnode) annotateParams(src *sourceFiles) {
	last := ""
	n.followFiles(&last, src)
}

func (n *dnode) followFiles(last *string, src *sourceFiles) {
	n.Loc.follow(last)
	if n.Range != nil {
		n.Range.Begin.follow(last)
		n.Range.End.follow(last)
	}
	if n.Kind == "ParmVarDecl" {
		n.readTypeText(src)
	}
	for i := range n.Inner {
		n.Inner[i].followFiles(last, src)
	}
}

// follow records the file of the place in l.file, and makes it the file of the places after it.
func (l *dloc) follow(last *string) {
	if l == nil {
		return
	}
	if l.SpellingLoc != nil || l.ExpansionLoc != nil {
		l.SpellingLoc.follow(last)
		l.ExpansionLoc.follow(last)
		return
	}
	if l.File != "" {
		*last = l.File
	}
	l.inFile = *last
}

// readTypeText reads the source from the start of the parameter to its name; for an unnamed
// parameter to the end of its last token (a `*` is one character).
func (n *dnode) readTypeText(src *sourceFiles) {
	if n.Loc == nil || n.Range == nil || n.Loc.Offset == nil || n.Range.Begin.Offset == nil ||
		n.Range.End.Offset == nil {
		return
	}
	file := n.Range.Begin.inFile
	if file == "" || n.Loc.inFile != file {
		return
	}
	to := *n.Loc.Offset
	if n.Name == "" {
		to = *n.Range.End.Offset + 1
	}
	if text, ok := src.slice(file, *n.Range.Begin.Offset, to); ok {
		n.typeText = text
		n.typeKnown = true
	}
}

// A function in a namespace has no ParmVarDecl in the dump of a using declaration, so the places
// come from the compiler's own lookup: `#pragma clang __debug dump NAME` prints the declarations
// that the name means (locate.go), and the probe run of the format strings reads them from its
// messages in the same run.

// lineScale puts a line and a column in one number.
const lineScale = 1 << 20

func textPos(l srcLoc) int { return l.line*lineScale + l.col }

// arraySpellings returns the parameters of the FunctionDecls of one lookup that are written as
// arrays, by the function type as the compiler prints it. A function declared more than once gets
// the flags of all its declarations together (a parameter is an array if any declaration says so).
func arraySpellings(group dumpGroup, src *sourceFiles) map[string][]bool {
	out := make(map[string][]bool)
	nodes := group.nodes
	for i := 0; i < len(nodes); i++ {
		top := nodes[i]
		if top.depth != 0 || top.hdr.kind != "FunctionDecl" || top.hdr.invalid {
			continue
		}
		key := normalizeType(top.hdr.typ)
		var flags []bool
		bad := slices.Contains(top.hdr.pre, "invalid")
		for j := i + 1; j < len(nodes) && nodes[j].depth > 0; j++ {
			n := nodes[j]
			if n.depth != 1 || n.hdr.kind != "ParmVarDecl" {
				continue
			}
			flags = append(flags, textParamArray(nodes, j, src))
			bad = bad || slices.Contains(n.hdr.pre, "invalid")
		}
		if bad {
			continue
		}
		if old, ok := out[key]; ok {
			if len(old) != len(flags) {
				out[key] = nil
				continue
			}
			for k := range old {
				flags[k] = flags[k] || old[k]
			}
		}
		out[key] = flags
	}
	return out
}

// textParamArray decides the ParmVarDecl at nodes[j] of a text dump.
func textParamArray(nodes []dumpNode, j int, src *sourceFiles) bool {
	h := nodes[j].hdr
	if !h.end.valid {
		h.end = h.begin // a range of one token prints one place: <col:25>
	}
	if !h.begin.valid || !h.end.valid || !h.at.valid || h.begin.file != h.at.file {
		return true
	}
	p := paramPlace{named: h.name != "", at: textPos(h.at), nameLen: len(h.name), end: textPos(h.end),
		hasDefault: slices.Contains(h.post, "cinit") || slices.Contains(h.post, "callinit") ||
			slices.Contains(h.post, "listinit")}
	p.pointerName = pointerTypedef(h.typ, h.canon)
	p.notPointer = h.typ != "" && !strings.Contains(cmp.Or(h.canon, h.typ), "*")
	if from, ok := src.offset(h.begin.file, h.begin.line, h.begin.col); ok {
		to, ok := src.offset(h.at.file, h.at.line, h.at.col)
		if !p.named {
			to, ok = src.offset(h.end.file, h.end.line, h.end.col+1)
		}
		if ok {
			p.typeText, p.typeKnown = src.slice(h.begin.file, from, to)
		}
	}
	if p.hasDefault {
		for k := j + 1; k < len(nodes) && nodes[k].depth > 1; k++ {
			if d := nodes[k].hdr; d.begin.valid {
				p.defaultAt = textPos(d.begin)
				p.defaultKnown = true
				break
			}
		}
	}
	return p.array()
}

// lookupSnippet is the text that makes the compiler print the declarations of NAME in the scope.
func lookupSnippet(scope, name string) string {
	if scope == "" {
		return "#pragma clang __debug dump " + name
	}
	parts := strings.Split(scope, "::")
	open := strings.Join(parts, " { namespace ")
	return "namespace " + open + " {\n#pragma clang __debug dump " + name + "\n" +
		strings.Repeat("}", len(parts))
}

// A lookupSite is a function whose declarations the compiler is asked to print: the query it came
// from and its name.
type lookupSite struct {
	set  int
	name string
}

// readArraySpellings sets ArrayParams of the overloads from the declarations that the compiler
// printed in the run of the function sets (funcRun), one group of lines for each lookup, in the
// order of the lookups. A run whose messages were cut short gives nothing.
func readArraySpellings(d *dump, dir string, lookups []lookupSite, sets []*FuncSet) {
	if len(lookups) == 0 || d == nil || d.stats.StderrTruncated {
		return
	}
	reader := newDumpReader(maxLocateDump, func(name string) string { return name }, nil)
	reader.Write(d.stderr)
	reader.finish()
	if reader.over {
		return
	}
	src := newSourceFiles(dir, d.deps)
	next := 0
	for _, lk := range lookups {
		for next < len(reader.groups) && reader.groups[next].name != lk.name {
			next++
		}
		if next == len(reader.groups) {
			return
		}
		spelled := arraySpellings(reader.groups[next], src)
		next++
		if sets[lk.set] == nil {
			continue
		}
		for oi := range sets[lk.set].Overloads {
			ov := &sets[lk.set].Overloads[oi]
			if ov.Kind != "function" || ov.Sig.T == nil || ov.Sig.T.Kind != ctype.Func {
				continue
			}
			flags, ok := spelled[normalizeType(ov.Sig.Text)]
			if ok && len(flags) == len(ov.Sig.T.Params) {
				ov.ArrayParams = flags
			}
		}
	}
}
