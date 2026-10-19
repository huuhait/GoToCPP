// Package explain is the reading side of the generated C++ (Phase 13 step 13.5): the annotated
// view that gocpp emit -annotate prints, and gocpp explain, which says what a Go++ construct
// lowers to and what a compiler message means.
//
// The annotated view is built from two things that the emitter already makes: the C++ with its
// #line directives (which say which .gp line each C++ line comes from), and the reason log of
// the lowering (lower.Why: why a variable is boxed, which conversion allocates, where the
// runtime checks). View joins them: each run of C++ lines that comes from one .gp line gets a
// header with that line's text and the reasons for it. The view is for reading; it is not
// C++ that compiles (the #line directives are gone), and the normal emit does not change.
package explain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/lower"
)

// GenRoot is the folder that the emitter is told to write the generated files in when the view
// is built (cppgen.EmitProgram's root). It makes the lines that have no .gp place (prototypes,
// the main wrapper) name their own file in a #line directive, so the view can tell them from the
// lines of the last .gp line before them. Nothing is written there.
const GenRoot = "/gocpp-annotate"

// stdGenRoot is what the #line directives of the generated lines of a standard package name
// instead of GenRoot (cppgen.EmitProgram), and stdPrefix starts the paths of its .gp files.
const (
	stdGenRoot = "$GOCPP/gen"
	stdPrefix  = "$GOCPP/"
)

// A File is one generated file, as cppgen.EmitProgram makes it.
type File struct {
	Path string // relative to the folder of the generated files: gpkg/<import path>/x.gp.cpp
	Data []byte
	Std  bool // a file of a package of the standard library
}

// Options are the inputs of Build that are not the C++ and the reasons.
type Options struct {
	// Source returns the text of the .gp file path (as the #line directives name it), or nil.
	// Without it, the headers of the view have no source text.
	Source func(path string) []byte

	// Kinds, when it is not empty, keeps only the reasons of these kinds.
	Kinds map[lower.Kind]bool

	// Std keeps the files of the standard library's packages and the reasons for their lines.
	// Without it the view is of the program's own packages.
	Std bool
}

// A Pos is a place in a .gp file.
type Pos struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// A Reason is a lower.Reason for the view.
type Reason struct {
	Line int    `json:"line,omitempty"` // when the reason is about another line than the chunk's: a continuation line of its statement
	Col  int    `json:"col"`
	Kind string `json:"kind"`
	Msg  string `json:"msg"`
	Inst string `json:"instance,omitempty"` // for an unplaced reason: the instance it was made for
}

// A Chunk is a run of generated lines that come from one .gp line (Src), or have no .gp line
// (the zero Src).
type Chunk struct {
	Src     Pos      `json:"-"`
	Inst    string   `json:"instance,omitempty"` // the generic function's instance whose body this is (gp_inst_<hash>_F)
	GP      *GPLine  `json:"gp,omitempty"`
	Reasons []Reason `json:"reasons,omitempty"`
	Cont    bool     `json:"continued,omitempty"` // the .gp line was shown by an earlier chunk
	Cpp     []string `json:"cpp"`
}

// A GPLine is the .gp place of a Chunk, with the text of the line when the source is known.
type GPLine struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text,omitempty"`
}

// A FileView is one generated file as chunks.
type FileView struct {
	Path   string  `json:"path"`
	Chunks []Chunk `json:"chunks"`
}

// A UnplacedReason is a reason whose .gp line has no C++ in any file.
type UnplacedReason struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Reason
}

// A View is the annotated output of a program.
type View struct {
	Files    []FileView       `json:"files"`
	Unplaced []UnplacedReason `json:"unplaced,omitempty"`

	reasons map[rkey][]Reason // by .gp line and instance, for Lines
}

// An rkey is the place of a reason: the instances of a generic function share the .gp lines of
// its body, and each has its own reasons for them.
type rkey struct {
	Pos
	Inst string
}

