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
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/winpath"
)

// HeaderSymbol is a name enumerated by clang, without guessed type or location facts.
// Macro distinguishes preprocessor names; declaration details stay lazy.
type HeaderSymbol struct {
	Name  string
	Macro bool
}

// HeaderIndex is immutable. Names returns a copy of the visible names in a C++ namespace;
// the empty scope is the global namespace. Class members are not requested by the editor.
type HeaderIndex struct {
	scopes     map[string][]HeaderSymbol
	namespaces map[string]map[string]string // canonical scope -> visible child name -> canonical scope
}

type headerIndexFile struct {
	Scopes     map[string][]HeaderSymbol
	Namespaces map[string]map[string]string
	Targets    map[string]string
}

func (index *HeaderIndex) Names(scope string) []HeaderSymbol {
	if index == nil {
		return nil
	}
	scope = strings.TrimPrefix(scope, "::")
	if index.namespaces != nil && scope != "" {
		current, ok := lookupNamespacePath(index.namespaces, "", strings.Split(scope, "::"))
		if !ok {
			return nil
		}
		scope = current
	}
	return slices.Clone(index.scopes[scope])
}

// Bytes is a conservative estimate for a cache owner's memory budget.
func (index *HeaderIndex) Bytes() int64 {
	var size int64 = 64
	for scope, names := range index.scopes {
		size += int64(len(scope)) + 128 + int64(cap(names))*64
		for _, name := range names {
			size += int64(len(name.Name))
		}
	}
	for scope, members := range index.namespaces {
		size += int64(len(scope)) + 128
		for name, target := range members {
			size += int64(len(name)+len(target)) + 96
		}
	}
	return size
}

const maxIndexOutput = 16 << 20

