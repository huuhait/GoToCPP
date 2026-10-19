package main

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/lower"
)

// This file shows the C++ compiler's types and names in gocpp's terms (Phase 13 README §2.1,
// step 13.1). The C++ messages name what the emitter wrote:
//
//	gp::slice<int64_t>              []int
//	gp::string                      string
//	gp::map<gp::string, long>       map[string]int
//	gp::func<void(gp::ptr<T>)>      func(*T)
//	gp::chan<gp::any>               chan any
//	gpkg_main::Point                main.Point
//	gpkg_github_dcom__a__engine::T  engine.T   (the namespace is ADR-010's flat escape of the import path)
//
// C and C++ types keep their C++ names (std::vector<int>, const char*, point), because the user
// wrote them that way; only what gocpp wrote is turned back. The compiler's template noise
// goes too: the inline namespaces (std::__cxx11::), the default arguments (std::allocator<T>)
// and the long names of the string types (std::basic_string<char, std::char_traits<char>, ...>
// is std::string).

// A nameTranslator rewrites the C++ names in the text of a compiler message.
type nameTranslator struct {
	// pkgName returns the name that the package with the import path shows in messages, or ""
	// for the default: the last element of the path (without a version suffix). The driver
	// gives the package names, and the full path for a name that two packages share.
	pkgName func(path string) string
	// windows says that long is 32 bits (int64_t is long long): the target is Windows.
	windows bool
	// hints are the Go types that the .gp line of the error mentions, by their canonical form
	// (canonGo): the C++ type of an int and of an int64 is the same, and so is the one of a
	// channel in every direction, so the message alone cannot say which Go type it is about. A
	// type of the message that has one such spelling on the line is shown with it. Nil: unknown.
	hints map[string][]string
	// dirChans says that the line has a channel type with a direction (<-chan T, chan<- T): a
	// "chan T" of the message may be any of them, and shows with its C++ name if the line does not
	// say which. Without a directional channel on the line it is a plain chan T.
	dirChans bool

	// shortFile writes the name of a file of a message the way gocpp shows files; nil: as it is.
	shortFile func(string) string

	depth     int  // how deep in the runtime types the translation is
	cppScalar bool // show the ambiguous scalars and channels with C++ names (no hint was found)
}

// akaRx matches clang's "'T' (aka 'U')": the type as written, then what it is. The aka says
// the same again for a runtime type (translate shows both as the gocpp type), and for a C++
// template it only spells out what the type is made of ('const std::vector<std::string>' (aka
// 'const vector<basic_string<char>>')); a typedef of a plain type keeps its aka.
var akaRx = regexp.MustCompile(`'([^']*)' \(aka '([^']*)'\)`)

// dropAka returns s without the "(aka ...)" that say nothing more (see akaRx).
func dropAka(s string) string {
	if !strings.Contains(s, "(aka ") {
		return s
	}
	return akaRx.ReplaceAllStringFunc(s, func(m string) string {
		sub := akaRx.FindStringSubmatch(m)
		typ, aka := sub[1], sub[2]
		runtime := strings.HasPrefix(aka, "gp::") || bareRuntimeRx.MatchString(aka) || aka == "string" || aka == "any" || aka == "error"
		if runtime || strings.Contains(aka, "<") && strings.Contains(typ, "<") {
			return "'" + typ + "'"
		}
		return m
	})
}

// message returns the C++ compiler message s for a report: translate, and without the clang
// "(aka ...)" of a runtime type, which says the same again.
func (t *nameTranslator) message(s string) string {
	s = dropAka(s)
	s = warnFlagRx.ReplaceAllString(s, "")
	s = quotedRx.ReplaceAllStringFunc(s, func(q string) string {
		// A match takes the "<" that the next one needs before its name: run until no more.
		for {
			r := bareRuntimeRx.ReplaceAllString(q, "${1}gp::${2}<")
			if r == q {
				return q
			}
			q = r
		}
	})
	s = strings.ReplaceAll(s, "gp::callback<", "gp::cgo::callback<")
	s = t.translate(s)
	s = tagRx.ReplaceAllString(s, "'")
	s = anonIfaceRx.ReplaceAllString(s, "interface{...}")
	s = anonStructRx.ReplaceAllString(s, "struct{...}")
	if t.shortFile != nil && strings.Contains(s, "(lambda at ") {
		s = lambdaAtRx.ReplaceAllStringFunc(s, func(m string) string {
			sub := lambdaAtRx.FindStringSubmatch(m)
			return "(lambda at " + t.shortFile(sub[1]) + ":" + sub[2] + ")"
		})
	}
	return sameAkaRx.ReplaceAllStringFunc(s, func(m string) string {
		// gcc's {aka 'U'} and clang's (aka 'U') say nothing more once both are the gocpp type.
		if sub := sameAkaRx.FindStringSubmatch(m); sub[1] == sub[2] || strings.HasSuffix(sub[2], "."+sub[1]) && !strings.ContainsAny(sub[1], ".<>:") {
			return "'" + sub[2] + "'" // 'Node' (aka 'main.Node'): the second says all
		}
		return m
	})
}

