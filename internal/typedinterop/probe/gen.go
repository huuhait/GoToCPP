// Package probe is step 12.2 of typed interop (Phase 12): the probe
// generator, the runner and the streaming reader of clang's AST dump.
//
// Generate turns the references of a package (collect.Set) into ONE probe
// translation unit: the package's #includes and, for each reference, a line
// that makes clang evaluate it,
//
//	using gp_probe_7 = gocpp_probe_internal_c<decltype((std::declval<std::vector<int64_t>&>().size()))>;
//
// Run compiles the unit with clang++ (-fsyntax-only, -ast-dump=json
// filtered to the probe names), reads the dump as a stream and returns a
// Table: for each reference, the canonical C++ type, the value category,
// constness, a constant value, or the error text of a failed probe.
//
// The contract, in short (docs/spec/typed-interop.md has the long form):
//
//   - One reference gives one expression probe (decltype((E)): a reference
//     type says "lvalue" or "xvalue", a plain type "prvalue"), or one type
//     probe when the reference names a type. A reference with a part that
//     isn't known to be a type or a value (the kind probe gave no answer)
//     gets a second, alternative line; the first that compiles wins.
//   - A reference in a constant context (collect.Ref.WantValue) or a macro
//     also gets a value probe (an integer through a template argument:
//     ConstantExpr.value; a float as its exact bits; a string literal read
//     from the AST).
//   - A probe that fails is not in the dump. The compiler's messages are
//     attributed to the probe line they are about (also errors in headers
//     caused by an instantiation that the line asked for, through the note
//     "requested here"), so a failure carries clang's error and the notes
//     that follow it (the overload candidates). Messages that belong to no
//     probe (an unrelated header with an error) are reported, and don't stop
//     the run.
//   - Every probe is independent: an error in one line leaves the others
//     alone (-ferror-limit=0). That is what the cache relies on (ADR-088): the
//     file is a preamble and one Unit per reference, a result is stored per
//     unit under the contents of the headers the preamble reads, and the units
//     that are not cached are run in batches (units.go, cache.go).
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/winpath"
)

// A Role says what a line of the probe file asks.
type Role string

const (
	RoleMain   Role = "main"   // gp_probe_N: the expression's type and value category, or the type that a name stands for
	RoleAlt    Role = "alt"    // gp_probe_N_alt: the other reading of an ambiguous reference
	RoleValue  Role = "value"  // gp_probe_N_v: the integer, float or bool value of a constant
	RoleString Role = "string" // gp_probe_N_s: the value of a string literal (read from the AST)
	RoleAlias  Role = "alias"  // gp_decl_H: the type of a local variable, named once
)

// A Form says whether a main probe is an expression or a type.
type Form string

const (
	FormExpr Form = "expr" // decltype((E))
	FormType Form = "type" // the type itself
)

// A Line is a line of the probe file that belongs to a reference.
type Line struct {
	Ref  int
	Role Role
	// Form is FormExpr or FormType for a main or alternative line.
	Form Form
	Text string
	// Needs are the names of the aliases that an alias line uses (RoleAlias only).
	Needs []string
}

// A GenConfig is what Generate needs besides the references.
type GenConfig struct {
	// Includes are the #include texts of the package, as the kind probe
	// takes them: <vector>, "./physics.h", interop.ExternC(...) for a header
	// that the build includes inside extern "C" { }.
	Includes []string

	// Kinds is the kind probe's answer (interop.ProbeKinds) for the paths of
	// Set.KindPaths. A path that is missing (or all of them, when Kinds is
	// nil) is of unknown kind: references that depend on it get both
	// readings.
	Kinds map[string]interop.Kind

	// StringToCStr makes the stand-in of a Go string convert to const char*
	// as it does to std::string_view (the automatic conversion of step 12.6).
	// By default the probe mirrors the generated code of Phase 04: only
	// std::string_view.
	StringToCStr bool
}

