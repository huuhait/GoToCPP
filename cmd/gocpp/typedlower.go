package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/lower"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// typedLowerEnv is the hidden switch of typed code generation (Phase 12 step 12.6, ADR-087):
// GOCPP_TYPED_INTEROP=1 makes gocpp build, run and emit probe the C/C++ names of each package with
// clang and write the C++ from the real types (lower.Interop.SetTyped). It is a developer switch
// until the mode flag of step 12.11 (-interop=typed|opaque); unset, nothing changes.
const typedLowerEnv = "GOCPP_TYPED_INTEROP"

// typedLowering reports whether the switch is on.
func typedLowering() bool { return os.Getenv(typedLowerEnv) == "1" }

// attachTyped runs the typed pipeline of pkg (collector, probes, declarations) and gives its
// report to in. A package of the standard library, a build without clang++ and a probe that
// fails leave in as it is: the opaque lowering then runs, which is always correct. A build for
// another system than this one is typed only when the headers are those of the C++ standard
// library (their types are the same on both): other headers are the host's, not the target's.
func (d *driver) attachTyped(in *lower.Interop, pkg *check.Package) {
	if in == nil || pkg.Load.Std || !typedLowering() {
		return
	}
	cross := d.check.goos != runtime.GOOS || d.check.goarch != runtime.GOARCH
	if cross && !stdHeadersOnly(pkg) {
		return
	}
	opts := typedinterop.Options{Decls: true, StringToCStr: true, Target: ftype.LP64}
	if d.check.goos == "windows" {
		opts.Target = ftype.LLP64
	}
	rep, err := typedinterop.Analyze(d.ctx, pkg, d.check.fset, opts)
	if err != nil {
		if d.interrupted() == nil && !errors.Is(err, typedinterop.ErrNoClang) {
			fmt.Fprintf(d.stderr, "gocpp %s: typed interop: %v (using opaque mode for %s)\n", d.name, err, pkg.Load.ImportPath)
		}
		return
	}
	in.SetTyped(rep, cross)
}

// stdHeadersOnly reports whether every header that pkg imports is a header of the C++ standard
// library (no .h, no local or module header).
func stdHeadersOnly(pkg *check.Package) bool {
	for _, f := range pkg.Load.Files {
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
