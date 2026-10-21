package gocppmod

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ParseSyntax parses the text of a gocpp.mod file into its statements, with
// their comments. name is the file name in the errors and in the result.
// The result has every statement that could be read, also when there are
// errors.
func ParseSyntax(name string, data []byte) (*FileSyntax, ErrorList) {
	p := &parser{name: name, file: &FileSyntax{Name: name}}
	if len(data) > maxFileSize {
		return p.file, ErrorList{{Filename: name, Pos: Position{Line: 1, LineRune: 1},
			Err: fmt.Sprintf("file is too large (%d bytes, the limit is %d)", len(data), maxFileSize)}}
	}
	p.text = string(data)
	// The start of each line, as an int32 (the file is at most MaxFileSize),
	// so that a file of blank lines takes 4 bytes a line.
	p.offs = append(p.offs, 0)
	for i := 0; i < len(p.text); i++ {
		if p.text[i] == '\n' {
			p.offs = append(p.offs, int32(i+1))
		}
	}
	p.n = len(p.offs)
	if p.offs[p.n-1] == int32(len(p.text)) {
		// The newline that ends the last line starts no new line.
		p.n--
		p.endsNL = true
	}
	p.size = len(data)
	p.parseFile()
	return p.file, p.errs
}

// A word is a token of a line: its text as written and where it starts.
type word struct {
	text string
	off  int // byte offset in the line
}

// A lexed line is what lexLine found in a line.
type lexed struct {
	words   []word
	comment *word // the // comment, or nil
}

type parser struct {
	name string
	text string  // the file
	offs []int32 // the byte offset of each line in the file
	n    int     // the number of lines
	errs ErrorList
	file *FileSyntax
	i    int // the next line

	endsNL bool // the file ends with a newline
	size   int  // the length of the file
}

// MaxFileSize is the size of the largest gocpp.mod or vendor/modules.txt that
// is read: 16 MiB. A file may come from a dependency, and a longer one is an
// error, not a long wait.
const MaxFileSize = 16 << 20

// maxFileSize is MaxFileSize; the tests lower it.
var maxFileSize = MaxFileSize

// eofPos is the position of the end of the file, where the error of a block
// that doesn't end is (as in the go command).
func (p *parser) eofPos() Position {
	if p.endsNL || p.n == 0 {
		return Position{Line: p.n + 1, LineRune: 1, Byte: p.size}
	}
	last := p.line(p.n - 1)
	return Position{Line: p.n, LineRune: runeCol(last, len(last)), Byte: p.size}
}

func (p *parser) errAt(at Position, format string, args ...any) {
	p.errs = append(p.errs, Error{Filename: p.name, Pos: at, Err: fmt.Sprintf(format, args...)})
}

// line returns line number n, without its newline and carriage returns.
func (p *parser) line(n int) string {
	end := len(p.text)
	if n+1 < len(p.offs) {
		end = int(p.offs[n+1]) - 1
	}
	return strings.TrimRight(p.text[p.offs[n]:end], "\r")
}

func (p *parser) pos(line, off int) Position {
	return Position{Line: line + 1, LineRune: runeCol(p.line(line), off), Byte: int(p.offs[line]) + off}
}

func (p *parser) errorf(line, off int, format string, args ...any) {
	p.errs = append(p.errs, Error{Filename: p.name, Pos: p.pos(line, off), Err: fmt.Sprintf(format, args...)})
}