// Build joins the generated files with the reasons into a View. The files must have been made
// with the root GenRoot. The order of everything follows the files and the reasons, so the view
// is the same on every run.
func Build(files []File, reasons []lower.Reason, opt Options) *View {
	src := &sources{read: opt.Source, lines: make(map[string][]string)}
	v := &View{reasons: make(map[rkey][]Reason)}

	// The chunks first, without reasons.
	var fvs []FileView
	placed := make(map[rkey]bool)        // the (line, instance) places that have a chunk
	hostLines := make(map[hostKey][]int) // the .gp lines of the chunks of a file and instance, sorted
	for _, f := range files {
		if f.Std && !opt.Std {
			continue
		}
		fv := FileView{Path: f.Path}
		for _, c := range parse(f.Data) {
			// The blank lines between chunks are the view's own: one blank line separates two chunks.
			for len(c.lines) > 0 && strings.TrimSpace(c.lines[len(c.lines)-1]) == "" {
				c.lines = c.lines[:len(c.lines)-1]
			}
			if len(c.lines) == 0 {
				continue
			}
			ch := Chunk{Src: c.src, Inst: c.inst, Cpp: c.lines}
			if c.src != (Pos{}) {
				ch.GP = &GPLine{File: c.src.File, Line: c.src.Line, Text: src.line(c.src)}
				k := rkey{c.src, c.inst}
				if !placed[k] {
					placed[k] = true
					hk := hostKey{c.src.File, c.inst}
					hostLines[hk] = append(hostLines[hk], c.src.Line)
				}
			}
			fv.Chunks = append(fv.Chunks, ch)
		}
		fvs = append(fvs, fv)
	}
	for _, ls := range hostLines {
		sort.Ints(ls)
	}

	// The reasons, each on the chunk of its line. A reason on a continuation line of a statement
	// (an argument on the third line of a call) goes to the chunk of the line that starts the
	// statement; it keeps its own line number.
	home := func(r lower.Reason) rkey {
		k := rkey{Pos{r.File, r.Line}, r.Inst}
		// The lowering says which statement a reason is in (Reason.StmtLine): the reason goes to
		// the chunk where that statement starts, also if its own line has a chunk (the lone "}" that
		// closes a block in the middle of the statement).
		if r.StmtLine != 0 {
			if sk := (rkey{Pos{r.File, r.StmtLine}, r.Inst}); placed[sk] {
				return sk
			}
		}
		if placed[k] {
			return k
		}
		// A reason about a declaration, on a line with no C++: the nearest chunk before it, if
		// that line leaves a bracket open. Never a chunk further back, which could be the header
		// of the block that the line is in.
		ls := hostLines[hostKey{r.File, r.Inst}]
		if i := sort.SearchInts(ls, r.Line); i > 0 && src.continues(r.File, ls[i-1], r.Line) {
			return rkey{Pos{r.File, ls[i-1]}, r.Inst}
		}
		return k
	}
	for _, r := range reasons {
		if len(opt.Kinds) > 0 && !opt.Kinds[r.Kind] || !opt.Std && strings.HasPrefix(r.File, stdPrefix) {
			continue
		}
		k := home(r)
		vr := Reason{Col: r.Col, Kind: string(r.Kind), Msg: r.Msg}
		if k.Line != r.Line {
			vr.Line = r.Line
		}
		v.reasons[k] = append(v.reasons[k], vr)
	}
	shown := make(map[rkey]bool)
	for _, fv := range fvs {
		for i := range fv.Chunks {
			ch := &fv.Chunks[i]
			if ch.GP == nil {
				continue
			}
			k := rkey{ch.Src, ch.Inst}
			if shown[k] {
				ch.Cont = true
			} else {
				shown[k] = true
				ch.Reasons = v.reasons[k]
			}
		}
		v.Files = append(v.Files, fv)
	}
	// The reasons of lines that no chunk shows, in the order of the reasons.
	for _, r := range reasons {
		if len(opt.Kinds) > 0 && !opt.Kinds[r.Kind] || !opt.Std && strings.HasPrefix(r.File, stdPrefix) || shown[home(r)] {
			continue
		}
		v.Unplaced = append(v.Unplaced, UnplacedReason{File: r.File, Line: r.Line, Reason: Reason{Col: r.Col, Kind: string(r.Kind), Msg: r.Msg, Inst: r.Inst}})
	}
	return v
}

// A hostKey is a .gp file and an instance: the chunks that a reason can be moved to.
type hostKey struct {
	file, inst string
}

// continues reports whether the .gp line to is inside a statement or a block that the line from
// opens: at the start of line to, a bracket opened on the lines from..to-1 is still open. The
// text is read the way Go's scanner would, so brackets in strings, runes and comments don't count.
func (s *sources) continues(file string, from, to int) bool {
	if s.read == nil || from >= to {
		return false
	}
	s.line(Pos{file, 1}) // loads the file
	ls := s.lines[file]
	if to > len(ls) {
		return false
	}
	depth := 0
	raw, block := false, false
	for n := from; n < to; n++ {
		line := ls[n-1]
		for i := 0; i < len(line); i++ {
			c := line[i]
			switch {
			case block:
				if c == '*' && i+1 < len(line) && line[i+1] == '/' {
					block = false
					i++
				}
			case raw:
				if c == '`' {
					raw = false
				}
			case c == '/' && i+1 < len(line) && line[i+1] == '*':
				block = true
				i++
			case c == '`':
				raw = true
			case c == '"' || c == '\'':
				for i++; i < len(line) && line[i] != c; i++ {
					if line[i] == '\\' {
						i++
					}
				}
			case c == '/' && i+1 < len(line) && line[i+1] == '/':
				i = len(line)
			case c == '(' || c == '[' || c == '{':
				depth++
			case c == ')' || c == ']' || c == '}':
				depth--
			}
		}
	}
	return depth > 0
}