var (
	// warnFlagRx matches the option that gcc puts at the end of a message: [-fpermissive] and
	// [-Wtemplate-body] say how to turn the error off, which no user of gocpp can.
	warnFlagRx = regexp.MustCompile(` \[-(?:fpermissive|Wtemplate-body)\]$`)
	// anonStructRx matches the C++ name of an anonymous struct type (struct{...} in Go).
	anonStructRx = regexp.MustCompile(`\bgp_anon_[0-9a-f]+\b`)
	// anonIfaceRx matches the C++ name of an anonymous interface type (interface{...} in Go).
	anonIfaceRx = regexp.MustCompile(`\bgp_anon_i[0-9a-f]+\b`)
	// lambdaAtRx matches clang's name of a lambda, which has the whole path of its file.
	lambdaAtRx = regexp.MustCompile(`\(lambda at ([^:)]+):(\d+:\d+)\)`)
	// sameAkaRx matches "'T' {aka 'U'}" and "'T' (aka 'U')".
	sameAkaRx = regexp.MustCompile(`'([^']*)' [({]aka '([^']*)'[)}]`)
)

// bareRuntimeRx matches a template of the runtime without its namespace in a quoted type:
// clang leaves out "gp::" at the outermost type of a name that came through a using declaration
// ('slice<long>', 'map<string, long>', 'callback<...>').
var bareRuntimeRx = regexp.MustCompile(`(^|[^\w:.>])(slice|array|ptr|cptr|map|chan|func|newtype|callback|complex)<`)

// tagRx matches the "class " that gcc writes before the name of a type in quotes ('class
// std::vector<long int>').
var tagRx = regexp.MustCompile(`'(?:class|struct|union|enum) `)

