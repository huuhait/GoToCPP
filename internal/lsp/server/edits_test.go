package server

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// applyEdits applies edits to text the way an editor does: every range is
// in the original text (LSP 3.17 "TextEdit[]"), and the ranges must not
// overlap.
func applyEdits(t testing.TB, text string, edits []protocol.TextEdit) string {
	t.Helper()
	m := protocol.NewMapper(text)
	type span struct {
		start, end int
		text       string
	}
	spans := make([]span, len(edits))
	for i, e := range edits {
		start, end, err := m.RangeOffsets(e.Range)
		if err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
		spans[i] = span{start, end, e.NewText}
	}
	slices.SortStableFunc(spans, func(a, b span) int { return a.start - b.start })
	var b strings.Builder
	at := 0
	for i, s := range spans {
		if s.start < at {
			t.Fatalf("edit %d overlaps the one before: %+v", i, edits)
		}
		b.WriteString(text[at:s.start])
		b.WriteString(s.text)
		at = s.end
	}
	b.WriteString(text[at:])
	return b.String()
}

// TestComputeEdits checks small cases, with the exact edits.
func TestComputeEdits(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
		want                []string // "line:char-line:char text"
	}{
		{"same", "a\nb\n", "a\nb\n", nil},
		{"indent", "a\nb\nc\n", "a\n\tb\nc\n", []string{`1:0-1:0 "\t"`}},
		{"one char in a line", "func f()  {\n", "func f() {\n", []string{`0:9-0:10 ""`}},
		{"insert line", "a\nc\n", "a\nb\nc\n", []string{`1:0-1:0 "b\n"`}},
		{"delete lines", "a\nb\nb\nc\n", "a\nc\n", []string{`1:0-3:0 ""`}},
		{"add final line end", "a\nb", "a\nb\n", []string{`1:1-1:1 "\n"`}},
		{"remove final line end", "a\nb\n", "a\nb", []string{`1:1-2:0 ""`}},
		{"empty before", "", "a\n", []string{`0:0-0:0 "a\n"`}},
		{"empty after", "a\nb\n", "", []string{`0:0-2:0 ""`}},
		// UTF-16: é is one unit, 😀 two.
		{"utf-16", "x := \"é😀\"  +  1\n", "x := \"é😀\" + 1\n", []string{`0:11-0:14 "+"`}},
		{"in a rune", "é\n", "è\n", []string{`0:0-0:1 "è"`}},
		{"emoji to emoji", "😀\n", "😁\n", []string{`0:0-0:2 "😁"`}},
		// CRLF: the line ends stay; a new line gets "\r\n" too.
		{"crlf indent", "a\r\nb\r\n", "a\n\tb\n", []string{`1:0-1:0 "\t"`}},
		{"crlf insert", "a\r\nc\r\n", "a\nb\nc\n", []string{`1:0-1:0 "b\r\n"`}},
		{"crlf join", "a\r\nb\r\nc\r\n", "a b\nc\n", []string{`0:1-1:0 " "`}},
		{"lone cr", "a\rb\n", "a b\n", []string{`0:1-0:2 " "`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := protocol.NewMapper(tc.before)
			edits := computeEdits(context.Background(), m, tc.after)
			var got []string
			for _, e := range edits {
				got = append(got, fmt.Sprintf("%d:%d-%d:%d %s", e.Range.Start.Line, e.Range.Start.Character,
					e.Range.End.Line, e.Range.End.Character, quote(e.NewText)))
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("edits:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
			want := tc.after
			if strings.Contains(tc.before, "\r\n") {
				want = strings.ReplaceAll(want, "\n", "\r\n")
			}
			if res := applyEdits(t, tc.before, edits); res != want {
				t.Errorf("applied: %q, want %q", res, want)
			}
		})
	}
}

// noCRLF returns s without "\r\n".
func noCRLF(s string) string {
	for strings.Contains(s, "\r\n") {
		s = strings.ReplaceAll(s, "\r\n", "\n")
	}
	return s
}

func quote(s string) string {
	return "\"" + strings.NewReplacer("\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(s) + "\""
}

// TestComputeEditsRandom checks on random texts that the edits turn the
// text into the other one, also with line ends "\r\n" (which stay).
func TestComputeEditsRandom(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	words := []string{"a", "b", "c", "\t", " ", "é", "😀", "{", "}", "x := 1", "\r"}
	text := func() string {
		var b strings.Builder
		for range r.Intn(30) {
			for range r.Intn(4) {
				b.WriteString(words[r.Intn(len(words))])
			}
			if r.Intn(10) > 0 {
				b.WriteString("\n")
			}
		}
		return b.String()
	}
	for i := range 3000 {
		before, after := text(), text()
		// A "\r" before "\n" belongs to the line end: keep them out of
		// the plain texts.
		before, after = noCRLF(before), noCRLF(after)
		if res := applyEdits(t, before, computeEdits(context.Background(), protocol.NewMapper(before), after)); res != after {
			t.Fatalf("case %d: %q -> %q gives %q", i, before, after, res)
		}
		// The same text with "\r\n": the result has "\r\n" too (the line
		// end of the first line).
		crlf := strings.ReplaceAll(before, "\n", "\r\n")
		if !strings.Contains(crlf, "\r\n") {
			continue
		}
		res := applyEdits(t, crlf, computeEdits(context.Background(), protocol.NewMapper(crlf), after))
		if strings.Contains(strings.ReplaceAll(res, "\r\n", ""), "\n") || strings.ReplaceAll(res, "\r\n", "\n") != after {
			t.Fatalf("case %d, CRLF: %q -> %q gives %q", i, crlf, after, res)
		}
	}
}

// TestCommonLines checks that the diff finds a longest common subsequence
// (against a simple quadratic LCS) on small random cases.
func TestCommonLines(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for i := range 2000 {
		a := make([]int, r.Intn(20))
		b := make([]int, r.Intn(20))
		for k := range a {
			a[k] = r.Intn(4)
		}
		for k := range b {
			b[k] = r.Intn(4)
		}
		got := commonLines(context.Background(), a, b, maxDiffWork)
		for k, p := range got {
			if a[p.x] != b[p.y] || k > 0 && (p.x <= got[k-1].x || p.y <= got[k-1].y) {
				t.Fatalf("case %d: %v %v: bad pairs %v", i, a, b, got)
			}
		}
		if want := lcsLen(a, b); len(got) != want {
			t.Errorf("case %d: %v %v: %d common lines, want %d", i, a, b, len(got), want)
		}
	}
}

func lcsLen(a, b []int) int {
	l := make([][]int, len(a)+1)
	for i := range l {
		l[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				l[i][j] = l[i+1][j+1] + 1
			} else {
				l[i][j] = max(l[i+1][j], l[i][j+1])
			}
		}
	}
	return l[0][0]
}

// TestComputeEditsCapped checks that the edits are still right when the
// line diff stops early: after its cost cap, or when the context is done.
func TestComputeEditsCapped(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for i := range 500 {
		a := make([]int, r.Intn(60))
		b := make([]int, r.Intn(60))
		for k := range a {
			a[k] = r.Intn(5)
		}
		for k := range b {
			b[k] = r.Intn(5)
		}
		got := commonLines(context.Background(), a, b, r.Intn(50))
		for k, p := range got {
			if a[p.x] != b[p.y] || k > 0 && (p.x <= got[k-1].x || p.y <= got[k-1].y) {
				t.Fatalf("case %d: %v %v: bad pairs %v", i, a, b, got)
			}
		}
	}
	// A big file whose every line changes, with a done context: the diff
	// stops at once, and the edits are right.
	var before, after strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&before, "line %d\n", i)
		fmt.Fprintf(&after, "\tline %d\n", i)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	edits := computeEdits(ctx, protocol.NewMapper(before.String()), after.String())
	if got := applyEdits(t, before.String(), edits); got != after.String() {
		t.Errorf("the edits are wrong")
	}
}

