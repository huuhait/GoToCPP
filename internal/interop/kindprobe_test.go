package interop

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests of the kind probe's kinds and of the macro probe, with the real
// compilers (see nsprobe_test.go).

// kindHeaders are local headers for the kinds of C/C++ names.
var kindHeaders = map[string]string{
	"ui.h": `#pragma once
enum class Color { Red, Green };
enum Plain { PA, PB };
struct CS { int x; };
typedef struct CS CS_t;
int over(int);
int over(double);
int single(int);
template <class T> T ftmpl(T);
template <class T> constexpr bool vtmpl = true;
template <class T, int N> struct mixed {};
template <template <class> class X> struct tt {};
namespace glm { struct vec3 {}; }
// A macro that stands for a namespace.
#define mylib mylib_v2
namespace mylib_v2 { inline int x; }
#define RAYWHITE_LIKE (Color::Green)
#define FN_MACRO(a) ((a) + 1)
#define EMPTY_MACRO
#define GONE 1
#undef GONE
// A typedef of a class whose instantiation fails: only a path through it
// instantiates it.
template <class T> struct BadT { static_assert(sizeof(T) == 0, "gp test: bad"); typedef int x; };
typedef BadT<int> BadI;
#ifdef GOCPP_TEST_FLAG
namespace flag_ns {}
#endif
// Templates whose parameters none of gp_ktmpl's shapes takes.
template <class, class, int> struct cci {};
template <int, int, class> struct iic {};
namespace glm2 { enum qualifier { defaultp }; template <int L, class T, qualifier Q = defaultp> struct vec {}; }
template <template <class> class, class> struct tc {};
template <class, template <class> class> struct ct {};
template <class T, class U = int, int N = 3> struct dflt {};
template <class A, class B, int N> using aliasCCI = cci<A, B, N>;
template <int N, class T, class U> using fnp = T (*)(U);
template <class T> using alias1 = T*;
// Variable templates with values first, and mixed.
template <int N> constexpr int nvar = N;
template <int N, class T> constexpr int v4 = N;
template <class T, int N> constexpr int v5 = N;
// An aggregate: a type, not a template, though T{...} works.
struct Agg { int a, b; };
typedef Agg AggT;
// A path through Deep<int> stops the compiler (the instantiation is too
// deep).
template <class T> struct Deep { using type = typename Deep<T*>::type; };
typedef Deep<int> DeepI;
typedef Deep<char> DeepC;
typedef Deep<long> DeepL;
typedef Deep<short> DeepS;
`,
	"flagdir/only_with_flag.h": `#pragma once
namespace only_with_flag_ns {}
`,
}

var kindIncludes = []string{"<vector>", "<algorithm>", "<chrono>", "<iostream>", "<string>", "<filesystem>",
	"<array>", "<bitset>", "<cmath>", "<cerrno>", "<cstdio>", "<type_traits>", "<memory>", "<tuple>", "<utility>",
	"<ranges>", `"./ui.h"`}