// Index enumerates declarations with -ast-list and macros with -dM, never an AST dump.
// Like Decls it shares the runner's compiler identity, dependency validation, bounded memory,
// disk variants and single-flight table. Callers must run it off the request path.
func (rn *Runner) Index(ctx context.Context, cfg Config, includes []string) (*HeaderIndex, error) {
	if cfg.CXX.Family != build.Clang {
		return nil, ErrNoClang
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var source strings.Builder
	for _, include := range includes {
		if err := interop.ValidateInclude(include); err != nil {
			return nil, err
		}
		header, externC := interop.SplitExternC(include)
		header = winpath.IncludeSpelling(header)
		if externC {
			fmt.Fprintf(&source, "extern \"C\" {\n#include %s\n}\n", header)
		} else {
			fmt.Fprintf(&source, "#include %s\n", header)
		}
	}
	text := source.String()
	dir, write := rn.diskFor(cfg)
	tc := rn.toolchain(ctx, cfg, dir, write)
	pre := withToolchain(PreKey(cfg, &Source{Text: "header-index-v5\n" + text}), tc)
	name := entryName("i", selfID(), pre)
	lookup := func() *HeaderIndex {
		v := rn.findVariant(dir, name, "i")
		if v == nil {
			return nil
		}
		var file headerIndexFile
		if json.Unmarshal(v.Part, &file) != nil || file.Scopes == nil || file.Targets == nil ||
			!sameIndexTargets(file.Targets) {
			return nil
		}
		return &HeaderIndex{scopes: file.Scopes, namespaces: file.Namespaces}
	}
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if index := lookup(); index != nil {
			return index, nil
		}
		rn.mu.Lock()
		if other := rn.flying[name]; other != nil {
			rn.mu.Unlock()
			select {
			case <-other.done:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			if other.err != nil && !errors.Is(other.err, context.Canceled) {
				return nil, other.err
			}
			continue
		}
		fl := &flight{done: make(chan struct{})}
		if rn.flying == nil {
			rn.flying = make(map[string]*flight)
		}
		rn.flying[name] = fl
		rn.mu.Unlock()

		index, err := rn.makeIndex(ctx, cfg, includes, text, dir, name, write, tc, lookup)
		rn.mu.Lock()
		delete(rn.flying, name)
		fl.err = err
		close(fl.done)
		rn.mu.Unlock()
		return index, err
	}
	return nil, errors.New("the header index kept being taken by other callers")
}

func (rn *Runner) makeIndex(
	ctx context.Context, cfg Config, includes []string, text, dir, name string,
	write bool, tc toolchain, lookup func() *HeaderIndex,
) (*HeaderIndex, error) {
	if dir != "" {
		release, _ := acquireLock(ctx, dir, name, effTimeout(cfg))
		if release != nil {
			defer release()
		}
	}
	if index := lookup(); index != nil {
		return index, nil
	}
	start := time.Now()
	// Discover and bind inputs before producing any names. A discovery run is not an index
	// product: its output is used only to locate required reads and missing shadow paths.
	_, discovery, err := indexOutput(ctx, cfg, []string{"-E", "-o", os.DevNull}, text)
	if err != nil {
		return nil, err
	}
	read := interop.ParseDeps(discovery.depData, discovery.srcPath, cfg.Dir)
	slices.Sort(read)
	read = slices.Compact(read)
	metadata := new(Runner)
	deps := metadata.finalDeps(ctx, cfg, text, includes, read, nil, tc)
	info := metadata.includeInfo(ctx, cfg, text, read, false, "")
	if !info.complete || !info.successful || len(read) == 0 || racyDeps(deps, start) {
		return nil, errors.New("header index inputs changed or could not be validated")
	}
	required, err := captureIndexDeps(read)
	if err != nil {
		return nil, err
	}
	resolution, err := captureIndexResolution(append(slices.Clone(deps), cfg.Dir))
	if err != nil {
		return nil, err
	}
	depsHash := DepsHash(deps)
	decls, out, err := indexOutput(ctx, cfg, []string{"-fsyntax-only", "-Xclang", "-ast-list"}, text)
	if err != nil {
		return nil, err
	}
	macros, macroOut, err := indexOutput(ctx, cfg, []string{"-E", "-dM"}, text)
	if err != nil {
		return nil, err
	}
	// Do not offer compiler predefines as if the user's header declared them.
	builtin, _, err := indexOutput(ctx, cfg, []string{"-E", "-dM"}, "")
	if err != nil {
		return nil, err
	}
	namespaceText, namespaceOut, err := indexOutput(ctx, cfg, []string{"-E", "-P"}, text)
	if err != nil {
		return nil, err
	}
	for _, output := range []*auxOut{out, macroOut, namespaceOut} {
		actual := interop.ParseDeps(output.depData, output.srcPath, cfg.Dir)
		slices.Sort(actual)
		actual = slices.Compact(actual)
		if !slices.Equal(actual, read) {
			return nil, errors.New("header index dependency resolution changed during production")
		}
	}
	index := parseHeaderIndex(decls, macros, builtin)
	if err := validateIndexNames(ctx, cfg, text, index); err != nil {
		return nil, err
	}
	if changed, err := completeIndexNamespaces(ctx, index, namespaceText); err != nil {
		return nil, err
	} else if changed {
		// Validate visible names in their destination namespaces, including ambiguity/shadowing.
		if err := validateIndexNames(ctx, cfg, text, index); err != nil {
			return nil, err
		}
	}
	if !required.unchanged() || !resolution.unchanged() || racyDeps(deps, start) || DepsHash(deps) != depsHash {
		return nil, errors.New("header index inputs changed during name validation")
	}
	index.pruneNamespaceMembers()
	if index.Bytes() > maxIndexOutput {
		return nil, errors.New("header index exceeds its byte budget")
	}
	body, err := json.Marshal(headerIndexFile{
		Scopes: index.scopes, Namespaces: index.namespaces, Targets: resolution.targets,
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rn.addVariant(dir, name, "i", "header names "+cfg.CXX.Name(),
		variant{Deps: deps, DepsHash: depsHash, Part: body}, write)
	return index, nil
}

// Required reads must keep their identity and content. Missing shadow candidates in deps
// deliberately have a hash, but a header actually read by clang may never become one of them.
type indexDependency struct {
	path string
	info os.FileInfo
	hash string
}

type indexDependencies []indexDependency

func captureIndexDeps(paths []string) (indexDependencies, error) {
	var deps indexDependencies
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("required index header disappeared: %s", path)
		}
		hash := hashFile(path)
		if hash == "missing" || hash == "unreadable" {
			return nil, fmt.Errorf("required index header cannot be read: %s", path)
		}
		deps = append(deps, indexDependency{path: path, info: info, hash: hash})
	}
	if !deps.unchanged() {
		return nil, errors.New("required index headers changed while being captured")
	}
	return deps, nil
}

func (deps indexDependencies) unchanged() bool {
	for _, dep := range deps {
		info, err := os.Stat(dep.path)
		if err != nil || !info.Mode().IsRegular() || !os.SameFile(dep.info, info) ||
			dep.info.Size() != info.Size() || !dep.info.ModTime().Equal(info.ModTime()) {
			return false
		}
		oldCT, oldOK := changeTime(dep.info)
		newCT, newOK := changeTime(info)
		if oldOK != newOK || oldOK && oldCT != newCT || hashFile(dep.path) != dep.hash {
			return false
		}
	}
	return true
}

type indexResolution struct {
	nodes   map[string]os.FileInfo
	targets map[string]string // empty target: intentionally missing shadow candidate
}

func captureIndexResolution(paths []string) (*indexResolution, error) {
	state := &indexResolution{nodes: make(map[string]os.FileInfo), targets: make(map[string]string)}
	for _, path := range paths {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		state.targets[path] = resolved
		for _, spelling := range []string{path, resolved} {
			if spelling == "" {
				continue
			}
			var prefixes []string
			for i := len(filepath.VolumeName(spelling)) + 1; i < len(spelling); i++ {
				if os.IsPathSeparator(spelling[i]) {
					prefixes = append(prefixes, spelling[:i])
				}
			}
			prefixes = append(prefixes, spelling)
			for _, prefix := range prefixes {
				if _, seen := state.nodes[prefix]; seen {
					continue
				}
				info, err := os.Lstat(prefix)
				if err != nil && !os.IsNotExist(err) {
					return nil, err
				}
				state.nodes[prefix] = info
			}
		}
	}
	if !state.unchanged() {
		return nil, errors.New("header include resolution changed while being captured")
	}
	return state, nil
}

func sameIndexTargets(targets map[string]string) bool {
	for path, target := range targets {
		current, err := filepath.EvalSymlinks(path)
		if target == "" {
			if !os.IsNotExist(err) {
				return false
			}
		} else if err != nil || current != target {
			return false
		}
	}
	return true
}

func (state *indexResolution) unchanged() bool {
	for path, before := range state.nodes {
		after, err := os.Lstat(path)
		if before == nil {
			if !os.IsNotExist(err) {
				return false
			}
			continue
		}
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return false
		}
		if before.IsDir() {
			continue // unrelated directory entries may change while clang uses temporary files
		}
		oldCT, oldOK := changeTime(before)
		newCT, newOK := changeTime(after)
		if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) ||
			oldOK != newOK || oldOK && oldCT != newCT {
			return false
		}
	}
	return sameIndexTargets(state.targets)
}

