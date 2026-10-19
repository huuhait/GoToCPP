package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func render(src string) string {
	out, _ := renderMarkdown(src, func(href string, _ int) string { return href })
	return out
}

func TestRenderMarkdown(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"paragraph", "a\nb\n\nc", "<p>a\nb</p>\n<p>c</p>\n"},
		{"heading", "## Hello, *world* `x`", "<h2 id=\"hello-world-x\">Hello, <em>world</em> <code>x</code><a class=\"anchor\" href=\"#hello-world-x\" aria-label=\"link to this heading\">#</a></h2>\n"},
		{"code span", "use `a<b` and ``a`b``", "<p>use <code>a&lt;b</code> and <code>a`b</code></p>\n"},
		{"emphasis", "*a* **b** ***c*** _d_ snake_case_name 2*3*4", "<p><em>a</em> <strong>b</strong> <em><strong>c</strong></em> <em>d</em> snake_case_name 2<em>3</em>4</p>\n"},
		{"unmatched", "a * b ** c", "<p>a * b ** c</p>\n"},
		{"link", "[go](https://go.dev \"Go\") and [rel](a/b.md#c)", "<p><a href=\"https://go.dev\" title=\"Go\">go</a> and <a href=\"a/b.md#c\">rel</a></p>\n"},
		{"link with parens and code", "[`f(x)`](a_(b).md)", "<p><a href=\"a_(b).md\"><code>f(x)</code></a></p>\n"},
		{"autolink", "<https://go.dev/dl/?mode=json&include=all> and https://example.com/x.", "<p><a href=\"https://go.dev/dl/?mode=json&amp;include=all\">https://go.dev/dl/?mode=json&amp;include=all</a> and <a href=\"https://example.com/x\">https://example.com/x</a>.</p>\n"},
		{"escapes", `\*not em\* and \<tag\> & <T>`, "<p>*not em* and &lt;tag&gt; &amp; &lt;T&gt;</p>\n"},
		{"image", "![alt text](img/a.png)", "<p><img src=\"img/a.png\" alt=\"alt text\"></p>\n"},
		{"comment dropped", "a\n\n<!-- test: x -->\n```go\nx < y\n```\n", "<p>a</p>\n<pre><code class=\"language-go\">x &lt; y\n</code></pre>\n"},
		{"fence keeps tabs", "```\n\tif x {\n\t}\n```", "<pre><code>\tif x {\n\t}\n</code></pre>\n"},
		{"tilde fence", "~~~text\n```\n~~~", "<pre><code class=\"language-text\">```\n</code></pre>\n"},
		{"tight list", "- a\n- b\n  more\n- c", "<ul>\n<li>a</li>\n<li>b\nmore</li>\n<li>c</li>\n</ul>\n"},
		{"ordered list start", "3. a\n4. b", "<ol start=\"3\">\n<li>a</li>\n<li>b</li>\n</ol>\n"},
		{"loose list", "- a\n\n- b", "<ul>\n<li>\n<p>a</p>\n</li>\n<li>\n<p>b</p>\n</li>\n</ul>\n"},
		{"nested list", "- a\n  - b\n  - c\n- d", "<ul>\n<li>a\n<ul>\n<li>b</li>\n<li>c</li>\n</ul></li>\n<li>d</li>\n</ul>\n"},
		{"code in item", "1. step:\n\n   ```sh\n   x\n   ```\n2. next", "<ol>\n<li>\n<p>step:</p>\n<pre><code class=\"language-sh\">x\n</code></pre>\n</li>\n<li>\n<p>next</p>\n</li>\n</ol>\n"},
		{"two lists", "- a\n\n* b", "<ul>\n<li>a</li>\n</ul>\n<ul>\n<li>b</li>\n</ul>\n"},
		{"quote", "> a\n> b\n\n> c", "<blockquote>\n<p>a\nb</p>\n</blockquote>\n<blockquote>\n<p>c</p>\n</blockquote>\n"},
		{"table", "| A | B |\n|---|:-:|\n| 1 | `x\\|y` |\n| 2 |", "<div class=\"table-wrap\"><table>\n<thead><tr><th>A</th><th style=\"text-align:center\">B</th></tr></thead>\n<tbody>\n<tr><td>1</td><td style=\"text-align:center\"><code>x|y</code></td></tr>\n<tr><td>2</td><td style=\"text-align:center\"></td></tr>\n</tbody></table></div>\n"},
		{"not a table", "| a | b |\n|---|\n", "<p>| a | b |\n|---|</p>\n"},
		{"hr", "a\n\n---\n\nb", "<p>a</p>\n<hr>\n<p>b</p>\n"},
		{"hard break", "a  \nb\\\nc", "<p>a<br>\nb<br>\nc</p>\n"},
		{"reference link", "[x][r] and [r]\n\n[r]: https://e.com/", "<p><a href=\"https://e.com/\">x</a> and <a href=\"https://e.com/\">r</a></p>\n"},
		{"indented code", "    x := 1\n    y", "<pre><code>x := 1\ny\n</code></pre>\n"},
		{"inline html", "press <kbd>Esc</kbd><br>", "<p>press <kbd>Esc</kbd><br></p>\n"},
		{"duplicate headings", "# A\n# A\n# A", ""},
		{"setext headings", "Title\nmore title\n=====\n\nSub\n---\n", "<h1 id=\"title-more-title\">Title more title<a class=\"anchor\" href=\"#title-more-title\" aria-label=\"link to this heading\">#</a></h1>\n<h2 id=\"sub\">Sub<a class=\"anchor\" href=\"#sub\" aria-label=\"link to this heading\">#</a></h2>\n"},
		{"consecutive reference definitions", "[a] [b]\n\n[a]: /x\n[b]: <https://e.com/y>\n", "<p><a href=\"/x\">a</a> <a href=\"https://e.com/y\">b</a></p>\n"},
		{"text after a comment", "<!-- c --> text after\n\n<!-- c\nd --> more\n", "<p>text after</p>\n<p>more</p>\n"},
		{"pre with a blank line", "<pre>\na\n\n*b*\n</pre>\n\nafter", "<pre>\na\n\n*b*\n</pre>\n<p>after</p>\n"},
		{"lazy line after a fenced block in a quote", "> ```\n> x\n> ```\nlazy", "<blockquote>\n<pre><code>x\n</code></pre>\n</blockquote>\n<p>lazy</p>\n"},
		{"lazy line continues a quoted paragraph", "> a\nb", "<blockquote>\n<p>a\nb</p>\n</blockquote>\n"},
		{"code after five spaces in an item", "-     code\n", "<ul>\n<li><pre><code>code\n</code></pre></li>\n</ul>\n"},
		{"code of several lines in an item", "-     a\n      b\n", "<ul>\n<li><pre><code>a\nb\n</code></pre></li>\n</ul>\n"},
		{"lazy line does not continue an open fence", "> ```\n> code\nafter", "<blockquote>\n<pre><code>code\n</code></pre>\n</blockquote>\n<p>after</p>\n"},
		{"details block", "<details>\n<summary>S</summary>\n\ntext\n\n</details>", "<details>\n<summary>S</summary>\n<p>text</p>\n</details>\n"},
		{"rule of three", "*foo**bar*", "<p><em>foo**bar</em></p>\n"},
		{"rule of three, strong", "**foo*bar***", "<p><strong>foo<em>bar</em></strong></p>\n"},
		{"entities", "&#x1F600; &#12345678; &amp; &copy; &x", "<p>&#x1F600; &amp;#12345678; &amp; &copy; &amp;x</p>\n"},
		{"bare urls", "(see https://e.com/x). and https://e.com/y, then.", "<p>(see <a href=\"https://e.com/x\">https://e.com/x</a>). and <a href=\"https://e.com/y\">https://e.com/y</a>, then.</p>\n"},
		{"escaped pipe in a code span of a cell", "| a | b |\n|---|---|\n| `x\\|y` | z |", "<div class=\"table-wrap\"><table>\n<thead><tr><th>a</th><th>b</th></tr></thead>\n<tbody>\n<tr><td><code>x|y</code></td><td>z</td></tr>\n</tbody></table></div>\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := render(c.in)
			if c.want == "" { // only the ids
				for _, id := range []string{`id="a"`, `id="a-1"`, `id="a-2"`} {
					if !strings.Contains(got, id) {
						t.Errorf("missing %s in\n%s", id, got)
					}
				}
				return
			}
			if got != c.want {
				t.Errorf("render(%q)\n got: %q\nwant: %q", c.in, got, c.want)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"1. Call a C function":                       "1-call-a-c-function",
		"D12. Must for errors":                       "d12-must-for-errors",
		"Use `std::vector`, `std::map`":              "use-stdvector-stdmap",
		"`gocpp doc`: documentation in the terminal": "gocpp-doc-documentation-in-the-terminal",
		"Libraries / --buildmode":                    "libraries----buildmode",
	} {
		_, r := renderMarkdown("# "+in, func(h string, _ int) string { return h })
		if got := r.headings[0].ID; got != want {
			t.Errorf("%q: id %q, want %q", in, got, want)
		}
	}
}