// A Source is a generated probe file.
type Source struct {
	// Text is the C++ file. The same references, includes and kinds always
	// give the same text.
	Text string

	// Lines maps a line number of Text (from 1) to what it asks for.
	Lines map[int]Line

	// Skipped maps the ID of a reference that has no probe to the reason.
	Skipped map[int]string

	// NRefs is the number of references (len of the Table's results), Keys
	// their keys.
	NRefs int
	Keys  []string

	// Probes is the number of lines of expression and type probes.
	Probes int

	// HelperFrom..HelperTo are the lines (from 1, inclusive) of the helper
	// block: the stand-ins and templates that every probe line uses. An error
	// of the compiler on one of them means that a header of the package
	// collides with a helper name, and the run fails.
	HelperFrom, HelperTo int

	// Includes are the headers of the #include lines, as written ("./a.hpp"
	// with the quotes, <vector> with the brackets).
	Includes []string

	// Preamble is the part of Text before the first probe line: the title, the
	// headers, the package's includes and the helper block. It is the context
	// of every probe: the cache keys it, and the headers it reads are the
	// dependencies of every result (cache.go).
	Preamble string

	// Units are the probes of the references that have any, one per reference, in
	// the order of Text. A unit holds everything a reference's answer depends
	// on, so that its result can be cached and reused in any other probe file with
	// the same preamble (units.go).
	Units []Unit

	// aliasAt is the line of each alias in Text.
	aliasAt map[string]int
}

// A Unit is the probe of one reference: the lines of the local-variable aliases
// it uses (all of them, also those that an earlier reference's lines declare in
// the full file) and its own lines (the main probe, the alternative reading, the
// value probes).
type Unit struct {
	Ref int
	// Aliases are the alias lines the unit's lines use, in the order that they
	// must be declared (a dependency comes first).
	Aliases []Line
	// Lines are the unit's own lines, and At their line numbers in Source.Text.
	Lines []Line
	At    []int
	// Hash identifies the unit's text without its reference number: the same
	// probe in another file has the same hash.
	Hash string
}

// unitHash returns the hash of the canonical text of a unit's lines.
func unitHash(aliases, lines []Line) string {
	h := sha256.New()
	for _, a := range aliases {
		fmt.Fprintf(h, "a%d:%s\n", len(a.Text), a.Text)
	}
	for _, l := range lines {
		t := canonLine(l)
		fmt.Fprintf(h, "%s %s %d:%s\n", l.Role, l.Form, len(t), t)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// canonLine returns the text of a probe line with the reference's number
// replaced in the name that the line declares (the first "gp_probe_N", always
// the declaration), so that two references with the same probe have the same text.
func canonLine(l Line) string {
	return strings.Replace(l.Text, "gp_probe_"+strconv.Itoa(l.Ref), "gp_probe_@", 1)
}

// helpers is the part of the probe file after the includes: the stand-ins of
// gocpp's types and the helper templates.
const helpers = `namespace gocpp_probe_internal_pr {
struct string {
  string() = default;
  template <size_t N> string(const char (&)[N]) noexcept;  // a literal copies (Phase 04, step 4.9)
  explicit string(std::string_view) noexcept;  // string(x) of a std::string or a std::string_view copies
  operator std::string_view() const noexcept;
  std::strong_ordering operator<=>(const string&) const noexcept;
  bool operator==(const string&) const noexcept;
%s  size_t size() const noexcept;
  const char* data() const noexcept;
  const uint8_t& operator[](size_t) const noexcept;
};
template <class T> struct slice {
  using value_type = T;
  slice() = default;
  slice(std::initializer_list<T>) noexcept;
  operator std::span<T>() const noexcept;
  T* begin() const noexcept;
  T* end() const noexcept;
  T* data() const noexcept;
  size_t size() const noexcept;
  T& operator[](size_t) const noexcept;
};
template <class T, size_t N> struct array {
  using value_type = T;
  array() = default;
  array(std::initializer_list<T>) noexcept;
  T* begin() const noexcept;
  T* end() const noexcept;
  T* data() const noexcept;
  constexpr size_t size() const noexcept { return N; }
  T& operator[](size_t) const noexcept;
};
template <class K, class V> struct map {
  using key_type = K;
  using mapped_type = V;
  map() = default;
  map(std::initializer_list<std::pair<const K, V>>) noexcept;
  size_t size() const noexcept;
  V& operator[](const K&) const noexcept;
};
template <class T> constexpr decltype(auto) deref(T&& t) {
  if constexpr (std::is_pointer_v<std::remove_cvref_t<T>>) return (*t);
  else return std::forward<T>(t);
}
using std::begin;  // begin(c) finds std::begin and the begin of c's own namespace (ADL)
template <class C, class = void> struct range_t {
  using key = int64_t;
  using val = std::decay_t<decltype(*begin(std::declval<C&>()))>;
};
template <class C> struct range_t<C, std::void_t<typename C::mapped_type>> {
  using key = typename C::key_type;
  using val = typename C::mapped_type;
};
void not_exact();  // not constexpr: calling it makes the constant evaluation fail
template <class T> concept arith = std::is_arithmetic_v<T> || std::is_enum_v<T> ||
    std::is_same_v<T, __int128> || std::is_same_v<T, unsigned __int128>;
template <class T> requires arith<std::remove_cvref_t<T>> constexpr __int128 enc(const T& v) {
  using U = std::remove_cvref_t<T>;
  if constexpr (std::is_floating_point_v<U>) {
    // A float is read as the bits of a double: a wider type (long double) must round-trip,
    // else the evaluation fails (no constant) instead of giving a rounded value.
    if constexpr (sizeof(U) > sizeof(double)) { if (!((U)(double)v == v)) not_exact(); }
    return (__int128)std::bit_cast<uint64_t>((double)v);
  } else if constexpr (std::is_same_v<U, unsigned __int128>) {
    if (v > (unsigned __int128)std::numeric_limits<__int128>::max()) not_exact();  // above 2^127-1: not exact in __int128
    return (__int128)v;
  } else return (__int128)v;
}
template <size_t N> constexpr const char* str_of(const char (&a)[N]) { return a; }
template <class T> constexpr int str_of(const T&) { return 0; }
}
template <auto V> struct gocpp_probe_internal_val {};
template <class T> struct gocpp_probe_internal_c {};
`

// aliasName returns the name of the alias that holds the type of a local
// variable.
func aliasName(def *collect.Ref) string { return "gp_decl_" + hashOf(def.Key) }

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:5])
}

