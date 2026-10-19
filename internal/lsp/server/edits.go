package server

import (
	"context"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// computeEdits returns the edits that turn the text of m into after, for
// textDocument/formatting and the code actions. The edits are small (like
// gopls's): a line diff finds the lines that stay, and each changed part
// is cut down to the bytes that really change, so the editor keeps the
// cursors and marks on the lines that don't change. The ranges are in
// UTF-16 units (protocol.Mapper), and an edit never splits a character
// or a "\r\n".
//
// Line ends: the diff compares lines without their line ends ("\n" and
// "\r\n" are the same), so a file with "\r\n" line ends keeps them. A line
// that the edits add gets the line end of the first line of the text
// (gocpp fmt itself always writes "\n"). The result is nil when nothing
// changes.
//
// The line diff has a cost cap (maxDiffWork) and stops when ctx is done:
// then the rest of the text is replaced in bigger pieces. The edits are
// right either way.
func computeEdits(ctx context.Context, m *protocol.Mapper, after string) []protocol.TextEdit {
	before := m.Text()
	if before == after {
		return nil
	}
	a, b := splitLines(before), splitLines(after)
	eol := "\n"
	if len(a) > 0 && a[0].end-a[0].content == 2 {
		eol = "\r\n"
	}
	// Each distinct line (its text, and whether it has a line end) gets a
	// number; the diff compares the numbers.
	ids := make(map[string]int)
	key := func(text string, l line) int {
		k := text[l.start:l.content]
		if l.end > l.content {
			k += "\n"
		}
		id, ok := ids[k]
		if !ok {
			id = len(ids)
			ids[k] = id
		}
		return id
	}
	ka := make([]int, len(a))
	for i, l := range a {
		ka[i] = key(before, l)
	}
	kb := make([]int, len(b))
	for i, l := range b {
		kb[i] = key(after, l)
	}
	e := editor{m: m, before: before, after: after, a: a, b: b, eol: eol}
	i, j := 0, 0
	common := sameLines(before, after, a, b, ka, kb)
	if common == nil {
		common = commonLines(ctx, ka, kb, maxDiffWork)
	}
	for _, p := range append(common, pair{len(a), len(b)}) {
		if p.x > i || p.y > j {
			e.hunk(i, p.x, j, p.y)
		}
		i, j = p.x+1, p.y+1
	}
	return e.edits
}

// A line is one line of a text, as byte offsets: start, the end of its
// text (content), and the end of its line end (end; the start of the next
// line). The line end is "\n", "\r\n", or nothing for the last line.
type line struct{ start, content, end int }

// splitLines returns the lines of text, by the rules of protocol.Mapper:
// only "\n" ends a line, and "\r" before it belongs to the line end. A
// text that ends with "\n" has no empty last line here.
func splitLines(text string) []line {
	var lines []line
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && text[end] != '\n' {
			end++
		}
		content := end
		if end < len(text) {
			end++ // the \n
			if content > start && text[content-1] == '\r' {
				content--
			}
		}
		lines = append(lines, line{start, content, end})
		start = end
	}
	return lines
}

// An editor makes the edits of computeEdits, one changed part (hunk) at a
// time, in order.
type editor struct {
	m             *protocol.Mapper
	before, after string
	a, b          []line // the lines of before and after
	eol           string // the line end of an added line
	edits         []protocol.TextEdit
}

// hunk adds the edits that replace the lines a[i0:i1] by b[j0:j1]. When
// both have the same number of lines, each line is changed on its own (a
// line that only gets a new indentation keeps its line end and the text
// after the change); else the lines are replaced as a whole.
func (e *editor) hunk(i0, i1, j0, j1 int) {
	if i1-i0 == j1-j0 {
		for k := range i1 - i0 {
			l, n := e.a[i0+k], e.b[j0+k]
			newText := e.after[n.start:n.content]
			if n.end > n.content { // the new line has a line end
				if l.end > l.content {
					newText += e.before[l.content:l.end] // keep the old one
				} else {
					newText += e.eol
				}
			}
			e.add(l.start, e.before[l.start:l.end], newText)
		}
		return
	}
	start := len(e.before)
	if i0 < len(e.a) {
		start = e.a[i0].start
	}
	end := start
	if i1 > i0 {
		end = e.a[i1-1].end
	}
	var newText []byte
	for _, n := range e.b[j0:j1] {
		newText = append(newText, e.after[n.start:n.content]...)
		if n.end > n.content {
			newText = append(newText, e.eol...)
		}
	}
	e.add(start, e.before[start:end], string(newText))
}