// lexLine splits line number n into words and a comment. It reports the
// characters that can't be in a file, and drops the line then.
//
// A word runs to the next white space. A quote (", ' or `) in a word, also
// in the middle, runs to the next quote of the same kind and keeps white
// space and // in it; a backslash keeps the next character in the word. A
// // outside quotes starts a comment. A quote without its end runs to the
// end of the line: the statement reports it when it unquotes the word.
func (p *parser) lexLine(n int) lexed {
	line := p.line(n)
	var lx lexed
	for i := 0; i < len(line); {
		c, size := utf8.DecodeRuneInString(line[i:])
		if unicode.IsSpace(c) {
			i += size
			continue
		}
		if strings.HasPrefix(line[i:], "//") {
			lx.comment = &word{text: line[i:], off: i}
			return lx
		}
		j := i
	scan:
		for j < len(line) {
			r, sz := utf8.DecodeRuneInString(line[j:])
			switch {
			case unicode.IsSpace(r), strings.HasPrefix(line[j:], "//"):
				break scan
			case strings.HasPrefix(line[j:], "/*"):
				p.errorf(n, j, "mod files must use // comments (not /* */ comments)")
				return lexed{}
			case !isIdent(r):
				p.errorf(n, j, "unexpected input character %#q", r)
				return lexed{}
			case r == '"' || r == '\'' || r == '`':
				if k := closingQuote(line, j); k >= 0 {
					j = k + 1
				} else {
					j = len(line)
				}
			case r == '\\':
				j += sz
				if j < len(line) && line[j] == '\r' {
					// A CR in the middle of a line is white space, so the backslash would have nothing to
					// keep, and Format, which writes the next word after one space, would give the
					// backslash that space (found by fuzzing: Format was not idempotent).
					p.errorf(n, j, "unexpected input character %#q", '\r')
					return lexed{}
				}
				if j < len(line) {
					_, sz = utf8.DecodeRuneInString(line[j:])
					j += sz
				}
			default:
				j += sz
			}
		}
		lx.words = append(lx.words, word{line[i:j], i})
		i = j
	}
	return lx
}

// closingQuote returns the index of the quote that closes the string that
// starts at line[i], or -1. A " or ' string has backslash escapes; a `
// string has none.
func closingQuote(line string, i int) int {
	q := line[i]
	for j := i + 1; j < len(line); j++ {
		switch {
		case line[j] == q:
			return j
		case line[j] == '\\' && q != '`':
			j++
		}
	}
	return -1
}

// isIdent reports whether c can be in a word: any printable character that
// is not white space.
func isIdent(c rune) bool {
	return !unicode.IsSpace(c) && unicode.IsPrint(c)
}

func texts(ws []word) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = w.text
	}
	return out
}

func (p *parser) comment(n int, w *word) Comment {
	return Comment{Start: p.pos(n, w.off), Token: w.text}
}

func (p *parser) parseFile() {
	var before []Comment // whole-line comments waiting for a statement
	blank := false       // a blank line since the last statement or comment block
	flush := func() {
		if len(before) == 0 {
			return
		}
		p.file.Stmt = append(p.file.Stmt, &CommentBlock{Comments: Comments{Before: before}, Start: before[0].Start, Blank: blank})
		before, blank = nil, false
	}
	for p.i < p.n {
		n := p.i
		p.i++
		lx := p.lexLine(n)
		switch {
		case len(lx.words) == 0 && lx.comment == nil:
			if strings.TrimSpace(p.line(n)) == "" {
				flush()
				blank = true
			}
			// else: a lexical error; the line is dropped
		case len(lx.words) == 0:
			before = append(before, p.comment(n, lx.comment))
		default:
			stmt := p.parseStmt(n, lx)
			if stmt == nil {
				before = nil
				continue
			}
			setBlank(stmt, blank)
			stmt.Comment().Before = before
			before, blank = nil, false
			p.file.Stmt = append(p.file.Stmt, stmt)
		}
	}
	flush()
}

func setBlank(s Stmt, blank bool) {
	switch s := s.(type) {
	case *Line:
		s.Blank = blank
	case *Block:
		s.Blank = blank
	}
}

