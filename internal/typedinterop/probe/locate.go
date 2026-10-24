package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/winpath"
)

// Declaration locations for the editor (step 12.13c2).
//
// Locate answers, for a bounded set of names that a program writes, where the header declares
// them and what the declaration is, from one or a few compiler runs over the program's includes.
// It is the sibling of Index (names) and Members (member names): the same Runner, compiler
// identity, dependency validation, disk variants and single flight; its entries are their own
// kind (locateKeyVersion in the key), so no older cache entry is read as an answer. Like them it must
// run off the request path.

// LocateName asks for a name of a namespace (Scope is "" for the global scope, "std::chrono").
// A Macro is found in the preprocessor's output instead of the declarations.
type LocateName struct {
	Scope string
	Name  string
	Macro bool
}

// LocateMembers asks for members of one receiver type, as Members spells it. A member is
// looked up the way the compiler looks up an unqualified name in a class derived from the
// receiver: overloads, inherited members and the members of an instantiated template are the
// declarations the compiler finds.
type LocateMembers struct {
	Receiver string
	Names    []string
}

// A LocateRequest is what one Locate call asks.
type LocateRequest struct {
	Names   []LocateName
	Members []LocateMembers
}

// Limits of one request: more names are not looked up.
const (
	maxLocateNames  = 96
	maxLocateRounds = 3
	maxLocateDump   = 48 << 20
)

// Locations are the answers to a request. A name that is not in it was not found, was not
// asked for in a form that can be looked up, or has a declaration that cannot be shown exactly:
// a caller shows nothing for it.
type Locations struct {
	named   map[string][]Declaration
	members map[string][]Declaration

	deps    []string  // the files that the compiler runs read, and the places that shadow them
	started time.Time // when the call began: a file that changed after it is not what was read
	dir     string
}

func namedKey(scope, name string) string { return scope + "\x00" + name }

// Named returns the declarations of a name of a namespace.
func (l *Locations) Named(scope, name string) []Declaration {
	if l == nil {
		return nil
	}
	return slices.Clone(l.named[namedKey(scope, strings.TrimPrefix(name, "::"))])
}

// Member returns the declarations of a member of the receiver type, spelled as it was asked.
func (l *Locations) Member(receiver, name string) []Declaration {
	if l == nil {
		return nil
	}
	return slices.Clone(l.members[namedKey(receiver, name)])
}

// Bytes is a conservative estimate for a cache owner's memory budget.
func (l *Locations) Bytes() int64 {
	if l == nil {
		return 0
	}
	size := int64(64)
	for _, m := range []map[string][]Declaration{l.named, l.members} {
		for key, decls := range m {
			size += int64(len(key)) + 96
			for _, d := range decls {
				size += int64(len(d.File)+len(d.Signature)+len(d.Doc)+len(d.Kind)) + 96
			}
		}
	}
	return size
}

// Len is the number of names that have declarations.
func (l *Locations) Len() int {
	if l == nil {
		return 0
	}
	return len(l.named) + len(l.members)
}

// A roundAnswer is the answer to one query of one run.
type roundAnswer struct {
	Decls  []Declaration `json:"decls,omitempty"`
	Follow []usingTarget `json:"follow,omitempty"`
	Failed bool          `json:"failed,omitempty"`
}

type locateFile struct {
	Answers map[string]roundAnswer
	Targets map[string]string
}

// A locateItem is one query of a run with the place of its lines in the probe file.
type locateItem struct {
	key      string
	scope    string // the qualification of the names that the renderer writes
	name     string
	members  bool
	macro    bool
	first    int // the first and last line of its text
	last     int
	headings []string // the names of the lookups, in order
}

