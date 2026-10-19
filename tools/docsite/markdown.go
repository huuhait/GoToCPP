package main

// A small Markdown renderer for the docs of this repository: the part of CommonMark and GitHub
// Markdown that the docs use (headings, paragraphs, fenced code, block quotes, nested lists,
// tables, code spans, emphasis, links, images, autolinks and a few HTML tags), with no
// dependency. It is not a general implementation: a construct it does not know is rendered as
// plain text, never dropped silently, and TestRenderRealDocs renders every doc of the
// repository to find the constructs that it should know.

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A line is one line of a document with its number (counting from 1) in the file.
type line struct {
	text string
	no   int
}

// A heading is a heading of a page: it gets an anchor.
type heading struct {
	Level int
	ID    string
	Text  string // plain text
}

// A renderer renders one page. LinkFunc rewrites the destination of a link (it also records the
// link); it is called once for each link and image, in document order.
type renderer struct {
	linkFunc func(href string, line int) string
	refs     map[string]string // link reference definitions, keys lower-cased
	headings []heading
	slugs    map[string]int
	used     map[string]bool // the ids given so far
	problems []string        // constructs this renderer cannot render faithfully: errors of docsite -check
	curLine  int
	title    string // the first level-1 heading
}

// renderMarkdown renders the Markdown source to HTML.
func renderMarkdown(src string, linkFunc func(href string, line int) string) (out string, r *renderer) {
	r = &renderer{linkFunc: linkFunc, refs: map[string]string{}, slugs: map[string]int{}, used: map[string]bool{}}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	var lines []line
	for i, l := range strings.Split(src, "\n") {
		lines = append(lines, line{l, i + 1})
	}
	lines = r.collectRefs(lines)
	return r.blocks(lines, false), r
}

var refDefRx = regexp.MustCompile(`^ {0,3}\[([^\]\n]+)\]:\s*(\S+)(?:\s+(?:"[^"]*"|'[^']*'|\([^)]*\)))?\s*$`)

// collectRefs removes the link reference definitions ([name]: url) outside code, and records them.
func (r *renderer) collectRefs(lines []line) []line {
	var out []line
	fence := ""
	prevDef := false
	for i, l := range lines {
		if f := fenceOf(l.text); f != "" {
			switch {
			case fence == "":
				fence = f
			case strings.HasPrefix(f, fence[:1]) && len(f) >= len(fence) && strings.TrimSpace(l.text) == f:
				fence = ""
			}
		}
		if fence == "" && (i == 0 || prevDef || strings.TrimSpace(lines[i-1].text) == "") {
			if m := refDefRx.FindStringSubmatch(l.text); m != nil {
				r.refs[strings.ToLower(strings.Join(strings.Fields(m[1]), " "))] = strings.TrimSuffix(strings.TrimPrefix(m[2], "<"), ">")
				prevDef = true
				continue
			}
		}
		prevDef = false
		out = append(out, l)
	}
	return out
}

// fenceOf returns the fence (``` or ~~~, with its length) that opens or closes a fenced code
// block on the line, or "".
func fenceOf(s string) string {
	t := strings.TrimLeft(s, " ")
	if len(s)-len(t) > 3 {
		return ""
	}
	for _, c := range []string{"`", "~"} {
		n := 0
		for n < len(t) && t[n] == c[0] {
			n++
		}
		if n >= 3 {
			if c == "`" && strings.Contains(t[n:], "`") {
				return ""
			}
			return t[:n]
		}
	}
	return ""
}

