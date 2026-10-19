package benchdata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// SchemaVersion is the version of the JSON files. It changes when a field
// changes its meaning; readers reject a newer one.
const SchemaVersion = 1

// The implementations of a benchmark. "gocpp-g++:O2" and "gocpp-clang++:O2"
// are the same .gp program built by gocpp with that C++ compiler and that
// flavor of the build (below); go is the same file built as main.go by the Go
// toolchain; cpp-g++ and cpp-clang++ are the hand-written C++20 twins.
const (
	ImplGocppGxx   = "gocpp-g++:O2"
	ImplGocppClang = "gocpp-clang++:O2"
	ImplGo         = "go"
	ImplCppGxx     = "cpp-g++"
	ImplCppClang   = "cpp-clang++"
)

// The flavors of a gocpp build, in the order of the tables: "default" is what
// `gocpp build` does without flags today (-O0 -g), "O2" is GOCPP_CXXFLAGS="-O2
// -g0" (the release flags, without LTO), "release" is `gocpp build -release`
// (-O2 and LTO, Phase 06 step 6.11; skipped while gocpp has no such flag).
var Flavors = []string{"default", "O2", "release"}

// The C++ compilers of the gocpp builds and of the twins.
var Compilers = []string{"g++", "clang++"}

// GocppImpl returns the implementation name of a gocpp build with the compiler
// cxx ("g++" or "clang++") and the flavor.
func GocppImpl(cxx, flavor string) string { return "gocpp-" + cxx + ":" + flavor }

// SplitGocpp splits a gocpp implementation name into its compiler and flavor
// (empty for the bare "gocpp-g++" of the compile-time records).
func SplitGocpp(impl string) (cxx, flavor string, ok bool) {
	rest, found := strings.CutPrefix(impl, "gocpp-")
	if !found {
		return "", "", false
	}
	cxx, flavor, _ = strings.Cut(rest, ":") // the compile-time records name the compiler only
	return cxx, flavor, true
}

// ImplsFor lists the implementations of a run with the given flavors, in the
// order of the tables: Go, the gocpp builds (by flavor, g++ then clang++), the twins.
func ImplsFor(flavors []string) []string {
	out := []string{ImplGo}
	for _, f := range flavors {
		for _, cxx := range Compilers {
			out = append(out, GocppImpl(cxx, f))
		}
	}
	return append(out, ImplCppGxx, ImplCppClang)
}

// AllImpls lists the implementations of the default run (the O2 flavor).
var AllImpls = ImplsFor([]string{"O2"})

// ImplRank orders implementations for the tables: Go, the gocpp builds by
// flavor and compiler, then the twins.
func ImplRank(impl string) int {
	if cxx, fl, ok := SplitGocpp(impl); ok {
		r := 100 + 10*slices.Index(Flavors, fl)
		if cxx != "g++" {
			r++
		}
		return r
	}
	switch impl {
	case ImplGo:
		return 0
	case ImplCppGxx:
		return 1000
	case ImplCppClang:
		return 1001
	}
	return 2000
}

// IsGocpp reports whether impl is a gocpp build.
func IsGocpp(impl string) bool { return strings.HasPrefix(impl, "gocpp-") }

// CppTwin returns the C++ twin implementation built with the same compiler as
// the gocpp implementation impl ("" if impl is not a gocpp build).
func CppTwin(impl string) string {
	cxx, _, ok := SplitGocpp(impl)
	switch {
	case !ok:
		return ""
	case cxx == "g++":
		return ImplCppGxx
	case cxx == "clang++":
		return ImplCppClang
	}
	return ""
}

// Optimized reports whether the gocpp build impl is an optimized flavor (the
// ones that are compared with the optimized C++ twins).
func Optimized(impl string) bool {
	_, fl, ok := SplitGocpp(impl)
	return ok && fl != "default"
}

// The categories of the benchmark programs, in the order of the tables.
var Categories = []string{"cpu", "data", "dispatch", "runtime", "generics"}

// Status of one measured (program, implementation) pair.
const (
	StatusOK          = "ok"
	StatusSkipped     = "skipped"      // not built on purpose: a package that does not exist yet, no twin
	StatusBuildFailed = "build-failed" // the compiler or the toolchain failed
	StatusRunFailed   = "run-failed"   // exit status not 0, a timeout
	StatusMismatch    = "mismatch"     // the output differs from the reference implementation's
)

// Machine describes where a run happened, so that numbers from two machines
// (or from a loaded one) are never compared by accident.
type Machine struct {
	CPU        string     `json:"cpu"`
	Cores      int        `json:"cores"`
	MemGiB     int        `json:"mem_gib"`
	OS         string     `json:"os"`
	Go         string     `json:"go"`
	Gxx        string     `json:"gxx"`
	Clang      string     `json:"clangxx"`
	Gocpp      string     `json:"gocpp"`
	LoadBefore [3]float64 `json:"load_before"`
	LoadAfter  [3]float64 `json:"load_after"`
	Pinned     string     `json:"pinned,omitempty"`     // the cpu list of taskset, "" when not pinned
	GoMaxProcs int        `json:"gomaxprocs,omitempty"` // the parallelism the benchmarked programs (and Go's garbage collector) get: the cpus of the pin list, else the runner's GOMAXPROCS
}

