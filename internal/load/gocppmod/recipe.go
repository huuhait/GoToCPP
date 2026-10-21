package gocppmod

import (
	"fmt"
	"go/build/constraint"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// The recipe of a C or C++ library (Phase 06 step 6.1d, docs/spec/modules.md
// "C and C++ libraries", ADR-024, ADR-129).
//
// A recipe is the body of a brace block of gocpp.mod:
//
//	cxx github.com/ocornut/imgui {
//		src      imgui.cpp imgui_draw.cpp backends/*.cpp
//		exclude  backends/*vulkan*
//		include  . backends
//		define   IMGUI_DISABLE_OBSOLETE_FUNCTIONS
//		if linux {
//			pkg-config glfw3
//		}
//	}
//
// gocpp.mod itself keeps the block opaque (Block.Raw, see parseBraceBody), so
// that other tools and newer gocpp versions can share the file. This file
// reads the lines of one block: a statement is a verb and its arguments, which
// are the words of the line as in a //gocpp: directive (white space separates,
// quotes and backslashes keep it, // starts a comment). The statements of an
// if block, if cond { ... } else if cond { ... } else { ... }, apply only to the
// targets for which the condition holds; a condition is a //go:build
// constraint (go/build/constraint), so linux || darwin, !windows and
// windows && amd64 work, and it is evaluated against the tags of the target
// (Cond.Match). A line that gocpp does not understand is an error at its
// line and column: ignoring a flag would build the wrong library. Nothing
// here looks at the file system and nothing is executed: the paths are only
// checked as text (relative, no "..", no drive letters), the flags are checked
// by the build with the flag allow-list (build.ResolveRecipe).

// Limits on a recipe, because a recipe can come from a dependency.
const (
	// MaxRecipeLine is the longest line of a recipe, in bytes.
	MaxRecipeLine = 16 << 10

	// MaxRecipeStatements is the number of statements of a recipe, and
	// MaxRecipeArgs the number of arguments of one.
	MaxRecipeStatements = 4096
	MaxRecipeArgs       = 4096

	// MaxRecipeIfDepth is how deep if blocks nest, and MaxRecipeConditions
	// the number of conditions (if and else if) of a recipe.
	MaxRecipeIfDepth    = 4
	MaxRecipeConditions = 1024

	// MaxRecipeErrors is the number of errors that ParseRecipe reports; it
	// stops after them, with one more that says so.
	MaxRecipeErrors = 100
)

// The statements of a recipe.
const (
	RecipeSrc           = "src"            // sources to compile
	RecipeExclude       = "exclude"        // sources to leave out
	RecipeInclude       = "include"        // -I folders, for the library and its users
	RecipeSystemInclude = "system-include" // -isystem folders, for the library and its users
	RecipeDefine        = "define"         // -D, for the library and its users
	RecipePrivateDefine = "private-define" // -D for the library only
	RecipeCflags        = "cflags"         // flags of every compile of the library
	RecipeCxxflags      = "cxxflags"       // flags of the C++ compiles of the library
	RecipeLdflags       = "ldflags"        // flags of the link
	RecipeLink          = "link"           // -l libraries of the link
	RecipePkgConfig     = "pkg-config"     // pkg-config packages: compile flags for both, libs for the link
	RecipeStd           = "std"            // the C++ standard of the library
	RecipeCstd          = "cstd"           // the C standard of the library
)

// recipeVerbs are the statements, with what their arguments are called in the
// usage message.
var recipeVerbs = map[string]string{
	RecipeSrc: "file...", RecipeExclude: "file...", RecipeInclude: "folder...", RecipeSystemInclude: "folder...",
	RecipeDefine: "NAME[=value]...", RecipePrivateDefine: "NAME[=value]...",
	RecipeCflags: "flag...", RecipeCxxflags: "flag...", RecipeLdflags: "flag...",
	RecipeLink: "library...", RecipePkgConfig: "package...", RecipeStd: "c++17", RecipeCstd: "c11",
}

// A Recipe is the statements of one cxx block.
type Recipe struct {
	// Module is the module path of the cxx line.
	Module string

	// File is the name of the gocpp.mod file, for messages.
	File string

	// Stmts are the statements, in file order.
	Stmts []*RecipeStmt

	// Tags are the tags that the conditions name, each once, at the place of
	// its first use: the build can warn about one that nobody defines.
	Tags []RecipeTag
}

// A RecipeTag is a tag in the condition of an if or an else if.
type RecipeTag struct {
	Name      string
	Line, Col int // the place in gocpp.mod, from 1 (Col counts runes)
}

// A RecipeStmt is one statement of a recipe.
type RecipeStmt struct {
	Line int    // the line in gocpp.mod, from 1
	Verb string // src, include, ...
	Args []string
	Cond Cond   // when the statement applies, which is always if empty
	Text string // the line as written, without its comment and its indentation
}

// A Cond is the condition under which a statement applies: the conditions of
// the if blocks around it, and the negation of those of the earlier branches
// of an else, joined by &&. Its expression is a //go:build constraint.
type Cond struct {
	expr constraint.Expr // nil: always
}

// Match reports whether the condition holds for the target whose tags tag
// answers for (the loader's, load.BuildTags). An empty condition holds for
// every target.
func (c Cond) Match(tag func(name string) bool) bool {
	return c.expr == nil || c.expr.Eval(tag)
}

// String is the condition as a constraint ("linux || darwin", "!windows &&
// amd64"), or "" for a statement that always applies.
func (c Cond) String() string {
	if c.expr == nil {
		return ""
	}
	return c.expr.String()
}

var (
	macroNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	libNameRE   = regexp.MustCompile(`^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$`)
	cstdRE      = regexp.MustCompile(`^(c|gnu)(89|90|99|11|17|18|23|2x|2y)$`)

	// oldCondRE is the condition of the form that ADR-129 removed: os,os: or !os:
	// at the start of a statement, with the verb glued to it or not.
	oldCondRE = regexp.MustCompile(`^(!?)([a-z0-9_]+(?:,[a-z0-9_]+)*):([a-z-]*)$`)
)

// Recipes returns the recipes of the file: the brace blocks of the form
// cxx module/path { ... }, by module path, as blocks (RecipeOf reads one).
// A block of another shape (cxx a b {), a second block for the same module
// and a path that is not a module path are errors, at the line of the
// block; a block that is not closed was reported by Parse, and is left out.
func (f *File) Recipes() (map[string]*Block, ErrorList) {
	var errs ErrorList
	byPath := make(map[string]*Block)
	for _, s := range f.Unknown {
		b, ok := s.(*Block)
		if !ok || b.Open != '{' || b.Token[0] != "cxx" || b.Unterminated {
			continue
		}
		bad := func(format string, args ...any) {
			errs = append(errs, Error{Filename: f.Syntax.Name, Pos: b.Start, Err: fmt.Sprintf(format, args...)})
		}
		if len(b.Token) != 2 {
			bad("usage: cxx module/path { ... }")
			continue
		}
		p, why := unquote(b.Token[1])
		if why == "" {
			if why = CheckPath(p); why != "" {
				why = "invalid module path " + fmt.Sprintf("%q", p) + ": " + why
			}
		}
		if why != "" {
			bad("cxx %s { ... }: %s", b.Token[1], why)
			continue
		}
		if old := byPath[p]; old != nil {
			bad("multiple cxx recipes for %s (the first is at line %d)", p, old.Start.Line)
			continue
		}
		byPath[p] = b
	}
	return byPath, errs
}

// ParseRecipe reads the lines of the cxx block b of the file name, which is
// the recipe of the module path module. It returns the statements that could be
// read and an error for each line that could not (at its line and column in
// the file), and never stops at the first one; after MaxRecipeErrors errors it
// stops, with one more error that says so.
func ParseRecipe(name, module string, b *Block) (*Recipe, ErrorList) {
	p := &recipeParser{name: name, module: module, r: &Recipe{Module: module, File: name}}
	for i, line := range b.Raw {
		p.readLine(b.Start.Line+1+i, line)
		if p.stop {
			break
		}
	}
	if !p.stop {
		for _, f := range p.stack {
			p.failAt(f.line, f.text, f.off, "the %s is not closed: a } is missing", f.what)
		}
	}
	// The errors of unclosed blocks come last, but belong to their lines.
	slices.SortStableFunc(p.errs, func(a, b Error) int { return a.Pos.Line - b.Pos.Line })
	return p.r, p.errs
}

// A recipeParser reads the lines of one recipe, one after the other.
type recipeParser struct {
	name, module string
	r            *Recipe
	errs         ErrorList

	// stack are the blocks that are open, innermost last.
	stack []*ifFrame

	// conds is the number of conditions read, for the limit.
	conds int

	// tagSeen are the tags of the conditions that are in Recipe.Tags already.
	tagSeen map[string]bool

	// stop is true after an error that ends the reading (a limit).
	stop bool
}

// An ifFrame is an open block of a recipe: the branch of an if chain that
// is being read, or a block that is not an if (an error was reported for it,
// and its lines are read on so that its } is not another error).
type ifFrame struct {
	line, off int    // where the block opens: the line, and the byte offset of its keyword
	text      string // the text of that line
	what      string // "if block", "else block", "else if block" or "block"

	// outer is the condition of the enclosing branch (nil: none), prior the
	// conditions of the earlier branches of this chain, ored (nil: this is the
	// first branch), and cur the condition of the current branch: outer, and
	// not prior, and the condition written for it.
	outer, prior, cur constraint.Expr

	// written is the condition of the current branch as written (nil for else).
	written constraint.Expr

	hasElse bool // the chain has its else: no branch can follow
}

// wholeLine is the offset that failAt takes for an error about the whole
// line, which has no column.
const wholeLine = -1

// failAt reports an error at the line no, whose text is line, at the byte
// offset off in it (wholeLine: no column). The text of the message starts with
// the module.
func (p *recipeParser) failAt(no int, line string, off int, format string, args ...any) {
	if p.stop {
		return
	}
	pos := Position{Line: no}
	if off >= 0 && off <= len(line) {
		pos.LineRune = runeCol(line, off)
	}
	if len(p.errs) >= MaxRecipeErrors {
		p.errs = append(p.errs, Error{Filename: p.name, Pos: pos,
			Err: fmt.Sprintf("cxx %s: more than %d errors in the recipe: the rest is not read",
				p.module, MaxRecipeErrors)})
		p.stop = true
		return
	}
	p.errs = append(p.errs, Error{Filename: p.name, Pos: pos,
		Err: fmt.Sprintf("cxx %s: ", p.module) + fmt.Sprintf(format, args...)})
}

// cond is the condition that holds in the innermost block, or nil.
func (p *recipeParser) cond() constraint.Expr {
	if len(p.stack) == 0 {
		return nil
	}
	return p.stack[len(p.stack)-1].cur
}

// push opens a block.
func (p *recipeParser) push(f *ifFrame) {
	p.stack = append(p.stack, f)
}

// pushOther opens a block that is not an if (after an error), in which cond
// holds. The line no, whose text is line, has its keyword at the offset off.
func (p *recipeParser) pushOther(no int, line string, off int, cond constraint.Expr) {
	p.push(&ifFrame{line: no, off: off, text: line, what: "block", outer: cond, cur: cond})
}

// pop closes the innermost block, if there is one, and returns it.
func (p *recipeParser) pop() *ifFrame {
	if len(p.stack) == 0 {
		return nil
	}
	top := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	return top
}

// readLine reads the line number no of the file, whose text is line.
func (p *recipeParser) readLine(no int, line string) {
	fail := func(off int, format string, args ...any) {
		p.failAt(no, line, off, format, args...)
	}
	if len(line) > MaxRecipeLine {
		fail(wholeLine, "the line is too long (%d bytes, the limit is %d)", len(line), MaxRecipeLine)
		return
	}
	if !utf8.ValidString(line) {
		fail(wholeLine, "the line is not valid UTF-8")
		return
	}
	if j := strings.IndexFunc(line, func(c rune) bool { return c < ' ' && c != '\t' || c == 0x7f }); j >= 0 {
		fail(j, "the line has a control character (%q)", line[j])
		return
	}
	words, comment := scanWords(line)
	if len(words) == 0 {
		return // blank or a comment
	}
	text := line
	if comment >= 0 {
		text = line[:comment]
	}
	text = strings.TrimSpace(text)
	first, last := words[0], words[len(words)-1]
	opens := opensBlock(line, words)
	switch {
	case first.text == "}":
		p.closeLine(no, line, words, opens)
	case strings.HasPrefix(first.text, "}"):
		fail(first.off, `a "}" is a word of its own: write "} else {" with a space`)
		p.pop() // as the file reader does: the block is closed
		if opens {
			p.pushOther(no, line, first.off, p.cond())
		}
	case first.text == "if":
		p.ifLine(no, line, words, opens)
	case len(first.text) > 2 && strings.HasPrefix(first.text, "if") &&
		strings.ContainsAny(first.text[2:3], "(!"):
		fail(first.off, `write a space after "if": if condition {`)
		if opens {
			p.pushOther(no, line, first.off, p.cond())
		}
	case first.text == "else" || first.text == "else{":
		fail(first.off, `"else" without "if": it goes after the closing brace of an if block, `+
			`on the same line (} else {)`)
		if opens {
			p.pushOther(no, line, first.off, p.cond())
		}
	case opens:
		fail(last.off, "%q: a block in a recipe is an if block (if condition { ... }); "+
			"other blocks are not part of the recipe language", last.text)
		p.pushOther(no, line, last.off, p.cond())
	default:
		if len(p.r.Stmts) >= MaxRecipeStatements {
			fail(wholeLine, "the recipe has more than %d statements", MaxRecipeStatements)
			p.stop = true
			return
		}
		if s := p.stmt(no, line, text, words); s != nil {
			s.Cond = Cond{p.cond()}
			p.r.Stmts = append(p.r.Stmts, s)
		}
	}
}

// opensBlock reports whether the line, whose words are words, opens a block
// for the file reader (parseBraceBody): its last word is {; or it is an if
// line or an else line (the first word, or the one after a }) whose last word
// ends with { (if windows{), or whose { is in the comment at its end
// (if //go:build windows {). The reader and the recipe reader both ask this,
// so that they agree on where a block closes; the recipe reader reports the
// { that is not where it belongs.
func opensBlock(line string, words []word) bool {
	last := words[len(words)-1]
	if last.text == "{" {
		return true
	}
	head := words[0].text
	if head == "}" && len(words) > 1 {
		head = words[1].text
	}
	if head = strings.TrimSuffix(head, "{"); head != "if" && head != "else" {
		return false
	}
	if strings.HasSuffix(last.text, "{") && !strings.ContainsAny(last.text, "\"'`") {
		return true
	}
	return strings.HasSuffix(strings.TrimRight(line, " \t\r"), "{")
}

// condition reads the condition of an if or an else if from the words of the
// line: the text between the word at index from (the if) and the { that ends
// the line, and records its tags (Recipe.Tags). It returns nil after
// reporting the error.
func (p *recipeParser) condition(no int, line string, words []word, from int) constraint.Expr {
	fail := func(off int, format string, args ...any) {
		p.failAt(no, line, off, format, args...)
	}
	open := words[len(words)-1] // the {
	start := words[from].off + len(words[from].text)
	text := line[start:open.off]
	cond := strings.TrimSpace(text)
	if cond == "" {
		fail(open.off, "%s without a condition: want %s condition {", words[from].text, words[from].text)
		return nil
	}
	p.conds++
	if p.conds > MaxRecipeConditions {
		fail(words[from].off, "the recipe has more than %d conditions", MaxRecipeConditions)
		p.stop = true
		return nil
	}
	condOff := start + strings.Index(text, cond)
	expr, err := constraint.Parse("//go:build " + cond)
	if err != nil {
		// The offset of a syntax error is in the text of the condition.
		off, msg := condOff, err.Error()
		if se, ok := err.(*constraint.SyntaxError); ok {
			off, msg = condOff+se.Offset, se.Err
		}
		fail(min(off, len(line)), "the condition %q is not valid: %s (a condition is a build constraint: "+
			"linux, !windows, linux || darwin, windows && amd64)", shorten(cond), msg)
		return nil
	}
	p.noteTags(no, line, condOff, cond)
	return expr
}

// noteTags adds the tags of the condition cond, which is at the byte offset
// off of the line no, to the tags of the recipe, each once.
func (p *recipeParser) noteTags(no int, line string, off int, cond string) {
	isTagChar := func(c byte) bool {
		return c == '_' || c == '.' || c >= 0x80 ||
			'0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
	}
	for i := 0; i < len(cond); {
		if !isTagChar(cond[i]) {
			i++
			continue
		}
		j := i
		for j < len(cond) && isTagChar(cond[j]) {
			j++
		}
		tag := cond[i:j]
		if !p.tagSeen[tag] {
			if p.tagSeen == nil {
				p.tagSeen = make(map[string]bool)
			}
			p.tagSeen[tag] = true
			p.r.Tags = append(p.r.Tags, RecipeTag{Name: tag, Line: no, Col: runeCol(line, off+i)})
		}
		i = j
	}
}

// shorten returns s, cut after 60 runes if it is longer, for a message.
func shorten(s string) string {
	const limit = 60
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	n := 0
	for i := range s {
		if n == limit {
			return s[:i] + "..."
		}
		n++
	}
	return s
}

// ifLine reads an if line: if condition {. opens is true if the file reader
// sees the line open a block.
func (p *recipeParser) ifLine(no int, line string, words []word, opens bool) {
	first, last := words[0], words[len(words)-1]
	if last.text != "{" {
		msg := "an if line ends with { (if condition {), and the statements go on the lines after it"
		switch {
		case opens && strings.HasSuffix(last.text, "{"):
			msg = `put a space before the "{" at the end of an if line: if condition {`
		case opens:
			msg = `the "{" of an if line is not in a comment: if condition {`
		}
		p.failAt(no, line, last.off, "%s", msg)
		if opens {
			p.pushOther(no, line, first.off, p.cond())
		}
		return
	}
	f := &ifFrame{line: no, off: first.off, text: line, what: "if block", outer: p.cond()}
	if len(p.stack) >= MaxRecipeIfDepth {
		p.failAt(no, line, first.off, "if blocks nest more than %d deep: "+
			"write a flat list of conditions (if linux && arm64 { ... })", MaxRecipeIfDepth)
	}
	f.written = p.condition(no, line, words, 0)
	f.cur = and(f.outer, f.written)
	p.push(f)
}

// closeLine reads a line that starts with }: it closes the innermost block,
// and may be } else {, or } else if condition { that opens the next branch.
// opens is true if the file reader sees the line open a block.
func (p *recipeParser) closeLine(no int, line string, words []word, opens bool) {
	first := words[0]
	top := p.pop()
	if top == nil {
		p.failAt(no, line, first.off, "unexpected }: no if block is open")
		if opens {
			p.pushOther(no, line, first.off, nil)
		}
		return
	}
	if len(words) == 1 {
		return
	}
	next, last := words[1], words[len(words)-1]
	if next.text != "else" && next.text != "else{" {
		p.failAt(no, line, next.off,
			"after a } only else may follow on the line: put %q on a line of its own", next.text)
		if opens {
			p.pushOther(no, line, first.off, top.outer)
		}
		return
	}
	// } else {   or   } else if condition {
	if last.text != "{" {
		msg := "an else line ends with { (} else {, or } else if condition {)"
		switch {
		case opens && strings.HasSuffix(last.text, "{"):
			msg = `put a space before the "{" at the end of an else line`
		case opens:
			msg = `the "{" of an else line is not in a comment`
		}
		p.failAt(no, line, last.off, "%s", msg)
		if opens {
			p.pushOther(no, line, first.off, top.outer)
		}
		return
	}
	var frame *ifFrame
	switch {
	case len(words) == 3:
		frame = &ifFrame{line: no, off: next.off, text: line, what: "else block"}
	case words[2].text == "if":
		frame = &ifFrame{line: no, off: next.off, text: line, what: "else if block"}
		frame.written = p.condition(no, line, words, 2)
	default:
		p.failAt(no, line, words[2].off,
			"after else comes { or if (} else {, or } else if condition {), not %q", words[2].text)
		p.pushOther(no, line, first.off, top.outer)
		return
	}
	frame.outer = top.outer
	frame.prior = top.prior
	if top.written != nil {
		frame.prior = or(frame.prior, top.written)
	}
	frame.hasElse = top.hasElse
	if top.hasElse {
		p.failAt(no, line, next.off, "else after else: the else branch is the last of its if block")
	}
	if frame.what == "else block" {
		frame.hasElse = true
	}
	frame.cur = and(frame.outer, not(frame.prior))
	frame.cur = and(frame.cur, frame.written)
	p.push(frame)
}

// and returns the condition that holds when a and b do; nil is always.
func and(a, b constraint.Expr) constraint.Expr {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	return &constraint.AndExpr{X: a, Y: b}
}

// or returns the condition that holds when a or b does; a is nil for none.
func or(a, b constraint.Expr) constraint.Expr {
	if a == nil {
		return b
	}
	return &constraint.OrExpr{X: a, Y: b}
}

// not returns the opposite of a, or nil (always) for none.
func not(a constraint.Expr) constraint.Expr {
	if a == nil {
		return nil
	}
	return &constraint.NotExpr{X: a}
}

// oldCondition recognizes the condition of the form that ADR-129 removed at
// the start of a statement: linux:, linux,darwin:, !windows:, with a space
// after a comma or the verb glued to the colon (windows:link). It returns the
// names, and "!" if they are negated.
func oldCondition(words []word) (names []string, neg string, ok bool) {
	text := words[0].text
	for i := 1; strings.HasSuffix(text, ",") && i < len(words); i++ {
		text += words[i].text
	}
	m := oldCondRE.FindStringSubmatch(text)
	if m == nil {
		return nil, "", false
	}
	if _, isVerb := recipeVerbs[m[3]]; m[3] != "" && !isVerb {
		return nil, "", false
	}
	return strings.Split(m[2], ","), m[1], true
}

// stmt reads the words of one statement line. It reports an error and
// returns nil if the line is not valid.
func (p *recipeParser) stmt(no int, line, text string, words []word) *RecipeStmt {
	fail := func(off int, format string, args ...any) {
		p.failAt(no, line, off, format, args...)
	}
	for _, w := range words {
		switch w.text {
		case "{", "}", "(", ")":
			fail(w.off, "%q: blocks inside a recipe are not part of the recipe language "+
				"(an if block is if condition {, with the { at the end of the line "+
				"and the } on a line of its own)", w.text)
			return nil
		}
	}
	s := &RecipeStmt{Line: no, Text: text}
	first := words[0]
	if names, neg, ok := oldCondition(words); ok {
		for i := range names {
			names[i] = neg + names[i]
		}
		fail(first.off, "a condition is written as a block, if %s { ... }, since ADR-129: "+
			"the prefix before the statement is not part of the recipe language", strings.Join(names, " || "))
		return nil
	}
	s.Verb = first.text
	usage, ok := recipeVerbs[s.Verb]
	if !ok {
		fail(first.off, "unknown statement %q: gocpp does not know it, and ignoring it could build the "+
			"wrong library (src, exclude, include, system-include, define, private-define, cflags, "+
			"cxxflags, ldflags, link, pkg-config, std, cstd; if condition { ... } for a system)", s.Verb)
		return nil
	}
	args := words[1:]
	if len(args) == 0 {
		fail(first.off, "usage: %s %s", s.Verb, usage)
		return nil
	}
	if len(args) > MaxRecipeArgs {
		fail(first.off, "%s has more than %d arguments", s.Verb, MaxRecipeArgs)
		return nil
	}
	for _, w := range args {
		a, why := SplitWord(w.text)
		if why != "" {
			fail(w.off, "%s: %s in %s", s.Verb, why, w.text)
			return nil
		}
		s.Args = append(s.Args, a)
	}
	if msg := checkRecipeArgs(s); msg != "" {
		fail(first.off, "%s", msg)
		return nil
	}
	return s
}

// checkRecipeArgs checks the arguments of s as text. It returns the message
// or "". It rewrites the arguments of link to -l form.
func checkRecipeArgs(s *RecipeStmt) string {
	switch s.Verb {
	case RecipeSrc, RecipeExclude, RecipeInclude, RecipeSystemInclude:
		glob := s.Verb == RecipeSrc || s.Verb == RecipeExclude
		for i, a := range s.Args {
			p, why := CheckRecipePath(a, glob)
			if why != "" {
				return fmt.Sprintf("%s %q: %s", s.Verb, a, why)
			}
			s.Args[i] = p
		}
	case RecipeDefine, RecipePrivateDefine:
		for _, a := range s.Args {
			name, _, _ := strings.Cut(a, "=")
			if !macroNameRE.MatchString(name) {
				return fmt.Sprintf("%s %q: the name of a macro is letters, digits and _, not starting with a digit (NAME or NAME=value)", s.Verb, a)
			}
		}
	case RecipeLink:
		for i, a := range s.Args {
			lib := strings.TrimPrefix(a, "-l")
			if !libNameRE.MatchString(lib) {
				return fmt.Sprintf("link %q: want a library name (m, pthread, -lm)", a)
			}
			s.Args[i] = "-l" + lib
		}
	case RecipePkgConfig:
		for _, a := range s.Args {
			if msg := checkPkgConfigName(a); msg != "" {
				return msg
			}
			if strings.HasSuffix(strings.ToLower(a), ".pc") {
				return fmt.Sprintf("invalid pkg-config package name %q: a name that ends in .pc is a file, and a recipe can't name files (pkg-config finds the packages itself, in PKG_CONFIG_PATH)", a)
			}
		}
	case RecipeStd:
		if len(s.Args) != 1 || !stdRE.MatchString(s.Args[0]) {
			return fmt.Sprintf("invalid C++ standard %q: usage: std c++17 (also gnu++20, ...)", strings.Join(s.Args, " "))
		}
	case RecipeCstd:
		if len(s.Args) != 1 || !cstdRE.MatchString(s.Args[0]) {
			return fmt.Sprintf("invalid C standard %q: usage: cstd c11 (also gnu17, ...)", strings.Join(s.Args, " "))
		}
	}
	return ""
}

// CheckRecipePath checks a path of a recipe as text and returns it cleaned
// (slash-separated), or the reason it is not valid. A path is relative to the
// module's folder and stays in it: not absolute, with no ".." element, no \
// (write /), no ":" (a drive letter or a stream name on Windows) and no
// control character. With glob, the characters * ? [ are patterns that match
// inside one path element (** is not supported); else they are not allowed.
func CheckRecipePath(p string, glob bool) (string, string) {
	switch {
	case p == "":
		return "", "empty path"
	case strings.ContainsRune(p, 0):
		return "", "the path has a NUL byte"
	case strings.ContainsAny(p, `\`):
		return "", `use / in a path, not \`
	case strings.HasPrefix(p, "/"):
		return "", "an absolute path: a recipe's paths are inside the module (relative to its folder)"
	case strings.Contains(p, ":"):
		return "", `the path has a ":" (a drive letter or a stream name): a recipe's paths are inside the module`
	case strings.HasPrefix(p, "~"):
		return "", "a path that starts with ~ is not expanded: a recipe's paths are inside the module"
	case strings.Contains(p, "$"):
		return "", "$ is not expanded in a recipe's paths"
	}
	for _, c := range p {
		if c < ' ' || c == 0x7f {
			return "", "the path has a control character"
		}
	}
	for elem := range strings.SplitSeq(p, "/") {
		switch {
		case elem == "..":
			return "", `a ".." element: a recipe's paths are inside the module`
		case elem != "." && strings.TrimRight(elem, ". ") != elem:
			return "", fmt.Sprintf("the element %q ends in a dot or a space", elem)
		case strings.Contains(elem, "**"):
			return "", `"**" is not supported: a pattern matches inside one folder (src a/*.c b/*.c)`
		}
	}
	if !glob && strings.ContainsAny(p, "*?[") {
		return "", "a pattern is not allowed here (only src and exclude take patterns)"
	}
	if glob {
		if _, err := path.Match(p, ""); err != nil {
			return "", "bad pattern: " + err.Error()
		}
	}
	clean := path.Clean(p)
	if strings.HasPrefix(clean, "~") {
		return "", "a path that starts with ~ is not expanded: a recipe's paths are inside the module"
	}
	return clean, ""
}