// sources reads .gp files once and splits them into lines.
type sources struct {
	read  func(string) []byte
	lines map[string][]string
}

// line returns the text of the line p, without its indentation, or "".
func (s *sources) line(p Pos) string {
	if s.read == nil {
		return ""
	}
	ls, ok := s.lines[p.File]
	if !ok {
		if data := s.read(p.File); data != nil {
			ls = strings.Split(string(data), "\n")
		}
		s.lines[p.File] = ls
	}
	if p.Line < 1 || p.Line > len(ls) {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(ls[p.Line-1], "\r"))
}

// A rawChunk is parse's chunk.
type rawChunk struct {
	src   Pos
	inst  string // the instance whose function this chunk is in, or ""
	lines []string
}

// instDefRx finds the definition of an instance of a generic function: a line at the left margin
// that names gp_inst_<hash>_F( and opens the body.
var instDefRx = regexp.MustCompile(`^[^\s/#].*\b(gp_inst_[0-9a-f]+_\w+)\(.*\{$`)

// parse splits the C++ of one generated file into chunks by its #line directives. C++ counts
// the lines on from the last directive, so each line has the .gp line of the directive plus the
// lines since. A directive that names a file under GenRoot, and the lines before the first
// directive, are the generated file itself: no .gp line. A blank line never starts a chunk.
func parse(data []byte) []rawChunk {
	var out []rawChunk
	var cur Pos     // the place of the next line; the zero Pos: generated
	var inst string // the instance whose body the next line is in
	text := string(data)
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	for _, line := range strings.Split(text, "\n") {
		if file, n, ok := lineDirective(line); ok {
			if g := filepath.ToSlash(file); g == GenRoot || strings.HasPrefix(g, GenRoot+"/") || g == stdGenRoot || strings.HasPrefix(g, stdGenRoot+"/") {
				cur = Pos{}
			} else {
				cur = Pos{file, n}
			}
			continue
		}
		here := cur
		if cur != (Pos{}) {
			cur.Line++
		}
		if m := instDefRx.FindStringSubmatch(line); m != nil && !strings.HasSuffix(line, ";") {
			inst = m[1]
		}
		if len(out) == 0 || ((here != out[len(out)-1].src || inst != out[len(out)-1].inst) && strings.TrimSpace(line) != "") {
			out = append(out, rawChunk{src: here, inst: inst})
		}
		c := &out[len(out)-1]
		c.lines = append(c.lines, line)
		if line == "}" {
			inst = "" // the end of the function: the next chunk is not in the instance
		}
	}
	return out
}

// lineDirective parses `#line N "file"`.
func lineDirective(line string) (file string, n int, ok bool) {
	rest, ok := strings.CutPrefix(line, "#line ")
	if !ok {
		return "", 0, false
	}
	num, quoted, ok := strings.Cut(rest, " ")
	if !ok {
		return "", 0, false
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return "", 0, false
	}
	file, ok = unquoteC(quoted)
	return file, n, ok
}

// unquoteC reverses cppir.CString: the escapes of a C++ string literal, with named escapes and
// 3-digit octal bytes.
func unquoteC(s string) (string, bool) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", false
	}
	s = s[1 : len(s)-1]
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			b = append(b, c)
			continue
		}
		i++
		if i >= len(s) {
			return "", false
		}
		switch c = s[i]; {
		case c >= '0' && c <= '7':
			if i+2 >= len(s) {
				return "", false
			}
			v, err := strconv.ParseUint(s[i:i+3], 8, 8)
			if err != nil {
				return "", false
			}
			b = append(b, byte(v))
			i += 2
		default:
			named := map[byte]byte{'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v'}
			if r, ok := named[c]; ok {
				b = append(b, r)
			} else {
				b = append(b, c) // \" \\ \?
			}
		}
	}
	return string(b), true
}