// kindCases are the paths of TestProbeKinds and their kinds. A case with
// alt can also have that kind (see the comments).
var kindCases = []struct {
	path      string
	want, alt Kind
}{
	{path: "std", want: Namespace},
	{path: "std::chrono", want: Namespace},
	{path: "std::filesystem", want: Namespace},
	{path: "std::chrono::steady_clock", want: Type},
	{path: "std::chrono::steady_clock::now", want: Value}, // a static member function
	{path: "std::cout", want: Value},
	{path: "std::string", want: Type},        // a typedef of a specialization
	{path: "std::string::npos", want: Value}, // a static data member
	{path: "std::size_t", want: Type},
	{path: "std::filesystem::path", want: Type},
	// Templates with type parameters, with values, and with both.
	{path: "std::vector", want: Template},
	{path: "std::tuple", want: Template},
	{path: "std::array", want: Template},
	{path: "std::bitset", want: Template},
	{path: "std::integer_sequence", want: Template},
	{path: "std::enable_if_t", want: Template}, // an alias template
	{path: "mixed", want: Template},
	{path: "tt", want: Template}, // a template of templates
	// The braced line (class template argument deduction) takes the
	// shapes that gp_ktmpl doesn't.
	{path: "std::ranges::subrange", want: Template}, // class, class, an enum value
	{path: "cci", want: Template},
	{path: "iic", want: Template},
	{path: "glm2::vec", want: Template}, // glm's vec<L, T, Q>
	{path: "tc", want: Template},
	{path: "ct", want: Template},
	{path: "dflt", want: Template},
	{path: "aliasCCI", want: Template}, // an alias template of such a class
	{path: "fnp", want: Template},      // an alias template of a type that is not a class
	{path: "alias1", want: Template},
	{path: "Agg", want: Type}, // Agg{...} works too: the type line comes first
	{path: "AggT", want: Type},
	// Values that take template arguments.
	{path: "std::max", want: ValueTemplate},
	{path: "std::get", want: ValueTemplate},
	{path: "std::make_unique", want: ValueTemplate},
	{path: "std::is_same_v", want: ValueTemplate}, // a variable template
	{path: "std::in_place_index", want: ValueTemplate},
	{path: "std::in_place_type", want: ValueTemplate},
	{path: "nvar", want: ValueTemplate}, // values
	{path: "v4", want: ValueTemplate},   // a value, then a type
	{path: "v5", want: ValueTemplate},   // a type, then a value
	{path: "ftmpl", want: ValueTemplate},
	{path: "vtmpl", want: ValueTemplate},
	// An overloaded function: libstdc++'s std::sqrt has template overloads
	// for integers too; libc++'s may not.
	{path: "std::sqrt", want: ValueTemplate, alt: Value},
	{path: "printf", want: Value},
	{path: "over", want: Value}, // overloaded, no templates
	{path: "single", want: Value},
	// A header package's C-level names (ui.Color is ::Color).
	{path: "Color", want: Type}, // an enum class
	{path: "Color::Red", want: Value},
	{path: "Plain", want: Type},
	{path: "PA", want: Value},
	{path: "CS", want: Type}, // a C struct
	{path: "CS_t", want: Type},
	{path: "CS::x", want: Value},
	// glm is a namespace too: glm.vec3 is ::glm::vec3, not ::vec3.
	{path: "glm", want: Namespace},
	{path: "glm::vec3", want: Type},
	{path: "vec3", want: NotFound},
	// Macros, also one that stands for a namespace, and paths through one.
	{path: "mylib", want: Macro},
	{path: "mylib::x", want: Macro},
	{path: "mylib_v2::x", want: Value},
	{path: "RAYWHITE_LIKE", want: Macro},
	{path: "FN_MACRO", want: Macro},
	{path: "EMPTY_MACRO", want: Macro},
	{path: "GONE", want: NotFound},
	{path: "errno", want: Macro},
	{path: "EOF", want: Macro},
	{path: "std::EOF", want: Macro},
	// Not found, or not classified.
	{path: "stdd", want: NotFound},
	{path: "std::vectorr", want: NotFound},
	{path: "std::cout::flush", want: NotFound}, // a member of an object
	{path: "std::integral", want: NotFound},    // a concept
	// The compilers differ (see ProbeKinds).
	{path: "std::string::size", want: Value, alt: NotFound},
	{path: "std::string::find", want: ValueTemplate, alt: NotFound},
	{path: "CS::CS", want: Value, alt: NotFound}, // the injected class name
	// Built-in macros that -dM doesn't list (see ProbeMacros).
	{path: "__LINE__", want: Macro},
	{path: "__has_include", want: Macro},
	// The instantiation fails inside ui.h, for this path's lines: gcc
	// still finds the typedef after that, clang doesn't. It is not an
	// include error either way.
	{path: "BadI::x", want: Type, alt: NotFound},
	{path: "BadI", want: Type},
	// C++ keywords, each followed by a path that must keep its kind: a
	// keyword must not hide the lines after it.
	{path: "class", want: NotFound},
	{path: "std::cerr", want: Value},
	{path: "template", want: NotFound},
	{path: "std::wstring", want: Type},
	{path: "struct", want: NotFound},
	{path: "std::unique_ptr", want: Template},
	{path: "asm", want: NotFound},
	{path: "std::min", want: ValueTemplate},
	{path: "operator", want: NotFound},
	{path: "std::chrono::seconds", want: Type},
	{path: "__attribute__", want: NotFound},
	{path: "PB", want: Value},
	{path: "namespace", want: NotFound},
	{path: "std::pair", want: Template},
	{path: "std::requires", want: NotFound},
	{path: "std::this_thread", want: NotFound}, // <thread> is not included
	{path: "std::ranges", want: Namespace},
}