// translate returns s with the gocpp types and names in it rewritten (see the top of the file).
// Text that is no name of gocpp is not changed. A name that doesn't parse (a type that the
// compiler cut with "...") stays as it is.
func (t *nameTranslator) translate(s string) string {
	if !strings.Contains(s, "gp::") && !strings.Contains(s, "gpkg_") && !strings.Contains(s, "std::") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '"' { // a string literal of the C++ code is not a name
			if end := strings.IndexByte(s[i+1:], '"'); end >= 0 && !strings.Contains(s[i+1:i+1+end], "\\") {
				b.WriteString(s[i : i+end+2])
				i += end + 2
				continue
			}
		}
		j := i
		if strings.HasPrefix(s[i:], "::") && boundary(s, i, true) {
			j = i + 2 // a leading ::, as in ::gpkg_main::T
		}
		if (j > i || boundary(s, i, false)) && startsName(s[j:]) {
			if out, n, ok := t.parseName(s[j:]); ok {
				b.WriteString(out)
				i = j + n
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// boundary reports whether a name may start at s[i]: the byte before it is no part of a name.
// Before a leading "::" it also can't be ">" (a nested name after template arguments).
func boundary(s string, i int, leading bool) bool {
	if i == 0 {
		return true
	}
	c := s[i-1]
	if wordBefore(s, i) || c == ':' {
		return false
	}
	return !(leading && c == '>')
}

func isWordByte(c byte) bool {
	return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

// wordLen returns the length of the identifier character at the start of s (a letter or digit
// of Unicode, as gcc and clang accept in names, or _), or 0.
func wordLen(s string) int {
	if s == "" {
		return 0
	}
	if s[0] < utf8.RuneSelf {
		if isWordByte(s[0]) {
			return 1
		}
		return 0
	}
	if r, n := utf8.DecodeRuneInString(s); unicode.IsLetter(r) || unicode.IsDigit(r) {
		return n
	}
	return 0
}

// wordBefore reports whether an identifier character ends s[:i].
func wordBefore(s string, i int) bool {
	r, n := utf8.DecodeLastRuneInString(s[:i])
	if n == 1 {
		return isWordByte(s[i-1])
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func startsName(s string) bool {
	return strings.HasPrefix(s, "gp::") || strings.HasPrefix(s, "gpkg_") || strings.HasPrefix(s, "std::")
}

// A comp is one component of a qualified name: gp, slice<int>, ...
type comp struct {
	name    string
	args    []string // the template arguments, as written (trimmed)
	hasArgs bool
}

// parseName reads the qualified name at the start of s (startsName) and returns what it is
// shown as and the bytes it took. ok is false if the name is not complete (unbalanced <>).
func (t *nameTranslator) parseName(s string) (out string, n int, ok bool) {
	var comps []comp
	j := 0
	for {
		k := j
		for k < len(s) && wordLen(s[k:]) > 0 {
			k += wordLen(s[k:])
		}
		if k == j {
			return "", 0, false
		}
		c := comp{name: s[j:k]}
		j = k
		if j < len(s) && s[j] == '<' {
			end := matchAngle(s, j)
			if end < 0 {
				return "", 0, false
			}
			c.args, c.hasArgs = splitArgs(s[j+1:end]), true
			j = end + 1
		}
		comps = append(comps, c)
		if strings.HasPrefix(s[j:], "::") && j+2 < len(s) && (wordLen(s[j+2:]) > 0 && !('0' <= s[j+2] && s[j+2] <= '9')) {
			j += 2
			continue
		}
		break
	}
	switch {
	case comps[0].name == "gp" && len(comps) > 1:
		return t.gpName(comps), j, true
	case strings.HasPrefix(comps[0].name, "gpkg_"):
		if out, ok := t.pkgQualified(comps); ok {
			return out, j, true
		}
		return "", 0, false
	case comps[0].name == "std" && len(comps) > 1:
		return t.stdName(comps), j, true
	}
	return "", 0, false
}

// matchAngle returns the index of the ">" that closes the "<" at s[open], or -1. Parentheses
// and brackets nest too (a function type's arguments, an array bound), and a message that
// breaks off (a newline, "...") doesn't match.
func matchAngle(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '<', '(', '[':
			depth++
		case '>', ')', ']':
			depth--
			if depth == 0 {
				if s[i] != '>' {
					return -1
				}
				return i
			}
			if depth < 0 {
				return -1
			}
		case '\n', ';', '{', '}':
			return -1
		}
	}
	return -1
}

// splitArgs splits template arguments at the commas that are not inside <>, () or [].
func splitArgs(s string) []string {
	var args []string
	depth, from := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<', '(', '[', '{':
			depth++
		case '>', ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(s[from:i]))
				from = i + 1
			}
		}
	}
	if rest := strings.TrimSpace(s[from:]); rest != "" || len(args) > 0 {
		args = append(args, rest)
	}
	return args
}

// cppComps writes comps as C++ names, with the arguments translated (and std's default
// arguments dropped).
func (t *nameTranslator) cppComps(comps []comp) string {
	var b strings.Builder
	for i, c := range comps {
		if i > 0 {
			b.WriteString("::")
		}
		b.WriteString(c.name)
		if c.hasArgs {
			b.WriteByte('<')
			for k, a := range t.cppArgs(c) {
				if k > 0 {
					b.WriteString(", ")
				}
				b.WriteString(a)
			}
			b.WriteByte('>')
		}
	}
	return b.String()
}

// cppArgs returns the translated arguments of c, a component of a C++ name.
func (t *nameTranslator) cppArgs(c comp) []string {
	args := make([]string, len(c.args))
	for i, a := range c.args {
		args[i] = t.translate(a)
	}
	return args
}

// pkgDisplay returns the name that the package with the import path shows as.
func (t *nameTranslator) pkgDisplay(path string) string {
	if t.pkgName != nil {
		if n := t.pkgName(path); n != "" {
			return n
		}
	}
	return defaultPkgName(path)
}

var majorSuffixRx = regexp.MustCompile(`^v[0-9]+$`)

// defaultPkgName is the name of the package with the import path when nothing better is known:
// its last element, the one before it for a major version (a/b/v2 is b), without ".vN" (yaml.v3
// is yaml).
func defaultPkgName(path string) string {
	elems := strings.Split(path, "/")
	n := elems[len(elems)-1]
	if majorSuffixRx.MatchString(n) && len(elems) > 1 {
		n = elems[len(elems)-2]
	}
	if i := strings.LastIndex(n, ".v"); i > 0 && majorSuffixRx.MatchString(n[i+1:]) {
		n = n[:i]
	}
	return n
}

func (t *nameTranslator) pkgQualified0(comps []comp) (string, bool) {
	path, ok := lower.PackagePath(comps[0].name)
	if !ok || path == "" || comps[0].hasArgs {
		return "", false
	}
	var b strings.Builder
	b.WriteString(t.pkgDisplay(path))
	for _, c := range comps[1:] {
		b.WriteByte('.')
		b.WriteString(lower.Unescape(c.name))
		if c.hasArgs {
			b.WriteByte('[')
			for i, a := range c.args {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(t.goArg(a))
			}
			b.WriteByte(']')
		}
	}
	return b.String(), true
}

// gpName shows a name of the runtime: the types as gocpp types, the rest as C++.
func (t *nameTranslator) gpName(comps []comp) string {
	if len(comps) == 3 && comps[1].name == "cgo" && comps[2].name == "callback" && len(comps[2].args) == 2 && comps[2].args[1] != "..." {
		return t.goArg(comps[2].args[1]) // gp::cgo::callback<R(A), gp::func<R(A)>>: the Go function type
	}
	c := comps[1]
	if len(comps) == 2 {
		if s, ok := t.gpType(c); ok {
			return s
		}
	}
	return t.cppComps(comps)
}

// gpType returns the gocpp type for the runtime type c (gp::slice<T>, ...).
func (t *nameTranslator) gpType(c comp) (string, bool) {
	return t.resolved(func() (string, bool) { return t.gpType0(c) })
}

// pkgQualified shows gpkg_x::A::B<int64_t> as x.A.B[int].
func (t *nameTranslator) pkgQualified(comps []comp) (string, bool) {
	return t.resolved(func() (string, bool) { return t.pkgQualified0(comps) })
}

// resolved runs show, which writes a type of gocpp, and, for the outermost type, settles what
// the C++ type cannot say. A Go type that the line of the error mentions with the same shape
// (canonGo) is shown as the line has it ([]int64 for gp::slice<int64_t>, <-chan int for
// gp::chan<int64_t>). Without one, or with two that differ, the parts that are ambiguous show
// with C++ names (gp::chan<T>, int64_t): never a guess.
func (t *nameTranslator) resolved(show func() (string, bool)) (string, bool) {
	t.depth++
	s, ok := show()
	t.depth--
	if !ok || t.depth > 0 {
		return s, ok
	}
	if sp := t.hints[canonGo(s)]; len(sp) == 1 {
		return sp[0], true
	}
	if ambiguousRx.MatchString(s) {
		t.cppScalar = true
		s, ok = show()
		t.cppScalar = false
	}
	return s, ok
}

// ambiguousRx matches the Go types that have the same C++ type as others: int and int64, uint
// and uint64 and uintptr, and channels, whose direction C++ doesn't have.
var ambiguousRx = regexp.MustCompile(`\b(?:int|uint)\b|\bchan\b`)

var (
	canonIntRx  = regexp.MustCompile(`\b(?:int64|int)\b`)
	canonUintRx = regexp.MustCompile(`\b(?:uint64|uintptr|uint)\b`)
	canonDirRx  = regexp.MustCompile(`<-chan |chan<- `)
	canonByteRx = regexp.MustCompile(`\b(?:byte|uint8)\b`)
	canonRuneRx = regexp.MustCompile(`\b(?:rune|int32)\b`)
)

// canonGo returns a Go type with the spellings that have one C++ type unified: int64 is int,
// uint64 and uintptr are uint, byte is uint8, rune is int32, and a channel has no direction.
func canonGo(s string) string {
	s = canonDirRx.ReplaceAllString(s, "chan ")
	s = canonIntRx.ReplaceAllString(s, "int")
	s = canonUintRx.ReplaceAllString(s, "uint")
	s = canonByteRx.ReplaceAllString(s, "uint8")
	return canonRuneRx.ReplaceAllString(s, "int32")
}

// setHints sets the Go types that the message is about (see hints).
func (t *nameTranslator) setHints(spellings []string) {
	t.hints = make(map[string][]string)
	t.dirChans = false
	for _, sp := range spellings {
		if strings.Contains(sp, "<-chan ") || strings.Contains(sp, "chan<- ") {
			t.dirChans = true
		}
	}
	for _, sp := range spellings {
		k := canonGo(sp)
		if !slices.Contains(t.hints[k], sp) {
			t.hints[k] = append(t.hints[k], sp)
		}
	}
}

func (t *nameTranslator) gpType0(c comp) (string, bool) {
	a := c.args
	if slices.ContainsFunc(a, func(x string) bool { return x == "" || x == "..." }) {
		return "", false // the compiler cut the type
	}
	switch {
	case !c.hasArgs:
		switch c.name {
		case "string", "any", "error", "complex64", "complex128":
			return c.name, true
		}
	case c.name == "slice" && len(a) == 1:
		return "[]" + t.goArg(a[0]), true
	case c.name == "array" && len(a) == 2:
		return "[" + strings.TrimRight(a[1], "uUlL") + "]" + t.goArg(a[0]), true
	case (c.name == "ptr" || c.name == "cptr") && len(a) == 1:
		return "*" + t.goArg(a[0]), true
	case c.name == "complex" && len(a) == 1 && (a[0] == "float" || a[0] == "double"):
		return map[string]string{"float": "complex64", "double": "complex128"}[a[0]], true
	case c.name == "map" && len(a) == 2:
		return "map[" + t.goArg(a[0]) + "]" + t.goArg(a[1]), true
	case c.name == "chan" && len(a) == 1:
		if t.cppScalar && t.dirChans {
			return "gp::chan<" + t.goArg(a[0]) + ">", true // C++ has no direction: not "chan T"
		}
		return "chan " + t.goArg(a[0]), true
	case c.name == "newtype" && len(a) == 2:
		return t.goArg(a[0]), true
	case c.name == "func" && len(a) == 1:
		return t.funcType(a[0])
	}
	return "", false
}

// funcType shows the argument of gp::func, "R(A, B)", as func(A, B) R.
func (t *nameTranslator) funcType(sig string) (string, bool) {
	// The parameters start at the first "(" that is not inside <> (the result may be a
	// type like gp::func<void()>), and end at its ")".
	open, angle := -1, 0
	for i := 0; i < len(sig) && open < 0; i++ {
		switch sig[i] {
		case '<':
			angle++
		case '>':
			angle--
		case '(':
			if angle == 0 {
				open = i
			}
		}
	}
	if open < 0 {
		return "", false
	}
	closeIdx, depth := -1, 0
	for i := open; i < len(sig) && closeIdx < 0; i++ {
		switch sig[i] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				closeIdx = i
			}
		}
	}
	if closeIdx < 0 {
		return "", false
	}
	result := strings.TrimSpace(sig[:open])
	var params []string
	for _, p := range splitArgs(sig[open+1 : closeIdx]) {
		if p != "" && p != "void" {
			params = append(params, t.goArg(p))
		}
	}
	s := "func(" + strings.Join(params, ", ") + ")"
	switch {
	case result == "" || result == "void":
	case strings.HasPrefix(result, "std::tuple<") && strings.HasSuffix(result, ">"):
		var rs []string
		for _, r := range splitArgs(result[len("std::tuple<") : len(result)-1]) {
			rs = append(rs, t.goArg(r))
		}
		s += " (" + strings.Join(rs, ", ") + ")"
	default:
		s += " " + t.goArg(result)
	}
	return s, true
}

// goArg shows a template argument of a runtime type as a gocpp type: a C++ scalar type that
// the emitter writes for a gocpp number type is that number type (int64_t is int), and other
// text is translated like a message.
func (t *nameTranslator) goArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if s, ok := t.scalar(arg); ok {
		return s
	}
	return t.translate(arg)
}