// TestCommonLinesStopsWhenTheContextIsDone counts the work of the differ (no clock): a done
// context ends it after the first check, a context that is not done lets it run to the cost cap.
func TestCommonLinesStopsWhenTheContextIsDone(t *testing.T) {
	const lines = 20000
	a := make([]int, lines)
	b := make([]int, lines)
	for i := range lines {
		a[i] = i
		b[i] = lines + i // no line in common: the diff walks until the cap
	}
	workUsed := func(ctx context.Context) int {
		size := 2*((len(a)+len(b)+1)/2) + 2
		d := &differ{ctx: ctx, a: a, b: b, v1: make([]int, size), v2: make([]int, size), left: maxDiffWork}
		d.compare(0, len(a), 0, len(b))
		return maxDiffWork - d.left
	}
	if work := workUsed(context.Background()); work < maxDiffWork {
		t.Fatalf("a live context used %d steps of %d: the input does not exercise the cap", work, maxDiffWork)
	}
	done, cancel := context.WithCancel(context.Background())
	cancel()
	// The context is looked at every 64 rounds; a round costs 2*round+2 steps.
	const firstCheck = 64 * (2*64 + 2)
	if work := workUsed(done); work > firstCheck {
		t.Errorf("a done context used %d steps, want at most %d", work, firstCheck)
	}
}