// TestLinkFuncOrder: every link and image calls the rewrite function once, with its line.
func TestLinkFuncOrder(t *testing.T) {
	var seen []string
	renderMarkdown("a [x](one.md)\n\n- [y](two.md#h) ![i](three.png)\n\n| t |\n|---|\n| [z](four.md) |\n", func(h string, line int) string {
		seen = append(seen, h+":"+string(rune('0'+line)))
		return h
	})
	want := "one.md:1 two.md#h:3 three.png:3 four.md:7"
	if got := strings.Join(seen, " "); got != want {
		t.Errorf("links %q, want %q", got, want)
	}
}

var (
	codeBlockRx = regexp.MustCompile(`(?s)<pre.*?</pre>|<code>.*?</code>`)
	tagStripRx  = regexp.MustCompile(`<[^>]+>`)
)

// TestRenderRealDocs renders every doc of the repository and looks for what a renderer that does
// not know a construct leaves behind: fences, list markers and emphasis marks in the text.
func TestRenderRealDocs(t *testing.T) {
	root := filepath.Join("..", "..", "docs")
	n := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		n++
		html, r := renderMarkdown(string(b), func(h string, _ int) string { return h })
		for _, pr := range r.problems {
			t.Errorf("%s:%s", p, strings.TrimPrefix(pr, "line "))
		}
		text := tagStripRx.ReplaceAllString(codeBlockRx.ReplaceAllString(html, ""), "")
		for i, l := range strings.Split(text, "\n") {
			l = strings.TrimSpace(l)
			switch {
			case strings.HasPrefix(l, "```"), strings.HasPrefix(l, "~~~"):
				t.Errorf("%s: a code fence in the text: %q", p, l)
			case strings.HasPrefix(l, "- "), strings.HasPrefix(l, "> "), strings.HasPrefix(l, "|---"), strings.HasPrefix(l, "| ---"):
				t.Errorf("%s: a block marker in the text (line %d of the text): %q", p, i, l)
			}
		}
		if strings.Contains(html, "\x00") {
			t.Errorf("%s: an escape mark was left in the output", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 100 {
		t.Errorf("only %d docs found", n)
	}
}

// TestUniqueIDs: the ids of the headings of a page are all different, also when a heading is
// called like the id that a repeated heading got.
func TestUniqueIDs(t *testing.T) {
	_, r := renderMarkdown("# Dup\n# Dup\n# Dup 1\n# Dup 1\n# Dup\n## A-1\n## A\n## A\n", func(h string, _ int) string { return h })
	seen := map[string]bool{}
	for _, h := range r.headings {
		if seen[h.ID] {
			t.Errorf("the id %q is used twice: %v", h.ID, r.headings)
		}
		seen[h.ID] = true
	}
	if r.headings[0].ID != "dup" || r.headings[1].ID != "dup-1" {
		t.Errorf("the first two ids of a repeated heading are GitHub's: %v", r.headings)
	}
}

// TestTitleIsTheFirstH1: the title of a page is its first level-1 heading, not a level-2 heading
// before it and not a later level-1 heading.
func TestTitleIsTheFirstH1(t *testing.T) {
	_, r := renderMarkdown("## A\n# B\n# C\n", func(h string, _ int) string { return h })
	if r.title != "B" {
		t.Errorf("title %q", r.title)
	}
	if _, r := renderMarkdown("## only\n", func(h string, _ int) string { return h }); r.title != "" {
		t.Errorf("a page with no level-1 heading has the title %q", r.title)
	}
}

// TestUnsupportedConstructs: what the renderer cannot render faithfully is reported (docsite
// -check fails on it) and not dropped silently.
func TestUnsupportedConstructs(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"unclosed comment", "a\n\n<!-- never closed\ntext", "an HTML comment is not closed"},
		{"unclosed fence", "```go\nx := 1\n\ntext", "a code fence is not closed"},
		{"footnote", "text[^1]\n\n[^1]: note", "footnote"},
		{"undefined reference", "see [x][nope]", "the link reference [nope] is not defined"},
		{"table with extra cells", "| a |\n|---|\n| 1 | 2 |", "a table row has 2 cells, more than the header's 1"},
		{"table with a different delimiter row", "| a | b |\n|---|\n| 1 | 2 |", "a table header has 2 cells and its delimiter row 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, r := renderMarkdown(c.in, func(h string, _ int) string { return h })
			if !strings.Contains(strings.Join(r.problems, "\n"), c.want) {
				t.Errorf("problems %q, want one with %q", r.problems, c.want)
			}
		})
	}
	// A page that is fine has none, also with a shortcut reference that is not defined.
	_, r := renderMarkdown("a [maybe] b [x](y)\n\n| a | b |\n|---|---|\n| 1 |\n\n```\nok\n```\n", func(h string, _ int) string { return h })
	if len(r.problems) != 0 {
		t.Errorf("problems for a good page: %q", r.problems)
	}
}