func TestProbeKinds(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		writeFiles(t, dir, kindHeaders)
		c := newCache()
		var paths []string
		for _, kc := range kindCases {
			if slices.Contains(paths, kc.path) {
				t.Fatalf("%s is twice in kindCases: the probe asks once", kc.path)
			}
			paths = append(paths, kc.path)
		}
		res, err := c.probe(context.Background(), cxx, dir, nil, kindIncludes, paths)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.IncludeErrors) > 0 {
			t.Fatalf("include errors: %v", res.IncludeErrors)
		}
		for _, kc := range kindCases {
			if got := res.Kinds[kc.path]; got != kc.want && (kc.alt == Unknown || got != kc.alt) {
				t.Errorf("%s: %v, want %v", kc.path, got, kc.want)
			}
		}
		if n := c.runs.Load(); n != 1 {
			t.Errorf("%d compiler runs, want 1", n)
		}

		// The macro probe agrees: a path is a Macro exactly when one of its
		// parts is a macro.
		ms, err := c.probeMacros(context.Background(), cxx, dir, nil, kindIncludes)
		if err != nil {
			t.Fatal(err)
		}
		builtin := []string{"__LINE__", "__has_include"}
		for _, kc := range kindCases {
			parts, _ := splitPath(kc.path)
			if got, want := res.Kinds[kc.path] == Macro, slices.ContainsFunc(parts, func(p string) bool { return ms.Kind(p) != NotMacro || slices.Contains(builtin, p) }); got != want {
				t.Errorf("%s: the kind probe says %v, the macro probe says a part is a macro: %v", kc.path, res.Kinds[kc.path], want)
			}
		}
	})
}

func TestProbeMacros(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		writeFiles(t, dir, kindHeaders)
		c := newCache()
		ctx := context.Background()
		ms, err := c.probeMacros(ctx, cxx, dir, nil, []string{"<cstdio>", "<cerrno>", `"./ui.h"`})
		if err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]MacroKind{
			"EOF": ObjectLike, "errno": ObjectLike, "stdin": ObjectLike,
			"mylib": ObjectLike, "RAYWHITE_LIKE": ObjectLike, "EMPTY_MACRO": ObjectLike, "FN_MACRO": FunctionLike,
			"GONE": NotMacro, "printf": NotMacro, "Color": NotMacro, "": NotMacro,
			// The compiler's own macros are there too (see macroprobe.go).
			"__cplusplus": ObjectLike, "__GNUC__": ObjectLike,
			// Only in the gnu++ modes.
			"linux": NotMacro, "unix": NotMacro,
		} {
			if got := ms.Kind(name); got != want {
				t.Errorf("Kind(%q) = %v, want %v", name, got, want)
			}
		}
		names := ms.Names()
		if len(names) != ms.Len() || ms.Len() < 100 || !slices.IsSorted(names) || !slices.Contains(names, "FN_MACRO") {
			t.Errorf("Names(): %d names, Len %d, sorted %v", len(names), ms.Len(), slices.IsSorted(names))
		}

		// The flags reach the compiler.
		flagged, err := c.probeMacros(ctx, cxx, dir, []string{"-std=gnu++20", "-DGOCPP_TEST_FLAG=1"}, []string{"<cstdio>", "<cerrno>", `"./ui.h"`})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"GOCPP_TEST_FLAG", "unix"} {
			if flagged.Kind(name) != ObjectLike {
				t.Errorf("with the flags: Kind(%q) = %v, want ObjectLike", name, flagged.Kind(name))
			}
		}
		if ms.Kind("GOCPP_TEST_FLAG") != NotMacro {
			t.Error("the flags changed the result without flags")
		}

		// Without includes: the predefined macros.
		pre, err := c.probeMacros(ctx, cxx, dir, nil, nil)
		if err != nil || pre.Kind("__cplusplus") != ObjectLike || pre.Kind("EOF") != NotMacro {
			t.Errorf("macros without includes: %d macros, %v", pre.Len(), err)
		}

		// An include that doesn't preprocess.
		if _, err := c.probeMacros(ctx, cxx, dir, nil, []string{"<cstdio>", "<gocpp_no_such_header.h>"}); err == nil || !strings.Contains(err.Error(), "gocpp_no_such_header.h") {
			t.Errorf("macros of a missing header: %v, want an error about it", err)
		}
		if _, err := c.probeMacros(ctx, cxx, dir, nil, []string{"<cstdio>", "x.h"}); err == nil || !strings.Contains(err.Error(), "macro probe: bad include") {
			t.Errorf("a bad include: %v", err)
		}
		if _, err := c.probeMacros(ctx, cxx, dir, []string{"-Dx\x00y"}, []string{"<cstdio>"}); err == nil || !strings.Contains(err.Error(), "macro probe: bad flag") {
			t.Errorf("a bad flag: %v", err)
		}
	})
}