// parseStmt makes the statement that starts in line n, with the words and
// comment lx, and reads the lines of its block.
func (p *parser) parseStmt(n int, lx lexed) Stmt {
	first, last := lx.words[0], lx.words[len(lx.words)-1]
	if first.text == ")" || first.text == "}" {
		p.errorf(n, first.off, "syntax error (unexpected %s)", first.text)
		return nil
	}
	bad := false
	if k, ok := keywordParen(first.text); ok {
		p.errorf(n, first.off, "syntax error (missing space between %s and %s)", k, first.text[len(k):])
		bad = true // the body of its block is read, and dropped with it
	}
	if bad && len(lx.words) == 1 && (strings.HasSuffix(first.text, "(") || strings.HasSuffix(first.text, "{")) {
		// require( : the body of its block is read, and dropped.
		b := &Block{Start: p.pos(n, first.off), Open: first.text[len(first.text)-1]}
		if b.Open == '{' {
			p.parseBraceBody(b)
		} else {
			p.parseParenBody(b)
		}
		return nil
	}
	if bad && last.text != "(" && last.text != "{" {
		return nil
	}
	if last.text != "(" && last.text != "{" {
		l := &Line{Start: p.pos(n, lx.words[0].off), Token: texts(lx.words)}
		if lx.comment != nil {
			l.Suffix = []Comment{p.comment(n, lx.comment)}
		}
		return l
	}
	if len(lx.words) == 1 {
		p.errorf(n, last.off, "syntax error (unexpected %s: a block needs a verb)", last.text)
		return nil
	}
	b := &Block{
		Start: p.pos(n, lx.words[0].off),
		Token: texts(lx.words[:len(lx.words)-1]),
		Open:  last.text[0],
	}
	if lx.comment != nil {
		b.Suffix = []Comment{p.comment(n, lx.comment)}
	}
	if b.Open == '{' {
		p.parseBraceBody(b)
	} else {
		p.parseParenBody(b)
	}
	if bad {
		return nil
	}
	return b
}

// keywordParen reports whether w is a keyword of gocpp.mod right before a
// "(" or "{", such as require( : a mistake that would otherwise be an unknown
// statement, silently kept.
func keywordParen(w string) (string, bool) {
	for k := range knownVerbs {
		if rest, ok := strings.CutPrefix(w, k); ok && (strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "{")) {
			return k, true
		}
	}
	return "", false
}

// parseParenBody reads the lines of a block that ( opens, up to the line
// that is only ).
func (p *parser) parseParenBody(b *Block) {
	var before []Comment
	blank := false
	for p.i < p.n {
		m := p.i
		p.i++
		bl := p.lexLine(m)
		switch {
		case len(bl.words) == 0 && bl.comment == nil:
			if strings.TrimSpace(p.line(m)) == "" {
				if len(before) > 0 {
					// A blank line after comments: keep one.
					if before[len(before)-1].Token != "" {
						before = append(before, Comment{})
					}
				} else if len(b.Line) > 0 {
					blank = true
				}
			}
		case len(bl.words) == 0:
			before = append(before, p.comment(m, bl.comment))
		case bl.words[0].text == ")":
			if len(bl.words) > 1 {
				p.errorf(m, bl.words[1].off, "syntax error (expected newline after closing paren)")
			}
			b.Close = p.pos(m, bl.words[0].off)
			b.CloseBefore = trimBlanks(before)
			if bl.comment != nil {
				b.CloseSuffix = []Comment{p.comment(m, bl.comment)}
			}
			return
		default:
			l := &Line{Start: p.pos(m, bl.words[0].off), Token: texts(bl.words), InBlock: true, Blank: blank}
			if bl.comment != nil {
				l.Suffix = []Comment{p.comment(m, bl.comment)}
			}
			l.Before = before
			before, blank = nil, false
			b.Line = append(b.Line, l)
		}
	}
	b.Unterminated = true
	b.CloseBefore = trimBlanks(before)
	p.errAt(p.eofPos(), "syntax error (unterminated block started at %s:%d:%d)", p.name, b.Start.Line, b.Start.LineRune)
}