// A gen builds the file.
type gen struct {
	b       strings.Builder
	line    int
	lines   map[int]Line
	skipped map[int]string
	aliases map[string]bool
	kinds   map[string]interop.Kind

	// aliasLine and aliasAt are the alias lines that were emitted, by name, and where.
	aliasLine map[string]Line
	aliasAt   map[string]int
}

func (g *gen) emit(text string, l *Line) {
	g.b.WriteString(text)
	g.b.WriteByte('\n')
	g.line++
	if l != nil {
		l.Text = text
		g.lines[g.line] = *l
	}
}

// Generate returns the probe file for the references of set.
func Generate(set *collect.Set, cfg GenConfig) *Source {
	g := &gen{lines: make(map[int]Line), skipped: make(map[int]string), aliases: make(map[string]bool), kinds: cfg.Kinds,
		aliasLine: make(map[string]Line), aliasAt: make(map[string]int)}
	headers, helperFrom, helperTo := g.preamble("// gocpp typed interop: probe file (generated)", cfg.Includes, cfg.StringToCStr)
	preamble := g.b.String()
	var units []Unit

	for _, ref := range set.Refs {
		if why := skipReason(ref, cfg.Kinds); why != "" {
			g.skipped[ref.ID] = why
			continue
		}
		r := &rend{kinds: cfg.Kinds, g: g}
		ls := g.refLines(ref, r)
		if len(ls) == 0 {
			g.skipped[ref.ID] = r.reason
			if r.reason == "" {
				g.skipped[ref.ID] = "no probe"
			}
			continue
		}
		// A local variable's alias comes before the first line that uses it.
		for _, a := range r.pre {
			a := a
			name := aliasNameOf(a.Text)
			g.aliases[name] = true
			g.emit(a.Text, &a)
			g.aliasLine[name] = a
			g.aliasAt[name] = g.line
		}
		u := Unit{Ref: ref.ID, Aliases: g.closure(r.used)}
		for _, l := range ls {
			l := l
			g.emit(l.Text, &l)
			u.Lines = append(u.Lines, l)
			u.At = append(u.At, g.line)
		}
		u.Hash = unitHash(u.Aliases, u.Lines)
		units = append(units, u)
	}
	src := &Source{Text: g.b.String(), Lines: g.lines, Skipped: g.skipped, NRefs: len(set.Refs), HelperFrom: helperFrom, HelperTo: helperTo, Includes: headers,
		Preamble: preamble, Units: units, aliasAt: g.aliasAt}
	for _, r := range set.Refs {
		src.Keys = append(src.Keys, r.Key)
	}
	for _, l := range g.lines {
		if l.Role == RoleMain || l.Role == RoleAlt {
			src.Probes++
		}
	}
	return src
}