func indexOutput(ctx context.Context, cfg Config, mode []string, text string) (string, *auxOut, error) {
	buf := &limitWriter{max: maxIndexOutput}
	out, err := runAuxTo(ctx, cfg, mode, text, func(string) io.Writer { return buf })
	if err != nil {
		return "", nil, err
	}
	if out.exitErr != nil || out.timedOut || buf.truncated {
		return "", nil, fmt.Errorf("incomplete header index (%v, timeout=%v, limit=%v)",
			out.exitErr, out.timedOut, buf.truncated)
	}
	return buf.buf.String(), out, nil
}

func parseHeaderIndex(decls, macros, builtin string) *HeaderIndex {
	scopes := make(map[string]map[string]bool)
	add := func(scope, name string, macro bool) {
		if !identRx.MatchString(name) {
			return
		}
		names := scopes[scope]
		if names == nil {
			names = make(map[string]bool)
			scopes[strings.Clone(scope)] = names
		}
		previous, exists := names[name]
		if !exists || macro && !previous {
			// A retained substring must not pin a mostly discarded compiler output buffer.
			names[strings.Clone(name)] = previous || macro
		}
	}
	for _, line := range strings.Split(decls, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "::")
		valid := true
		for _, part := range parts {
			valid = valid && identRx.MatchString(part)
		}
		if valid {
			add(strings.Join(parts[:len(parts)-1], "::"), parts[len(parts)-1], false)
		}
	}
	predefined := macroNames(builtin)
	for name := range macroNames(macros) {
		if !predefined[name] {
			add("", name, true)
		}
	}
	index := &HeaderIndex{scopes: make(map[string][]HeaderSymbol)}
	for scope, names := range scopes {
		for name, macro := range names {
			index.scopes[scope] = append(index.scopes[scope], HeaderSymbol{Name: name, Macro: macro})
		}
		slices.SortFunc(index.scopes[scope], func(a, b HeaderSymbol) int {
			return strings.Compare(a.Name, b.Name)
		})
	}
	return index
}

