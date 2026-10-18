package gocppmod

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// The recipe of a C or C++ library (Phase 06 step 6.1d, docs/spec/modules.md
// "C and C++ libraries", ADR-024).
//
// A recipe is the body of a brace block of gocpp.mod:
//
//	cxx github.com/ocornut/imgui {
//		src      imgui.cpp imgui_draw.cpp backends/*.cpp
//		exclude  backends/*vulkan*
//		include  . backends
//		define   IMGUI_DISABLE_OBSOLETE_FUNCTIONS
//		linux:   pkg-config glfw3
//	}
//
// gocpp.mod itself keeps the block opaque (Block.Raw, see parseBraceBody), so
// that other tools and newer gocpp versions can share the file. This file
// reads the lines of one block: a statement, with an optional OS condition
// before it ("linux:", "linux,darwin:", "!windows:"), is a verb and its
// arguments, which are the words of the line as in a //gocpp: directive (white
// space separates, quotes and backslashes keep it, // starts a comment). A
// line that gocpp does not understand is an error at its line: ignoring a
// flag would build the wrong library. Nothing here looks at the file
// system and nothing is executed: the paths are only checked as text
// (relative, no "..", no drive letters), the flags are checked by the build
// with the flag allow-list (build.ResolveRecipe).

// Limits on a recipe, because a recipe can come from a dependency.
const (
	// MaxRecipeLine is the longest line of a recipe, in bytes.
	MaxRecipeLine = 16 << 10

	// MaxRecipeStatements is the number of statements of a recipe, and
	// MaxRecipeArgs the number of arguments of one.
	MaxRecipeStatements = 4096
	MaxRecipeArgs       = 4096
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
}

// A RecipeStmt is one line of a recipe.
type RecipeStmt struct {
	Line int    // the line in gocpp.mod, from 1
	Verb string // src, include, ...
	Args []string
	Cond Cond   // the OS condition, which matches every OS if empty
	Text string // the line as written, without its comment and its indentation
}

// A Cond is the condition of a statement: "linux:" (the OS is linux),
// "linux,darwin:" (one of them) or "!windows:" (not that one).
type Cond struct {
	OS  []string // empty: every OS
	Not bool     // the OS is none of OS
}

// Match reports whether the condition holds for the target system goos.
func (c Cond) Match(goos string) bool {
	if len(c.OS) == 0 {
		return true
	}
	return slices.Contains(c.OS, goos) != c.Not
}

func (c Cond) String() string {
	if len(c.OS) == 0 {
		return ""
	}
	s := strings.Join(c.OS, ",") + ":"
	if c.Not {
		s = "!" + s
	}
	return s
}

// knownOS are the operating systems of a condition: the GOOS values.
var knownOS = map[string]bool{
	"linux": true, "windows": true, "darwin": true, "freebsd": true, "netbsd": true, "openbsd": true,
	"dragonfly": true, "solaris": true, "illumos": true, "aix": true, "android": true, "ios": true,
	"js": true, "wasip1": true, "plan9": true,
}

var (
	macroNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	libNameRE   = regexp.MustCompile(`^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$`)
	cstdRE      = regexp.MustCompile(`^(c|gnu)(89|90|99|11|17|18|23|2x|2y)$`)
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
// read and an error for each line that could not (at its line in the file),
// and never stops at the first one.
func ParseRecipe(name, module string, b *Block) (*Recipe, ErrorList) {
	r := &Recipe{Module: module, File: name}
	var errs ErrorList
	for i, line := range b.Raw {
		no := b.Start.Line + 1 + i
		fail := func(format string, args ...any) {
			errs = append(errs, Error{Filename: name, Pos: Position{Line: no, LineRune: 1}, Err: fmt.Sprintf("cxx %s: ", module) + fmt.Sprintf(format, args...)})
		}
		if len(line) > MaxRecipeLine {
			fail("the line is too long (%d bytes, the limit is %d)", len(line), MaxRecipeLine)
			continue
		}
		if !utf8.ValidString(line) {
			fail("the line is not valid UTF-8")
			continue
		}
		if j := strings.IndexFunc(line, func(c rune) bool { return c < ' ' && c != '\t' || c == 0x7f }); j >= 0 {
			fail("the line has a control character (%q)", line[j])
			continue
		}
		words, comment := scanWords(line)
		if len(words) == 0 {
			continue // blank or a comment
		}
		text := line
		if comment >= 0 {
			text = line[:comment]
		}
		text = strings.TrimSpace(text)
		if len(r.Stmts) >= MaxRecipeStatements {
			fail("the recipe has more than %d statements", MaxRecipeStatements)
			break
		}
		if s := r.stmt(no, text, words, fail); s != nil {
			r.Stmts = append(r.Stmts, s)
		}
	}
	return r, errs
}

// stmt reads the words of one line. fail reports an error at the line.
func (r *Recipe) stmt(no int, text string, words []word, fail func(format string, args ...any)) *RecipeStmt {
	for _, w := range words {
		switch w.text {
		case "{", "}", "(", ")":
			fail("%q: blocks inside a recipe are not part of the recipe language", w.text)
			return nil
		}
	}
	s := &RecipeStmt{Line: no, Text: text}
	first := words[0].text
	if strings.Contains(first, ":") {
		if !strings.HasSuffix(first, ":") {
			fail("unknown statement %q (write an OS condition as a word of its own: linux: src x.cpp)", first)
			return nil
		}
		cond, why := parseCond(strings.TrimSuffix(first, ":"))
		if why != "" {
			fail("%s", why)
			return nil
		}
		s.Cond = cond
		words = words[1:]
		if len(words) == 0 {
			fail("the condition %s has no statement after it", first)
			return nil
		}
	}
	s.Verb = words[0].text
	usage, ok := recipeVerbs[s.Verb]
	if !ok {
		fail("unknown statement %q: gocpp does not know it, and ignoring it could build the wrong library (src, exclude, include, system-include, define, private-define, cflags, cxxflags, ldflags, link, pkg-config, std, cstd)", s.Verb)
		return nil
	}
	args := words[1:]
	if len(args) == 0 {
		fail("usage: %s %s", s.Verb, usage)
		return nil
	}
	if len(args) > MaxRecipeArgs {
		fail("%s has more than %d arguments", s.Verb, MaxRecipeArgs)
		return nil
	}
	for _, w := range args {
		a, why := SplitWord(w.text)
		if why != "" {
			fail("%s: %s in %s", s.Verb, why, w.text)
			return nil
		}
		s.Args = append(s.Args, a)
	}
	if msg := checkRecipeArgs(s); msg != "" {
		fail("%s", msg)
		return nil
	}
	return s
}

// parseCond reads the OS condition without its colon: os, os,os or !os.
func parseCond(c string) (Cond, string) {
	var cond Cond
	if rest, ok := strings.CutPrefix(c, "!"); ok {
		cond.Not, c = true, rest
	}
	for _, os := range strings.Split(c, ",") {
		if !knownOS[os] {
			return Cond{}, fmt.Sprintf("unknown operating system %q in the condition %q: want linux, windows, darwin, ...", os, c+":")
		}
		if slices.Contains(cond.OS, os) {
			return Cond{}, fmt.Sprintf("the operating system %q is repeated in the condition", os)
		}
		cond.OS = append(cond.OS, os)
	}
	if cond.Not && len(cond.OS) != 1 {
		return Cond{}, "a condition with ! names one operating system (!windows:)"
	}
	return cond, ""
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