// TestSameLines: when only the white space of the lines changes, the lines
// correspond one to one, and the edits are right (applied, they give the
// new text) and few (one per changed line); the pairs are a common
// subsequence of the two texts. Texts that are not like that don't take the
// shortcut.
func TestSameLines(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	words := []string{"if x {", "}", "return a", "x := f(1, 2)", "", "// c", "for {", "}", "}"}
	for n := range 300 {
		var before, after strings.Builder
		changed := 0
		for range 5 + rng.Intn(40) {
			w := words[rng.Intn(len(words))]
			ind := strings.Repeat("\t", rng.Intn(4))
			ind2 := strings.Repeat("\t", rng.Intn(4))
			if rng.Intn(5) == 0 && w != "" {
				w = strings.Replace(w, " ", "  ", 1) // inner spaces
			}
			after.WriteString(ind2 + strings.ReplaceAll(w, "  ", " ") + "\n")
			before.WriteString(ind + w + "\n")
			if ind+w != ind2+strings.ReplaceAll(w, "  ", " ") {
				changed++
			}
		}
		b, a := before.String(), after.String()
		m := protocol.NewMapper(b)
		edits := computeEdits(context.Background(), m, a)
		if got := applyEdits(t, b, edits); got != a {
			t.Fatalf("case %d: the edits give\n%q\nwant\n%q", n, got, a)
		}
		if len(edits) > changed {
			t.Errorf("case %d: %d edits for %d changed lines", n, len(edits), changed)
		}
		la, lb := splitLines(b), splitLines(a)
		ka, kb := lineKeys(b, a, la, lb)
		pairs := sameLines(b, a, la, lb, ka, kb)
		if b != a && pairs == nil {
			t.Fatalf("case %d: no shortcut for a change of white space only", n)
		}
		last := -1
		for _, p := range pairs {
			if p.x != p.y || p.x <= last || ka[p.x] != kb[p.y] {
				t.Fatalf("case %d: the pair %v is not a common line in order", n, p)
			}
			last = p.x
		}
	}
	// Not the shortcut: another number of lines, or a line that changes
	// more than its white space.
	for _, c := range [][2]string{{"a\nb\n", "a\nb\nc\n"}, {"a\nb\n", "a\nc\n"}, {"x := 1\n", "x := 2\n"}} {
		la, lb := splitLines(c[0]), splitLines(c[1])
		ka, kb := lineKeys(c[0], c[1], la, lb)
		if sameLines(c[0], c[1], la, lb, ka, kb) != nil {
			t.Errorf("%q to %q takes the shortcut", c[0], c[1])
		}
	}
}

// lineKeys numbers the distinct lines of two texts, like computeEdits.
func lineKeys(before, after string, a, b []line) (ka, kb []int) {
	ids := map[string]int{}
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
	for _, l := range a {
		ka = append(ka, key(before, l))
	}
	for _, l := range b {
		kb = append(kb, key(after, l))
	}
	return ka, kb
}