var (
	atxRx      = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$`)
	hrRx       = regexp.MustCompile(`^ {0,3}(?:(?:-[ \t]*){3,}|(?:\*[ \t]*){3,}|(?:_[ \t]*){3,})$`)
	bulletRx   = regexp.MustCompile(`^( {0,3})([-*+])([ \t]+|$)`)
	orderedRx  = regexp.MustCompile(`^( {0,3})(\d{1,9})([.)])([ \t]+|$)`)
	tableSepRx = regexp.MustCompile(`^ {0,3}\|?[ \t]*:?-+:?[ \t]*(\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$`)
	preOpenRx  = regexp.MustCompile(`(?i)^ {0,3}<(pre|script|style)(?:[ \t>]|$)`)
	setextRx   = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	htmlOpenRx = regexp.MustCompile(`^ {0,3}<(/?)(details|summary|div|p|table|thead|tbody|tr|td|th|ul|ol|li|pre|blockquote|center|br|hr|img|h[1-6])(?:[ \t/>]|$)`)
)

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

// indentOf returns the width in columns of the leading white space of s (a tab goes to the next
// multiple of 4).
func indentOf(s string) int {
	col := 0
	for _, c := range []byte(s) {
		switch c {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return col
		}
	}
	return col
}

// stripIndent removes up to n columns of leading white space from s; a tab that is cut in two
// leaves its rest as spaces.
func stripIndent(s string, n int) string {
	col := 0
	for i := 0; i < len(s) && col < n; i++ {
		switch s[i] {
		case ' ':
			col++
		case '\t':
			w := 4 - col%4
			if col+w > n {
				return strings.Repeat(" ", col+w-n) + s[i+1:]
			}
			col += w
		default:
			return s[i:]
		}
		if col >= n || i+1 == len(s) {
			return s[i+1:]
		}
	}
	return s
}

// listMarker parses a list marker at the start of s: the kind ("-", "*", "+", or "1." / "1)" as
// "." / ")"), the start number, the width of the marker with its indent and the spaces after it
// (the item's content column), and whether s is a marker at all.
func listMarker(s string) (kind string, num int, content int, ok bool) {
	if m := bulletRx.FindStringSubmatch(s); m != nil {
		return m[2], 0, markerWidth(len(m[1]), 1, m[3], s), true
	}
	if m := orderedRx.FindStringSubmatch(s); m != nil {
		n := 0
		fmt.Sscanf(m[2], "%d", &n)
		return m[3], n, markerWidth(len(m[1]), len(m[2])+1, m[4], s), true
	}
	return "", 0, 0, false
}

// markerWidth is the content column of an item: the indent, the marker, and one to four spaces
// (a line with 5 or more spaces after the marker is code inside the item, so the content column
// is the one after one space; an empty item line has the column after one space too).
func markerWidth(indent, marker int, spaces, s string) int {
	rest := s[indent+marker:]
	if strings.TrimSpace(rest) == "" {
		return indent + marker + 1
	}
	w := indentOf(spaces)
	if w > 4 {
		w = 1
	}
	return indent + marker + w
}

// isParagraphLine reports whether the last of the lines is a line of a paragraph, which a lazy
// continuation line can continue: not a blank line, a fence or the inside of a fenced block, a
// heading or a rule.
func isParagraphLine(ls []line) bool {
	last := ls[len(ls)-1].text
	return !isBlank(last) && fenceOf(last) == "" && !inFence(ls) && !atxRx.MatchString(last) && !hrRx.MatchString(last)
}

// startsBlock reports whether s starts a block that interrupts a paragraph.
func startsBlock(s string) bool {
	if isBlank(s) || atxRx.MatchString(s) || hrRx.MatchString(s) || fenceOf(s) != "" {
		return true
	}
	if strings.HasPrefix(strings.TrimLeft(s, " "), ">") && indentOf(s) < 4 {
		return true
	}
	if m := bulletRx.FindStringSubmatch(s); m != nil && strings.TrimSpace(s[len(m[0]):]) != "" {
		return true
	}
	if m := orderedRx.FindStringSubmatch(s); m != nil && m[2] == "1" && strings.TrimSpace(s[len(m[0]):]) != "" {
		return true
	}
	return false
}

// blocks renders the lines as a sequence of blocks. tight renders a paragraph without <p> (the
// items of a tight list).
func (r *renderer) blocks(lines []line, tight bool) string {
	var b strings.Builder
	i := 0
	for i < len(lines) {
		l := lines[i]
		s := l.text
		r.curLine = l.no
		switch {
		case isBlank(s):
			i++
		case indentOf(s) >= 4:
			// indented code
			var code []string
			j := i
			for j < len(lines) && (isBlank(lines[j].text) || indentOf(lines[j].text) >= 4) {
				code = append(code, stripIndent(lines[j].text, 4))
				j++
			}
			for len(code) > 0 && isBlank(code[len(code)-1]) {
				code = code[:len(code)-1]
			}
			b.WriteString("<pre><code>" + html.EscapeString(strings.Join(code, "\n")) + "\n</code></pre>\n")
			i = j
		case fenceOf(s) != "":
			i = r.fenced(&b, lines, i)
		case atxRx.MatchString(s):
			m := atxRx.FindStringSubmatch(s)
			r.heading(&b, len(m[1]), strings.TrimSpace(m[2]))
			i++
		case hrRx.MatchString(s):
			b.WriteString("<hr>\n")
			i++
		case strings.HasPrefix(strings.TrimSpace(s), "<!--"):
			// An HTML comment: the markers of the tests and the notes for authors. It is not shown;
			// text after its end on the same line is.
			k, rest := commentEnd(lines, i)
			if k < 0 {
				r.problem("an HTML comment is not closed")
				i = len(lines)
				break
			}
			if isBlank(rest) {
				i = k + 1
			} else {
				lines[k] = line{rest, lines[k].no}
				i = k
			}
		case preOpenRx.MatchString(s):
			// <pre>, <script> and <style> end at their closing tag, not at a blank line.
			closeTag := "</" + preOpenRx.FindStringSubmatch(s)[1] + ">"
			j := i
			for j < len(lines) {
				b.WriteString(stripComments(lines[j].text) + "\n")
				j++
				if strings.Contains(strings.ToLower(lines[j-1].text), closeTag) {
					break
				}
			}
			i = j
		case htmlOpenRx.MatchString(s):
			j := i
			for j < len(lines) && !isBlank(lines[j].text) {
				b.WriteString(stripComments(lines[j].text) + "\n")
				j++
			}
			i = j
		case strings.HasPrefix(strings.TrimLeft(s, " "), ">"):
			i = r.quote(&b, lines, i)
		case isListStart(s):
			i = r.list(&b, lines, i)
		case isTableStart(lines, i):
			i = r.table(&b, lines, i)
		default:
			i = r.paragraph(&b, lines, i, tight)
		}
	}
	return b.String()
}

// commentEnd finds the end of the HTML comment that starts on line i: the line k that holds the
// "-->" and the text after it, or -1.
func commentEnd(lines []line, i int) (int, string) {
	for k := i; k < len(lines); k++ {
		t := lines[k].text
		from := 0
		if k == i {
			from = strings.Index(t, "<!--") + 4
		}
		if e := strings.Index(t[from:], "-->"); e >= 0 {
			return k, t[from+e+3:]
		}
	}
	return -1, ""
}

func isListStart(s string) bool {
	_, _, _, ok := listMarker(s)
	return ok
}

func stripComments(s string) string {
	for {
		a := strings.Index(s, "<!--")
		if a < 0 {
			return s
		}
		e := strings.Index(s[a:], "-->")
		if e < 0 {
			return s[:a]
		}
		s = s[:a] + s[a+e+3:]
	}
}

func (r *renderer) fenced(b *strings.Builder, lines []line, i int) int {
	open := lines[i].text
	fence := fenceOf(open)
	indent := indentOf(open)
	info := strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(open, " "), fence[:1]))
	lang, _, _ := strings.Cut(info, " ")
	var code []string
	j := i + 1
	closed := false
	for ; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j].text)
		if strings.HasPrefix(t, fence) && strings.Trim(t, fence[:1]) == "" && indentOf(lines[j].text) < 4 {
			j++
			closed = true
			break
		}
		code = append(code, stripIndent(lines[j].text, indent))
	}
	if !closed {
		r.problem("a code fence is not closed")
	}
	cls := ""
	if lang != "" {
		cls = ` class="language-` + html.EscapeString(lang) + `"`
	}
	b.WriteString("<pre><code" + cls + ">" + html.EscapeString(strings.Join(code, "\n")))
	if len(code) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("</code></pre>\n")
	return j
}

func (r *renderer) heading(b *strings.Builder, level int, text string) {
	inner := r.inline(text)
	plain := plainText(inner)
	id := r.slug(plain)
	r.headings = append(r.headings, heading{level, id, plain})
	if level == 1 && r.title == "" {
		r.title = plain
	}
	fmt.Fprintf(b, "<h%d id=\"%s\">%s<a class=\"anchor\" href=\"#%s\" aria-label=\"link to this heading\">#</a></h%d>\n", level, id, inner, id, level)
}

var (
	tagRx    = regexp.MustCompile(`<[^>]*>`)
	slugDrop = regexp.MustCompile(`[^\p{L}\p{N} _-]`)
)

// plainText returns the text of rendered HTML.
func plainText(h string) string {
	return strings.TrimSpace(html.UnescapeString(tagRx.ReplaceAllString(h, "")))
}

// slug returns the anchor of a heading as GitHub makes it: lower case, punctuation dropped,
// spaces to hyphens, and -1, -2 for a repeated one.
func (r *renderer) slug(text string) string {
	base := strings.ReplaceAll(slugDrop.ReplaceAllString(strings.ToLower(text), ""), " ", "-")
	s := base
	if r.used[s] {
		for {
			r.slugs[base]++
			s = fmt.Sprintf("%s-%d", base, r.slugs[base])
			if !r.used[s] {
				break
			}
		}
	}
	r.used[s] = true
	return s
}

// problem records a construct that the renderer does not support (it would be rendered wrongly
// or dropped): docsite -check fails on it.
func (r *renderer) problem(format string, a ...any) {
	r.problems = append(r.problems, fmt.Sprintf("line %d: ", r.curLine)+fmt.Sprintf(format, a...))
}

func (r *renderer) quote(b *strings.Builder, lines []line, i int) int {
	var inner []line
	j := i
	for j < len(lines) {
		t := lines[j].text
		trim := strings.TrimLeft(t, " ")
		if strings.HasPrefix(trim, ">") && indentOf(t) < 4 {
			trim = trim[1:]
			trim = strings.TrimPrefix(trim, " ")
			inner = append(inner, line{trim, lines[j].no})
			j++
			continue
		}
		// Lazy continuation of a paragraph.
		if !isBlank(t) && len(inner) > 0 && isParagraphLine(inner) && !startsBlock(t) {
			inner = append(inner, lines[j])
			j++
			continue
		}
		break
	}
	b.WriteString("<blockquote>\n" + r.blocks(inner, false) + "</blockquote>\n")
	return j
}

type item struct {
	lines []line
	loose bool
}

func (r *renderer) list(b *strings.Builder, lines []line, i int) int {
	kind, start, _, _ := listMarker(lines[i].text)
	ordered := kind == "." || kind == ")"
	var items []item
	loose := false
	j := i
	for j < len(lines) {
		k, _, content, ok := listMarker(lines[j].text)
		if !ok || k != kind || indentOf(lines[j].text) >= 4 {
			break
		}
		// Begin an item: the first line, then every line that belongs to it.
		first := line{itemFirstLine(lines[j].text), lines[j].no}
		it := item{lines: []line{first}}
		j++
		blankSeen := false
		for j < len(lines) {
			t := lines[j].text
			if isBlank(t) {
				// A blank line belongs to the item if an indented line follows.
				n := j
				for n < len(lines) && isBlank(lines[n].text) {
					n++
				}
				if n < len(lines) && indentOf(lines[n].text) >= content {
					for ; j < n; j++ {
						it.lines = append(it.lines, line{"", lines[j].no})
					}
					blankSeen = true
					continue
				}
				break
			}
			if indentOf(t) >= content {
				it.lines = append(it.lines, line{stripIndent(t, content), lines[j].no})
				blankSeen = false
				j++
				continue
			}
			// Lazy continuation: a line of the item's last paragraph.
			if !blankSeen && isParagraphLine(it.lines) && !startsBlock(t) && !isListStart(t) {
				it.lines = append(it.lines, line{strings.TrimLeft(t, " \t"), lines[j].no})
				j++
				continue
			}
			break
		}
		items = append(items, it)
		// Another item of this list: after blank lines, the same kind of marker.
		n := j
		for n < len(lines) && isBlank(lines[n].text) {
			n++
		}
		if n >= len(lines) {
			break
		}
		if k2, _, _, ok2 := listMarker(lines[n].text); !ok2 || k2 != kind || indentOf(lines[n].text) >= 4 {
			break
		}
		if n > j {
			loose = true
			items[len(items)-1].loose = true
		}
		j = n
	}
	// An item whose own blocks are separated by a blank line makes the list loose too.
	for _, it := range items {
		if looseInside(it.lines) {
			loose = true
		}
	}
	tag := "ul"
	attr := ""
	if ordered {
		tag = "ol"
		if start != 1 {
			attr = fmt.Sprintf(` start="%d"`, start)
		}
	}
	b.WriteString("<" + tag + attr + ">\n")
	for _, it := range items {
		b.WriteString("<li>")
		inner := r.blocks(trimBlankEnds(it.lines), !loose)
		if !loose {
			inner = strings.TrimSuffix(inner, "\n")
		} else {
			inner = "\n" + inner
		}
		b.WriteString(inner + "</li>\n")
	}
	b.WriteString("</" + tag + ">\n")
	return j
}

// itemFirstLine returns the first line of a list item without its marker. After 5 or more
// spaces the rest is code inside the item, so it keeps its indent but one space.
func itemFirstLine(s string) string {
	var end int
	var spaces string
	if m := bulletRx.FindStringSubmatch(s); m != nil {
		end, spaces = len(m[0]), m[3]
	} else if m := orderedRx.FindStringSubmatch(s); m != nil {
		end, spaces = len(m[0]), m[4]
	}
	rest := s[end:]
	if w := indentOf(spaces); w > 4 {
		rest = strings.Repeat(" ", w-1) + rest
	}
	return rest
}

func trimBlankEnds(ls []line) []line {
	for len(ls) > 0 && isBlank(ls[len(ls)-1].text) {
		ls = ls[:len(ls)-1]
	}
	return ls
}

// inFence reports whether the lines end inside a fenced code block.
func inFence(ls []line) bool {
	fence := ""
	for _, l := range ls {
		if f := fenceOf(l.text); f != "" {
			if fence == "" {
				fence = f
			} else if strings.HasPrefix(f, fence[:1]) && len(f) >= len(fence) {
				fence = ""
			}
		}
	}
	return fence != ""
}

// looseInside reports whether the lines of an item have a blank line between two blocks (not
// inside a fenced block, and not before a nested list's item).
func looseInside(ls []line) bool {
	fence := ""
	for i, l := range ls {
		if f := fenceOf(l.text); f != "" {
			if fence == "" {
				fence = f
			} else if strings.HasPrefix(f, fence[:1]) && len(f) >= len(fence) {
				fence = ""
			}
		}
		if fence == "" && isBlank(l.text) && i > 0 && i < len(ls)-1 {
			// Only a blank line directly in this item counts: when the next non-blank line is indented
			// it belongs to a nested block, and the nested list decides for itself.
			n := i
			for n < len(ls) && isBlank(ls[n].text) {
				n++
			}
			if n < len(ls) && indentOf(ls[n].text) == 0 && !isBlank(ls[i-1].text) {
				return true
			}
		}
	}
	return false
}

func isTableStart(lines []line, i int) bool {
	if i+1 >= len(lines) || !strings.Contains(lines[i].text, "|") {
		return false
	}
	sep := lines[i+1].text
	return strings.Contains(sep, "-") && tableSepRx.MatchString(sep) && len(splitRow(lines[i].text)) == len(splitRow(sep))
}

// splitRow splits a table row into its cells at the pipes that are not escaped, also inside code
// spans, as GitHub does; \| is a pipe in the cell.
func splitRow(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && s[i+1] == '|':
			cur.WriteByte('|')
			i++
		case c == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if strings.TrimSpace(cur.String()) != "" || !strings.HasSuffix(s, "|") {
		cells = append(cells, strings.TrimSpace(cur.String()))
	}
	return cells
}

func (r *renderer) table(b *strings.Builder, lines []line, i int) int {
	head := splitRow(lines[i].text)
	sep := splitRow(lines[i+1].text)
	align := make([]string, len(sep))
	for k, c := range sep {
		l, rr := strings.HasPrefix(c, ":"), strings.HasSuffix(c, ":")
		switch {
		case l && rr:
			align[k] = ` style="text-align:center"`
		case rr:
			align[k] = ` style="text-align:right"`
		case l:
			align[k] = ` style="text-align:left"`
		}
	}
	b.WriteString("<div class=\"table-wrap\"><table>\n<thead><tr>")
	for k, c := range head {
		fmt.Fprintf(b, "<th%s>%s</th>", align[k], r.inline(c))
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	j := i + 2
	for j < len(lines) && !isBlank(lines[j].text) && strings.Contains(lines[j].text, "|") {
		r.curLine = lines[j].no
		cells := splitRow(lines[j].text)
		if len(cells) > len(head) {
			r.problem("a table row has %d cells, more than the header's %d: the extra ones would be dropped (is a | inside a code span escaped as \\|?)", len(cells), len(head))
		}
		b.WriteString("<tr>")
		for k := range head {
			c := ""
			if k < len(cells) {
				c = cells[k]
			}
			fmt.Fprintf(b, "<td%s>%s</td>", align[k], r.inline(c))
		}
		b.WriteString("</tr>\n")
		j++
	}
	b.WriteString("</tbody></table></div>\n")
	return j
}

// checkTable records a table whose header and delimiter row differ in the number of cells: it
// is a paragraph for GitHub and for this renderer, which is surely not what the author meant.
func (r *renderer) checkTable(lines []line, i int) {
	if i+1 < len(lines) && strings.Contains(lines[i].text, "|") && strings.Contains(lines[i+1].text, "-") && tableSepRx.MatchString(lines[i+1].text) &&
		len(splitRow(lines[i].text)) != len(splitRow(lines[i+1].text)) {
		r.problem("a table header has %d cells and its delimiter row %d", len(splitRow(lines[i].text)), len(splitRow(lines[i+1].text)))
	}
}

func (r *renderer) paragraph(b *strings.Builder, lines []line, i int, tight bool) int {
	var text []string
	j := i
	r.checkTable(lines, i)
	for j < len(lines) {
		t := lines[j].text
		if j > i && setextRx.MatchString(t) {
			level := 2
			if strings.Contains(t, "=") {
				level = 1
			}
			r.heading(b, level, strings.Join(text, " "))
			return j + 1
		}
		if j > i && (startsBlock(t) || strings.HasPrefix(strings.TrimSpace(t), "<!--") || isTableStart(lines, j)) {
			break
		}
		text = append(text, strings.TrimLeft(t, " \t"))
		j++
	}
	s := strings.Join(text, "\n")
	s = strings.TrimRight(s, " \t")
	if tight {
		b.WriteString(r.inline(s) + "\n")
	} else {
		b.WriteString("<p>" + r.inline(s) + "</p>\n")
	}
	return j
}

// ---- inline ----

// An inline node is either finished HTML, or a run of * or _ that can open or close emphasis.
type node struct {
	html      string
	delim     byte // '*' or '_', or 0 for html
	count     int
	canOpen   bool
	canClose  bool
	origCount int
	active    bool
	dead      bool   // between a matched pair: stays literal
	before    string // closing tags that emphasis adds before this node's delimiters
	after     string // opening tags that emphasis adds after this node's delimiters
}

var (
	autolinkRx = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9+.-]{1,31}:[^\s<>]*|[a-zA-Z0-9.!#$%&'*+/=?^_{|}~-]+@[a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)+)>`)
	htmlTagRx  = regexp.MustCompile(`^</?(br|kbd|sub|sup|b|i|em|strong|code|a|img|span|details|summary|mark|small|wbr)(?:\s[^<>]*)?/?>`)
	bareURLRx  = regexp.MustCompile(`https?://[^\s<>]+`)
)