// ast-list also prints unqualified parameter/local names. A single batched name-only
// syntax check removes those, and distinguishes namespaces from classes without an AST dump.
// Each using declaration has its own namespace so equal unqualified names cannot collide.
func validateIndexNames(ctx context.Context, cfg Config, text string, index *HeaderIndex) error {
	type candidate struct {
		scope string
		name  string
	}
	var source strings.Builder
	source.WriteString(text)
	line := strings.Count(text, "\n")
	lines := make(map[int]candidate)
	for _, scope := range sortedKeys(index.scopes) {
		for _, symbol := range index.scopes[scope] {
			if symbol.Macro {
				continue
			}
			name := symbol.Name
			if scope != "" {
				name = scope + "::" + name
			}
			if source.Len()+len(name)+80 > maxIndexOutput {
				return errors.New("header name validation exceeds its source budget")
			}
			line++
			fmt.Fprintf(&source, "namespace gocpp_index_%d { using ::%s; }\n", line, name)
			lines[line] = candidate{scope: scope, name: symbol.Name}
		}
	}
	line++
	sentinelLine := line
	source.WriteString("static_assert(false, \"gocpp_index_validation_complete\");\n")
	out, err := runAux(ctx, cfg, []string{"-fsyntax-only", "-ferror-limit=0",
		"-fno-caret-diagnostics", "-Wno-everything"}, source.String())
	if err != nil {
		return err
	}
	var exit *exec.ExitError
	if out.timedOut || out.truncated || !errors.As(out.exitErr, &exit) || exit.ExitCode() != 1 {
		return errors.New("incomplete or abnormal header name validation")
	}
	invalid := make(map[candidate]bool)
	namespaces := map[string]bool{"": true}
	groups := ParseDiags(out.stderr, out.srcPath)
	complete := false
	for _, group := range groups {
		if group.Err.File == probeFileName && group.Err.Line == sentinelLine &&
			group.Err.Severity == "error" && strings.Contains(group.Err.Msg, "gocpp_index_validation_complete") {
			complete = true
			continue
		}
		decl, ok := lines[group.Err.Line]
		if !ok || group.Err.File != probeFileName || group.Err.Severity == "fatal error" {
			return errors.New("header name validation failed outside a candidate")
		}
		if strings.Contains(group.Err.Msg, "using declaration cannot refer to a namespace") {
			name := decl.name
			if decl.scope != "" {
				name = decl.scope + "::" + name
			}
			namespaces[name] = true
			if index.namespaces != nil {
				if node, ok := lookupNamespacePath(index.namespaces, "", strings.Split(name, "::")); ok {
					namespaces[node] = true
				}
			}
		} else {
			invalid[decl] = true
		}
	}
	if !complete {
		return errors.New("header name validation did not reach the end of its input")
	}
	for scope, names := range index.scopes {
		if !namespaces[scope] {
			delete(index.scopes, scope)
			continue
		}
		filtered := slices.DeleteFunc(names, func(symbol HeaderSymbol) bool {
			return !symbol.Macro && invalid[candidate{scope: scope, name: symbol.Name}]
		})
		// Clip alone would retain the original, potentially local-heavy backing allocation.
		compact := make([]HeaderSymbol, len(filtered))
		copy(compact, filtered)
		index.scopes[scope] = compact
	}
	if index.namespaces == nil {
		for namespace := range namespaces {
			if _, exists := index.scopes[namespace]; !exists {
				index.scopes[strings.Clone(namespace)] = nil
			}
		}
	}
	return nil
}