// add adds the edit that replaces old, which starts at the byte offset base
// of the text, by new, cut down to the part that changes: the bytes that
// old and new have in common at the start and at the end stay. A cut is
// never inside a UTF-8 character or between the "\r" and the "\n" of a
// line end, because protocol.Mapper can't point there.
func (e *editor) add(base int, old, new string) {
	p := 0
	for p < len(old) && p < len(new) && old[p] == new[p] {
		p++
	}
	for p > 0 && (p < len(old) && !utf8.RuneStart(old[p]) || p < len(new) && !utf8.RuneStart(new[p])) {
		p--
	}
	if p > 0 && old[p-1] == '\r' && p < len(old) && old[p] == '\n' {
		p--
	}
	s := 0
	for s < len(old)-p && s < len(new)-p && old[len(old)-1-s] == new[len(new)-1-s] {
		s++
	}
	for s > 0 && !utf8.RuneStart(old[len(old)-s]) {
		s--
	}
	if s > 0 && old[len(old)-s] == '\n' && len(old)-s > 0 && old[len(old)-s-1] == '\r' {
		s--
	}
	if p == len(old)-s && p == len(new)-s {
		return // nothing changes
	}
	e.edits = append(e.edits, protocol.TextEdit{
		Range: protocol.Range{
			Start: e.m.Position(base + p),
			End:   e.m.Position(base + len(old) - s),
		},
		NewText: new[p : len(new)-s],
	})
}

// sameLines is the shortcut for the usual formatting, where only the white
// space of the lines changes (the indentation, the spaces around an
// operator): when both texts have the same number of lines and every line
// of before is, without its white space, the line of after at the same
// place, the lines correspond one to one and the common lines are the ones
// that are equal at the same place. That takes one pass, where the diff of
// a 1,000-line file that lost its indentation takes 10 ms. It returns nil
// when the texts are not like that (the diff runs then). The pairs are a
// common subsequence, not always a longest one (a line could match another
// copy of itself further down), so the edits are right, and a little
// longer in rare cases.
func sameLines(before, after string, a, b []line, ka, kb []int) []pair {
	if len(a) != len(b) {
		return nil
	}
	squash := func(s string) string {
		out := make([]byte, 0, len(s))
		for i := range len(s) {
			if c := s[i]; c != ' ' && c != '\t' && c != '\r' {
				out = append(out, c)
			}
		}
		return string(out)
	}
	for i := range a {
		if ka[i] != kb[i] && squash(before[a[i].start:a[i].content]) != squash(after[b[i].start:b[i].content]) {
			return nil
		}
	}
	out := make([]pair, 0, len(a)) // not nil: nil says there is no shortcut
	for i := range a {
		if ka[i] == kb[i] {
			out = append(out, pair{i, i})
		}
	}
	return out
}

// A pair is a line of a (x) that is the same as a line of b (y).
type pair struct{ x, y int }

// maxDiffWork is the most work (steps of bisect) that computeEdits spends
// on the line diff of a text: about 4M steps, a few milliseconds. A
// 1,000-line file that lost all its indentation needs 1.3M steps; a
// 20,000-line one would need 530M (about 0.8 s), and then the cap cuts the
// diff short.
const maxDiffWork = 1 << 22

// commonLines returns a longest common subsequence of a and b (or, in rare
// cases, a common subsequence that is a little shorter), as the pairs of
// the equal lines, in order. It stops looking for common lines after
// limit steps of work, or when ctx is done: the lines it has not looked at
// yet then count as different. It is the linear-space form of Myers's diff
// algorithm ("An O(ND) Difference Algorithm and Its Variations", 1986):
// it runs in O((N+M)·D) time and O(N+M) memory, where D is the number of
// lines that differ. bisect walks the two paths the way diff-match-patch's
// diff_bisect does (it skips the diagonals of a path that left the
// graph); the code is gocpp's own.
func commonLines(ctx context.Context, a, b []int, limit int) []pair {
	n := (len(a) + len(b) + 1) / 2
	d := &differ{ctx: ctx, a: a, b: b, v1: make([]int, 2*n+2), v2: make([]int, 2*n+2), left: limit}
	d.compare(0, len(a), 0, len(b))
	return d.out
}

// A differ holds the state of commonLines.
type differ struct {
	ctx    context.Context
	left   int  // the work left (bisect)
	done   bool // no work left, or ctx is done
	a, b   []int
	v1, v2 []int // the furthest points of the paths (bisect), reused
	out    []pair
}