// Locate runs the request. Names that a using declaration introduces are followed to the
// declarations they name, in up to maxLocateRounds runs.
func (rn *Runner) Locate(ctx context.Context, cfg Config, includes []string,
	req LocateRequest,
) (*Locations, error) {
	if cfg.CXX.Family != build.Clang {
		return nil, ErrNoClang
	}
	todo := validLocateNames(req.Names)
	members := validLocateMembers(req.Members)
	answers := make(map[string]roundAnswer)
	var read []string // the files that the runs read
	started := time.Now()
	for round := 0; round < maxLocateRounds && (len(todo) > 0 || round == 0 && len(members) > 0); round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		got, deps, err := rn.locateRound(ctx, cfg, includes, todo, members)
		if err != nil {
			return nil, err
		}
		read = append(read, deps...)
		members = nil // members are asked in the first round only
		var next []LocateName
		for key, answer := range got {
			answers[key] = answer
			for _, target := range answer.Follow {
				query := LocateName{Scope: target.Scope, Name: target.Name}
				if _, seen := answers["n\x00"+namedKey(query.Scope, query.Name)]; !seen &&
					!slices.Contains(next, query) && len(next) < maxLocateNames {
					next = append(next, query)
				}
			}
		}
		todo = next
	}
	slices.Sort(read)
	locations := &Locations{
		named:   make(map[string][]Declaration),
		members: make(map[string][]Declaration),
		deps:    slices.Compact(read),
		started: started,
		dir:     cfg.Dir,
	}
	var resolve func(key string, depth int) ([]Declaration, bool)
	resolve = func(key string, depth int) ([]Declaration, bool) {
		answer, found := answers[key]
		if !found || answer.Failed || depth > maxLocateRounds {
			return nil, false
		}
		decls := slices.Clone(answer.Decls)
		for _, target := range answer.Follow {
			more, ok := resolve("n\x00"+namedKey(target.Scope, target.Name), depth+1)
			if !ok {
				return nil, false // a part of the set is unknown: so is the set
			}
			decls = append(decls, more...)
		}
		slices.SortFunc(decls, func(a, b Declaration) int {
			return strings.Compare(fmt.Sprint(a.File, a.Line, a.Col), fmt.Sprint(b.File, b.Line, b.Col))
		})
		decls = slices.CompactFunc(decls, func(a, b Declaration) bool {
			return a.File == b.File && a.Line == b.Line && a.Col == b.Col
		})
		return decls, len(decls) > 0
	}
	for _, query := range req.Names {
		if decls, ok := resolve("n\x00"+namedKey(query.Scope, query.Name), 0); ok {
			locations.named[namedKey(query.Scope, query.Name)] = decls
		}
	}
	for _, group := range req.Members {
		for _, name := range group.Names {
			if decls, ok := resolve("m\x00"+namedKey(group.Receiver, name), 0); ok {
				locations.members[namedKey(group.Receiver, name)] = decls
			}
		}
	}
	return locations, nil
}

// validLocateNames keeps the names that can be looked up, each once, up to the limit.
func validLocateNames(names []LocateName) []LocateName {
	out := make([]LocateName, 0, len(names))
	for _, query := range names {
		if validLocateName(query) && len(out) < maxLocateNames && !slices.Contains(out, query) {
			out = append(out, query)
		}
	}
	return out
}

// cxxKeywords are the words that no name of a header can be (the C++20 keywords and the
// alternative operator tokens). The compiler's dump of a lookup of one of them has no heading for
// it, so one such name in a request would fail the whole run.
var cxxKeywords = map[string]bool{}

func init() {
	for _, word := range strings.Fields(`alignas alignof and and_eq asm auto bitand bitor bool break
		case catch char char8_t char16_t char32_t class compl concept const consteval constexpr
		constinit const_cast continue co_await co_return co_yield decltype default delete do double
		dynamic_cast else enum explicit export extern false float for friend goto if inline int long
		mutable namespace new noexcept not not_eq nullptr operator or or_eq private protected public
		register reinterpret_cast requires return short signed sizeof static static_assert
		static_cast struct switch template this thread_local throw true try typedef typeid typename
		union unsigned using virtual void volatile wchar_t while xor xor_eq`) {
		cxxKeywords[word] = true
	}
}

// lookupIdent reports whether the text can be looked up as a name: an identifier of the program,
// not a keyword and not an internal name.
func lookupIdent(name string) bool {
	return identRx.MatchString(name) && !strings.HasPrefix(name, "gocpp_") && !cxxKeywords[name]
}