// Namespace visibility is not in ast-list. Read just namespace relationships from the
// preprocessed tokens, then let the compiler validate every added qualified name. This
// avoids whole AST dumps and never puts namespace lookup on the request path.
type namespaceEdge struct {
	from    string
	context string
	target  string
	alias   bool
	declare bool
}

// Visibility propagates over the finite set of declared namespaces. Child namespace bindings
// point at canonical nodes; ancestor imports never materialize detail::detail::detail paths.
func completeIndexNamespaces(ctx context.Context, index *HeaderIndex, text string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	events, err := indexNamespaceEdges(ctx, text)
	if err != nil {
		return false, err
	}
	graph := newNamespaceGraph(ctx)
	// Resolve each declaration's target with the visibility at that declaration. In particular,
	// a later namespace that shadows an imported name must not retarget an earlier alias.
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if event.declare {
			if err := graph.declare(event.from); err != nil {
				return false, err
			}
			continue
		}
		target, err := graph.lookup(event.context, event.target)
		if err != nil {
			return false, err
		}
		if target == "\x00" {
			return false, fmt.Errorf("namespace target %s is not visible from %s", event.target, event.context)
		}
		if event.alias {
			parent, name := indexNamespaceParent(event.from)
			if err := graph.bind(parent, name, target); err != nil {
				return false, err
			}
		} else if err := graph.use(event.from, target); err != nil {
			return false, err
		}
	}
	if err := graph.settle(); err != nil {
		return false, err
	}

	// Namespace bindings and declaration names share the same finite canonical nodes.
	names := make(map[string]map[string]HeaderSymbol)
	directNames := make(map[string]map[string]bool)
	for node := range graph.direct {
		names[node] = make(map[string]HeaderSymbol)
		directNames[node] = make(map[string]bool)
	}
	for scope, symbols := range index.scopes {
		node := ""
		if scope != "" {
			var err error
			node, err = graph.lookup("", "::"+scope)
			if err != nil {
				return false, err
			}
		}
		if names[node] == nil {
			continue
		}
		for _, symbol := range symbols {
			names[node][symbol.Name] = symbol
			if _, namespace := index.scopes[indexQualifiedName(scope, symbol.Name)]; !namespace {
				directNames[node][symbol.Name] = true
			}
		}
	}
	changed := false
	// ast-list may omit an inline namespace from a qualified spelling. Keep the explicit
	// declaration names too; the validation pass checks them at their canonical parent.
	for node, bindings := range graph.direct {
		for name := range bindings {
			if _, exists := names[node][name]; !exists {
				names[node][name] = HeaderSymbol{Name: name}
				changed = true
			}
		}
	}
	total := graph.facts
	for node := range names {
		total += len(names[node])
	}
	if total > 65536 {
		return false, errors.New("too many direct namespace facts to index")
	}
	for round := 0; ; round++ {
		more := false
		for from, targets := range graph.uses {
			for target := range targets {
				if err := ctx.Err(); err != nil {
					return false, err
				}
				for name, symbol := range names[target] {
					if _, exists := names[from][name]; exists || symbol.Macro {
						continue
					}
					names[from][name] = symbol
					total++
					more = true
					changed = true
				}
				if total > 65536 {
					return false, errors.New("too many visible namespace facts to index")
				}
			}
		}
		if !more {
			break
		}
		if round > len(names) {
			return false, errors.New("namespace declaration visibility did not converge")
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	index.scopes = make(map[string][]HeaderSymbol)
	for scope, declarations := range names {
		symbols := make([]HeaderSymbol, 0, len(declarations))
		for _, symbol := range declarations {
			symbols = append(symbols, symbol)
		}
		slices.SortFunc(symbols, func(a, b HeaderSymbol) int {
			return strings.Compare(a.Name, b.Name)
		})
		index.scopes[scope] = symbols
		for name := range directNames[scope] {
			if _, namespace := graph.direct[scope][name]; !namespace {
				delete(graph.visible[scope], name) // a direct value/type shadows imported namespaces
			}
		}
	}
	index.namespaces = graph.visible
	return changed, nil
}

// The graph records declaration-time bindings, rather than guessing a qualified name from
// physical AST keys. Using directives are edges between canonical nodes and are closed before
// a later target is resolved. Aliases keep the node selected at their declaration.
type namespaceGraph struct {
	ctx        context.Context
	direct     map[string]map[string]string
	ids        map[string]string
	visible    map[string]map[string]string
	uses       map[string]map[string]bool
	dirty      bool
	scopeBytes int
	facts      int
}

func newNamespaceGraph(ctx context.Context) *namespaceGraph {
	return &namespaceGraph{
		ctx: ctx, direct: map[string]map[string]string{"": {}},
		ids: map[string]string{"": ""}, uses: make(map[string]map[string]bool), dirty: true,
	}
}

func (graph *namespaceGraph) declare(scope string) error {
	parent := ""
	for _, name := range strings.Split(scope, "::") {
		node := indexQualifiedName(parent, name)
		if graph.direct[node] == nil {
			if len(graph.direct) >= 2048 || graph.scopeBytes+len(node) > maxIndexOutput {
				return errors.New("too many canonical namespace nodes to index")
			}
			node = strings.Clone(node)
			graph.direct[node] = make(map[string]string)
			graph.ids[node] = node
			graph.scopeBytes += len(node)
		}
		if err := graph.bind(parent, name, node); err != nil {
			return err
		}
		parent = node
	}
	return nil
}

func (graph *namespaceGraph) bind(parent, name, target string) error {
	bindings := graph.direct[parent]
	if bindings == nil || graph.direct[target] == nil {
		return errors.New("namespace binding refers to an undeclared scope")
	}
	if previous, exists := bindings[name]; !exists || previous != target {
		bindings[strings.Clone(name)] = graph.ids[target]
		graph.dirty = true
	}
	return nil
}

func (graph *namespaceGraph) use(from, target string) error {
	if graph.direct[from] == nil || graph.direct[target] == nil {
		return errors.New("namespace using directive refers to an undeclared scope")
	}
	if graph.uses[from] == nil {
		graph.uses[graph.ids[from]] = make(map[string]bool)
	}
	if !graph.uses[from][target] {
		graph.uses[from][graph.ids[target]] = true
		graph.dirty = true
	}
	return nil
}

func (graph *namespaceGraph) settle() error {
	if err := graph.ctx.Err(); err != nil {
		return err
	}
	if !graph.dirty {
		return nil
	}
	visible := make(map[string]map[string]string)
	facts := 0
	for node, direct := range graph.direct {
		visible[node] = make(map[string]string)
		for name, target := range direct {
			visible[node][name] = target
			facts++
		}
	}
	for round := 0; ; round++ {
		more := false
		for from, targets := range graph.uses {
			for target := range targets {
				if err := graph.ctx.Err(); err != nil {
					return err
				}
				for name, binding := range visible[target] {
					if _, direct := graph.direct[from][name]; direct {
						continue
					}
					previous, exists := visible[from][name]
					if !exists {
						visible[from][name] = binding
						facts++
						more = true
					} else if previous != binding && previous != "\x00" {
						visible[from][name] = "\x00"
						more = true
					}
				}
				if facts > 65536 {
					return errors.New("too many visible namespace bindings to index")
				}
			}
		}
		if !more {
			break
		}
		if round > 2*len(visible)+1 {
			return errors.New("namespace bindings did not converge")
		}
	}
	graph.visible = visible
	graph.facts = facts
	graph.dirty = false
	return nil
}

func (graph *namespaceGraph) lookup(scope, target string) (string, error) {
	if err := graph.settle(); err != nil {
		return "", err
	}
	if target == "" {
		return "\x00", nil
	}
	absolute := strings.HasPrefix(target, "::")
	parts := strings.Split(strings.TrimPrefix(target, "::"), "::")
	if absolute {
		scope = ""
	}
	for {
		// Only the first component uses enclosing-scope lookup. Once it binds, every later
		// component is resolved in that canonical namespace, exactly as completed lookups do.
		if _, exists := graph.visible[scope][parts[0]]; exists {
			if node, ok := lookupNamespacePath(graph.visible, scope, parts); ok {
				return node, nil
			}
			return "\x00", nil
		}
		if absolute || scope == "" {
			return "\x00", nil
		}
		scope, _ = indexNamespaceParent(scope)
	}
}

func indexNamespaceParent(scope string) (string, string) {
	if at := strings.LastIndex(scope, "::"); at >= 0 {
		return scope[:at], scope[at+2:]
	}
	return "", scope
}

func lookupNamespacePath(members map[string]map[string]string, scope string, parts []string) (string, bool) {
	for _, part := range parts {
		target, exists := members[scope][part]
		if !exists || target == "\x00" {
			return "", false
		}
		scope = target
	}
	return scope, true
}

func (index *HeaderIndex) pruneNamespaceMembers() {
	for scope, members := range index.namespaces {
		visible := make(map[string]bool)
		for _, symbol := range index.scopes[scope] {
			visible[symbol.Name] = true
		}
		for name, target := range members {
			if !visible[name] || target == "\x00" {
				delete(members, name)
			}
		}
	}
}

func indexQualifiedName(scope, name string) string {
	if scope == "" {
		return name
	}
	return scope + "::" + name
}

func indexNamespaceEdges(ctx context.Context, text string) ([]namespaceEdge, error) {
	tokens := namespaceTokens{text: text}
	type frame struct {
		scope  string
		active bool
	}
	current := frame{active: true}
	var stack []frame
	var edges []namespaceEdge
	scopeBytes := 0
	qualify := func(scope, name string) (string, error) {
		scopeBytes += len(scope) + len(name) + 2
		if scopeBytes > maxIndexOutput {
			return "", errors.New("namespace event paths exceed their byte budget")
		}
		return indexQualifiedName(scope, name), nil
	}
	previous := ""
	for token := tokens.next(); token != ""; token = tokens.next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(edges) > 16384 {
			return nil, errors.New("too many namespace events to index")
		}
		switch {
		case token == "namespace" && current.active:
			tokens.skipAttributes()
			name := tokens.qualified()
			tokens.skipAttributes()
			switch next := tokens.next(); next {
			case "=":
				target := tokens.qualified()
				if name != "" && target != "" && tokens.next() == ";" {
					from, err := qualify(current.scope, name)
					if err != nil {
						return nil, err
					}
					edges = append(edges, namespaceEdge{
						from: from, context: current.scope, target: target, alias: true,
					})
				}
			case "{":
				stack = append(stack, current)
				if name == "" {
					current.active = false // anonymous namespaces aren't named index scopes
				} else {
					parent := current.scope
					scope, err := qualify(parent, name)
					if err != nil {
						return nil, err
					}
					current.scope = scope
					edges = append(edges, namespaceEdge{from: current.scope, context: parent, declare: true})
					if previous == "inline" {
						edges = append(edges, namespaceEdge{from: parent, target: "::" + current.scope})
					}
				}
			}
		case token == "using" && current.active && tokens.peek() == "namespace":
			tokens.next()
			target := tokens.qualified()
			if target != "" && tokens.next() == ";" {
				edges = append(edges, namespaceEdge{
					from: current.scope, context: current.scope, target: target,
				})
			}
		case token == "extern" && current.active && strings.HasPrefix(tokens.peek(), "\""):
			tokens.next()
			if tokens.peek() == "{" {
				tokens.next()
				stack = append(stack, current) // linkage blocks don't change namespace scope
			}
		case token == "{":
			stack = append(stack, current)
			current.active = false // class, function, lambda, initializer, or other non-namespace block
		case token == "}" && len(stack) > 0:
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}
		previous = token
	}
	return edges, nil
}