// inline renders the inline content of a paragraph, heading or table cell.
func (r *renderer) inline(s string) string {
	nodes := r.inlineNodes(s)
	processEmphasis(nodes)
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(n.before)
		b.WriteString(n.html)
		b.WriteString(n.after)
	}
	return b.String()
}

func (r *renderer) inlineNodes(s string) []*node {
	var nodes []*node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			nodes = append(nodes, &node{html: linkifyText(text.String())})
			text.Reset()
		}
	}
	emit := func(h string) {
		flush()
		nodes = append(nodes, &node{html: h})
	}
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && s[i+1] == '\n':
			emit("<br>\n")
			i += 2
		case c == '\\' && i+1 < len(s) && isPunct(s[i+1]):
			text.WriteString("\x00" + string(s[i+1]) + "\x00")
			i += 2
		case c == '`':
			n := 0
			for i+n < len(s) && s[i+n] == '`' {
				n++
			}
			end := findBackticks(s, i+n, n)
			if end < 0 {
				text.WriteString(s[i : i+n])
				i += n
				continue
			}
			code := strings.ReplaceAll(s[i+n:end], "\n", " ")
			if len(code) > 2 && code[0] == ' ' && code[len(code)-1] == ' ' && strings.TrimSpace(code) != "" {
				code = code[1 : len(code)-1]
			}
			emit("<code>" + html.EscapeString(code) + "</code>")
			i = end + n
		case c == '!' && i+1 < len(s) && s[i+1] == '[':
			if h, n, ok := r.linkAt(s, i+1, true); ok {
				emit(h)
				i += 1 + n
			} else {
				text.WriteByte(c)
				i++
			}
		case c == '[':
			if i+1 < len(s) && s[i+1] == '^' {
				r.problem("a footnote ([^...]) is not supported")
			}
			if h, n, ok := r.linkAt(s, i, false); ok {
				emit(h)
				i += n
			} else {
				text.WriteByte(c)
				i++
			}
		case c == '<':
			if m := autolinkRx.FindStringSubmatch(s[i:]); m != nil {
				href := m[1]
				if !strings.Contains(href, ":") {
					href = "mailto:" + href
				}
				emit(fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(r.linkFunc(href, r.curLine)), html.EscapeString(m[1])))
				i += len(m[0])
			} else if strings.HasPrefix(s[i:], "<!--") && strings.Contains(s[i:], "-->") {
				i += strings.Index(s[i:], "-->") + 3
			} else if m := htmlTagRx.FindString(s[i:]); m != "" {
				emit(m)
				i += len(m)
			} else {
				text.WriteString("&lt;")
				i++
			}
		case c == '*' || c == '_':
			n := 0
			for i+n < len(s) && s[i+n] == c {
				n++
			}
			before, after := rune(' '), rune(' ')
			if i > 0 {
				before, _ = utf8.DecodeLastRuneInString(s[:i])
			}
			if i+n < len(s) {
				after, _ = utf8.DecodeRuneInString(s[i+n:])
			}
			leftFlank := !unicode.IsSpace(after) && (!isPunctRune(after) || unicode.IsSpace(before) || isPunctRune(before))
			rightFlank := !unicode.IsSpace(before) && (!isPunctRune(before) || unicode.IsSpace(after) || isPunctRune(after))
			canOpen, canClose := leftFlank, rightFlank
			if c == '_' {
				canOpen = leftFlank && (!rightFlank || isPunctRune(before))
				canClose = rightFlank && (!leftFlank || isPunctRune(after))
			}
			flush()
			nodes = append(nodes, &node{delim: c, count: n, origCount: n, canOpen: canOpen, canClose: canClose, active: true, html: s[i : i+n]})
			i += n
		case c == '\n':
			t := text.String()
			text.Reset()
			if strings.HasSuffix(t, "  ") {
				text.WriteString(strings.TrimRight(t, " "))
				emit("<br>\n")
			} else {
				text.WriteString(strings.TrimRight(t, " ") + "\n")
			}
			i++
		case c == '&':
			if m := entityRx.FindString(s[i:]); m != "" {
				text.WriteString(m)
				i += len(m)
			} else {
				text.WriteString("&amp;")
				i++
			}
		case c == '>':
			text.WriteString("&gt;")
			i++
		case c == '"':
			text.WriteString("&quot;")
			i++
		default:
			text.WriteByte(c)
			i++
		}
	}
	flush()
	// Escaped punctuation was kept marked until now.
	for _, n := range nodes {
		n.html = unmarkEscapes(n.html)
	}
	return nodes
}

