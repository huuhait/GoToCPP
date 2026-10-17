package interop

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// extern "C" for C headers (Phase 04 README §2.9, step 4.14).
//
// The build includes a C header without a __cplusplus guard inside an
// extern "C" { } block, so that its functions get C linkage (C names, not
// C++-mangled ones) and link with the C code that defines them. The check
// (internal/check) decides which headers that is: for a local .h header,
// it asks CheckCHeader. The probes must see the headers as the build does
// (a C header without a guard can redeclare a C function with C++
// linkage, which is an error in C++ and fine inside the block), so an
// include that the build wraps is passed to them marked with ExternC, and
// the probe file wraps it too.

// externCPrefix starts an include text marked with ExternC.
const externCPrefix = `extern "C" `

// ExternC returns the include text inc (<lua.h>, "./legacy.h") marked so
// that ProbeKinds, ProbeNamespaces and ProbeMacros include the header inside
// an extern "C" { } block, as the build does: extern "C" "./legacy.h". The
// mark is part of the include set, so the cache keeps the two apart.
func ExternC(inc string) string { return externCPrefix + inc }

// SplitExternC returns the include text inc without the mark of ExternC,
// and whether it had the mark.
func SplitExternC(inc string) (string, bool) {
	return strings.CutPrefix(inc, externCPrefix)
}

// A CHeader is what CheckCHeader found out about a local header.
type CHeader struct {
	// C reports whether the header parses as C: the C compiler checked a
	// file that includes it (-fsyntax-only -x c) without an error. It is
	// false also when the header isn't found: then the probes report it.
	C bool

	// Guard reports whether the header's own text has the word
	// __cplusplus, as in the usual guard:
	//
	//	#ifdef __cplusplus
	//	extern "C" {
	//	#endif
	//
	// Only the header's own text counts, not the headers that it
	// includes, and a __cplusplus in a comment or a string counts too.
	Guard bool
}

