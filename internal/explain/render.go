package explain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/lower"
)

// A Live is the live example of a topic: the .gp source that ships with gocpp and the annotated
// C++ that the real compiler writes for it (gocpp emit -annotate), made each time.
type Live struct {
	Source string `json:"source"`
	Cpp    string `json:"cpp"`
	// Gated is the phase whose gates were opened to emit the example, when this gocpp
	// does not accept it yet (0 otherwise).
	Gated int `json:"gated,omitempty"`
}

// textWidth is the width that the text of topics and messages is wrapped to.
const textWidth = 92

// Wrap wraps the paragraphs of s (separated by blank lines) to width columns. A line that starts
// with a space or a dash is kept as it is (a list or a code line).
func Wrap(s string, width int) string {
	var out strings.Builder
	for i, para := range strings.Split(strings.Trim(s, "\n"), "\n\n") {
		if i > 0 {
			out.WriteString("\n")
		}
		if strings.HasPrefix(para, " ") || strings.HasPrefix(para, "-") {
			out.WriteString(para + "\n")
			continue
		}
		line := ""
		for _, w := range strings.Fields(para) {
			switch {
			case line == "":
				line = w
			case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) > width:
				out.WriteString(line + "\n")
				line = w
			default:
				line += " " + w
			}
		}
		out.WriteString(line + "\n")
	}
	return out.String()
}

// WriteTopic writes a topic for the terminal. live is nil when the example was not made.
func WriteTopic(w io.Writer, t *Topic, live *Live) error {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s: %s\n\n", t.Name, t.Title)
	b.WriteString(Wrap(t.Text, textWidth))
	b.WriteString("\n")
	if t.Spec != "" {
		fmt.Fprintf(&b, "Specification: docs/spec/%s\n", t.Spec)
	}
	if len(t.See) > 0 {
		fmt.Fprintf(&b, "See also: gocpp explain %s\n", strings.Join(t.See, ", gocpp explain "))
	}
	if len(t.Aliases) > 0 {
		fmt.Fprintf(&b, "Also known as: %s\n", strings.Join(t.Aliases, ", "))
	}
	if live != nil {
		fmt.Fprintf(&b, "\nExample (%s):\n\n", t.Example)
		for _, l := range strings.Split(strings.TrimRight(live.Source, "\n"), "\n") {
			if l == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString("    " + l + "\n")
		}
		b.WriteString("\nThe C++ that gocpp emit -annotate writes for it")
		if live.Gated != 0 {
			fmt.Fprintf(&b, " (this gocpp checks for Phase %02d features, so gocpp build rejects this program until Phase %02d; the C++ is what the emitter writes with the gates open)", live.Gated, live.Gated)
		}
		b.WriteString(":\n\n")
		for _, l := range strings.Split(strings.TrimRight(live.Cpp, "\n"), "\n") {
			if l == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString("    " + l + "\n")
		}
	}
	_, err := w.Write(b.Bytes())
	return err
}