// namespaceTokens skips literals and comments as complete tokens, including C++ raw strings.
// Only identifiers, :: and braces/semicolons matter to the namespace relationship reader.
type namespaceTokens struct {
	text  string
	at    int
	saved string
}

func (tokens *namespaceTokens) peek() string {
	if tokens.saved == "" {
		tokens.saved = tokens.next()
	}
	return tokens.saved
}

func (tokens *namespaceTokens) skipAttributes() {
	for {
		close := ""
		open := tokens.peek()
		switch open {
		case "__attribute__", "__declspec":
			tokens.next()
			if tokens.peek() != "(" {
				return
			}
			open = "("
			close = ")"
		case "[":
			close = "]"
		default:
			return
		}
		tokens.next()
		depth := 1
		for depth > 0 {
			token := tokens.next()
			switch token {
			case "":
				return
			case open:
				depth++
			case close:
				depth--
			}
		}
	}
}

func (tokens *namespaceTokens) qualified() string {
	var parts []string
	absolute := tokens.peek() == "::"
	if absolute {
		tokens.next()
	}
	for identRx.MatchString(tokens.peek()) {
		parts = append(parts, tokens.next())
		if tokens.peek() != "::" {
			break
		}
		tokens.next()
	}
	if len(parts) == 0 {
		return ""
	}
	name := strings.Join(parts, "::")
	if absolute {
		name = "::" + name
	}
	return name
}