// compare adds the common lines of a[a0:a1] and b[b0:b1] to out.
func (d *differ) compare(a0, a1, b0, b1 int) {
	for a0 < a1 && b0 < b1 && d.a[a0] == d.b[b0] {
		d.out = append(d.out, pair{a0, b0})
		a0, b0 = a0+1, b0+1
	}
	tail := 0
	for a1 > a0 && b1 > b0 && d.a[a1-1] == d.b[b1-1] {
		a1, b1 = a1-1, b1-1
		tail++
	}
	if a0 < a1 && b0 < b1 && !d.done {
		// Split the rest at a point of a shortest path, and do each part.
		// If there is no good point (no common line, or a point at a
		// corner, which should not happen), the rest has no common lines:
		// the diff is a little longer, but still right.
		if x, y, ok := d.bisect(a0, a1, b0, b1); ok && a0 <= x && x <= a1 && b0 <= y && y <= b1 &&
			(x > a0 || y > b0) && (x < a1 || y < b1) {
			d.compare(a0, x, b0, y)
			d.compare(x, a1, y, b1)
		}
	}
	for k := range tail {
		d.out = append(d.out, pair{a1 + k, b1 + k})
	}
}

// bisect finds the point where a shortest path through the edit graph of
// a[a0:a1] and b[b0:b1] (whose first and last lines differ) can be split:
// it walks the paths from both corners at once until they meet. ok is
// false when they don't meet in time, which means the two have no line in
// common.
func (d *differ) bisect(a0, a1, b0, b1 int) (x, y int, ok bool) {
	a, b := d.a, d.b
	n, m := a1-a0, b1-b0
	maxD := (n + m + 1) / 2
	off := maxD // index of diagonal 0
	size := 2*maxD + 2
	v1, v2 := d.v1[:size], d.v2[:size]
	for i := range size {
		v1[i], v2[i] = -1, -1
	}
	v1[off+1], v2[off+1] = 0, 0
	delta := n - m
	// With an odd delta, the forward path finds the overlap; with an even
	// one, the backward path does.
	front := delta%2 != 0
	// The diagonals to skip at the ends, after a path left the graph.
	k1start, k1end, k2start, k2end := 0, 0, 0, 0
	for dd := 0; dd < maxD; dd++ {
		// A round costs about 2*dd+2 steps, and the snakes (counted
		// below).
		d.left -= 2*dd + 2
		if d.left < 0 || dd%64 == 63 && d.ctx.Err() != nil {
			d.done = true
			return 0, 0, false
		}
		// One step of the forward path.
		for k1 := -dd + k1start; k1 <= dd-k1end; k1 += 2 {
			i := off + k1
			var x1 int
			if k1 == -dd || (k1 != dd && v1[i-1] < v1[i+1]) {
				x1 = v1[i+1]
			} else {
				x1 = v1[i-1] + 1
			}
			y1 := x1 - k1
			for x1 < n && y1 < m && a[a0+x1] == b[b0+y1] {
				x1, y1 = x1+1, y1+1
				d.left--
			}
			v1[i] = x1
			switch {
			case x1 > n:
				k1end += 2 // off the right of the graph
			case y1 > m:
				k1start += 2 // off the bottom
			case front:
				j := off + delta - k1
				if j >= 0 && j < size && v2[j] != -1 && x1 >= n-v2[j] {
					return a0 + x1, b0 + y1, true
				}
			}
		}
		// One step of the backward path (on the reversed lines).
		for k2 := -dd + k2start; k2 <= dd-k2end; k2 += 2 {
			i := off + k2
			var x2 int
			if k2 == -dd || (k2 != dd && v2[i-1] < v2[i+1]) {
				x2 = v2[i+1]
			} else {
				x2 = v2[i-1] + 1
			}
			y2 := x2 - k2
			for x2 < n && y2 < m && a[a1-1-x2] == b[b1-1-y2] {
				x2, y2 = x2+1, y2+1
				d.left--
			}
			v2[i] = x2
			switch {
			case x2 > n:
				k2end += 2
			case y2 > m:
				k2start += 2
			case !front:
				j := off + delta - k2
				if j >= 0 && j < size && v1[j] != -1 {
					x1 := v1[j]
					y1 := off + x1 - j
					if x1 <= n && y1 >= 0 && y1 <= m && x1 >= n-x2 {
						return a0 + x1, b0 + y1, true
					}
				}
			}
		}
	}
	return 0, 0, false
}