// topicJSON is the JSON of a topic.
type topicJSON struct {
	Name    string   `json:"name"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Aliases []string `json:"aliases,omitempty"`
	Text    []string `json:"text"`
	Spec    string   `json:"spec,omitempty"`
	See     []string `json:"see,omitempty"`
	Example *Live    `json:"example,omitempty"`
}

// WriteTopicJSON writes a topic as JSON.
func WriteTopicJSON(w io.Writer, t *Topic, live *Live) error {
	j := topicJSON{Name: t.Name, Title: t.Title, Summary: t.Summary, Aliases: t.Aliases, Spec: t.Spec, See: t.See, Example: live}
	j.Text = strings.Split(strings.TrimSpace(t.Text), "\n\n")
	return writeJSON(w, j)
}

func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false) // the C++ and the messages have < and >: no \u003c
	e.SetIndent("", "  ")
	return e.Encode(v)
}

// WriteList writes the list of topics (gocpp explain -list).
func WriteList(w io.Writer, asJSON bool) error {
	if asJSON {
		type item struct {
			Name    string   `json:"name"`
			Title   string   `json:"title"`
			Summary string   `json:"summary"`
			Aliases []string `json:"aliases,omitempty"`
		}
		var out []item
		for _, t := range Topics() {
			out = append(out, item{t.Name, t.Title, t.Summary, t.Aliases})
		}
		return writeJSON(w, out)
	}
	var b bytes.Buffer
	width := 0
	for _, t := range Topics() {
		width = max(width, len(t.Name))
	}
	for _, t := range Topics() {
		fmt.Fprintf(&b, "%-*s  %s\n", width, t.Name, t.Summary)
	}
	b.WriteString("\ngocpp explain <topic> explains a topic with a live example; gocpp explain file.gp:LINE shows the\n" +
		"C++ of one line; gocpp explain \"<error message>\" explains a message (gocpp explain -list -messages).\n")
	_, err := w.Write(b.Bytes())
	return err
}

// WriteMessageList writes the messages that gocpp explain knows (gocpp explain -list -messages).
func WriteMessageList(w io.Writer, asJSON bool) error {
	ms := Messages()
	if asJSON {
		type item struct {
			ID      string `json:"id"`
			Source  string `json:"source"`
			Message string `json:"message"`
			Phase   int    `json:"phase,omitempty"`
			Title   string `json:"title"`
		}
		var out []item
		for _, m := range ms {
			text := m.Text
			if text == "" {
				text = m.Pattern
			}
			out = append(out, item{m.ID, m.Source, text, m.Phase, m.Title})
		}
		return writeJSON(w, out)
	}
	var b bytes.Buffer
	for _, m := range ms {
		text := m.Text
		if text == "" {
			text = m.Pattern
		}
		fmt.Fprintf(&b, "%s\n", text)
	}
	_, err := w.Write(b.Bytes())
	return err
}

// WriteMessage writes the explanations of an error message.
func WriteMessage(w io.Writer, msg string, ms []Message, asJSON bool) error {
	if asJSON {
		type item struct {
			ID     string `json:"id"`
			Source string `json:"source"`
			Phase  int    `json:"phase,omitempty"`
			Title  string `json:"title"`
			Why    string `json:"why"`
			Fix    string `json:"fix,omitempty"`
			Topic  string `json:"topic,omitempty"`
			Spec   string `json:"spec,omitempty"`
		}
		out := struct {
			Message string `json:"message"`
			Matches []item `json:"matches"`
		}{Message: StripPosition(msg), Matches: []item{}}
		for _, m := range ms {
			out.Matches = append(out.Matches, item{m.ID, m.Source, m.Phase, m.Title, m.Why, m.Fix, m.Topic, m.Spec})
		}
		return writeJSON(w, out)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s\n", StripPosition(msg))
	for i, m := range ms {
		if i > 0 {
			b.WriteString("\n----\n")
		}
		if m.Title != StripPosition(msg) && m.Text == "" || m.Text != "" && m.Title != m.Text {
			fmt.Fprintf(&b, "\n%s (reported by the %s)\n\n", m.Title, m.Source)
		} else {
			fmt.Fprintf(&b, "\nreported by the %s\n\n", m.Source)
		}
		b.WriteString(Wrap(m.Why, textWidth))
		if m.Fix != "" {
			b.WriteString("\nWhat to do:\n")
			b.WriteString(indent(Wrap(m.Fix, textWidth-2), "  "))
		}
		if m.Topic != "" {
			fmt.Fprintf(&b, "\nThe mechanism: gocpp explain %s\n", m.Topic)
		}
		if m.Spec != "" {
			fmt.Fprintf(&b, "Specification: docs/spec/%s\n", m.Spec)
		}
	}
	_, err := w.Write(b.Bytes())
	return err
}

func indent(s, prefix string) string {
	var b strings.Builder
	for _, l := range strings.SplitAfter(s, "\n") {
		if l == "" {
			continue
		}
		b.WriteString(prefix + l)
	}
	return b.String()
}

// Suggest returns the names of topics that are close to name (a typo, a prefix), best first,
// at most 3.
func Suggest(name string) []string {
	name = strings.ToLower(strings.TrimSpace(name))
	type cand struct {
		name string
		dist int
	}
	var cs []cand
	add := func(topic, alias string) {
		d := editDistance(name, alias)
		if strings.HasPrefix(alias, name) || strings.Contains(alias, name) && len(name) >= 3 {
			d = min(d, 1)
		}
		if d <= max(2, len(name)/3) {
			cs = append(cs, cand{topic, d})
		}
	}
	for _, t := range Topics() {
		add(t.Name, t.Name)
		for _, a := range t.Aliases {
			add(t.Name, a)
		}
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].dist < cs[j].dist })
	var out []string
	seen := map[string]bool{}
	for _, c := range cs {
		if !seen[c.name] && len(out) < 3 {
			seen[c.name] = true
			out = append(out, c.name)
		}
	}
	return out
}

// editDistance is the Levenshtein distance of a and b.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

// OnlyFiles returns the view with only the files that keep accepts.
func (v *View) OnlyFiles(keep func(path string) bool) *View {
	out := &View{reasons: v.reasons, Unplaced: v.Unplaced}
	for _, f := range v.Files {
		if keep(f.Path) {
			out.Files = append(out.Files, f)
		}
	}
	return out
}

// SourceFiles returns the .gp files that the C++ of the view comes from, in the order of first use.
func (v *View) SourceFiles() []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range v.Files {
		for _, c := range f.Chunks {
			if c.Src.File != "" && !seen[c.Src.File] {
				seen[c.Src.File] = true
				out = append(out, c.Src.File)
			}
		}
	}
	return out
}

// KindDescription returns the one-line summary of the kind k (its topic's), or "".
func KindDescription(k lower.Kind) string {
	if t := FindTopic(string(k)); t != nil {
		return t.Summary
	}
	return ""
}

// Rel returns a copy of the view whose .gp paths (the ones that are written out: in the JSON) are
// shortened by rel, as WriteText shortens them.
func (v *View) Rel(rel func(string) string) *View {
	out := &View{reasons: v.reasons}
	for _, f := range v.Files {
		out.Files = append(out.Files, FileView{Path: f.Path, Chunks: RelChunks(f.Chunks, rel)})
	}
	for _, u := range v.Unplaced {
		u.File = rel(u.File)
		out.Unplaced = append(out.Unplaced, u)
	}
	return out
}

// RelChunks returns a copy of the chunks with the paths of their .gp lines shortened by rel.
func RelChunks(cs []Chunk, rel func(string) string) []Chunk {
	out := make([]Chunk, len(cs))
	for i, c := range cs {
		if c.GP != nil {
			gp := *c.GP
			gp.File = rel(gp.File)
			c.GP = &gp
		}
		out[i] = c
	}
	return out
}
