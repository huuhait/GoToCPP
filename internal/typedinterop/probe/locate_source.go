package probe

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// The files that a declaration run read, as text: the compiler's columns are byte columns, the
// editor's are UTF-16 code units, and a doc comment is the text around the declaration. Only a
// file that the run itself read can be a location (the list of its dependencies decides), and a
// place is accepted only when the name stands there in the text.

const (
	maxLocateFileBytes  = 8 << 20  // a larger header gets no locations
	maxLocateTotalBytes = 64 << 20 // the text of all files of a run
	maxDocLines         = 40
	maxDocBytes         = 4 << 10
)

// normalizePath makes the compiler's spelling of a file name absolute and normalized: relative
// to dir, cleaned; a path with a ".." element is resolved through the file system first,
// because a link before the ".." makes the lexical parent a different folder. A name that
// cannot be resolved gives "".
func normalizePath(dir, name string) string {
	if name == "" {
		return ""
	}
	if !filepath.IsAbs(name) && dir != "" {
		// (not filepath.Join: it would clean the ".." lexically, before the links are looked at)
		name = dir + string(filepath.Separator) + name
	}
	if hasDotDotElement(name) {
		resolved, err := filepath.EvalSymlinks(name)
		if err != nil {
			return ""
		}
		name = resolved
	}
	if !filepath.IsAbs(name) {
		return ""
	}
	return filepath.Clean(name)
}

// foldPathCase says that the file system does not tell paths apart by case: the compiler may
// spell a file another way in a place than in the list of its dependencies.
var foldPathCase = runtime.GOOS == "windows"