// scalar returns the gocpp number type for the C++ scalar type name.
func (t *nameTranslator) scalar(name string) (string, bool) {
	s, ok := t.scalar0(name)
	if t.cppScalar { // int, int64 and C's long are one C++ type: say which one it is
		switch s {
		case "int":
			s = "int64_t"
		case "uint":
			s = "uint64_t"
		}
	}
	return s, ok
}

func (t *nameTranslator) scalar0(name string) (string, bool) {
	name = strings.Join(strings.Fields(strings.TrimPrefix(name, "const ")), " ")
	name = strings.TrimPrefix(name, "std::")
	if s, ok := scalarTypes[name]; ok {
		return s, true
	}
	long, ulong, longlong, ulonglong := "int", "uint", "int64", "uint64"
	if t.windows {
		long, ulong, longlong, ulonglong = "int32", "uint32", "int", "uint"
	}
	switch name {
	case "long", "long int", "signed long", "signed long int":
		return long, true
	case "unsigned long", "long unsigned int", "unsigned long int":
		return ulong, true
	case "long long", "long long int", "signed long long":
		return longlong, true
	case "unsigned long long", "long long unsigned int", "unsigned long long int":
		return ulonglong, true
	}
	return "", false
}

// scalarTypes maps the C++ names of the scalar types that don't depend on the target to gocpp's.
// int is int32 (the C++ int is int32_t; gocpp's int is int64_t), and uint8_t is byte.
var scalarTypes = map[string]string{
	"int64_t": "int", "uint64_t": "uint", "int32_t": "int32", "uint32_t": "uint32",
	"int16_t": "int16", "uint16_t": "uint16", "int8_t": "int8", "uint8_t": "byte", "uintptr_t": "uintptr",
	"int": "int32", "unsigned int": "uint32", "unsigned": "uint32",
	"short": "int16", "short int": "int16", "unsigned short": "uint16", "short unsigned int": "uint16",
	"signed char": "int8", "unsigned char": "byte",
	"double": "float64", "float": "float32", "bool": "bool",
}