func TestProbeMacrosCache(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		writeFiles(t, dir, kindHeaders)
		c := newCache()
		ctx := context.Background()
		var runs int64
		probe := func(why string, wantRun bool, flags, includes []string, macro string, want MacroKind) {
			t.Helper()
			ms, err := c.probeMacros(ctx, cxx, dir, flags, includes)
			if err != nil {
				t.Fatalf("%s: %v", why, err)
			}
			if got := ms.Kind(macro); got != want {
				t.Errorf("%s: Kind(%q) = %v, want %v", why, macro, got, want)
			}
			if wantRun {
				runs++
			}
			if n := c.macroRuns.Load(); n != runs {
				t.Errorf("%s: %d compiler runs so far, want %d", why, n, runs)
				runs = n
			}
		}
		includes := []string{"<cstdio>", `"./ui.h"`}
		probe("first call", true, nil, includes, "FN_MACRO", FunctionLike)
		probe("the same again", false, nil, includes, "FN_MACRO", FunctionLike)
		probe("another flag", true, []string{"-DGOCPP_TEST_FLAG"}, includes, "GOCPP_TEST_FLAG", ObjectLike)
		probe("the flag again", false, []string{"-DGOCPP_TEST_FLAG"}, includes, "GOCPP_TEST_FLAG", ObjectLike)
		probe("no flag again", false, nil, includes, "GOCPP_TEST_FLAG", NotMacro)
		probe("other includes", true, nil, []string{`"./ui.h"`}, "EOF", NotMacro)
		writeFiles(t, dir, map[string]string{"ui.h": "#pragma once\n#define FN_MACRO 2\n"})
		probe("ui.h changed", true, nil, includes, "FN_MACRO", ObjectLike)
		probe("ui.h changed, cached", false, nil, includes, "FN_MACRO", ObjectLike)

		// Calls for one include set at the same time run the compiler once.
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				if ms, err := c.probeMacros(ctx, cxx, dir, nil, []string{"<vector>"}); err != nil || ms.Kind("__cplusplus") != ObjectLike {
					t.Errorf("concurrent probe: %v", err)
				}
			})
		}
		wg.Wait()
		if n := c.macroRuns.Load(); n != runs+1 {
			t.Errorf("%d compiler runs for 8 calls at the same time, want 1", n-runs)
		}
	})
}