// Result is the measurements of one program with one implementation.
type Result struct {
	Program  string  `json:"program"`
	Category string  `json:"category"`
	Impl     string  `json:"impl"`
	Status   string  `json:"status"`
	Note     string  `json:"note,omitempty"`
	Output   string  `json:"output,omitempty"` // the first 16 hex digits of the SHA-256 of stdout
	BinSize  int64   `json:"bin_size,omitempty"`
	Stripped int64   `json:"bin_stripped,omitempty"`
	BuildMS  int64   `json:"build_ms,omitempty"`
	Wall     []int64 `json:"wall_ns,omitempty"` // one value per measured run (the warm-up run is not in it)
	CPU      []int64 `json:"cpu_ns,omitempty"`  // user + system time of the process
	RSS      []int64 `json:"rss_kb,omitempty"`  // peak resident set
}

// WallStat returns the statistics of the wall times.
func (r Result) WallStat() Stat { return Summarize(r.Wall) }

// CPUStat returns the statistics of the CPU times.
func (r Result) CPUStat() Stat { return Summarize(r.CPU) }

// RSSStat returns the statistics of the peak resident sets.
func (r Result) RSSStat() Stat { return Summarize(r.RSS) }

// Compile is the measurements of one compile-time benchmark.
type Compile struct {
	Name   string  `json:"name"`   // hello, pkgs20, generics100, lines1000
	Impl   string  `json:"impl"`   // gocpp-g++, gocpp-clang++, go
	Mode   string  `json:"mode"`   // cold (empty caches), warm (runtime and std cached, program changed), noop (nothing changed)
	Flavor string  `json:"flavor"` // gocpp only: default (-O0 -g), O2 or release
	Status string  `json:"status"`
	Note   string  `json:"note,omitempty"`
	WallNS []int64 `json:"wall_ns,omitempty"`
}

// Report is one run of the suite.
type Report struct {
	Schema  int       `json:"schema"`
	Commit  string    `json:"commit"`
	Dirty   bool      `json:"dirty,omitempty"`
	Date    string    `json:"date"` // UTC, RFC 3339
	Size    string    `json:"size"` // full or smoke
	Reps    int       `json:"reps"`
	Warmup  int       `json:"warmup"`
	Machine Machine   `json:"machine"`
	Results []Result  `json:"results"`
	Compile []Compile `json:"compile,omitempty"`
}

// Sort puts the results in a fixed order (category, program, implementation
// in the order of AllImpls), so that the same measurements always give the
// same bytes.
func (r *Report) Sort() {
	catRank := func(c string) int {
		for i, x := range Categories {
			if x == c {
				return i
			}
		}
		return len(Categories)
	}
	sort.SliceStable(r.Results, func(i, j int) bool {
		a, b := r.Results[i], r.Results[j]
		if catRank(a.Category) != catRank(b.Category) {
			return catRank(a.Category) < catRank(b.Category)
		}
		if a.Program != b.Program {
			return a.Program < b.Program
		}
		return ImplRank(a.Impl) < ImplRank(b.Impl)
	})
	sort.SliceStable(r.Compile, func(i, j int) bool {
		a, b := r.Compile[i], r.Compile[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Mode != b.Mode {
			return a.Mode < b.Mode
		}
		if a.Flavor != b.Flavor {
			return a.Flavor < b.Flavor
		}
		return ImplRank(a.Impl) < ImplRank(b.Impl)
	})
}

// MergeResults puts the results of the programs of add into r: every result of
// r for a program that add has is dropped, then add's results are appended.
func MergeResults(r, add *Report) {
	progs := map[string]bool{}
	for _, x := range add.Results {
		progs[x.Program] = true
	}
	var out []Result
	for _, x := range r.Results {
		if !progs[x.Program] {
			out = append(out, x)
		}
	}
	r.Results = append(out, add.Results...)
	r.Sort()
}

// Find returns the result of program with impl, or nil.
func (r *Report) Find(program, impl string) *Result {
	for i := range r.Results {
		if r.Results[i].Program == program && r.Results[i].Impl == impl {
			return &r.Results[i]
		}
	}
	return nil
}

// Programs returns the program names in table order, each once.
func (r *Report) Programs() []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range r.Results {
		if !seen[x.Program] {
			seen[x.Program] = true
			out = append(out, x.Program)
		}
	}
	return out
}

// Marshal returns the indented JSON of the report, results sorted.
func (r *Report) Marshal() ([]byte, error) {
	r.Sort()
	r.Schema = SchemaVersion
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal parses a report and rejects an unknown schema version.
func Unmarshal(data []byte) (*Report, error) {
	var r Report
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	if r.Schema != SchemaVersion {
		return nil, fmt.Errorf("schema version %d, this tool reads %d", r.Schema, SchemaVersion)
	}
	return &r, nil
}

// ReadReport reads and parses the report file name.
func ReadReport(name string) (*Report, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	r, err := Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return r, nil
}

// WriteReport writes the report to the file name (its folder is created).
func WriteReport(name string, r *Report) error {
	data, err := r.Marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, data, 0o644)
}