// A macro may be named like a keyword: <stdbool.h> defines true and false, <assert.h> defines
// static_assert.
func validLocateName(query LocateName) bool {
	if query.Macro {
		return identRx.MatchString(query.Name) && !strings.HasPrefix(query.Name, "gocpp_") &&
			query.Scope == ""
	}
	if !lookupIdent(query.Name) {
		return false
	}
	if query.Scope == "" {
		return true
	}
	for _, part := range strings.Split(query.Scope, "::") {
		if !lookupIdent(part) {
			return false
		}
	}
	return !query.Macro || query.Scope == ""
}

// validLocateMembers keeps the groups whose receiver is a class type that a class can derive
// from, with the names that are identifiers; a pointer receiver is looked up as its pointee.
func validLocateMembers(groups []LocateMembers) []LocateMembers {
	var out []LocateMembers
	total := 0
	for _, group := range groups {
		if _, ok := memberBase(group.Receiver); !ok {
			continue
		}
		var names []string
		for _, name := range group.Names {
			if lookupIdent(name) && !slices.Contains(names, name) && total < maxLocateNames {
				names = append(names, name)
				total++
			}
		}
		if len(names) > 0 {
			out = append(out, LocateMembers{Receiver: group.Receiver, Names: names})
		}
	}
	return out
}

// memberBase returns the text of the class that a derived class can name for a receiver type.
func memberBase(receiver string) (string, bool) {
	if len(receiver) == 0 || len(receiver) > 4096 || unsafeName(receiver) != "" {
		return "", false
	}
	typ, err := ctype.Parse(receiver)
	if err != nil || !memberTypeSafe(typ, 0) {
		return "", false
	}
	if typ.Kind == ctype.Pointer {
		typ = typ.Elem
	}
	if typ == nil || typ.Kind != ctype.Named {
		return "", false
	}
	bare := *typ // the class does not depend on how the variable holds it
	bare.Const, bare.Volatile = false, false
	return bare.String(), true
}

// locateSource writes the probe file of a run: the includes, a variable that ends the run, and
// the lookups. The pragma prints "lookup results for NAME:" once for each line, in order.
func locateSource(includes []string, names []LocateName, members []LocateMembers,
) (string, []*locateItem, error) {
	var source strings.Builder
	for _, include := range includes {
		if err := interop.ValidateInclude(include); err != nil {
			return "", nil, err
		}
		header, externC := interop.SplitExternC(include)
		header = winpath.IncludeSpelling(header)
		if externC {
			fmt.Fprintf(&source, "extern \"C\" {\n#include %s\n}\n", header)
		} else {
			fmt.Fprintf(&source, "#include %s\n", header)
		}
	}
	line := strings.Count(source.String(), "\n")
	var items []*locateItem
	emit := func(text string) int {
		source.WriteString(text)
		source.WriteByte('\n')
		line++
		return line
	}
	emit("char gocpp_loc_end;")
	for _, query := range names {
		if query.Macro {
			items = append(items, &locateItem{key: "n\x00" + namedKey(query.Scope, query.Name),
				name: query.Name, macro: true})
			continue
		}
		item := &locateItem{key: "n\x00" + namedKey(query.Scope, query.Name), scope: query.Scope,
			name: query.Name, headings: []string{query.Name}}
		if query.Scope == "" {
			item.first = emit("#pragma clang __debug dump " + query.Name)
			item.last = item.first
		} else {
			parts := strings.Split(query.Scope, "::")
			item.first = emit("namespace " + strings.Join(parts, " { namespace ") + " {")
			emit("#pragma clang __debug dump " + query.Name)
			item.last = emit(strings.Repeat("}", len(parts)))
		}
		items = append(items, item)
	}
	for index, group := range members {
		base, _ := memberBase(group.Receiver)
		first := emit(fmt.Sprintf("struct gocpp_loc_class_%d : public %s { void gocpp_loc_method() {",
			index, base))
		for _, name := range group.Names {
			emit("#pragma clang __debug dump " + name)
		}
		last := emit("} };")
		// One class answers all its names; an error in it fails them all.
		for _, name := range group.Names {
			items = append(items, &locateItem{key: "m\x00" + namedKey(group.Receiver, name),
				scope: group.Receiver, name: name, members: true, first: first, last: last,
				headings: []string{name}})
		}
	}
	emit("#pragma clang __debug dump gocpp_loc_end")
	if source.Len() > maxIndexOutput {
		return "", nil, errors.New("locate source exceeds its budget")
	}
	return source.String(), items, nil
}