// stdName shows a name of the standard library: the inline namespaces (__cxx11, libc++'s __1)
// go, the default arguments of the containers and strings go, and the string types get their
// short names.
func (t *nameTranslator) stdName(comps []comp) string {
	var kept []comp
	for _, c := range comps {
		if !c.hasArgs && (c.name == "__cxx11" || c.name == "__1" || c.name == "__debug") {
			continue
		}
		c.args = t.cppArgs(c)
		if defaultsDropped[c.name] {
			for len(c.args) > 1 && defaultArg(c.args[len(c.args)-1], c.args[0]) {
				c.args = c.args[:len(c.args)-1]
			}
		}
		kept = append(kept, c)
	}
	if last := kept[len(kept)-1]; len(kept) == 2 && (last.name == "basic_string" || last.name == "basic_string_view") && len(last.args) == 1 {
		prefix := map[string]string{"char": "", "wchar_t": "w", "char8_t": "u8", "char16_t": "u16", "char32_t": "u32"}
		if p, ok := prefix[last.args[0]]; ok {
			return "std::" + p + strings.TrimPrefix(last.name, "basic_")
		}
	}
	return writeTranslated(kept)
}

// defaultsDropped are the templates of std whose trailing default arguments (defaultArg) the
// compiler writes out, and that are shown without them.
var defaultsDropped = map[string]bool{
	"vector": true, "deque": true, "list": true, "forward_list": true, "set": true, "multiset": true,
	"map": true, "multimap": true, "unordered_set": true, "unordered_multiset": true,
	"unordered_map": true, "unordered_multimap": true, "basic_string": true, "basic_string_view": true,
	"queue": false, "stack": false,
}