func TestProbeKindsFlags(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		writeFiles(t, dir, kindHeaders)
		c := newCache()
		ctx := context.Background()
		var runs int64
		probe := func(why string, wantRun bool, flags, includes []string, cases ...kindCase) {
			t.Helper()
			res, err := c.probe(ctx, cxx, dir, flags, includes, candsOf(cases))
			if err != nil {
				t.Fatalf("%s: %v", why, err)
			}
			checkKinds(t, res, cases)
			if wantRun {
				runs++
			}
			if n := c.runs.Load(); n != runs {
				t.Errorf("%s: %d compiler runs so far, want %d", why, n, runs)
				runs = n
			}
		}
		includes := []string{"<vector>", `"./ui.h"`}
		probe("no flags", true, nil, includes, kindCase{"flag_ns", NotFound}, kindCase{"Color", Type})
		probe("no flags, cached", false, []string{}, includes, kindCase{"flag_ns", NotFound}, kindCase{"Color", Type})
		probe("a -D flag", true, []string{"-DGOCPP_TEST_FLAG"}, includes, kindCase{"flag_ns", Namespace}, kindCase{"Color", Type})
		probe("a -D flag, cached", false, []string{"-DGOCPP_TEST_FLAG"}, includes, kindCase{"flag_ns", Namespace})
		probe("another -D flag", true, []string{"-DGOCPP_TEST_FLAG=2"}, includes, kindCase{"flag_ns", Namespace})
		// The flags come after the probe's -std=c++20.
		probe("-std", true, []string{"-std=c++23"}, []string{"<expected>"}, kindCase{"std::expected", Template})

		// A header that only a -I flag finds.
		res, err := c.probe(ctx, cxx, dir, nil, []string{"<only_with_flag.h>"}, []string{"only_with_flag_ns"})
		if err != nil || len(res.IncludeErrors) != 1 || res.IncludeErrors[0].Kind != HeaderNotFound {
			t.Errorf("without -I: %+v, %v; want the header not found", res, err)
		}
		runs++
		probe("with -I", true, []string{"-I" + dir + "/flagdir"}, []string{"<only_with_flag.h>"}, kindCase{"only_with_flag_ns", Namespace})

		// A changed header with flags is noticed too.
		writeFiles(t, dir, map[string]string{"flagdir/only_with_flag.h": "#pragma once\nnamespace only_with_flag_v2 {}\n"})
		probe("the header changed", true, []string{"-I" + dir + "/flagdir"}, []string{"<only_with_flag.h>"},
			kindCase{"only_with_flag_ns", NotFound}, kindCase{"only_with_flag_v2", Namespace})

		if _, err := c.probe(ctx, cxx, dir, []string{"-Dx\x00y"}, includes, nil); err == nil || !strings.Contains(err.Error(), "kind probe: bad flag") {
			t.Errorf("a bad flag: %v", err)
		}
	})
}