// locateKeyVersion is part of the key of every locate entry. It changes when the answers that a
// run gives change for the same source and compiler, so an entry of an older version is never
// looked up. v2: redeclarations, templates, doc cuts. v3: the comments of system headers are kept.
// v4: a declaration has its compiler type, the tag of a typedef and the flag Defined. v5: the tag
// has its qualified name. v6: a function has its canonical parameter types, and a static function
// does not redeclare.
const locateKeyVersion = "locate-v6"

// locateEntryName is the name of the cache entry of a locate source.
func locateEntryName(version string, cfg Config, text string, tc toolchain) string {
	pre := withToolchain(PreKey(cfg, &Source{Text: version + "\n" + text}), tc)
	return entryName("l", selfID(), pre)
}

// locateRound makes one run for the names and the members, or reads it from the cache.
func (rn *Runner) locateRound(ctx context.Context, cfg Config, includes []string,
	names []LocateName, members []LocateMembers,
) (map[string]roundAnswer, []string, error) {
	text, items, err := locateSource(includes, names, members)
	if err != nil {
		return nil, nil, err
	}
	produce := func(set *sourceSet, answers map[string]roundAnswer) ([]*auxOut, error) {
		return rn.produceLocate(ctx, cfg, text, set, items, answers)
	}
	return rn.cachedRound(ctx, cfg, includes, locateKeyVersion, text, produce)
}

// A producer makes the answers of a run from the compiler's output, and returns the outputs whose
// dependency lists must be the ones the run was bound to.
type producer func(set *sourceSet, answers map[string]roundAnswer) ([]*auxOut, error)

// cachedRound makes a run, or reads it from the cache. version is part of the key: it says which
// kind of run it is (declarations, or the body of a tag) and what its compiler flags are.
func (rn *Runner) cachedRound(ctx context.Context, cfg Config, includes []string,
	version, text string, produce producer,
) (map[string]roundAnswer, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	dir, write := rn.diskFor(cfg)
	tc := rn.toolchain(ctx, cfg, dir, write)
	name := locateEntryName(version, cfg, text, tc)
	lookup := func() (map[string]roundAnswer, []string) {
		v := rn.findVariant(dir, name, "l")
		if v == nil {
			return nil, nil
		}
		var file locateFile
		if json.Unmarshal(v.Part, &file) != nil || file.Answers == nil || file.Targets == nil ||
			!sameIndexTargets(file.Targets) {
			return nil, nil
		}
		return file.Answers, v.Deps
	}
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if answers, deps := lookup(); answers != nil {
			return answers, deps, nil
		}
		rn.mu.Lock()
		if other := rn.flying[name]; other != nil {
			rn.mu.Unlock()
			select {
			case <-other.done:
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
			if other.err != nil && !errors.Is(other.err, context.Canceled) {
				return nil, nil, other.err
			}
			continue
		}
		fl := &flight{done: make(chan struct{})}
		if rn.flying == nil {
			rn.flying = make(map[string]*flight)
		}
		rn.flying[name] = fl
		rn.mu.Unlock()

		answers, deps, err := rn.makeLocate(ctx, cfg, includes, text, produce, dir, name, write, tc,
			lookup)
		rn.mu.Lock()
		delete(rn.flying, name)
		fl.err = err
		close(fl.done)
		rn.mu.Unlock()
		return answers, deps, err
	}
	return nil, nil, errors.New("the locate query kept being taken by other callers")
}