var entityRx = regexp.MustCompile(`^&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});`)

func unmarkEscapes(h string) string {
	if !strings.Contains(h, "\x00") {
		return h
	}
	var b strings.Builder
	for i := 0; i < len(h); i++ {
		if h[i] == '\x00' && i+2 < len(h) && h[i+2] == '\x00' {
			b.WriteString(html.EscapeString(string(h[i+1])))
			i += 2
			continue
		}
		b.WriteByte(h[i])
	}
	return b.String()
}

// linkifyText turns the bare URLs of a text run into links (GitHub does too). The run is HTML
// already, with escapes marked; URLs inside it are plain.
func linkifyText(t string) string {
	if !strings.Contains(t, "http") {
		return t
	}
	return bareURLRx.ReplaceAllStringFunc(t, func(u string) string {
		trail := ""
		for len(u) > 0 && strings.ContainsRune(".,;:!?)'\"", rune(u[len(u)-1])) {
			trail = u[len(u)-1:] + trail
			u = u[:len(u)-1]
		}
		if strings.Contains(u, "&") || strings.Contains(u, "\x00") {
			return u + trail
		}
		return `<a href="` + u + `">` + u + `</a>` + trail
	})
}

func isPunct(c byte) bool {
	return strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", c) >= 0
}

func isPunctRune(r rune) bool {
	return r < 128 && isPunct(byte(r)) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// findBackticks returns the index of the next run of exactly n backticks at or after from.
func findBackticks(s string, from, n int) int {
	for i := from; i < len(s); i++ {
		if s[i] != '`' {
			continue
		}
		k := 0
		for i+k < len(s) && s[i+k] == '`' {
			k++
		}
		if k == n {
			return i
		}
		i += k - 1
	}
	return -1
}

// linkAt parses a link or image whose "[" is at s[at]; it returns the HTML and the number of
// bytes used from at.
func (r *renderer) linkAt(s string, at int, image bool) (string, int, bool) {
	// Find the matching ].
	depth := 0
	end := -1
	for k := at; k < len(s); k++ {
		switch s[k] {
		case '\\':
			k++
		case '`':
			n := 0
			for k+n < len(s) && s[k+n] == '`' {
				n++
			}
			if e := findBackticks(s, k+n, n); e >= 0 {
				k = e + n - 1
			} else {
				k += n - 1
			}
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = k
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return "", 0, false
	}
	label := s[at+1 : end]
	var dest, title string
	used := end + 1
	switch {
	case used < len(s) && s[used] == '(':
		d, t, n, ok := parseDest(s[used:])
		if !ok {
			return "", 0, false
		}
		dest, title, used = d, t, used+n
	default:
		// A reference: [label][ref], [label][] or [label].
		key := label
		if used < len(s) && s[used] == '[' {
			if e := strings.IndexByte(s[used:], ']'); e >= 0 {
				if k := s[used+1 : used+e]; k != "" {
					key = k
				}
				used += e + 1
			}
		}
		d, ok := r.refs[strings.ToLower(strings.Join(strings.Fields(key), " "))]
		if !ok {
			if key != label && !strings.ContainsAny(key, " `") {
				r.problem("the link reference [%s] is not defined", key)
			}
			return "", 0, false
		}
		dest = d
	}
	href := r.linkFunc(unescapeDest(dest), r.curLine)
	t := ""
	if title != "" {
		t = ` title="` + html.EscapeString(title) + `"`
	}
	if image {
		return fmt.Sprintf(`<img src="%s" alt="%s"%s>`, html.EscapeString(href), html.EscapeString(plainText(r.inline(label))), t), used - at + 1, true
	}
	return fmt.Sprintf(`<a href="%s"%s>%s</a>`, html.EscapeString(href), t, r.inline(label)), used - at, true
}

func unescapeDest(d string) string {
	var b strings.Builder
	for i := 0; i < len(d); i++ {
		if d[i] == '\\' && i+1 < len(d) && isPunct(d[i+1]) {
			i++
		}
		b.WriteByte(d[i])
	}
	return html.UnescapeString(b.String())
}

// parseDest parses "(dest "title")" at the start of s.
func parseDest(s string) (dest, title string, n int, ok bool) {
	i := 1
	skip := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
	}
	skip()
	if i < len(s) && s[i] == '<' {
		e := strings.IndexByte(s[i:], '>')
		if e < 0 {
			return "", "", 0, false
		}
		dest = s[i+1 : i+e]
		i += e + 1
	} else {
		depth := 0
		st := i
		for i < len(s) {
			c := s[i]
			if c == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if c == ' ' || c == '\t' || c == '\n' {
				break
			}
			if c == '(' {
				depth++
			} else if c == ')' {
				if depth == 0 {
					break
				}
				depth--
			}
			i++
		}
		dest = s[st:i]
	}
	skip()
	if i < len(s) && (s[i] == '"' || s[i] == '\'') {
		q := s[i]
		e := strings.IndexByte(s[i+1:], q)
		if e < 0 {
			return "", "", 0, false
		}
		title = s[i+1 : i+1+e]
		i += e + 2
		skip()
	}
	if i < len(s) && s[i] == ')' {
		return dest, title, i + 1, true
	}
	return "", "", 0, false
}

// processEmphasis is the delimiter-stack algorithm of CommonMark (without the cases of links
// inside emphasis, which the nodes make unnecessary: a link is finished HTML).
func processEmphasis(nodes []*node) {
	for ci := 0; ci < len(nodes); ci++ {
		closer := nodes[ci]
		if closer.delim == 0 || !closer.canClose || closer.count == 0 || closer.dead {
			continue
		}
		for {
			// Look back for an opener.
			oi := -1
			for k := ci - 1; k >= 0; k-- {
				o := nodes[k]
				if o.delim != closer.delim || !o.canOpen || o.count == 0 || o.dead {
					continue
				}
				// The rule of 3.
				if (o.canClose || closer.canOpen) && (o.origCount+closer.origCount)%3 == 0 &&
					(o.origCount%3 != 0 || closer.origCount%3 != 0) {
					continue
				}
				oi = k
				break
			}
			if oi < 0 {
				break
			}
			o := nodes[oi]
			use := 1
			if o.count >= 2 && closer.count >= 2 {
				use = 2
			}
			open, cl := "<em>", "</em>"
			if use == 2 {
				open, cl = "<strong>", "</strong>"
			}
			// The delimiter characters not used stay where they are; the tags go on the inner side.
			o.count -= use
			closer.count -= use
			o.after = open + o.after
			closer.before += cl
			// Delimiters between the two can no longer match.
			for k := oi + 1; k < ci; k++ {
				if nodes[k].delim != 0 {
					nodes[k].dead = true
				}
			}
			if closer.count == 0 {
				break
			}
		}
	}
	for _, n := range nodes {
		if n.delim != 0 {
			n.html = strings.Repeat(n.html[:1], n.count)
		}
	}
}