// BenchmarkKindProbe measures the probes on the include set of the plan's
// step 4.1: one kind probe run without paths (what the headers cost), with
// 50 paths, and with 50 paths of which 20 are not found (each failed line
// prints errors and notes), and one macro probe run.
func BenchmarkKindProbe(b *testing.B) {
	includes := []string{"<vector>", "<algorithm>", "<chrono>", "<iostream>", "<string>", "<filesystem>"}
	paths := []string{"std", "std::chrono", "std::filesystem", "std::ranges", "std::chrono::steady_clock",
		"std::chrono::steady_clock::now", "std::chrono::system_clock", "std::chrono::seconds", "std::chrono::milliseconds",
		"std::chrono::duration", "std::chrono::duration_cast", "std::chrono::time_point", "std::cout", "std::cerr", "std::cin",
		"std::endl", "std::string", "std::string::npos", "std::wstring", "std::string_view", "std::to_string", "std::stoi",
		"std::getline", "std::vector", "std::sort", "std::find", "std::find_if", "std::max", "std::min", "std::minmax",
		"std::accumulate", "std::reverse", "std::unique", "std::transform", "std::ranges::sort", "std::filesystem::path",
		"std::filesystem::exists", "std::filesystem::directory_iterator", "std::filesystem::current_path", "std::size_t",
		"std::move", "std::swap", "std::pair", "std::make_pair", "std::begin", "std::end", "std::ios", "std::ios::binary",
		"std::ostream", "std::allocator"}
	missing := slices.Clone(paths[:30])
	for i := range 20 {
		missing = append(missing, "std::no_such_name_"+strings.Repeat("x", i))
	}
	matrix := compilerMatrix()
	required := requiredCXX(b, matrix)
	for _, prog := range matrix {
		name := filepath.Base(prog)
		b.Run(name, func(b *testing.B) {
			cxx := lookTestCompiler(b, prog, slices.Contains(required, prog) || slices.Contains(required, name))
			dir := b.TempDir()
			ctx := context.Background()
			res, err := newCache().probe(ctx, cxx, dir, nil, includes, paths)
			if err != nil || len(res.IncludeErrors) > 0 {
				b.Fatalf("the probe doesn't work here: %v %v", err, res.IncludeErrors)
			}
			if len(paths) != 50 || res.Kinds["std::chrono::steady_clock"] != Type || res.Kinds["std::cout"] != Value {
				b.Fatalf("%d paths, kinds %v", len(paths), res.Kinds)
			}
			for _, c := range []struct {
				name  string
				paths []string
			}{{"no-paths", nil}, {"50-paths", paths}, {"50-paths-20-missing", missing}} {
				b.Run(c.name, func(b *testing.B) {
					for b.Loop() {
						if _, err := newCache().probe(ctx, cxx, dir, nil, includes, c.paths); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
			b.Run("macros", func(b *testing.B) {
				for b.Loop() {
					if _, err := newCache().probeMacros(ctx, cxx, dir, nil, includes); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("cached", func(b *testing.B) {
				c := newCache()
				if _, err := c.probe(ctx, cxx, dir, nil, includes, paths); err != nil {
					b.Fatal(err)
				}
				for b.Loop() {
					if _, err := c.probe(ctx, cxx, dir, nil, includes, paths); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func TestProbeKindsStopped(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		writeFiles(t, dir, kindHeaders)
		ctx := context.Background()
		includes := []string{"<vector>", `"./ui.h"`}
		// A path through Deep stops the compiler: it is NotFound, and the
		// probe runs again without it, for the others.
		cases := []kindCase{{"std", Namespace}, {"DeepI::type", NotFound}, {"Color", Type}, {"DeepC::type", NotFound}, {"std::vector", Template}}
		res, err := newCache().probe(ctx, cxx, dir, nil, includes, candsOf(cases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, cases)
		// Also in the namespace mode.
		res, err = newCache().probeNamespaces(ctx, cxx, dir, nil, includes, []string{"std", "DeepI::type", "Color"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"std", Namespace}, {"DeepI::type", NotNamespace}, {"Color", NotNamespace}})
		// More of them than maxReruns: the probe fails.
		paths := []string{"std", "DeepI::type", "DeepC::type", "DeepL::type", "DeepS::type"}
		if res, err := newCache().probe(ctx, cxx, dir, nil, includes, paths); err == nil || !strings.Contains(err.Error(), "it didn't check the whole probe file") {
			t.Errorf("probe with %d paths that stop the compiler = %+v, %v; want an error", len(paths)-1, res, err)
		}
	})
}

// nsCases are probeCases as the namespace mode answers them.
func nsCases() []kindCase {
	var cases []kindCase
	for _, c := range probeCases {
		cases = append(cases, kindCase{c.cand, nsAnswer(c.want)})
	}
	return cases
}

func TestProbeNamespaces(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := testDir(t)
		c := newCache()
		ctx := context.Background()
		res, err := c.probeNamespaces(ctx, cxx, dir, nil, probeIncludes, candsOf(probeCases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, nsCases())
		// A namespace answer doesn't answer for the kinds: one more run, and
		// then both are answered from the cache.
		res, err = c.probe(ctx, cxx, dir, nil, probeIncludes, candsOf(probeCases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, probeCases)
		res, err = c.probeNamespaces(ctx, cxx, dir, nil, probeIncludes, candsOf(probeCases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, nsCases())
		if n := c.runs.Load(); n != 2 {
			t.Errorf("%d runs, want 2", n)
		}
		// A namespace run doesn't replace the kinds the cache has: a new
		// name makes a namespace run for all the names of the call.
		res, err = c.probeNamespaces(ctx, cxx, dir, nil, probeIncludes, []string{"std", "Point", "new_name"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"std", Namespace}, {"Point", NotNamespace}, {"new_name", NotNamespace}})
		res, err = c.probe(ctx, cxx, dir, nil, probeIncludes, []string{"std", "Point", "::std::vector"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"std", Namespace}, {"Point", Type}, {"::std::vector", Template}})
		if n := c.runs.Load(); n != 3 {
			t.Errorf("%d runs, want 3", n)
		}
		// The public function, a reserved name, and include errors.
		res, err = ProbeNamespaces(ctx, cxx, dir, nil, []string{`"physics.h"`}, []string{"physics_ns", "stdd", "gp_x"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"physics_ns", Namespace}, {"stdd", NotNamespace}, {"gp_x", NotNamespace}})
		res, err = c.probeNamespaces(ctx, cxx, dir, nil, []string{"<gocpp_no_such_header.h>"}, []string{"std", "gp_x"})
		if err != nil || len(res.IncludeErrors) != 1 || res.Kinds["std"] != Unknown || res.Kinds["gp_x"] != NotNamespace {
			t.Errorf("include errors: %+v, %v", res, err)
		}
	})
}

func TestProbeKindsMessageFlags(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		// Flags that would change the messages that the probe reads have
		// no effect: the probe drops them.
		dir := testDir(t)
		flags := []string{"-Wfatal-errors", "-fmax-errors=1", "-ferror-limit=1", "-fmessage-length=20", "-fdiagnostics-color=always",
			"-fdiagnostics-show-caret", "-Wall", "-Werror", "-Wextra"}
		switch cxx.Family {
		case build.GCC:
			flags = append(flags, "-fdiagnostics-format=json", "-fdiagnostics-urls=always", "-ftrack-macro-expansion=2")
		case build.Clang:
			flags = append(flags, "-fdiagnostics-format=msvc", "-fdiagnostics-print-source-range-info", "-fcolor-diagnostics", "-fno-show-column")
		}
		cases := []kindCase{{"stdd", NotFound}, {"std", Namespace}, {"fmtt", NotFound}, {"::std::vector", Template}, {"Point", Type}}
		res, err := newCache().probe(context.Background(), cxx, dir, flags, []string{"<vector>", `"./physics.h"`}, candsOf(cases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, cases)
		ms, err := newCache().probeMacros(context.Background(), cxx, dir, flags, []string{"<vector>"})
		if err != nil || ms.Kind("__cplusplus") != ObjectLike {
			t.Errorf("macro probe with the flags: %d macros, %v", ms.Len(), err)
		}
	})
}

// TestProbeGeneratedErrors checks that the probes reject what the build of
// generated C++ rejects (quietFlags): with g++ too, a narrowing conversion
// in an inline function of a local header is an error of the header (the
// build's -Werror=narrowing), also when the package's flags have -Wall and
// -Werror, and a header with a warning only (a sign comparison) is fine.
func TestProbeGeneratedErrors(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"narrow.h": "#pragma once\n#include <cstdint>\nstruct NP { int x; };\ninline NP narrow_np(int64_t a) { return NP{a}; }\n",
			"warn.h":   "#pragma once\nnamespace warn_ns {}\ninline bool warn_cmp(int a, unsigned b) { return a < b; }\n",
		})
		for _, flags := range [][]string{nil, {"-Wall", "-Wextra", "-Werror"}} {
			res, err := newCache().probe(context.Background(), cxx, dir, flags, []string{`"./narrow.h"`}, []string{"NP"})
			if err != nil || len(res.IncludeErrors) != 1 || res.IncludeErrors[0].Kind != HeaderErrors {
				t.Errorf("narrow.h with flags %q: %+v, %v; want an error in the header", flags, res, err)
			}
			cases := []kindCase{{"warn_ns", Namespace}}
			res, err = newCache().probe(context.Background(), cxx, dir, append(flags, "-Wsign-compare"), []string{`"./warn.h"`}, candsOf(cases))
			if err != nil || len(res.IncludeErrors) > 0 {
				t.Fatalf("warn.h with flags %q: %+v, %v", flags, res, err)
			}
			checkKinds(t, res, cases)
		}
	})
}

// BenchmarkNamespaceProbe measures what the language server's probe costs
// with misspelled names: <vector>, std and 20 names that are not
// declared, in the namespace mode and with all the kinds.
func BenchmarkNamespaceProbe(b *testing.B) {
	includes := []string{"<vector>"}
	names := []string{"std"}
	for i := range 20 {
		names = append(names, "stdd"+strings.Repeat("x", i))
	}
	matrix := compilerMatrix()
	required := requiredCXX(b, matrix)
	for _, prog := range matrix {
		name := filepath.Base(prog)
		b.Run(name, func(b *testing.B) {
			cxx := lookTestCompiler(b, prog, slices.Contains(required, prog) || slices.Contains(required, name))
			dir := b.TempDir()
			ctx := context.Background()
			b.Run("no-names", func(b *testing.B) {
				for b.Loop() {
					if _, err := newCache().probeNamespaces(ctx, cxx, dir, nil, includes, nil); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("namespaces", func(b *testing.B) {
				for b.Loop() {
					if _, err := newCache().probeNamespaces(ctx, cxx, dir, nil, includes, names); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("kinds", func(b *testing.B) {
				for b.Loop() {
					if _, err := newCache().probe(ctx, cxx, dir, nil, includes, names); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