func (rn *Runner) makeLocate(ctx context.Context, cfg Config, includes []string, text string,
	produce producer, dir, name string, write bool, tc toolchain,
	lookup func() (map[string]roundAnswer, []string),
) (map[string]roundAnswer, []string, error) {
	if dir != "" {
		release, _ := acquireLock(ctx, dir, name, effTimeout(cfg))
		if release != nil {
			defer release()
		}
	}
	if answers, deps := lookup(); answers != nil {
		return answers, deps, nil
	}
	start := time.Now()
	// Bind the inputs before reading any answer, as the header index does: the files that the
	// compiler reads are validated, and must be the same in every run below.
	_, discovery, err := indexOutput(ctx, cfg, []string{"-E", "-o", os.DevNull}, text)
	if err != nil {
		return nil, nil, err
	}
	read := interop.ParseDeps(discovery.depData, discovery.srcPath, cfg.Dir)
	slices.Sort(read)
	read = slices.Compact(read)
	metadata := new(Runner)
	deps := metadata.finalDeps(ctx, cfg, text, includes, read, nil, tc)
	info := metadata.includeInfo(ctx, cfg, text, read, false, "")
	if !info.complete || !info.successful || len(read) == 0 || racyDeps(deps, start) {
		return nil, nil, errors.New("locate inputs changed or could not be validated")
	}
	required, err := captureIndexDeps(read)
	if err != nil {
		return nil, nil, err
	}
	resolution, err := captureIndexResolution(append(slices.Clone(deps), cfg.Dir))
	if err != nil {
		return nil, nil, err
	}
	depsHash := DepsHash(deps)
	set := newSourceSet(cfg.Dir, read)
	answers := make(map[string]roundAnswer)
	outputs := []*auxOut{discovery}
	produced, err := produce(set, answers)
	if err != nil {
		return nil, nil, err
	}
	outputs = append(outputs, produced...)
	for _, output := range outputs {
		actual := interop.ParseDeps(output.depData, output.srcPath, cfg.Dir)
		slices.Sort(actual)
		if !slices.Equal(slices.Compact(actual), read) {
			return nil, nil, errors.New("locate dependency resolution changed during production")
		}
	}
	if !required.unchanged() || !resolution.unchanged() || racyDeps(deps, start) ||
		DepsHash(deps) != depsHash {
		return nil, nil, errors.New("locate inputs changed during production")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(locateFile{Answers: answers, Targets: resolution.targets})
	if err != nil {
		return nil, nil, err
	}
	rn.addVariant(dir, name, "l", "declaration locations "+cfg.CXX.Name(),
		variant{Deps: deps, DepsHash: depsHash, Part: body}, write)
	return answers, deps, nil
}

// produceLocate runs the lookups of a probe file: the dump of the declarations, and the
// preprocessor's output for the macros.
func (rn *Runner) produceLocate(ctx context.Context, cfg Config, text string, set *sourceSet,
	items []*locateItem, answers map[string]roundAnswer,
) ([]*auxOut, error) {
	var outputs []*auxOut
	if slices.ContainsFunc(items, func(item *locateItem) bool { return !item.macro }) {
		dumped, err := rn.runLocateDump(ctx, cfg, text, set, items, answers)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, dumped)
	}
	if slices.ContainsFunc(items, func(item *locateItem) bool { return item.macro }) {
		includeOnly := text[:strings.Index(text, "char gocpp_loc_end;")]
		macros, macroOut, err := indexOutput(ctx, cfg, []string{"-E", "-dD"}, includeOnly)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, macroOut)
		locateMacros(set, macros, items, answers)
	}
	return outputs, nil
}