// cplusplusRx matches the word __cplusplus.
var cplusplusRx = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])__cplusplus(?:[^A-Za-z0-9_]|$)`)

// CheckCHeader tells whether the local header of the include text include
// ("./legacy.h", in quotes) is a C header: whether it parses as C with the
// C compiler cc, the folder dir of the package and the flags, and whether
// its text has a __cplusplus guard (see CHeader). The compiler checks a
// file with only #include "./legacy.h" in it, with -std=gnu17 (the
// standard of the package's .c files), -iquote dir and the flags, as for
// the package's .c files: so the header is found as in the build, and the
// flags are the package's C flags (GOCPP_CFLAGS, then those of
// //gocpp:cflags and pkg-config). The probe checks only that: it can't
// tell whether a flag is safe to run, which is the caller's job.
//
// The error is for a check that can't be done: bad arguments, a compiler
// that doesn't run, or a time limit (ctx, and at most probeTimeout). A
// header that doesn't parse as C is no error: CHeader.C is false.
//
// The results are cached in memory, per compiler, folder, flags and
// include (and on disk, see diskcache.go), and checked against the headers
// that the compiler read, the system headers too, and the headers that it didn't find (as
// ProbeKinds' results are; see runCHeaderCheck): a result holds until one
// of them changes or appears. Only without such a list (a compiler that
// writes none) is a result used for 5 seconds at most. CheckCHeader is
// safe for concurrent use.
func CheckCHeader(ctx context.Context, cc build.CXX, dir string, flags []string, include string) (CHeader, error) {
	return defaultCache.checkCHeader(ctx, cc, dir, flags, include)
}

// A cHeaderSet is what the cache knows of one header.
type cHeaderSet struct {
	filled  bool        // the fields below hold a result
	files   []fileState // what the result depends on
	res     CHeader
	expires time.Time // when not zero: when to check again
}

// checkCHeader is CheckCHeader with the cache c.
func (c *cache) checkCHeader(ctx context.Context, cc build.CXX, dir string, flags []string, include string) (CHeader, error) {
	if err := checkArgs(flags, []string{include}); err != nil {
		return CHeader{}, fmt.Errorf("parse-as-C check: %w", err)
	}
	if _, marked := SplitExternC(include); marked || include[0] != '"' {
		return CHeader{}, fmt.Errorf("parse-as-C check: bad include %q: want a local header in quotes", include)
	}
	if cc.Family != build.Clang && cc.Family != build.GCC {
		return CHeader{}, fmt.Errorf("parse-as-C check: %w", unknownFamily(cc))
	}
	if dir != "" {
		var err error
		if dir, err = filepath.Abs(dir); err != nil {
			return CHeader{}, err
		}
	}
	set, unlock, err := c.cheaders.lock(ctx, setKey(cc, dir, flags, []string{include}))
	if err != nil {
		return CHeader{}, err
	}
	defer unlock()
	if set.filled && c.valid(set.files, !set.expires.IsZero(), set.expires) {
		return set.res, nil
	}
	*set = cHeaderSet{}
	key, onDisk := c.disk.keyFor("c", cc, dir, flags, []string{include})
	if onDisk {
		if loaded, ok := c.loadCHeader(key); ok {
			*set = *loaded
			return set.res, nil
		}
	}
	c.cRuns.Add(1)

	// The header's text, for the guard. It is read before the compiler
	// runs, and the result is cached only when the file that the cache
	// checks later has the same content. A header that can't be read
	// doesn't parse as C either, and the probes report it.
	path := include[1 : len(include)-1]
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	path = filepath.Clean(path)
	var res CHeader
	text, err := os.ReadFile(path)
	exists := err == nil
	res.Guard = cplusplusRx.Match(text)

	ok, deps, depsOK, missing, start, err := runCHeaderCheck(ctx, cc, dir, flags, include)
	if err != nil {
		return CHeader{}, err
	}
	res.C = ok
	files, cacheable := dependsOn(deps, depsOK, !depsOK, start, dir, []string{include})
	for _, f := range files {
		if f.path == path && (f.exists != exists || exists && f.hash != sha256.Sum256(text)) {
			cacheable = false // it changed after it was read
		}
	}
	if cacheable {
		*set = cHeaderSet{filled: true, files: files, res: res}
		if !depsOK {
			set.expires = time.Now().Add(c.errorTTL)
		}
		// Not on disk when a header wasn't found: the places that are
		// watched are only some of the places where it can appear (not
		// the compiler's own folders, CPATH, ...), and the result would
		// stay wrong for good. In memory it stays until a place changes.
		if onDisk && !missing {
			c.storeCHeader(key, set)
		}
	}
	return res, nil
}

// runCHeaderCheck runs the C compiler cc over a file that includes the
// header include, and reports whether it had no error, the files that the
// answer depends on (and whether it has that list), and when it started.
//
// The files are the headers that the compiler read (the system headers
// too, -MD), and, when a header wasn't found, the places where it would be
// found if it appeared (missingPlaces). The compilers write no dependency
// file (-MD) when they stop at an error (-Wfatal-errors), and gcc none at
// all after an error: so after a failure, a second run only preprocesses
// (-E -M -MG: the headers that aren't found count as generated files, and
// the preprocessor goes on after them and after an #error), and its list
// is the one of the answer. That run is cheap, and only a header that
// isn't C pays it. The first error stops the parse, so only the first
// header that isn't found matters: one that is included after the first
// error can't change the answer when it appears.
//
// missing is true when a header wasn't found.
//
// Known limit: the first run fails by design, and a compiler that exits with an error status
// hides a cut output (see runCompiler), so a cut output could misclassify the failure.
func runCHeaderCheck(ctx context.Context, cc build.CXX, dir string, flags []string, include string) (ok bool, deps []string, depsOK, missing bool, start time.Time, err error) {
	tmp, err := probeTemp()
	if err != nil {
		return false, nil, false, false, start, fmt.Errorf("parse-as-C check: %w", err)
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "gp_cheader.c")
	if err := os.WriteFile(src, []byte("#include "+include+"\n"), 0o644); err != nil {
		return false, nil, false, false, start, fmt.Errorf("parse-as-C check: %w", err)
	}
	args := []string{"-std=gnu17"}
	if dir != "" {
		args = append(args, "-iquote", dir)
	}
	// The package's flags, then the check's own, which they can't turn
	// off: no warnings, and the first error stops the compiler (the
	// answer is there).
	args = append(args, flags...)
	run := func(mode []string, depFile string) (bool, []string, bool, []byte, error) {
		args := slices.Concat(args, mode, []string{"-w", "-fdiagnostics-color=never", "-MF", depFile, "-MT", "gp_probe", "-x", "c", src})
		ctx, cancel := context.WithTimeout(ctx, probeTimeout)
		defer cancel()
		out := &capBuffer{max: 1 << 16, stop: func() {}}
		cmd, err := runCompiler(ctx, cc, dir, tmp, args, out, out)
		if err := runFailure(ctx, cc.Name(), cmd, err, nil); err != nil {
			return false, nil, false, nil, fmt.Errorf("parse-as-C check: %w", err)
		}
		data, err := os.ReadFile(depFile)
		if err != nil {
			return cmd.ProcessState.Success(), nil, false, out.buf.Bytes(), nil
		}
		return cmd.ProcessState.Success(), parseDeps(data, src, dir), true, out.buf.Bytes(), nil
	}
	start = time.Now()
	ok, deps, depsOK, out, err := run([]string{"-fsyntax-only", "-Wfatal-errors", "-MD"}, filepath.Join(tmp, "gp_cheader.d"))
	if err != nil || ok && depsOK {
		return ok, deps, depsOK, false, start, err
	}
	places, known := missingPlaces(out, src, dir, flags)
	_, deps, depsOK, _, err = run([]string{"-E", "-M", "-MG"}, filepath.Join(tmp, "gp_cheader.mm.d"))
	return ok, append(deps, places...), depsOK && known, reportsMissing(out), start, err
}

// missingRx matches the compilers' message for an #include whose header
// isn't found (LC_ALL=C), at its place: gcc's "x.h: No such file or
// directory" and clang's "'x.h' file not found", which can go on with
// " with <angled> include; use \"quotes\" instead".
var missingRx = regexp.MustCompile(`^(.+?):\d+:(?:\d+:)? fatal error: (?:(.+): No such file or directory|'(.+)' file not found.*)$`)

// missingPlaces returns the places where the header that the compiler
// output out says wasn't found would be found if it appeared, when there
// is such a message: the folder of the file with the #include (not the
// probe file src), the package folder dir (-iquote), and the folders of
// the -iquote, -I, -isystem and -idirafter flags, in that order. These are
// all the places of a quoted include, and more than an angled one has (the
// two can't be told apart from the message), which costs at most a check
// that runs again. The system folders of the compiler are not watched: a
// header that appears there is not noticed, as for ProbeKinds (and as a
// changed system header isn't). known is false when out has a message for
// a header that isn't found, but not one that missingPlaces understands.
func missingPlaces(out []byte, src, dir string, flags []string) (places []string, known bool) {
	var name, from string
	for line := range strings.SplitSeq(string(out), "\n") {
		if m := missingRx.FindStringSubmatch(strings.TrimSuffix(line, "\r")); m != nil {
			from, name = m[1], m[2]+m[3]
			break
		}
	}
	if name == "" {
		lower := strings.ToLower(string(out))
		return nil, !strings.Contains(lower, "no such file") && !strings.Contains(lower, "file not found")
	}
	if filepath.IsAbs(name) {
		return []string{filepath.Clean(name)}, true
	}
	var folders []string
	if !samePath(from, src) {
		if !filepath.IsAbs(from) {
			from = filepath.Join(dir, from)
		}
		folders = append(folders, filepath.Dir(from))
	}
	if dir != "" {
		folders = append(folders, dir)
	}
	folders = append(folders, includeFolders(flags, dir)...)
	for _, f := range folders {
		places = append(places, filepath.Join(f, filepath.FromSlash(name)))
	}
	return places, true
}

// reportsMissing reports whether the compiler output out has a message for
// an #include whose header isn't found.
func reportsMissing(out []byte) bool {
	lower := strings.ToLower(string(out))
	return strings.Contains(lower, "no such file") || strings.Contains(lower, "file not found")
}

// includeFolders returns the folders of the -iquote, -I, -isystem and
// -idirafter flags (-Ifoo or -I foo), relative ones from the folder dir,
// where the compiler runs.
func includeFolders(flags []string, dir string) []string {
	var folders []string
	for i := 0; i < len(flags); i++ {
		for _, opt := range []string{"-iquote", "-isystem", "-idirafter", "-I"} {
			f, ok := strings.CutPrefix(flags[i], opt)
			if !ok {
				continue
			}
			if f == "" && i+1 < len(flags) {
				i++
				f = flags[i]
			}
			if f != "" {
				if !filepath.IsAbs(f) {
					f = filepath.Join(dir, f)
				}
				folders = append(folders, filepath.Clean(f))
			}
			break
		}
	}
	return folders
}

// findCC is build.FindCC, or a fake in the tests.
var findCC = build.FindCC

// ccLookups keeps the answers of CCompiler: per C++ compiler and value of
// GOCPP_CC, the C compiler, or an error for errorTTL.
var ccLookups struct {
	sync.Mutex
	m map[string]ccLookup
}

type ccLookup struct {
	cc  build.CXX
	err error
	at  time.Time
}

// CCompiler returns the C compiler of the C header check for the C++
// compiler cxx: the C compiler of the package's .c files (build.FindCC),
// or, when there is none and GOCPP_CC doesn't name one, cxx itself, which
// CheckCHeader runs in C mode (-x c): g++ and clang++ compile C when the C
// front end is installed, and CCompiler checks that it is with a small C
// file. The error says why neither works. The answers are kept for the
// life of the process (an error for 5 seconds), so a check of many
// headers looks for the compiler once.
func CCompiler(ctx context.Context, cxx build.CXX) (build.CXX, error) {
	env := os.Getenv("GOCPP_CC")
	key := setKey(cxx, env, nil, nil)
	ccLookups.Lock()
	l, ok := ccLookups.m[key]
	ccLookups.Unlock()
	if ok && (l.err == nil || time.Since(l.at) < errorTTL) {
		return l.cc, l.err
	}
	cc, err := findCC(ctx, cxx)
	if err != nil && env == "" && ctx.Err() == nil {
		if cerr := compilesC(ctx, cxx); cerr == nil {
			cc, err = cxx, nil
		} else {
			err = fmt.Errorf("%v; the C++ compiler doesn't compile C either: %v", err, cerr)
		}
	}
	if ctx.Err() == nil {
		ccLookups.Lock()
		if ccLookups.m == nil {
			ccLookups.m = make(map[string]ccLookup)
		}
		ccLookups.m[key] = ccLookup{cc, err, time.Now()}
		ccLookups.Unlock()
	}
	return cc, err
}

// compilesC reports whether the compiler c compiles a small C file in C
// mode (-x c), with the C standard of the C header check.
func compilesC(ctx context.Context, c build.CXX) error {
	if c.Family != build.Clang && c.Family != build.GCC {
		return unknownFamily(c)
	}
	tmp, err := probeTemp()
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "gp_c.c")
	// _Generic and a struct tag that is no type name are C, not C++.
	if err := os.WriteFile(src, []byte("struct gp_s { int x; };\nint gp_c = _Generic(1, int: 1, default: 0);\nstruct gp_s gp_v;\n"), 0o644); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	out := &capBuffer{max: 1 << 16, stop: cancel}
	cmd, err := runCompiler(ctx, c, "", tmp, []string{"-std=gnu17", "-fsyntax-only", "-w", "-x", "c", src}, out, out)
	if err := runFailure(ctx, c.Name(), cmd, err, out.buf.Bytes()); err != nil {
		return err
	}
	if !cmd.ProcessState.Success() {
		return fmt.Errorf("%s -x c failed (%v)%s", c.Name(), cmd.ProcessState, outputExcerpt(out.buf.Bytes()))
	}
	return nil
}

// CheckCHeaderFor is CheckCHeader with the C compiler of the C++ compiler
// cxx (CCompiler): check.Check's default for the C header check.
func CheckCHeaderFor(ctx context.Context, cxx build.CXX, dir string, flags []string, include string) (CHeader, error) {
	cc, err := CCompiler(ctx, cxx)
	if err != nil {
		return CHeader{}, err
	}
	return CheckCHeader(ctx, cc, dir, flags, include)
}
