package interop

import (
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// Hooks for the typed-interop probes of Phase 12 (internal/typedinterop/probe).
//
// The typed probe runs the same compiler the same way as the kind probe (the
// package's flags without the message flags, -std=c++20, -iquote dir, the
// warning policy, a dependency file), only with another mode (an AST dump of
// the probe declarations) and another reader of the output. So it reuses the
// argument list and the helpers below instead of repeating them. They only
// expose what nsprobe.go already does; nothing in the opaque pipeline uses
// them.

// ProbeCompilerArgs returns the arguments of a probe run of the compiler cxx:
// the common flags (-std=c++20, -iquote dir, the package's flags without the
// flags that change only how messages look, the compiler's own message flags,
// the dependency file depFile) around the mode flags (such as -fsyntax-only)
// and the source file src. dir is the package folder (absolute, or "").
func ProbeCompilerArgs(cxx build.CXX, mode []string, dir string, flags []string, depFile, src string) ([]string, error) {
	return compilerArgs(cxx, mode, dir, flags, depFile, src)
}

// ProbeEnv returns the environment of a probe run that works in the
// temporary folder tmp and the package folder dir (see probeEnv).
func ProbeEnv(tmp, dir string) []string { return probeEnv(tmp, dir) }

// ProbeLimit is the time limit of one run of a probe (probeLimit), for the probes of other packages
// (gocpp doc's FindHeader).
func ProbeLimit() time.Duration { return probeLimit() }

// ParseDeps returns the headers that the compiler read, from its dependency
// file data (make syntax, written with -MD -MT gp_probe), without the probe
// source src; a relative path is relative to dir (see parseDeps).
func ParseDeps(data []byte, src, dir string) []string { return parseDeps(data, src, dir) }