// runLocateDump runs the dump of the lookups and renders each group into answers.
func (rn *Runner) runLocateDump(ctx context.Context, cfg Config, text string, set *sourceSet,
	items []*locateItem, answers map[string]roundAnswer,
) (*auxOut, error) {
	reader := newDumpReader(maxLocateDump, set.resolve, nil)
	out, err := runDumpStderr(ctx, cfg, []string{"-fsyntax-only", "-fparse-all-comments",
		"-fretain-comments-from-system-headers", "-ferror-limit=0", "-fno-caret-diagnostics"}, text, reader)
	if err != nil {
		return nil, err
	}
	reader.finish()
	if reader.over || out.timedOut {
		return nil, errors.New("the declaration dump was too large or took too long")
	}
	// Every lookup prints one heading, in the order of the lines, and the run ends with its
	// own: anything else is a dump that this reader does not know.
	var expected []string
	for _, item := range items {
		if !item.macro {
			expected = append(expected, item.headings...)
		}
	}
	expected = append(expected, "gocpp_loc_end")
	if len(reader.groups) != len(expected) {
		return nil, fmt.Errorf("the dump has %d lookups, want %d", len(reader.groups), len(expected))
	}
	for i, group := range reader.groups {
		if group.name != expected[i] {
			return nil, fmt.Errorf("lookup %d is for %q, want %q", i, group.name, expected[i])
		}
	}
	if len(reader.groups[len(reader.groups)-1].nodes) == 0 {
		return nil, errors.New("the dump did not reach its end")
	}
	failed := make(map[int]bool) // lines with an error of the compiler in a lookup of the probe file
	for _, group := range ParseDiags(reader.diags.Bytes(), out.srcPath) {
		diag := group.Err
		if diag.Severity != "error" && diag.Severity != "fatal error" {
			continue
		}
		if !isProbeFile(diag.File, out.srcPath) || !slices.ContainsFunc(items, func(item *locateItem) bool {
			return !item.macro && diag.Line >= item.first && diag.Line <= item.last
		}) {
			return nil, fmt.Errorf("the headers have an error: %s", diag.Msg)
		}
		failed[diag.Line] = true
	}
	r := &renderer{set: set}
	next := 0
	for _, item := range items {
		if item.macro {
			continue
		}
		group := reader.groups[next]
		next++
		answer := roundAnswer{Failed: true}
		bad := false
		for line := range failed {
			if line >= item.first && line <= item.last {
				bad = true
			}
		}
		if !bad {
			qual := item.scope
			decls, follow, ok := r.declarations(group, qual, item.members)
			if ok {
				answer = roundAnswer{Decls: decls, Follow: follow}
			}
		}
		answers[item.key] = answer
	}
	return out, nil
}

// runDumpStderr is runAux with the standard error sent to w.
func runDumpStderr(ctx context.Context, cfg Config, mode []string, text string,
	w *dumpReader,
) (*auxOut, error) {
	release, err := gate.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), err)
	}
	defer release()
	tmp, err := os.MkdirTemp("", "gocpp-typed-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if tmp, err = filepath.Abs(tmp); err != nil {
		return nil, err
	}
	srcPath := filepath.Join(tmp, probeFileName)
	depPath := filepath.Join(tmp, "gp_probe.d")
	if err := os.WriteFile(srcPath, []byte(text), 0o644); err != nil {
		return nil, err
	}
	args, err := interop.ProbeCompilerArgs(cfg.CXX, mode, cfg.Dir, cfg.Flags, depPath, srcPath)
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, effTimeout(cfg))
	defer cancel()
	w.cancel = cancel
	cmd, err := build.Start(rctx, func() *exec.Cmd {
		c := cfg.CXX.Command(rctx, args...)
		c.Dir = cfg.Dir
		c.Env = interop.ProbeEnv(tmp, cfg.Dir)
		c.Stdout = io.Discard
		c.Stderr = w
		c.WaitDelay = probeWaitDelay
		build.InGroup(c)
		return c
	})
	if err != nil {
		return nil, fmt.Errorf("%s didn't start: %w", cfg.CXX.Name(), err)
	}
	stopWatch := build.WatchPipes(cmd) // a header that is a named pipe would make it wait for ever
	exitErr := cmd.Wait()
	pipeErr := stopWatch()
	timedOut := rctx.Err() != nil
	build.KillGroup(cmd)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), ctx.Err())
	}
	if pipeErr != nil {
		return nil, fmt.Errorf("%s: %w", cfg.CXX.Name(), pipeErr)
	}
	out := &auxOut{exitErr: exitErr, timedOut: timedOut, srcPath: srcPath}
	out.depData, _ = os.ReadFile(depPath)
	return out, nil
}

// ---------------------------------------------------------------- macros

var (
	markerRx   = regexp.MustCompile(`^# (\d+) "((?:[^"\\]|\\.)*)"`)
	defineRx   = regexp.MustCompile(`^#define ([A-Za-z_][A-Za-z0-9_]*)`)
	undefRx    = regexp.MustCompile(`^#undef ([A-Za-z_][A-Za-z0-9_]*)`)
	maxMacroSz = 240
)

// A macroDef is the last definition of a macro that the preprocessor's output shows.
type macroDef struct {
	file string
	line int
	text string
}