func (tokens *namespaceTokens) next() string {
	if tokens.saved != "" {
		value := tokens.saved
		tokens.saved = ""
		return value
	}
	for tokens.at < len(tokens.text) {
		start := tokens.at
		rest := tokens.text[start:]
		switch {
		case rest[0] <= ' ':
			tokens.at++
			continue
		case strings.HasPrefix(rest, "//"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				tokens.at = len(tokens.text)
			} else {
				tokens.at += end + 1
			}
			continue
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				tokens.at = len(tokens.text)
			} else {
				tokens.at += end + 4
			}
			continue
		}
		for _, prefix := range []string{"R\"", "u8R\"", "uR\"", "UR\"", "LR\""} {
			if !strings.HasPrefix(rest, prefix) {
				continue
			}
			open := strings.IndexByte(rest[len(prefix):], '(')
			if open < 0 {
				continue
			}
			open += len(prefix)
			close := ")" + rest[len(prefix):open] + "\""
			end := strings.Index(rest[open+1:], close)
			if end >= 0 {
				tokens.at += open + 1 + end + len(close)
				return "\"literal\""
			}
		}
		if rest[0] >= '0' && rest[0] <= '9' ||
			rest[0] == '.' && len(rest) > 1 && rest[1] >= '0' && rest[1] <= '9' {
			tokens.at++
			for tokens.at < len(tokens.text) {
				ch := tokens.text[tokens.at]
				previous := tokens.text[tokens.at-1]
				switch {
				case isIndexIdentStart(ch), ch >= '0' && ch <= '9', ch == '.':
					tokens.at++
				case ch == '\'' && tokens.at+1 < len(tokens.text) &&
					(isIndexIdentStart(tokens.text[tokens.at+1]) ||
						tokens.text[tokens.at+1] >= '0' && tokens.text[tokens.at+1] <= '9'):
					tokens.at++ // a preprocessing-number digit separator, not a character literal
				case (ch == '+' || ch == '-') && strings.ContainsRune("eEpP", rune(previous)):
					tokens.at++
				default:
					return tokens.text[start:tokens.at]
				}
			}
			return tokens.text[start:tokens.at]
		}
		if rest[0] == '\'' || rest[0] == '"' {
			quote := rest[0]
			tokens.at++
			for tokens.at < len(tokens.text) {
				ch := tokens.text[tokens.at]
				tokens.at++
				if ch == '\\' && tokens.at < len(tokens.text) {
					tokens.at++
				} else if ch == quote {
					break
				}
			}
			return tokens.text[start:tokens.at]
		}
		if isIndexIdentStart(rest[0]) {
			tokens.at++
			for tokens.at < len(tokens.text) {
				ch := tokens.text[tokens.at]
				if !isIndexIdentStart(ch) && (ch < '0' || ch > '9') {
					break
				}
				tokens.at++
			}
			return tokens.text[start:tokens.at]
		}
		if strings.HasPrefix(rest, "::") {
			tokens.at += 2
			return "::"
		}
		tokens.at++
		return tokens.text[start:tokens.at]
	}
	return ""
}

func isIndexIdentStart(ch byte) bool {
	return ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
}

func macroNames(text string) map[string]bool {
	names := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "#define ") {
			continue
		}
		rest := strings.TrimPrefix(line, "#define ")
		end := strings.IndexAny(rest, " (\t\r")
		if end >= 0 {
			rest = rest[:end]
		}
		if identRx.MatchString(rest) {
			names[rest] = true
		}
	}
	return names
}
