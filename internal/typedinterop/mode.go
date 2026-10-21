package typedinterop

import (
	"context"
	"fmt"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Step 12.11 (ADR-101): the interop mode. The flag -interop=typed|opaque|auto of the commands and
// the environment variable GOCPP_INTEROP choose between the two ways gocpp understands the C/C++
// names of a program:
//
//	opaque  the Phase 04 way: the C++ compiler is the type checker of everything that touches C++
//	typed   clang answers (probes) what each name is: early errors, constants (E1), promoted
//	        members (E2), exact . and ->, designated initializers, string conversions
//	auto    typed when clang++ is installed, else opaque, without a word
//
// ResolveMode applies the order of the sources (flag, GOCPP_INTEROP, GOCPP_TYPED_INTEROP, the
// default) and Decide the clang rule; the command line driver (cmd/gocpp/interop.go) and the
// language server (step 12.13) both use them, so that a mode means the same everywhere.

// Mode is the choice of the user.
type Mode int

const (
	// Auto is typed when clang++ is there, else opaque.
	Auto Mode = iota
	// Typed always uses the probes; no clang++ is an error.
	Typed
	// Opaque never runs a probe for the typed pipeline.
	Opaque
)

// Names of the environment variables of the mode.
const (
	// EnvMode is GOCPP_INTEROP: typed, opaque or auto. The flag -interop wins over it.
	EnvMode = "GOCPP_INTEROP"
	// EnvLegacy is GOCPP_TYPED_INTEROP, the switch of steps 12.6 to 12.10: 1 means typed, 0 opaque;
	// GOCPP_INTEROP wins over it.
	EnvLegacy = "GOCPP_TYPED_INTEROP"
)

// DefaultMode is the mode when no flag and no variable says anything: auto, so typed interop runs
// wherever clang++ is installed (ADR-131, which replaced ADR-101's opaque default).
const DefaultMode = Auto

// ParseMode reads "typed", "opaque" or "auto".
func ParseMode(s string) (Mode, bool) {
	switch s {
	case "typed":
		return Typed, true
	case "opaque":
		return Opaque, true
	case "auto":
		return Auto, true
	}
	return 0, false
}

func (m Mode) String() string {
	switch m {
	case Typed:
		return "typed"
	case Opaque:
		return "opaque"
	}
	return "auto"
}

// A Setting is a mode and where it came from.
type Setting struct {
	Mode Mode
	// Source says which input decided: "-interop", "GOCPP_INTEROP", "GOCPP_TYPED_INTEROP" or
	// "default". It is the name in messages ("-interop=typed needs clang++").
	Source string
}

// String is the setting as the user wrote it: "-interop=typed", "GOCPP_INTEROP=auto", or
// "the default mode".
func (s Setting) String() string {
	switch s.Source {
	case "-interop":
		return "-interop=" + s.Mode.String()
	case EnvMode:
		return EnvMode + "=" + s.Mode.String()
	case EnvLegacy:
		return EnvLegacy + "=" + map[bool]string{true: "1", false: "0"}[s.Mode == Typed]
	}
	return "the default mode"
}

// ModeError is a flag or variable value that is not a mode.
type ModeError struct {
	Source, Value string
}

func (e *ModeError) Error() string {
	if e.Source == "-interop" {
		return fmt.Sprintf("invalid -interop value %q: want typed, opaque or auto", e.Value)
	}
	return fmt.Sprintf("%s=%s: want typed, opaque or auto", e.Source, e.Value)
}

// ResolveMode returns the mode: the value of the flag -interop (empty when it was not given), else
// GOCPP_INTEROP, else GOCPP_TYPED_INTEROP (1 is typed, 0 is opaque, anything else is ignored: the
// variable was a developer switch), else DefaultMode. getenv reads the environment; an empty value
// counts as not set. A value that is not a mode is a *ModeError.
func ResolveMode(flagValue string, getenv func(string) string) (Setting, error) {
	if flagValue != "" {
		m, ok := ParseMode(flagValue)
		if !ok {
			return Setting{}, &ModeError{"-interop", flagValue}
		}
		return Setting{m, "-interop"}, nil
	}
	if v := strings.TrimSpace(getenv(EnvMode)); v != "" {
		m, ok := ParseMode(v)
		if !ok {
			return Setting{}, &ModeError{EnvMode, v}
		}
		return Setting{m, EnvMode}, nil
	}
	switch strings.TrimSpace(getenv(EnvLegacy)) {
	case "1":
		return Setting{Typed, EnvLegacy}, nil
	case "0":
		return Setting{Opaque, EnvLegacy}, nil
	}
	return Setting{DefaultMode, "default"}, nil
}

// A Decision is whether typed interop runs for a program.
type Decision struct {
	// On says that the probes run: the constants hook, the members hook, the interop checker and
	// the typed code generation.
	On bool

	// Err is set for the mode typed when clang++ is missing: the command must fail (it wraps
	// ErrNoClang).
	Err error

	// Note is the one line for -v when the mode is auto and the answer is off because clang++ is
	// missing (or, with the mode opaque or without C/C++ imports, why nothing runs).
	Note string
}

// Decide applies the clang rule to the setting for a build for this machine (DecideFor).
func Decide(ctx context.Context, s Setting, needed bool) Decision {
	return DecideFor(ctx, s, needed, "", "")
}

// ProbesTarget reports whether the probes answer for the target goos/goarch (empty: this machine):
// this machine's own target, and windows/amd64 from any system, where the probe runs a clang for the
// Windows target with the MinGW-w64 headers and the Win32 defaults (Phase 14 step 14.6, ADR-116).
// For any other target only the headers of the C++ standard library are typed (StdHeadersOnly).
func ProbesTarget(goos, goarch string) bool {
	return probe.ProbesTarget(goos, goarch)
}

// DecideFor applies the clang rule to the setting for a build for the target goos/goarch (empty:
// this machine). needed says whether the program has anything to
// probe (a package that is not the standard library's imports a C/C++ header): without it the
// answer is Off and clang++ is not looked for, so a program of Go alone pays nothing. Typed
// without clang++ is an error, auto falls back to opaque. The clang is the one for the target
// (probe.FindClangFor): for windows/amd64 the one that builds the Windows program or a clang that
// the target's flags make a cross compiler.
func DecideFor(ctx context.Context, s Setting, needed bool, goos, goarch string) Decision {
	switch {
	case s.Mode == Opaque:
		return Decision{Note: "interop: opaque mode (" + s.String() + ")"}
	case !needed:
		return Decision{Note: "interop: no C/C++ header is imported, nothing to probe"}
	}
	if _, err := probe.FindClangFor(ctx, goos, goarch); err != nil {
		if s.Mode == Typed {
			return Decision{Err: fmt.Errorf("%s: %w; install clang++, set GOCPP_CLANG to its path, or use -interop=opaque", s, err)}
		}
		return Decision{Note: "interop: no usable clang++ (not found, or it does not work), using opaque mode for C/C++ names (" + s.String() + "; -interop=opaque turns this note off)"}
	}
	return Decision{On: true, Note: "interop: typed mode (" + s.String() + ")"}
}

// Needed reports whether a package that is not the standard library's imports a C/C++ header (the
// packages of the program, in any order): typed interop has nothing to do
// for a program without header imports.
func Needed(pkgs []*load.Package) bool {
	for _, p := range pkgs {
		if p == nil || p.Std {
			continue
		}
		for _, f := range p.Files {
			if f.Info != nil && len(f.Info.Includes) > 0 {
				return true
			}
		}
	}
	return false
}

// HookOptions are the inputs of Hooks.
type HookOptions struct {
	// GOOS and GOARCH are the target of the check (empty: this machine).
	GOOS, GOARCH string

	// Runner is the cache of the compiler runs in memory; share it with the Analyze of the same
	// build. nil means a new one.
	Runner *probe.Runner
}

// Hooks returns the two hooks that make a check.Config typed: Constants (gap E1, step 12.8) and
// CppMembers (gap E2, step 12.9). The language server (step 12.13) and the command line driver
// both take their hooks from here. For a target other than this machine, the hooks answer only
// for packages whose headers are all the C++ standard library's (their types are the same on
// both systems; other headers are the host's, not the target's); windows/amd64 is probed like this
// machine's own target (ProbesTarget). Without clang++ the hooks
// answer nothing and check goes on in opaque mode.
func Hooks(o HookOptions) (check.ConstFunc, check.CppMembers) {
	rn := o.Runner
	if rn == nil {
		rn = new(probe.Runner)
	}
	consts := Constants(Options{Runner: rn})
	members := MembersHook(Options{Runner: rn, GOOS: o.GOOS, GOARCH: o.GOARCH})
	if ProbesTarget(o.GOOS, o.GOARCH) {
		return consts, members
	}
	return consts, func(ctx context.Context, p *check.Package, qs []check.MemberQuery) ([]check.MemberAnswer, error) {
		if !StdHeadersOnly(p) {
			return nil, nil
		}
		return members(ctx, p, qs)
	}
}

// StdHeadersOnly reports whether every header that the package p imports is a header of the C++
// standard library (no .h, no local or module header).
func StdHeadersOnly(p *check.Package) bool {
	for _, f := range p.Load.Files {
		if f.Info == nil {
			continue
		}
		for _, inc := range f.Info.Includes {
			if !cxxStdHeaders[strings.Trim(inc.IncludeText(), "<>")] {
				return false
			}
		}
	}
	return true
}

// cxxStdHeaders are the headers of the C++20 standard library, by the name between < and >.
var cxxStdHeaders = func() map[string]bool {
	m := make(map[string]bool)
	for _, h := range strings.Fields(`algorithm any array atomic barrier bit bitset cassert cctype cerrno cfenv cfloat
		charconv chrono cinttypes climits clocale cmath codecvt compare complex concepts condition_variable coroutine
		csetjmp csignal cstdarg cstddef cstdint cstdio cstdlib cstring ctime cuchar cwchar cwctype deque exception
		execution expected filesystem format forward_list fstream functional future initializer_list iomanip ios
		iosfwd iostream istream iterator latch limits list locale map memory memory_resource mutex new numbers numeric
		optional ostream queue random ranges ratio regex scoped_allocator semaphore set shared_mutex source_location
		span sstream stack stdexcept stop_token streambuf string string_view syncstream system_error thread tuple
		type_traits typeindex typeinfo unordered_map unordered_set utility valarray variant vector version`) {
		m[h] = true
	}
	return m
}()