// locateMacros finds the definitions of the macros that items ask for in the output of
// `-E -dD`: its line markers say the file and line of each #define. A macro is accepted when
// the line of the file really is its #define.
func locateMacros(set *sourceSet, output string, items []*locateItem, answers map[string]roundAnswer) {
	wanted := make(map[string]bool)
	for _, item := range items {
		if item.macro {
			wanted[item.name] = true
		}
	}
	defs := make(map[string]macroDef)
	file, line := "", 0
	for _, text := range strings.Split(output, "\n") {
		if m := markerRx.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[1])
			line = n - 1
			file = unescapeMarker(m[2])
			continue
		}
		line++
		if m := defineRx.FindStringSubmatch(text); m != nil && wanted[m[1]] {
			defs[m[1]] = macroDef{file: set.resolve(file), line: line, text: text}
		} else if m := undefRx.FindStringSubmatch(text); m != nil {
			delete(defs, m[1])
		}
	}
	for _, item := range items {
		if !item.macro {
			continue
		}
		answer := roundAnswer{Failed: true}
		if def, ok := defs[item.name]; ok {
			if decl, ok := macroDeclaration(set, item.name, def); ok {
				answer = roundAnswer{Decls: []Declaration{decl}}
			}
		}
		answers[item.key] = answer
	}
}

func unescapeMarker(text string) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) {
			i++
		}
		b.WriteByte(text[i])
	}
	return b.String()
}

var defineLineRx = regexp.MustCompile(`^\s*#\s*define\s+`)

func macroDeclaration(set *sourceSet, name string, def macroDef) (Declaration, bool) {
	if def.file == "" {
		return Declaration{}, false
	}
	source, ok := set.file(def.file).line(def.line)
	if !ok {
		return Declaration{}, false
	}
	head := defineLineRx.FindString(source)
	if head == "" || !strings.HasPrefix(source[len(head):], name) {
		return Declaration{}, false
	}
	loc := srcLoc{file: def.file, line: def.line, col: len(head) + 1, valid: true}
	line, col, length, placed := set.place(loc, name)
	if !placed {
		return Declaration{}, false
	}
	signature := def.text
	if len(signature) > maxMacroSz {
		signature = signature[:maxMacroSz] + " ..."
	}
	doc := set.docOf(srcLoc{}, loc, nil)
	if doc == "" {
		doc = set.leadingComment(def.file, def.line)
	}
	return Declaration{Kind: "macro", File: def.file, Line: line, Col: col, Length: length,
		Signature: signature, Doc: doc}, true
}

// leadingComment returns the comment lines (// style) right above a line, or "".
func (set *sourceSet) leadingComment(path string, line int) string {
	file := set.file(path)
	var lines []string
	for n := line - 1; n >= 1 && len(lines) < maxDocLines; n-- {
		text, ok := file.line(n)
		if !ok || !strings.HasPrefix(strings.TrimSpace(text), "//") {
			break
		}
		lines = append(lines, stripCommentMarks(text))
	}
	slices.Reverse(lines)
	return tidyDoc(lines)
}

// Resolve says whether name is a visible declaration or macro of the scope in the index, and
// which canonical namespace it is in (an alias or a rename of a namespace is followed, as Names
// does). A namespace is not one: its answer would be a dump of everything in it.
func (index *HeaderIndex) Resolve(scope, name string) (canonical string, macro, ok bool) {
	if index == nil {
		return "", false, false
	}
	canonical = strings.TrimPrefix(scope, "::")
	if index.namespaces != nil && canonical != "" {
		current, found := lookupNamespacePath(index.namespaces, "", strings.Split(canonical, "::"))
		if !found {
			return "", false, false
		}
		canonical = current
	}
	if index.namespaces != nil {
		if _, isNamespace := index.namespaces[canonical][name]; isNamespace {
			return "", false, false
		}
	} else if _, isNamespace := index.scopes[indexQualifiedName(canonical, name)]; isNamespace {
		return "", false, false
	}
	names := index.scopes[canonical]
	at, found := slices.BinarySearchFunc(names, name, func(symbol HeaderSymbol, name string) int {
		return strings.Compare(symbol.Name, name)
	})
	if !found {
		return "", false, false
	}
	return canonical, names[at].Macro, true
}