// closure returns the alias lines that the names use, with the aliases that
// those use, each once, a dependency before its user.
func (g *gen) closure(names []string) []Line {
	var out []Line
	seen := make(map[string]bool)
	var add func(name string)
	add = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		l, ok := g.aliasLine[name]
		if !ok {
			return
		}
		for _, n := range l.Needs {
			add(n)
		}
		out = append(out, l)
	}
	for _, n := range names {
		add(n)
	}
	return out
}

// preamble writes the first lines of a probe file: the title, the headers the
// helpers need, the package's includes and the helper block. It returns the
// includes' headers and the line range of the helper block.
func (g *gen) preamble(title string, includes []string, stringToCStr bool) (headers []string, helperFrom, helperTo int) {
	g.emit(title, nil)
	for _, h := range []string{"cstddef", "cstdint", "bit", "compare", "iterator", "string_view", "type_traits", "utility",
		// What gocpp/rt.hpp always includes (the cheap ones), so that std::string, std::vector, std::tuple
		// and std::unique_ptr resolve in the probe as they do in the build.
		"initializer_list", "string", "vector", "memory", "tuple", "array", "cstring", "limits", "new", "span"} {
		g.emit("#include <"+h+">", nil)
	}
	for _, inc := range includes {
		header, externC := interop.SplitExternC(inc)
		headers = append(headers, header)
		if externC {
			g.emit(`extern "C" {`, nil)
		}
		g.emit("#include "+winpath.IncludeSpelling(header), nil)
		if externC {
			g.emit("}", nil)
		}
	}
	extra := ""
	if stringToCStr {
		extra = "  operator const char*() const noexcept;\n"
	}
	text := fmt.Sprintf(helpers, extra)
	helperFrom = g.line + 1
	g.b.WriteString(text)
	g.line += strings.Count(text, "\n")
	return headers, helperFrom, g.line
}

// skipReason returns why a reference has no probe, or "".
func skipReason(ref *collect.Ref, kinds map[string]interop.Kind) string {
	switch {
	case ref.Incomplete != "":
		return "incomplete: " + ref.Incomplete
	case !ref.Standalone:
		return "name only: it is the function of a call or the template of an instance"
	case ref.Kind == collect.KindSlice:
		return "C++ has no slice expression"
	case ref.Kind == collect.KindOther:
		return "not an expression that C++ can evaluate"
	}
	if p, ok := ref.Expr.(*collect.Path); ok && p.Ns != "C" {
		if k, known := kinds[p.KindPath()]; known && k == interop.Namespace {
			return "a namespace"
		}
	}
	return ""
}

// refLines returns the probe lines of a reference: the primary form, the
// alternative reading when the reference is ambiguous, and the value probes.
func (g *gen) refLines(ref *collect.Ref, r *rend) []Line {
	var out []Line
	r.pre = nil
	primary, isType, ok := r.form(ref.Expr)
	ambiguous := r.ambiguous
	pre := r.pre
	if ok {
		out = append(out, g.probeLine(ref.ID, primary, isType, RoleMain))
		// A value probe: a constant context, or a macro.
		if !isType && (ref.WantValue || r.isMacro(ref.Expr)) {
			out = append(out, Line{Ref: ref.ID, Role: RoleValue, Text: fmt.Sprintf("using gp_probe_%d_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((%s))>;", ref.ID, primary)})
			out = append(out, Line{Ref: ref.ID, Role: RoleString, Text: fmt.Sprintf("constexpr auto gp_probe_%d_s = gocpp_probe_internal_pr::str_of((%s));", ref.ID, primary)})
		}
	}
	firstReason := r.reason
	if ambiguous {
		r2 := &rend{kinds: g.kinds, g: g, flip: true}
		alt, altType, ok2 := r2.form(ref.Expr)
		if ok2 {
			l := g.probeLine(ref.ID, alt, altType, RoleAlt)
			out = append(out, l)
			pre = append(pre, r2.pre...)
		} else if firstReason == "" {
			firstReason = r2.reason
		}
	}
	r.pre = pre
	r.reason = firstReason
	return out
}

func (g *gen) probeLine(id int, text string, isType bool, role Role) Line {
	form := FormExpr
	if isType {
		form = FormType
	}
	name := "gp_probe_" + strconv.Itoa(id)
	if role == RoleAlt {
		name += "_alt"
	}
	body := "decltype((" + text + "))"
	if isType {
		body = text
	}
	return Line{Ref: id, Role: role, Form: form, Text: fmt.Sprintf("using %s = gocpp_probe_internal_c<%s>;", name, body)}
}