// WriteText writes the view as text: for each generated file the line `//// path`, as gocpp emit
// writes it, then its chunks. rel shortens a .gp path for display (it may be nil).
func (v *View) WriteText(w io.Writer, rel func(string) string) error {
	if rel == nil {
		rel = func(s string) string { return s }
	}
	var b bytes.Buffer
	b.WriteString("// gocpp emit -annotate: the C++ with the .gp lines it comes from and the reasons for the\n" +
		"// lowering's choices. For reading, not for compiling. gocpp explain KIND explains a gp[KIND] note.\n")
	for _, f := range v.Files {
		fmt.Fprintf(&b, "//// %s\n", f.Path)
		for i, c := range f.Chunks {
			if c.GP != nil {
				header := fmt.Sprintf("// ---- %s:%d", rel(c.GP.File), c.GP.Line)
				if c.GP.Text != "" {
					header += ": " + truncate(c.GP.Text, 80)
				}
				if c.Inst != "" {
					header += "  [in " + c.Inst + "]"
				}
				if c.Cont {
					header += "  (continued)"
				}
				if i > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(header + "\n")
				for _, r := range c.Reasons {
					writeReason(&b, r)
				}
			} else {
				if i > 0 && len(c.Cpp) > 0 {
					b.WriteByte('\n')
				}
				for _, r := range c.Reasons { // WriteChunks of an unplaced reason
					writeReason(&b, r)
				}
			}
			for _, l := range c.Cpp {
				b.WriteString(l)
				b.WriteByte('\n')
			}
		}
	}
	if len(v.Unplaced) > 0 {
		b.WriteString("//// reasons for .gp lines that have no C++ of their own\n")
		for _, u := range v.Unplaced {
			fmt.Fprintf(&b, "// %s:%d\n", rel(u.File), u.Line)
			writeReason(&b, u.Reason)
		}
	}
	_, err := w.Write(b.Bytes())
	return err
}

// WriteChunks writes the chunks cs the way WriteText does, for gocpp explain file.gp:LINE.
func WriteChunks(w io.Writer, cs []Chunk, rel func(string) string) error {
	v := &View{Files: []FileView{{Chunks: cs}}}
	var b bytes.Buffer
	if err := v.WriteText(&b, rel); err != nil {
		return err
	}
	// Drop the two lines of the banner and the empty path line.
	text := b.String()
	for range 3 {
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
	}
	_, err := io.WriteString(w, text)
	return err
}

// writeReason writes one reason as comment lines, wrapped.
func writeReason(b *bytes.Buffer, r Reason) {
	prefix := fmt.Sprintf("// gp[%s]", r.Kind)
	if r.Line > 0 {
		prefix += fmt.Sprintf(" line %d", r.Line)
	}
	if r.Col > 0 {
		prefix += fmt.Sprintf(" col %d", r.Col)
	}
	prefix += ": "
	const width = 100
	indent := "//     "
	line := prefix
	first := true
	for _, word := range strings.Fields(r.Msg) {
		if !first && utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) > width {
			b.WriteString(line + "\n")
			line = indent + word
			continue
		}
		if first {
			line += word
			first = false
		} else {
			line += " " + word
		}
	}
	b.WriteString(line + "\n")
}

// truncate shortens s to at most n runes, with an ellipsis.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	rs := []rune(s)
	return string(rs[:n-1]) + "…"
}

// WriteJSON writes the view as indented JSON.
func (v *View) WriteJSON(w io.Writer) error {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false) // the C++ and the messages have < and >: no \u003c
	e.SetIndent("", "  ")
	return e.Encode(v)
}

// Lines returns the chunks of the view that come from the .gp lines first..last of file, in
// the order of the files: what gocpp explain file.gp:LINE shows.
func (v *View) Lines(file string, first, last int) []Chunk {
	var out []Chunk
	shown := make(map[rkey]bool)
	for _, f := range v.Files {
		for _, c := range f.Chunks {
			if c.Src.File == file && (c.Src.Line >= first && c.Src.Line <= last || v.hasReasonIn(rkey{c.Src, c.Inst}, first, last)) {
				// The first chunk of a line carries its reasons, also if an earlier chunk of
				// another range showed it in the whole view.
				k := rkey{c.Src, c.Inst}
				c.Cont = shown[k]
				c.Reasons = nil
				if !c.Cont {
					c.Reasons = v.reasons[k]
					shown[k] = true
				}
				out = append(out, c)
			}
		}
	}
	return out
}

// hasReasonIn reports whether the chunk place k has a reason about a continuation line in
// first..last.
func (v *View) hasReasonIn(k rkey, first, last int) bool {
	for _, r := range v.reasons[k] {
		if r.Line >= first && r.Line <= last {
			return true
		}
	}
	return false
}

// UnplacedLines returns the unplaced reasons for the lines first..last of file.
func (v *View) UnplacedLines(file string, first, last int) []UnplacedReason {
	var out []UnplacedReason
	for _, u := range v.Unplaced {
		if u.File == file && u.Line >= first && u.Line <= last {
			out = append(out, u)
		}
	}
	return out
}

// Kinds returns the kinds of reasons in the view, sorted: which mechanisms the program uses.
func (v *View) Kinds() []string {
	seen := make(map[string]bool)
	for _, f := range v.Files {
		for _, c := range f.Chunks {
			for _, r := range c.Reasons {
				seen[r.Kind] = true
			}
		}
	}
	for _, u := range v.Unplaced {
		seen[u.Kind] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