// writeTranslated writes comps whose arguments are translated already.
func writeTranslated(comps []comp) string {
	var b strings.Builder
	for i, c := range comps {
		if i > 0 {
			b.WriteString("::")
		}
		b.WriteString(c.name)
		if c.hasArgs {
			b.WriteString("<" + strings.Join(c.args, ", ") + ">")
		}
	}
	return b.String()
}

// defaultArg reports whether a (translated) template argument of a container is the default that
// the compiler writes out, for a container whose first argument is first: std::less<first> (and
// hash and equal_to of it), std::allocator<first> (or of the pair<const first, V> of a map), and
// std::char_traits<char>. A std::less<void> or a hash of another type is the user's.
func defaultArg(a, first string) bool {
	inner := func(prefix string) (string, bool) {
		rest, ok := strings.CutPrefix(a, prefix)
		if !ok {
			return "", false
		}
		rest = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), ">"))
		return rest, true
	}
	for _, p := range []string{"std::less<", "std::hash<", "std::equal_to<"} {
		if x, ok := inner(p); ok {
			return x == first
		}
	}
	if x, ok := inner("std::allocator<"); ok {
		return x == first || strings.HasPrefix(x, "std::pair<const "+first+",")
	}
	if x, ok := inner("std::char_traits<"); ok {
		return x == first
	}
	return false
}

// symbolName shows a C++ function name, as the linker prints it ("gpkg_main::T::M(long)") as
// the gocpp name of the function (main.T.M): the parameters go.
func (t *nameTranslator) symbolName(sym string) string {
	if i := strings.IndexByte(sym, '('); i > 0 {
		sym = sym[:i]
	}
	sym = strings.TrimSpace(sym)
	out := t.translate(sym)
	if strings.HasPrefix(sym, "gpkg_") {
		out = strings.ReplaceAll(out, "::", ".")
	}
	return out
}
