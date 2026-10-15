package interop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/huuhait/gocpp/internal/build"
)

// The kind probe (Phase 04 §2.2, probe 2) says what each C/C++ path that a
// package uses is: a namespace, a type, a template, a value or a macro. It
// grew out of Phase 01's namespace probe (§2.5, step 4), which decides
// whether std in std.vector is a C++ namespace; that decision is still the
// probe's first line per path, and the probe has a mode that asks only it
// (ProbeNamespaces). It asks the C++ compiler with one small file that holds
// the includes, a few helper lines, and a block of lines per path:
//
//	#include <vector>
//	; template <class> struct gp_inc_0; using gp_inc_0_c = ::gp_inc_0<int>;
//	#include "./physics.h"
//	; template <class> struct gp_inc_1; using gp_inc_1_c = ::gp_inc_1<int>;
//	struct gp_kp0 {}; ...                                 (the helper lines)
//	#if defined(std) || defined(chrono)
//	#error gp_kind_macro
//	#else
//	namespace gp_kn_0 = ::std::chrono;                       namespace
//	using gp_kt_0 = ::std::chrono;                           type
//	constexpr bool gp_km_0 = gp_ktmpl<::std::chrono>(...);   class or alias template
//	using gp_kv_0 = decltype(::std::chrono);                 value
//	template <...> auto gp_kw_0() -> decltype(::std::chrono<gp_kT...>);       value template (types)
//	template <...> auto gp_kx_0() -> decltype(::std::chrono<gp_kV...>);       value template (values)
//	template <...> auto gp_kd_0(...) -> decltype(::std::chrono{gp_ka...});    class template (CTAD)
//	template <...> auto gp_kc_0(...) -> decltype(::std::chrono(gp_ka...));    call
//	#endif
//	...
//	namespace gp_probe_end = ::gp_probe_no_such_namespace;
//	namespace gp_probe_eof {}
//
// The compiler checks it with -fsyntax-only and reports every error with
// its line. Each line of a block tests one kind: the first line without an
// error, in the order above, gives the path's kind (see classify). The
// lines never instantiate a template of the headers, except a class that
// a path goes through (::std::string::npos needs std::basic_string<char>);
// an error inside such an instantiation belongs to the path's line (see
// parseDiagnostics), not to the header. In the namespace mode, a block has
// only the #if, #error, #else, the namespace line and the #endif: that is
// the namespace probe of Phase 01, and a name that is not declared costs
// one failed lookup instead of eight (g++ looks for spelling suggestions in
// every failed lookup, about 0.8 ms each with <vector>).
//
// The other lines guard against what the headers and the paths can do to
// the lines after them (all seen with both compilers):
//   - The scope check after each include fails when the header doesn't end
//     at the top level: in the middle of a declaration (int f( or int x =),
//     which would otherwise swallow the next line and with it a path, or
//     inside a block that it leaves open (extern "C" {, a namespace, a
//     class, a function, an enum). The ";" ends an unfinished declaration,
//     a template can't have C linkage, and ::gp_inc_N is found only at the
//     top level. So the error is on the culprit's own line, before the
//     next header can use up the open brace (clang's recovery inside
//     libstdc++ did that for an open extern "C" { before <vector>).
//   - The #if keeps macros out of the C++ lines: a macro that expands to
//     an unbalanced brace would hide the lines after it, and a path with a
//     macro in it means what the macro says, not what its name says (a
//     macro is never a namespace). The #error makes such a path a Macro.
//     The macro probe (ProbeMacros) lists the same macros: both see the
//     macros as they are after the last include.
//   - The gp_probe_end alias always fails, and it uses no name that a
//     header can define as a macro: its error shows that the compiler got
//     to the end of the file, so a compiler that stops early (or prints
//     nothing) can't make every path look like a namespace.
//   - The last line takes the "expected '}' at end of input" error of what
//     the scope checks don't catch (an open extern "C++" {); its "to match
//     this '{'" note says which header that is.
//
// An error in a header (or at an #include line, for a header that isn't
// found) belongs to the include that brought the header in: the compilers
// print the include stack ("In file included from gp_probe.cpp:3:") before
// the first message in a header, and the stack's entry in the probe file is
// that #include. clang prints the stack again whenever the header changes;
// gcc prints the stack of each #include only once, so the parser remembers
// which include brought in each header. When an include has an error, the
// probe can't tell anything about the paths: they are Unknown, and the
// include errors are the result.

// A Kind is what the kind probe found out about a C/C++ path.
type Kind int

const (
	// Unknown means that the probe couldn't tell: an include failed
	// (Result.IncludeErrors), the probe failed, or there was nothing to
	// probe with (no includes, or the language server has no answer yet).
	// It is the zero value, so a path that is missing from Result.Kinds is
	// Unknown too.
	Unknown Kind = iota

	// Namespace means that the path is a C++ namespace or a namespace
	// alias (namespace fs = std::filesystem), possibly found through an
	// inline namespace: std, std::chrono, glm.
	Namespace

	// Type means that the path is a type: a class, struct, union or enum,
	// a typedef or an alias (std::string, std::chrono::steady_clock, Color
	// for enum class Color, a C struct name, std::size_t). What follows it
	// is reached with :: (std::string::npos).
	Type

	// Template means that the path is a class template or an alias
	// template, whatever its parameters are (std::vector, std::array,
	// std::bitset, std::ratio, std::integer_sequence, std::enable_if_t,
	// std::ranges::subrange, glm's vec<L, T, Q>): [...] after it are
	// template arguments, and the result is a type. (An alias template of
	// a type that is not a class, with parameters that none of gp_ktmpl's
	// shapes takes, is NotFound.)
	Template

	// Value means that the path is an object, a function (also an overload
	// set of functions that are not templates), an enumerator (Color::Red)
	// or a static member (std::string::npos, std::chrono::steady_clock::now):
	// what follows it is reached with ".", and [...] after it is an index.
	Value

	// ValueTemplate means that the path is a value that can take template
	// arguments: a function template, an overload set with one (std::max,
	// std::get, std::make_unique; also std::sqrt with libstdc++, whose
	// integer overloads are templates), or a variable template
	// (std::is_same_v, std::in_place_index). [...] after it are template
	// arguments; the result is a value. The probe can't tell function
	// templates from variable templates, and nothing needs it: both are
	// spelled f<args>.
	ValueTemplate

	// Macro means that a part of the path is a macro (errno, RAYWHITE,
	// EOF, or a macro that stands for a namespace name). The path is not
	// probed as C++: what it means depends on the macro, and ProbeMacros
	// says which parts are macros and whether they take arguments.
	Macro

	// NotFound means that none of the probe's lines compiled: nothing of
	// that name is declared, or it is something that the probe doesn't
	// classify (a concept, a class template's member without the template
	// arguments, a non-static member function with clang), or a C++
	// keyword. A name that starts with gp_ (reserved for gocpp) is always
	// NotFound.
	NotFound

	// NotNamespace is ProbeNamespaces' answer for a name that is not a
	// namespace (and not a macro): it asks nothing more about it.
	NotNamespace
)