// pathKey is what two spellings of the same file have in common: with fold, the case and the
// kind of separator are not told apart.
func pathKey(path string, fold bool) string {
	if !fold {
		return path
	}
	return strings.ToLower(strings.ReplaceAll(path, "/", `\`))
}

func hasDotDotElement(name string) bool {
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

// A sourceSet is the files of one run.
type sourceSet struct {
	fold    bool // case and separators do not tell paths apart (see foldPathCase)
	dir     string
	allowed map[string]string // pathKey of a normalized dependency -> that path
	files   map[string]*sourceFile
	bytes   int
}

type sourceFile struct {
	text    string
	offsets []int // the start of each line
	ok      bool
}

func newSourceSet(dir string, deps []string) *sourceSet {
	return newSourceSetFold(dir, deps, foldPathCase)
}

func newSourceSetFold(dir string, deps []string, fold bool) *sourceSet {
	set := &sourceSet{fold: fold, dir: dir, allowed: make(map[string]string),
		files: make(map[string]*sourceFile)}
	for _, dep := range deps {
		if path := normalizePath(dir, dep); path != "" {
			set.allowed[pathKey(path, fold)] = path
		}
	}
	return set
}

// resolve returns the normalized path of the spelling when it is a file of the run, or "".
func (set *sourceSet) resolve(spelled string) string {
	path := normalizePath(set.dir, spelled)
	if path == "" {
		return ""
	}
	return set.allowed[pathKey(path, set.fold)]
}

func (set *sourceSet) file(path string) *sourceFile {
	if f, seen := set.files[path]; seen {
		return f
	}
	f := &sourceFile{}
	set.files[path] = f
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxLocateFileBytes ||
		set.bytes+int(info.Size()) > maxLocateTotalBytes {
		return f
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return f
	}
	set.bytes += len(data)
	f.text = string(data)
	f.offsets = []int{0}
	for i := 0; i < len(f.text); i++ {
		if f.text[i] == '\n' {
			f.offsets = append(f.offsets, i+1)
		}
	}
	f.ok = true
	return f
}

// line returns line n (1-based) without its line ending.
func (f *sourceFile) line(n int) (string, bool) {
	if !f.ok || n < 1 || n > len(f.offsets) {
		return "", false
	}
	end := len(f.text)
	if n < len(f.offsets) {
		end = f.offsets[n] - 1
	}
	return strings.TrimSuffix(f.text[f.offsets[n-1]:end], "\r"), true
}

// utf16Units counts the UTF-16 code units of text. A byte that is not UTF-8 counts as one.
func utf16Units(text string) int {
	n := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
		i += size
	}
	return n
}

// place checks that name stands at the compiler's place and returns the editor's position:
// the 0-based line and the UTF-16 column, with the name's length in code units.
func (set *sourceSet) place(loc srcLoc, name string) (line, col, length int, ok bool) {
	if !loc.valid || loc.file == "" || name == "" || loc.col < 1 {
		return 0, 0, 0, false
	}
	text, found := set.file(loc.file).line(loc.line)
	if !found || loc.col-1 > len(text) || !strings.HasPrefix(text[loc.col-1:], name) {
		return 0, 0, 0, false
	}
	before := text[:loc.col-1]
	if loc.line == 1 && strings.HasPrefix(before, "\ufeff") {
		// The compiler counts the bytes of a byte order mark; an editor's text does not have it.
		before = before[len("\ufeff"):]
	}
	return loc.line - 1, utf16Units(before), utf16Units(name), true
}

// docOf returns the doc comment of a declaration. A comment in the style of a documentation
// tool (///, //!, /**, /*!, also after the declaration: ///<) is the declaration's own; a plain
// comment after the declaration on the line where it ends is preferred to a plain comment
// above, which in a header often names a group of declarations.
func (set *sourceSet) docOf(end, at srcLoc, comment *dumpNode) string {
	doxygen, leading := "", ""
	if comment != nil && comment.hdr.begin.valid && comment.hdr.begin.file != "" {
		leading, doxygen = set.commentText(comment.hdr.begin, comment.hdr.end)
	}
	if doxygen != "" {
		return doxygen
	}
	if trailing := set.trailingComment(end, at); trailing != "" {
		return trailing
	}
	return leading
}

// commentText reads the comment that the compiler attached, from its range (the text without
// the comment marks of the first line) to the end of the comment. It returns the text, and the
// text again when the comment is in the style of a documentation tool.
func (set *sourceSet) commentText(begin, end srcLoc) (plain, doxygen string) {
	file := set.file(begin.file)
	first, ok := file.line(begin.line)
	if !ok || begin.col < 1 || begin.col-1 > len(first) {
		return "", ""
	}
	last := begin.line
	if end.valid && end.file == begin.file && end.line > begin.line {
		// A longer comment is read up to one line more than the limit: tidyDoc cuts it there
		// and says so.
		last = min(end.line, begin.line+maxDocLines)
	}
	var lines []string
	for n := begin.line; n <= last; n++ {
		text, ok := file.line(n)
		if !ok {
			return "", ""
		}
		if n == begin.line {
			text = text[begin.col-1:]
			if gutter := first[:begin.col-1]; gutter != "" && strings.Trim(gutter, " \t*") == "" {
				// The text starts after the first star of a gutter ("** text"): it is a line of
				// the body, not the line of the opening mark.
				text = stripCommentMarks(gutter[strings.Index(gutter, "*"):] + text)
			}
			if last == n && end.valid && end.line == begin.line && end.col >= begin.col &&
				end.col <= len(first) {
				text = first[begin.col-1 : end.col] // the range ends on the last character
			}
		} else {
			text = stripCommentMarks(text)
		}
		text = strings.TrimRight(text, " \t")
		text = strings.TrimRight(strings.TrimSuffix(text, "*/"), " \t")
		lines = append(lines, text)
	}
	style := commentStyle(first[:begin.col-1])
	if !style {
		// The compiler joins neighbouring comments: a plain comment right above a documentation
		// comment is not part of it.
		for n := begin.line + 1; n <= last; n++ {
			text, _ := file.line(n)
			if startsDocComment(text) {
				lines, style = lines[n-begin.line:], true
				break
			}
		}
	}
	if style && len(lines) > 0 {
		lines[0] = strings.TrimPrefix(lines[0], "<") // ///< and //!<
	}
	body := tidyDoc(lines)
	if body == "" {
		return "", ""
	}
	if style {
		return body, body
	}
	return body, ""
}

// startsDocComment reports whether a line starts with a comment mark of a documentation tool.
func startsDocComment(line string) bool {
	line = strings.TrimLeft(line, " \t")
	for _, mark := range []string{"///", "//!", "/**", "/*!"} {
		if strings.HasPrefix(line, mark) {
			return true
		}
	}
	return false
}

// commentStyle reports whether the text before a comment's content ends with a comment mark of
// a documentation tool.
func commentStyle(before string) bool {
	for _, mark := range []string{"///", "//!", "/**", "/*!"} {
		if strings.HasSuffix(before, mark) {
			return true
		}
	}
	return false
}

// stripCommentMarks removes the marks at the start of a continuation line of a comment.
func stripCommentMarks(text string) string {
	text = strings.TrimLeft(text, " \t")
	// The gutter of "** text" (sqlite's comments) is all of its stars, but "**bold**" is text.
	if stars := len(text) - len(strings.TrimLeft(text, "*")); stars > 1 {
		rest := text[stars:]
		if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
			return rest
		}
	}
	for _, mark := range []string{"///", "//!", "//", "/**", "/*!", "/*", "*"} {
		if rest, ok := strings.CutPrefix(text, mark); ok {
			return rest
		}
	}
	return text
}

// tidyDoc removes the blank lines around the text and the one space that follows the comment
// mark, and bounds the size.
func tidyDoc(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	truncated := len(lines) > maxDocLines
	if truncated {
		lines = lines[:maxDocLines]
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimRight(strings.TrimPrefix(line, " "), " \t")
	}
	text := strings.Join(out, "\n")
	if len(text) > maxDocBytes {
		text = strings.ToValidUTF8(text[:maxDocBytes], "")
		truncated = true
	}
	if truncated {
		text += "\n..."
	}
	return strings.Clone(text)
}

// trailingComment returns the comment that follows the declaration on the line where it ends:
// "int x; // text" or "int x; /* text */". The column of an end is the start of the last token
// of the declaration; only text without quotes may stand between it and the comment mark, so
// that a "//" inside a string literal is not a comment. A declaration whose end is not in the
// file of its name (a macro expansion) uses the place of its name.
func (set *sourceSet) trailingComment(end, at srcLoc) string {
	loc := end
	if !loc.valid || loc.file == "" || loc.file != at.file {
		loc = at
	}
	if loc.file == "" || loc.col < 1 {
		return ""
	}
	text, ok := set.file(loc.file).line(loc.line)
	if !ok || loc.col > len(text) {
		return ""
	}
	tail := text[loc.col-1:]
	mark, block := strings.Index(tail, "//"), strings.Index(tail, "/*")
	switch {
	case mark < 0 && block < 0:
		return ""
	case block >= 0 && (mark < 0 || block < mark):
		if strings.ContainsAny(tail[:block], `"'`) {
			return ""
		}
		body, _, ok := strings.Cut(tail[block+2:], "*/")
		if !ok {
			return ""
		}
		body = strings.TrimPrefix(strings.TrimPrefix(body, "!"), "*")
		return tidyDoc([]string{strings.TrimPrefix(body, "<")})
	}
	if strings.ContainsAny(tail[:mark], `"'`) {
		return ""
	}
	body := strings.TrimPrefix(strings.TrimPrefix(tail[mark+2:], "/"), "!")
	return tidyDoc([]string{strings.TrimPrefix(body, "<")})
}