// parseBraceBody reads the lines of a block that { opens, up to the matching
// line that is only }. The lines are kept as they are (Block.Raw), and are
// not read as words: the body of a C++ library recipe may hold anything.
// Inside, a line that ends with { or ( opens a block of its own, and a line
// that is only the matching } or ) closes it, so that the first } at depth 0
// closes the recipe. A line that starts with the closer of an open block
// and ends with { (} else {, } else if windows {) closes it and opens the
// next one; a line that starts with the closer and has other words after it
// only closes (the recipe reader reports the words). A ) with no ( open is an
// ordinary line. Quotes and // comments are skipped when the words of a line
// are looked at, so a brace in them counts for nothing.
func (p *parser) parseBraceBody(b *Block) {
	type opener struct {
		closer byte
		pos    Position
	}
	var open []opener // the blocks that are open, innermost last
	for p.i < p.n {
		m := p.i
		p.i++
		line := p.line(m)
		words, comment := scanWords(line)
		if len(words) > 0 {
			f, l := words[0], words[len(words)-1]
			closes := len(open) > 0 && (f.text == "}" || f.text == ")") &&
				open[len(open)-1].closer == f.text[0]
			if len(open) > 0 && open[len(open)-1].closer == '}' && strings.HasPrefix(f.text, "}") {
				closes = true // }else { is an error of the recipe, but it closes a block, as } else { does
			}
			switch {
			case len(words) == 1 && (f.text == "}" || f.text == ")"):
				if len(open) == 0 && f.text == "}" {
					b.Close = p.pos(m, f.off)
					if comment >= 0 {
						b.CloseSuffix = []Comment{{Start: p.pos(m, comment), Token: line[comment:]}}
					}
					return
				}
				if closes {
					open = open[:len(open)-1]
				}
			case closes:
				open = open[:len(open)-1]
				if opensBlock(line, words) {
					open = append(open, opener{'}', p.pos(m, l.off)})
				}
			case opensBlock(line, words):
				open = append(open, opener{'}', p.pos(m, l.off)})
			case l.text == "(":
				open = append(open, opener{')', p.pos(m, l.off)})
			}
		}
		b.Raw = append(b.Raw, line)
	}
	b.Unterminated = true
	if len(open) > 0 {
		last := open[len(open)-1]
		p.errAt(p.eofPos(), "syntax error (unterminated block started at %s:%d:%d; "+
			"the %s at %s:%d:%d is not closed)", p.name, b.Start.Line, b.Start.LineRune,
			openerOf(last.closer), p.name, last.pos.Line, last.pos.LineRune)
		return
	}
	p.errAt(p.eofPos(), "syntax error (unterminated block started at %s:%d:%d)", p.name, b.Start.Line, b.Start.LineRune)
}

// openerOf returns the opening bracket that a closer closes.
func openerOf(closer byte) string {
	if closer == ')' {
		return `"("`
	}
	return `"{"`
}

// scanWords splits a line into words at white space, like lexLine but with no
// errors: a quote runs to its end or the end of the line, and // outside quotes
// starts a comment. It returns the words and the offset of the comment, or -1.
func scanWords(line string) ([]word, int) {
	var ws []word
	for i := 0; i < len(line); {
		c, size := utf8.DecodeRuneInString(line[i:])
		if unicode.IsSpace(c) {
			i += size
			continue
		}
		if strings.HasPrefix(line[i:], "//") {
			return ws, i
		}
		j := i
	scan:
		for j < len(line) {
			r, sz := utf8.DecodeRuneInString(line[j:])
			switch {
			case unicode.IsSpace(r), strings.HasPrefix(line[j:], "//"):
				break scan
			case r == '"' || r == '\'' || r == '`':
				if k := closingQuote(line, j); k >= 0 {
					j = k + 1
				} else {
					j = len(line)
				}
			case r == '\\':
				j += sz
				if j < len(line) && line[j] != '\r' { // a CR is the end of a line, not an ordinary character
					_, sz = utf8.DecodeRuneInString(line[j:])
					j += sz
				}
			default:
				j += sz
			}
		}
		ws = append(ws, word{line[i:j], i})
		i = j
	}
	return ws, -1
}

// trimBlanks drops the blank-line markers at the end of the comments.
func trimBlanks(c []Comment) []Comment {
	for len(c) > 0 && c[len(c)-1].Token == "" {
		c = c[:len(c)-1]
	}
	return c
}