// String returns the name of the constant, such as "Namespace".
func (k Kind) String() string {
	switch k {
	case Unknown:
		return "Unknown"
	case Namespace:
		return "Namespace"
	case Type:
		return "Type"
	case Template:
		return "Template"
	case Value:
		return "Value"
	case ValueTemplate:
		return "ValueTemplate"
	case Macro:
		return "Macro"
	case NotFound:
		return "NotFound"
	case NotNamespace:
		return "NotNamespace"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// A Result is what ProbeKinds found out.
type Result struct {
	// Kinds maps each path, spelled as it was passed, to its kind. When
	// IncludeErrors is not empty, every path is Unknown, except that a
	// path that starts with gp_ (reserved for gocpp) is always NotFound.
	Kinds map[string]Kind

	// IncludeErrors are the includes that didn't work, in include order: a
	// header that isn't found, or a header that doesn't compile. There is
	// at most one per include, with the compiler's first message about it.
	// After a header that isn't found the compiler stops, so the includes
	// after it are not checked.
	IncludeErrors []IncludeError
}

// An IncludeError is an include that didn't work.
type IncludeError struct {
	// Index is the index of the include in the includes passed to
	// ProbeKinds, and Include is that include: "<raylibb.h>".
	Index   int
	Include string

	// Kind says what kind of error it is, so that a caller can word its
	// message without knowing the compilers' messages.
	Kind IncludeErrorKind

	// File, Line and Column are where the compiler found the error when
	// it is in a header (also one that the include brings in). They are
	// "", 0 and 0 when the error is in the probe file: at the #include line
	// itself, because the header wasn't found, or at the check after it,
	// because the header ends in an unfinished declaration or an open
	// block (then Msg says so). Column is 0 when the compiler gave none.
	File         string
	Line, Column int

	// Msg is the compiler's first message about the include, such as
	// "'raylibb.h' file not found" (clang), "raylibb.h: No such file or
	// directory" (gcc) or "expected ';' after struct".
	Msg string
}

// An IncludeErrorKind says what is wrong with an include.
type IncludeErrorKind int

const (
	// HeaderErrors: the header, or a header that it includes, has an
	// error (IncludeError.File says where), or the compiler reported
	// another error than "not found" at the #include line (File is "").
	// It is the zero value.
	HeaderErrors IncludeErrorKind = iota

	// HeaderNotFound: the compiler didn't find the header of the include
	// ("'raylibb.h' file not found", "raylibb.h: No such file or
	// directory"). A header that isn't found by a header that the include
	// brings in is HeaderErrors, with the place of its #include line.
	HeaderNotFound

	// HeaderUnclosed: the header ends inside a declaration or a block
	// that it doesn't close (the scope check after the #include failed).
	// Msg says so, and then gives the compiler's message about the check
	// line of the probe file, which says little about the header.
	HeaderUnclosed
)

// String returns the name of the constant, such as "HeaderNotFound".
func (k IncludeErrorKind) String() string {
	switch k {
	case HeaderErrors:
		return "HeaderErrors"
	case HeaderNotFound:
		return "HeaderNotFound"
	case HeaderUnclosed:
		return "HeaderUnclosed"
	}
	return fmt.Sprintf("IncludeErrorKind(%d)", int(k))
}

// notFoundRx matches the compilers' messages for a header that isn't
// found: clang's "'x.h' file not found" and gcc's "x.h: No such file or
// directory" (with LC_ALL=C, so in English and with ASCII quotes). For
// <x.h> with only a quoted x.h next to the file, clang says "'x.h' file
// not found with <angled> include; use \"quotes\" instead" (and then
// includes it anyway), where gcc says it isn't found.
var notFoundRx = regexp.MustCompile(`^(?:'.*' file not found(?: with <angled> include; use "quotes" instead)?|.*: No such file or directory)$`)

// Error returns "include: msg", or "include: file:line:col: msg" for an
// error in a header.
func (e IncludeError) Error() string {
	if e.File == "" {
		return e.Include + ": " + e.Msg
	}
	pos := e.File + ":" + strconv.Itoa(e.Line)
	if e.Column > 0 {
		pos += ":" + strconv.Itoa(e.Column)
	}
	return e.Include + ": " + pos + ": " + e.Msg
}

// ProbeKinds asks the C++ compiler cxx what each of the paths is (a
// Kind), given the includes of a .gp file and the package's flags. It runs
// the compiler once, with -fsyntax-only, for all the paths. With includes,
// it runs also for an empty path list (or one with only reserved names),
// to check the includes: a .gp file that imports only C headers may have
// no paths, but its headers must be there.
//
// includes are the texts of the #include lines, in source order, each
// with its delimiters: "<raylib.h>", "\"./physics.h\"". dir is the folder
// of the .gp file: the compiler runs there, and a quoted include is looked
// up there after the folder of the file that includes it, before the
// system folders (it is passed as -iquote, which doesn't affect <...>
// includes). dir can be "" when there is no folder.
//
// flags are the package's compiler flags (from //gocpp:cflags and
// pkg-config, checked by the flag allow-list before they get here); they
// come after -std=c++20 and -iquote dir, so -std=gnu++23 and -I folders
// work as in the build, and before the probe's own flags, which they can't
// turn off. The probe can't tell whether a flag is safe to run: that is
// the caller's job.
//
// A path is a C++ name, such as "std" or "Color" (the C-level name of a
// header package's raylib.Color), or a qualified name, such as
// "std::chrono::steady_clock" or "::std::filesystem" (the leading :: is
// optional; the path is always looked up from the global namespace). Each
// part must be a C++ name: a letter or _, then letters, digits and _.
// Template arguments are not part of a path: for std.vector[int].iterator
// the caller asks for std::vector. Result.Kinds has an entry for each path.
//
// Without includes, every path is Unknown and nothing runs: nothing can
// declare anything (a .gp file can have a namespace rename and no header
// import). A path whose name starts with gp_ is NotFound without a probe
// line: gocpp reserves those names (ADR-010).
//
// The answers are the compiler's own, and g++ and clang++ differ in a few
// places:
//   - g++ declares namespace std even when no header does, so with only C
//     headers (<stdio.h>) std is a Namespace for g++ and NotFound for
//     clang++ (with <math.h>, whose libstdc++ wrapper declares std, both
//     say Namespace).
//   - Non-static member functions: g++ says Value (std::string::size, an
//     overload set without templates) or ValueTemplate
//     (std::string::find); clang++ says NotFound. The same goes for an
//     injected class name (S::S, std::string::basic_string).
//   - A path through a class whose instantiation fails (a typedef of a
//     broken specialization) can get any kind, and the compilers differ.
//     It is never an include error: the header compiles on its own, and
//     the build reports the real error at the use.
//
// When the compiler stops early at a fatal error on a path's lines (a
// template instantiation that is too deep, through such a class), that
// path is NotFound, and the probe runs again without it, at most
// maxReruns times; after that, the probe fails as for any compiler that
// stops early.
//
// An include that doesn't work (a header that isn't found, or doesn't
// compile) is an IncludeError in the result, and then every path is
// Unknown. The error is for the probe itself: bad arguments, a compiler
// that doesn't run, a time limit (ctx, and at most one minute), or
// compiler output that the probe doesn't understand. Then too the paths
// are Unknown, and the caller can go on without the probe.
//
// The results are cached in memory, per compiler, folder, flags and
// includes. The cache notices changes to the headers outside the system
// folders (the compiler's dependency list, -MMD) and to the quoted
// includes' files in dir. It doesn't notice changes to system headers or
// to the environment; a result with include errors is used for 5 seconds
// at most, so that installing a missing library is noticed. (A disk cache
// comes with the build cache of Phase 06; its key must also hold the
// environment that changes the include paths, such as CPATH and
// NIX_CFLAGS_COMPILE.) ProbeKinds is safe for concurrent use; calls for
// the same include set wait for each other, so that they run the compiler
// once. A call with paths that the cache doesn't have probes all the paths
// of the call again in one run: the headers take the time, not the paths.
func ProbeKinds(ctx context.Context, cxx build.CXX, dir string, flags, includes, paths []string) (Result, error) {
	return defaultCache.probe(ctx, cxx, dir, flags, includes, paths)
}

// ProbeNamespaces is ProbeKinds for callers that only need to know which
// names are namespaces (check.Check for the loader's namespace candidates,
// and so the language server): each kind in the Result is Namespace, Macro,
// NotNamespace (anything else, also a reserved gp_ name) or Unknown (as for
// ProbeKinds). It asks the compiler only the namespace question, which is
// much cheaper for names that are not declared (typos), and it shares the
// cache with ProbeKinds: a kind that ProbeKinds found answers it too (but
// not the other way round).
func ProbeNamespaces(ctx context.Context, cxx build.CXX, dir string, flags, includes, names []string) (Result, error) {
	return defaultCache.probeNamespaces(ctx, cxx, dir, flags, includes, names)
}

// nsAnswer returns the answer to the namespace question that the kind k
// gives.
func nsAnswer(k Kind) Kind {
	switch k {
	case Unknown, Namespace, Macro:
		return k
	}
	return NotNamespace
}

// probeTimeout limits one run of a probe, also when ctx has no deadline.
// The heaviest include sets take a few seconds.
const probeTimeout = time.Minute

// maxProbeOutput limits what the kind probe reads from the compiler. A
// probe with 200 bad paths prints about 0.5 MiB with g++ (each failed line
// prints notes); the cascade of errors of C++ headers that end up inside
// an open extern "C" block is 1 to 3 MiB. When the output is cut, the
// include errors found so far still count.
const maxProbeOutput = 8 << 20

// waitDelay is how long a probe waits for the compiler's output after the
// compiler exited or was stopped.
const waitDelay = 2 * time.Second

// checkInclude reports what is wrong with the include text inc, if
// anything. The text goes into the probe file as is, so it must be one
// header name on one line.
func checkInclude(inc string) error {
	if len(inc) >= 3 {
		inside := inc[1 : len(inc)-1]
		switch {
		case inc[0] == '<' && inc[len(inc)-1] == '>' && !strings.ContainsAny(inside, ">\n\r\x00"):
			return nil
		case inc[0] == '"' && inc[len(inc)-1] == '"' && !strings.ContainsAny(inside, "\"\n\r\x00"):
			return nil
		}
	}
	return fmt.Errorf("bad include %q: want <header> or \"header\"", inc)
}

// checkArgs checks the includes and the flags of a probe call.
func checkArgs(flags, includes []string) error {
	for _, inc := range includes {
		if err := checkInclude(inc); err != nil {
			return err
		}
	}
	for _, f := range flags {
		// A command line can't hold a NUL byte, and the cache keys use it
		// as a separator.
		if strings.Contains(f, "\x00") {
			return fmt.Errorf("bad flag %q: it has a NUL byte", f)
		}
	}
	return nil
}

// splitPath returns the parts of the path p ("::std::filesystem" gives
// std and filesystem), or an error if p is not a name or a qualified name.
func splitPath(p string) ([]string, error) {
	parts := strings.Split(strings.TrimPrefix(p, "::"), "::")
	for _, part := range parts {
		if !isCppName(part) {
			return nil, fmt.Errorf("kind probe: bad path %q: want a C++ name or a qualified name like ::std::filesystem", p)
		}
	}
	return parts, nil
}

// isCppName reports whether s is a C++ identifier: a letter or _, then
// letters, digits and _. Like Go (and load's check of namespace paths), it
// allows non-ASCII letters and digits. These characters can't change the
// lines of the probe file around the name; whether the compiler accepts
// the name, and whether it is a keyword, the compiler decides (a keyword
// gets an error on every line of its block, and no line after it, with
// both compilers: the tests have the keywords that start a construct).
func isCppName(s string) bool {
	for i, r := range s {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) || i == 0 && unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// includeLines is the number of lines of the probe file per include (the
// #include and the scope check).
const includeLines = 2

// The lines of a path's block, from its first line (blockIf). Each of the
// C++ lines from lineNamespace to lineCall tests one kind. In the namespace
// mode a block has only the lines up to lineNamespace, and then its #endif
// (at the offset nsBlockEndif).
const (
	blockIf        = iota // #if defined(part) || ...
	blockError            // #error: the path has a macro
	blockElse             // #else
	lineNamespace         // namespace gp_kn_N = path;
	lineType              // using gp_kt_N = path;
	lineTemplate          // a class or alias template: gp_ktmpl<path>
	lineValue             // using gp_kv_N = decltype(path);
	lineValueTmplT        // decltype(path<gp_kT...>) in a template: types
	lineValueTmplV        // decltype(path<gp_kV...>) in a template: values
	lineBraced            // decltype(path{gp_ka...}) in a template
	lineCall              // decltype(path(gp_ka...)) in a template
	blockEndif            // #endif
	blockLines            // the number of lines of a block
)

// The namespace mode's block: its #endif and its number of lines.
const (
	nsBlockEndif = lineNamespace + 1
	nsBlockLines = nsBlockEndif + 1
)

// helperLines are the lines that the blocks use, after the includes. The
// template line passes the path as a template template argument to
// gp_ktmpl, which has one overload per shape of template parameters:
// class... matches every template with only type parameters (std::vector,
// std::map, std::function), auto... the ones with only values
// (std::bitset, std::ratio), and the others std::array and std::span
// (class, auto...), std::enable_if_t and std::conditional_t (auto,
// class...), std::integer_sequence and std::integral_constant (class T,
// T...), and templates of templates. A template matches several shapes
// (template <class T> matches class... and class, auto...), so the
// overloads take tags that make one of them the best match: gp_kp5 is the
// best for an argument gp_kp5{}, then gp_kp4, and so on. A class template
// of another shape (std::ranges::subrange, glm's vec<L, T, Q>) passes the
// braced line instead (class template argument deduction), also through
// an alias template of a class.
const helperLines = `struct gp_kp0 {}; struct gp_kp1 : gp_kp0 {}; struct gp_kp2 : gp_kp1 {}; struct gp_kp3 : gp_kp2 {}; struct gp_kp4 : gp_kp3 {}; struct gp_kp5 : gp_kp4 {};
template <template <class...> class> constexpr bool gp_ktmpl(gp_kp5) { return true; }
template <template <auto...> class> constexpr bool gp_ktmpl(gp_kp4) { return true; }
template <template <class, auto...> class> constexpr bool gp_ktmpl(gp_kp3) { return true; }
template <template <auto, class...> class> constexpr bool gp_ktmpl(gp_kp2) { return true; }
template <template <class gp_kU, gp_kU...> class> constexpr bool gp_ktmpl(gp_kp1) { return true; }
template <template <template <class...> class...> class> constexpr bool gp_ktmpl(gp_kp0) { return true; }
`

// nHelperLines is the number of lines of helperLines.
var nHelperLines = strings.Count(helperLines, "\n")

// A probeFile is the source of a probe, with the places of its parts. Its
// lines are, from line 1: includeLines lines per include, the helper lines
// (when there are paths and all kinds are asked for), blockLen lines per
// path, the canary (the alias that must fail) and the last line.
type probeFile struct {
	src        []byte
	nIncludes  int
	nPaths     int
	kinds      bool // all kinds, not only namespaces
	blockLen   int  // blockLines, or nsBlockLines
	firstBlock int  // the first line of the first block
}

func (p *probeFile) canary() int { return p.firstBlock + p.blockLen*p.nPaths }

// helpers reports whether the line n is one of the helper lines.
func (p *probeFile) helpers(n int) bool {
	return n > includeLines*p.nIncludes && n < p.firstBlock
}

// block returns the index of the path whose block has the line n, and the
// line's offset in the block, or false.
func (p *probeFile) block(n int) (path, offset int, ok bool) {
	if n < p.firstBlock || n >= p.canary() {
		return 0, 0, false
	}
	return (n - p.firstBlock) / p.blockLen, (n - p.firstBlock) % p.blockLen, true
}

// makeProbe returns the probe file for the includes and the paths (each
// split into its parts): with all the kind lines (kinds), or only the
// namespace line.
func makeProbe(includes []string, paths [][]string, kinds bool) *probeFile {
	var b bytes.Buffer
	for i, inc := range includes {
		fmt.Fprintf(&b, "#include %s\n; template <class> struct gp_inc_%d; using gp_inc_%d_c = ::gp_inc_%d<int>;\n", inc, i, i, i)
	}
	pf := &probeFile{nIncludes: len(includes), nPaths: len(paths), kinds: kinds, blockLen: nsBlockLines, firstBlock: includeLines*len(includes) + 1}
	if kinds {
		pf.blockLen = blockLines
		if len(paths) > 0 {
			b.WriteString(helperLines)
			pf.firstBlock += nHelperLines
		}
	}
	for i, parts := range paths {
		b.WriteString("#if ")
		for j, p := range parts {
			if j > 0 {
				b.WriteString(" || ")
			}
			fmt.Fprintf(&b, "defined(%s)", p)
		}
		p := "::" + strings.Join(parts, "::")
		b.WriteString("\n#error gp_kind_macro\n#else\n")
		fmt.Fprintf(&b, "namespace gp_kn_%d = %s;\n", i, p)
		if kinds {
			fmt.Fprintf(&b, "using gp_kt_%d = %s;\n", i, p)
			fmt.Fprintf(&b, "constexpr bool gp_km_%d = gp_ktmpl<%s>(gp_kp5{});\n", i, p)
			fmt.Fprintf(&b, "using gp_kv_%d = decltype(%s);\n", i, p)
			fmt.Fprintf(&b, "template <class... gp_kT> auto gp_kw_%d() -> decltype(%s<gp_kT...>);\n", i, p)
			fmt.Fprintf(&b, "template <auto... gp_kV> auto gp_kx_%d() -> decltype(%s<gp_kV...>);\n", i, p)
			fmt.Fprintf(&b, "template <class... gp_kT> auto gp_kd_%d(gp_kT... gp_ka) -> decltype(%s{gp_ka...});\n", i, p)
			fmt.Fprintf(&b, "template <class... gp_kT> auto gp_kc_%d(gp_kT... gp_ka) -> decltype(%s(gp_ka...));\n", i, p)
		}
		b.WriteString("#endif\n")
	}
	b.WriteString("namespace gp_probe_end = ::gp_probe_no_such_namespace;\n")
	b.WriteString("namespace gp_probe_eof {}\n")
	pf.src = b.Bytes()
	return pf
}

// classify returns the kind of a path whose block had errors on the lines
// in failed (bit i for line i of the block), for a block with all the kind
// lines. The first C++ line that compiled gives the kind, in the order of
// the lines:
//   - a type also passes the braced and the call lines (a functional
//     cast), and so does a class template (class template argument
//     deduction), so these lines count only when the lines before them
//     failed;
//   - the braced line is for the class templates whose parameters none of
//     gp_ktmpl's shapes takes; a value never passes it (x{...} is not an
//     expression for an object or a function);
//   - the call line is for an overload set of functions, whose decltype
//     doesn't compile (std::sqrt has templates too, so it passes a
//     value-template line first).
//
// An error on the #if, #else or #endif line (a part named defined, say)
// makes the block useless: NotFound.
func classify(failed uint) Kind {
	ok := func(line int) bool { return failed&(1<<line) == 0 }
	switch {
	case !ok(blockError):
		return Macro
	case !ok(blockIf) || !ok(blockElse) || !ok(blockEndif):
		return NotFound
	case ok(lineNamespace):
		return Namespace
	case ok(lineType):
		return Type
	case ok(lineTemplate):
		return Template
	case ok(lineValue):
		return Value
	case ok(lineValueTmplT), ok(lineValueTmplV):
		return ValueTemplate
	case ok(lineBraced):
		return Template
	case ok(lineCall):
		return Value
	}
	return NotFound
}

// classifyNS is classify for a block of the namespace mode.
func classifyNS(failed uint) Kind {
	ok := func(line int) bool { return failed&(1<<line) == 0 }
	switch {
	case !ok(blockError):
		return Macro
	case ok(blockIf) && ok(blockElse) && ok(nsBlockEndif) && ok(lineNamespace):
		return Namespace
	}
	return NotNamespace
}

// A probeRun is what one run of the probe found out.
type probeRun struct {
	kinds   []Kind // per path of the run; nil if incErrs isn't empty
	incErrs []IncludeError

	// deps are the headers outside the system folders that the compiler
	// read, from its dependency file (-MMD), and depsOK says whether there
	// was one: after a header that isn't found, there is none.
	deps   []string
	depsOK bool

	start time.Time // when the compiler started
}

// familyArgs returns the flags that make the compiler cxx print its
// messages the way the parser reads them.
func familyArgs(cxx build.CXX) ([]string, error) {
	switch cxx.Family {
	case build.Clang:
		// -Qunused-arguments: no warnings about flags that a compiler
		// wrapper adds (NixOS). The include stacks of notes say which
		// include brought in a note's header (the "to match this '{'" of a
		// header that leaves a brace open). No spell checking: the probe
		// doesn't use "did you mean", and it costs time with many
		// misspelled paths.
		return []string{"-ferror-limit=0", "-Qunused-arguments", "-fno-caret-diagnostics",
			"-fdiagnostics-show-note-include-stack", "-fno-spell-checking", "-fmessage-length=0"}, nil
	case build.GCC:
		// Without macro tracking, gcc puts an error that comes from a
		// macro at the place where the macro is used, like clang. With it,
		// gcc prints the include stack of the header that defines the macro
		// (<features.h> for stdio.h's __BEGIN_DECLS), and the error would
		// go to the wrong include.
		return []string{"-fmax-errors=0", "-fno-diagnostics-show-caret", "-ftrack-macro-expansion=0", "-fmessage-length=0"}, nil
	}
	return nil, unknownFamily(cxx)
}

// messageFlags are the flags (and the starts of flags) that change only how
// the compiler prints its messages, not what the code means: the probe
// drops them from the package's flags, because it reads the messages. A
// flag after them can't always undo them: g++ 15 keeps
// -fdiagnostics-format=json after a later -fdiagnostics-format=text.
// -Werror and -Werror=x don't matter: -w turns every warning off.
var messageFlags = []string{"-fdiagnostics-", "-fno-diagnostics-", "-fmessage-length=", "-fmax-errors=", "-ferror-limit=",
	"-Wfatal-errors", "-Wno-fatal-errors", "-fcolor-diagnostics", "-fno-color-diagnostics", "-fcaret-diagnostics",
	"-fno-caret-diagnostics", "-fshow-column", "-fno-show-column", "-fshow-source-location", "-fno-show-source-location",
	"-fspell-checking", "-fno-spell-checking", "-ftemplate-backtrace-limit=", "-fconstexpr-backtrace-limit=",
	"-fmacro-backtrace-limit=", "-ftrack-macro-expansion", "-fno-track-macro-expansion"}

// probeFlags returns the package's flags without the messageFlags.
func probeFlags(flags []string) []string {
	return slices.DeleteFunc(slices.Clone(flags), func(f string) bool {
		return slices.ContainsFunc(messageFlags, func(m string) bool {
			if strings.HasSuffix(m, "=") || strings.HasSuffix(m, "-") {
				return strings.HasPrefix(f, m)
			}
			return f == m || strings.HasPrefix(f, m+"=")
		})
	})
}

// compilerArgs returns the arguments of a probe run: -std=c++20 and
// -iquote dir, the package's flags (without the messageFlags), the mode
// (-fsyntax-only, or -E -dM), the probe's own flags (which the package's
// flags can't turn off: for most flags the last one wins), the dependency
// file and the source src.
func compilerArgs(cxx build.CXX, mode []string, dir string, flags []string, depFile, src string) ([]string, error) {
	fam, err := familyArgs(cxx)
	if err != nil {
		return nil, err
	}
	args := []string{"-std=c++20"}
	if dir != "" {
		args = append(args, "-iquote", dir)
	}
	args = append(args, probeFlags(flags)...)
	args = append(args, mode...)
	args = append(args, "-w", "-Wno-fatal-errors", "-fdiagnostics-color=never")
	args = append(args, fam...)
	args = append(args, "-MMD", "-MF", depFile, "-MT", "gp_probe", "-x", "c++", src)
	return args, nil
}

// maxReruns is the number of times that the probe runs again without a
// path whose lines stopped the compiler (see ProbeKinds).
const maxReruns = 3

// runProbe runs the probe for the paths (split into parts) with the
// compiler cxx in the folder dir (absolute, or ""), with all the kind lines
// (kinds) or only the namespace lines. When a path's lines stop the
// compiler (errStopped), that path is NotFound (NotNamespace), and the
// probe runs again without it.
func runProbe(ctx context.Context, cxx build.CXX, dir string, flags, includes []string, paths [][]string, kinds bool) (*probeRun, error) {
	stopped := NotNamespace
	if kinds {
		stopped = NotFound
	}
	all := make([]Kind, len(paths))
	left := make([]int, len(paths)) // the indexes of the paths still probed
	for i := range left {
		left[i] = i
	}
	var start time.Time
	for try := 0; ; try++ {
		sub := make([][]string, len(left))
		for j, i := range left {
			sub[j] = paths[i]
		}
		run, err := runProbeOnce(ctx, cxx, dir, flags, includes, sub, kinds)
		if try == 0 && run != nil {
			start = run.start
		}
		var stop *errStopped
		if errors.As(err, &stop) && try < maxReruns {
			all[left[stop.path]] = stopped
			left = slices.Delete(left, stop.path, stop.path+1)
			if start.IsZero() {
				start = stop.start
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if !start.IsZero() {
			run.start = start // the first run's: the files must be older than that
		}
		if run.incErrs != nil {
			return run, nil
		}
		for j, i := range left {
			all[i] = run.kinds[j]
		}
		run.kinds = all
		return run, nil
	}
}

// runProbeOnce runs the probe once (see runProbe).
func runProbeOnce(ctx context.Context, cxx build.CXX, dir string, flags, includes []string, paths [][]string, kinds bool) (*probeRun, error) {
	tmp, err := probeTemp()
	if err != nil {
		return nil, fmt.Errorf("kind probe: %w", err)
	}
	defer os.RemoveAll(tmp)
	pf := makeProbe(includes, paths, kinds)
	src := filepath.Join(tmp, "gp_probe.cpp")
	if err := os.WriteFile(src, pf.src, 0o644); err != nil {
		return nil, fmt.Errorf("kind probe: %w", err)
	}
	depFile := filepath.Join(tmp, "gp_probe.d")
	args, err := compilerArgs(cxx, []string{"-fsyntax-only"}, dir, flags, depFile, src)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	out := &capBuffer{max: maxProbeOutput, stop: cancel}
	start := time.Now()
	cmd, err := runCompiler(ctx, cxx, dir, tmp, args, out, out)
	name := cxx.Name()
	if out.full {
		// The include errors so far are enough: then the paths are Unknown
		// anyway.
		if run, err := readOutput(pf, out.buf.Bytes(), src, includes); err == nil && run.incErrs != nil {
			run.start = start
			return run, nil
		}
		return nil, fmt.Errorf("kind probe: %s printed more than %d MiB", name, maxProbeOutput>>20)
	}
	if err := runFailure(ctx, name, cmd, err, out.buf.Bytes()); err != nil {
		return nil, fmt.Errorf("kind probe: %w", err)
	}

	run, err := readOutput(pf, out.buf.Bytes(), src, includes)
	if err != nil {
		var stop *errStopped
		if errors.As(err, &stop) {
			stop.start = start
		}
		return nil, fmt.Errorf("kind probe: %s: %w", name, err)
	}
	run.start = start
	if data, err := os.ReadFile(depFile); err == nil {
		run.deps, run.depsOK = parseDeps(data, src, dir), true
	}
	return run, nil
}

// errStopped is readOutput's error when the compiler stopped at a fatal
// error on the lines of a path, before the canary.
type errStopped struct {
	path  int       // the index of the path in the run
	msg   string    // the error message
	start time.Time // when the run started (set by runProbeOnce)
}

func (e *errStopped) Error() string { return e.msg }

// probeTemp makes a new temporary folder for a probe run and returns its
// absolute path.
func probeTemp() (string, error) {
	tmp, err := os.MkdirTemp("", "gocpp-probe-")
	if err != nil {
		return "", err
	}
	// The compiler runs in dir, so a relative TMPDIR must not stay relative.
	abs, err := filepath.Abs(tmp)
	if err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return abs, nil
}

// runCompiler runs the compiler cxx with args in the folder dir, with the
// temporary folder tmp, and waits for it and for its output. It runs in a
// process group of its own, and whatever the compiler left running is
// killed. The error is cmd.Wait's.
func runCompiler(ctx context.Context, cxx build.CXX, dir, tmp string, args []string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	// build.Start starts it again after ETXTBSY (a compiler wrapper that
	// was just written).
	cmd, err := build.Start(ctx, func() *exec.Cmd {
		cmd := cxx.Command(ctx, args...) // with the compiler's own Args (CXX="ccache g++")
		cmd.Dir = dir
		cmd.Env = probeEnv(tmp, dir)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		cmd.WaitDelay = waitDelay
		build.InGroup(cmd)
		return cmd
	})
	if err == nil {
		err = cmd.Wait()
		build.KillGroup(cmd) // whatever the compiler left running
	}
	return cmd, err
}

// runFailure returns the error for a run of the compiler name that didn't
// end on its own (cmd and err are runCompiler's, ctx the run's context, out
// its output), or nil when it exited, also with an error status.
func runFailure(ctx context.Context, name string, cmd *exec.Cmd, err error, out []byte) error {
	switch {
	case ctx.Err() != nil:
		return fmt.Errorf("%s was stopped: %w", name, ctx.Err())
	case cmd.ProcessState == nil:
		return fmt.Errorf("%s didn't start: %w", name, err)
	case errors.Is(err, exec.ErrWaitDelay):
		return fmt.Errorf("%s exited, but a process that it started kept its output open", name)
	case !cmd.ProcessState.Exited():
		return fmt.Errorf("%s: %v%s", name, cmd.ProcessState, outputExcerpt(out))
	}
	return nil
}

// unknownFamily is the error for a compiler that is neither clang nor gcc.
func unknownFamily(cxx build.CXX) error {
	return fmt.Errorf("unknown family %v of the C++ compiler %s", cxx.Family, cxx.Path)
}

// probeEnv returns the environment of the compiler: build.CompilerEnv
// (the process's own, with LC_ALL=C), with the temporary folder tmp, so
// that nothing the compiler writes stays behind, and with PWD set to the
// folder dir that it runs in (os/exec sets PWD only when it makes the
// environment itself).
func probeEnv(tmp, dir string) []string {
	// A later KEY=value wins (os/exec keeps the last one).
	env := append(build.CompilerEnv(), "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)
	if dir != "" && runtime.GOOS != "windows" {
		env = append(env, "PWD="+dir)
	}
	return env
}

// capBuffer keeps the first max bytes written to it. When more come, it
// calls stop and drops the rest.
type capBuffer struct {
	buf  bytes.Buffer
	max  int
	full bool
	stop func()
}

func (b *capBuffer) Write(p []byte) (int, error) {
	if b.full {
		return len(p), nil
	}
	if room := b.max - b.buf.Len(); len(p) > room {
		b.buf.Write(p[:room])
		b.full = true
		b.stop()
		return len(p), nil
	}
	return b.buf.Write(p)
}

// The compiler's messages, as gcc and clang print them with
// -fno-diagnostics-show-caret and -fno-caret-diagnostics.
var (
	// A message at a place: "file:line:col: error: msg". Some of gcc's
	// messages have no column. The file can contain ":" (C:\x.h). gcc's
	// "sorry, unimplemented" is an error too.
	diagRx = regexp.MustCompile(`^(.+?):(\d+):(?:(\d+):)? (fatal error|error|sorry, unimplemented|warning|note|remark): (.*)$`)

	// A line of an include stack: clang prints one "In file included from
	// file:line:" line per level, outermost first; gcc prints "In file
	// included from file:line," and then "                 from file:line:"
	// lines, innermost first.
	inclRx = regexp.MustCompile(`^(?:In file included from|\s+from) (.+?):(\d+)(?::\d+)?[:,]$`)

	// Any other error: "g++: error: unrecognized command-line option",
	// "cc1plus: fatal error: ...", "clang++: error: clang frontend command
	// failed", or a crash ("internal compiler error").
	otherErrRx = regexp.MustCompile(`(?:^|: )(?:fatal error|error|internal compiler error): `)

	// gcc's context lines before an error in a template's instantiation:
	// "x.h: In instantiation of 'struct B<int>':" starts a context, and
	// "gp_probe.cpp:32:25:   required from here" is the place in the
	// source that asked for the instantiation (gcc prints "required from
	// 'X'" lines for the steps in between).
	gccContextRx = regexp.MustCompile(`^.+?: (?:In [a-z ]+|At global scope)\b.*:$`)
	gccHereRx    = regexp.MustCompile(`^(.+?):(\d+):(?:\d+:)?\s+required from here$`)
)

// A diagnostic is one error or note that the compiler reported.
type diagnostic struct {
	severity  string // "note", or "error", "fatal error", "sorry, unimplemented"
	file      string // the file it is in, "" if the compiler gave no place
	line, col int
	msg       string // the message, without "error: "
	text      string // the whole line, for messages

	// probeLine is the line of the probe file it belongs to: its own line
	// when it is in the probe file, else the #include line that brought in
	// its header, or the line that asked for the template instantiation
	// that it is in. It is 0 when the probe can't tell.
	probeLine int
}

func (d *diagnostic) isError() bool { return d.severity != "note" }

// parseDiagnostics returns the errors and notes in the compiler output out,
// for the probe file at the path probe (as the compiler got it). A message
// in a header belongs to the #include of the probe file in the include
// stack printed before it. When no stack is printed before it, the header
// had a message before: clang prints no stack for the next message in the
// same header, and gcc prints the stack of an #include only once. So the
// parser remembers, for each header, the probe line that brought it in.
//
// An error in a header that comes from the instantiation of a template
// that a line of the probe file asked for (the class of
// ::BadTypedef::member, where BadTypedef is a specialization that doesn't
// compile) belongs to that line instead: the header compiles on its own.
// gcc says so before the error ("gp_probe.cpp:32:25:   required from
// here", and the context stays for the next errors until another one
// starts), clang in a note after it ("in instantiation of template class
// 'B<int>' requested here").
func parseDiagnostics(out []byte, probe string) []diagnostic {
	var diags []diagnostic
	var stack []string          // the files of the include stack just printed
	stackLine := 0              // the stack's line in the probe file, 0 if none
	instLine := 0               // gcc: the probe line of the instantiation context, 0 if none
	headers := map[string]int{} // header file -> probe line
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if m := inclRx.FindStringSubmatch(line); m != nil {
			if samePath(m[1], probe) {
				stackLine, _ = strconv.Atoi(m[2])
			} else {
				stack = append(stack, filepath.Clean(m[1]))
			}
			continue
		}
		if m := gccHereRx.FindStringSubmatch(line); m != nil {
			if samePath(m[1], probe) {
				instLine, _ = strconv.Atoi(m[2])
			}
			continue
		}
		m := diagRx.FindStringSubmatch(line)
		if m == nil {
			switch {
			case gccContextRx.MatchString(line):
				instLine = 0
			case otherErrRx.MatchString(line):
				diags = append(diags, diagnostic{severity: "error", text: line})
			}
			continue
		}
		d := diagnostic{severity: m[4], file: m[1], msg: m[5], text: line}
		d.line, _ = strconv.Atoi(m[2])
		d.col, _ = strconv.Atoi(m[3])
		switch file := filepath.Clean(d.file); {
		case samePath(d.file, probe):
			d.probeLine = d.line
			if d.isError() {
				instLine = 0
			}
		case stackLine != 0:
			d.probeLine = stackLine
			headers[file] = stackLine
			for _, f := range stack {
				headers[f] = stackLine
			}
		default:
			d.probeLine = headers[file]
		}
		if instLine != 0 && d.isError() && !samePath(d.file, probe) {
			d.probeLine = instLine
		}
		stack, stackLine = stack[:0], 0
		if d.severity != "warning" && d.severity != "remark" { // -w leaves none anyway
			diags = append(diags, d)
		}
	}
	// clang: an error in a header, then the notes of its instantiation
	// stack, the last one at the place that asked for it. Only the notes
	// that end in " requested here" count (in instantiation of template
	// class, of function template specialization, of member function, of
	// default argument ...): clang has other wordings for other contexts
	// ("required here", "needed here"), which the probe's lines don't make.
	for i := range diags {
		d := &diags[i]
		if !d.isError() || d.file == "" || samePath(d.file, probe) {
			continue
		}
		for _, note := range diags[i+1:] {
			if note.isError() {
				break
			}
			if samePath(note.file, probe) && strings.HasSuffix(note.msg, " requested here") {
				d.probeLine = note.line
			}
		}
	}
	return diags
}

// samePath reports whether the paths a and b, as a compiler prints them,
// name the same file. The compiler prints the probe file's path as it got
// it, so Clean is only for safety.
func samePath(a, b string) bool {
	return a == b || filepath.Clean(a) == filepath.Clean(b)
}

// readOutput reads the compiler output out of the probe pf at the path
// src: which lines of the paths have errors, and which includes.
func readOutput(pf *probeFile, out []byte, src string, includes []string) (*probeRun, error) {
	run := &probeRun{}
	failed := make([]uint, pf.nPaths) // per path, its lines with errors
	canary := false
	var unexpected []diagnostic
	var last *diagnostic // the last error
	diags := parseDiagnostics(out, src)
	for i, d := range diags {
		if !d.isError() {
			continue
		}
		last = &diags[i]
		switch n := d.probeLine; {
		case n >= 1 && n <= includeLines*pf.nIncludes:
			index := (n - 1) / includeLines
			ie := IncludeError{Index: index, Include: includes[index], Msg: d.msg}
			switch {
			case !samePath(d.file, src):
				ie.File, ie.Line, ie.Column = filepath.Clean(d.file), d.line, d.col
			case (n-1)%includeLines == 1:
				// The scope check after the #include.
				ie.Kind = HeaderUnclosed
				ie.Msg = "the header ends inside a declaration or a block that it doesn't close: " + d.msg
			case notFoundRx.MatchString(d.msg):
				ie.Kind = HeaderNotFound
			}
			run.incErrs = append(run.incErrs, ie)
		case n >= pf.firstBlock && n < pf.canary():
			path, offset, _ := pf.block(n)
			failed[path] |= 1 << offset
		case n == pf.canary():
			canary = true
		case n > pf.canary() && samePath(d.file, src):
			// The end of the file: a header leaves a brace open. Its
			// notes ("to match this '{'") say which one.
			if ie, ok := openBrace(d, diags[i+1:], pf, src, includes); ok {
				run.incErrs = append(run.incErrs, ie)
				break
			}
			unexpected = append(unexpected, d)
		default:
			// Also an error in the helper lines: a header that defines a
			// macro that breaks them.
			unexpected = append(unexpected, d)
		}
	}
	switch {
	case len(run.incErrs) > 0:
		// Keep the first error of each include, in include order. The
		// errors mostly come in that order anyway, but gcc reads the whole
		// file before it parses it, so a header that isn't found comes
		// before the syntax errors of the headers above it.
		slices.SortStableFunc(run.incErrs, func(a, b IncludeError) int { return a.Index - b.Index })
		run.incErrs = slices.CompactFunc(run.incErrs, func(a, b IncludeError) bool { return a.Index == b.Index })
		return run, nil
	case len(unexpected) > 0:
		// A driver error (a bad flag), a crash: the probe can't say what
		// it means.
		return nil, fmt.Errorf("unexpected error: %s", strings.ReplaceAll(unexpected[0].text, src, filepath.Base(src)))
	case !canary:
		msg := fmt.Sprintf("it didn't check the whole probe file (no error for its line %d)%s", pf.canary(), outputExcerpt(out))
		if last != nil && last.severity == "fatal error" {
			if path, _, ok := pf.block(last.probeLine); ok {
				// A path's lines stopped the compiler (a template
				// instantiation that is too deep).
				return nil, &errStopped{path: path, msg: msg}
			}
		}
		return nil, errors.New(msg)
	}
	run.kinds = make([]Kind, pf.nPaths)
	for i, f := range failed {
		if pf.kinds {
			run.kinds[i] = classify(f)
		} else {
			run.kinds[i] = classifyNS(f)
		}
	}
	return run, nil
}

// openBrace returns the include error for the error d at the end of the
// probe file pf (at the path src), from the notes after it (notes): the
// first note in a header that an include brought in, such as "to match
// this '{'", gives the include and the place.
func openBrace(d diagnostic, notes []diagnostic, pf *probeFile, src string, includes []string) (IncludeError, bool) {
	for _, note := range notes {
		if note.isError() {
			break
		}
		if n := note.probeLine; n >= 1 && n <= includeLines*pf.nIncludes && note.file != "" && !samePath(note.file, src) {
			index := (n - 1) / includeLines
			return IncludeError{Index: index, Include: includes[index], File: filepath.Clean(note.file),
				Line: note.line, Column: note.col, Msg: d.msg + " (" + note.msg + ")"}, true
		}
	}
	return IncludeError{}, false
}

// outputExcerpt returns the start of the compiler output out for an error
// message: "; its output: " and its first lines, or "; it printed nothing".
func outputExcerpt(out []byte) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return "; it printed nothing"
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 3 {
		lines = append(lines[:3], "...")
	}
	return "; its output: " + strings.Join(lines, " | ")
}

// parseDeps returns the files of the dependency file data (make syntax,
// from -MMD -MT gp_probe), without the probe file src itself. A relative
// path is relative to dir, where the compiler ran.
func parseDeps(data []byte, src, dir string) []string {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\\\n", " ")
	s, _ = strings.CutPrefix(s, "gp_probe:")
	var files []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		f := cur.String()
		cur.Reset()
		if !filepath.IsAbs(f) && dir != "" {
			f = filepath.Join(dir, f)
		}
		if f = filepath.Clean(f); !samePath(f, src) {
			files = append(files, f)
		}
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\t' || s[i+1] == '#'):
			// gcc and clang write a space in a path as "\ " and # as "\#".
			cur.WriteByte(s[i+1])
			i++
		case c == '$' && i+1 < len(s) && s[i+1] == '$':
			cur.WriteByte('$')
			i++
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return files
}
