package probe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
)

// Small runs of the compiler that the cache needs for what a probe run does not tell: the headers
// that a probe file reads when the run failed or was stopped (-E -MD: the preprocessor only),
// the headers that a file names and that are missing (-M -MG), and the places where the compiler looks
// for a header, with the toolchain it chose (-E -v).

// An auxOut is what an auxiliary run printed.
type auxOut struct {
	exitErr   error // callers that require a complete result must check the exit status
	stderr    []byte
	truncated bool // stderr is only the first part
	timedOut  bool // the compiler was stopped at the time limit: what it printed is only the first part
	depData   []byte
	srcPath   string
}

// runAux runs the compiler on the text with the mode's flags (-E ...) and returns what it
// printed on stderr and wrote to the dependency file. The standard output is dropped. A
// compiler that fails is not an error: the caller reads what it got.
func runAux(ctx context.Context, cfg Config, mode []string, text string) (*auxOut, error) {
	return runAuxTo(ctx, cfg, mode, text, nil)
}

// runAuxTo is runAux with the standard output sent to the writer that stdout makes from the name of the
// probe file (nil: dropped).
func runAuxTo(ctx context.Context, cfg Config, mode []string, text string, stdout func(srcPath string) io.Writer) (*auxOut, error) {
	return runAuxMode(ctx, cfg, func(string) []string { return mode }, text, stdout)
}

// auxRunContext makes the context of one auxiliary compiler run: it ends at the time limit of cfg.
// It is a variable so that a test can end a run when the test decides (a blocked compiler), not
// after a wall-clock time that a slow machine or the race detector can exceed.
var auxRunContext = func(ctx context.Context, cfg Config) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, effTimeout(cfg))
}

// runAuxMode lets a narrowly scoped compiler mode refer to the temporary source path.
func runAuxMode(ctx context.Context, cfg Config, mode func(string) []string, text string,
	stdout func(string) io.Writer,
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
	args, err := interop.ProbeCompilerArgs(cfg.CXX, mode(srcPath), cfg.Dir, cfg.Flags, depPath, srcPath)
	if err != nil {
		return nil, err
	}
	rctx, cancel := auxRunContext(ctx, cfg)
	defer cancel()
	stderr := &limitWriter{max: maxStderr}
	cmd, err := build.Start(rctx, func() *exec.Cmd {
		c := cfg.CXX.Command(rctx, args...)
		c.Dir = cfg.Dir
		c.Env = interop.ProbeEnv(tmp, cfg.Dir)
		c.Stdout = io.Discard
		if stdout != nil {
			c.Stdout = stdout(srcPath)
		}
		c.Stderr = stderr
		c.WaitDelay = probeWaitDelay
		build.InGroup(c)
		return c
	})
	if err != nil {
		return nil, fmt.Errorf("%s didn't start: %w", cfg.CXX.Name(), err)
	}
	exitErr := cmd.Wait()
	timedOut := rctx.Err() != nil
	build.KillGroup(cmd)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), ctx.Err())
	}
	out := &auxOut{exitErr: exitErr, stderr: stderr.buf.Bytes(), truncated: stderr.truncated,
		timedOut: timedOut, srcPath: srcPath}
	out.depData, _ = os.ReadFile(depPath)
	return out, nil
}

// timeoutError is the error of a stage whose compiler run was stopped at the time limit. Every
// stage (completion, index, validation) reports it the same way, so that callers can tell a
// timeout from a failed header with errors.Is(err, context.DeadlineExceeded).
func timeoutError(stage string) error {
	return fmt.Errorf("%s timed out: %w", stage, context.DeadlineExceeded)
}

// preprocessDeps returns the headers that the preprocessor reads for the text (sorted), or nil
// when it could not tell (the compiler failed, or a header is missing).
func preprocessDeps(ctx context.Context, cfg Config, text string) []string {
	out, err := runAux(ctx, cfg, []string{"-E", "-o", os.DevNull}, text)
	if err != nil || len(out.depData) == 0 {
		return nil
	}
	deps := interop.ParseDeps(out.depData, out.srcPath, cfg.Dir)
	slices.Sort(deps)
	return deps
}

// A toolchain is what `clang -E -v` says about how the compiler is set up: the folders where it
// looks for a header, in order, and a hash of them with the target and the GCC installation it
// chose. The hash is part of the key of the results: another libstdc++ (a gcc was installed, an
// xcode-select changed) is another compiler even with the same program file and version line.
type toolchain struct {
	Created int64    `json:"created"`
	Hash    string   `json:"hash"`
	Dirs    []string `json:"dirs"`
}

// toolchainTTL is how long a toolchain is believed: asking costs a compiler run (20 to 40 ms), and
// a warm run does not make one more than once an hour.
const toolchainTTL = time.Hour

// toolchain returns the toolchain of cfg, from the cache (memory, disk) or from the compiler. It is
// the zero value when there is no cache folder (a Runner that only keeps memory has no use for it) or the
// compiler does not say.
func (rn *Runner) toolchain(ctx context.Context, cfg Config, dir string, write bool) toolchain {
	if dir == "" {
		return toolchain{}
	}
	// One question at a time (the parts of a call run together and would all ask), and the same for
	// every run of a Config whatever its limits are.
	rn.tcMu.Lock()
	defer rn.tcMu.Unlock()
	cfg.MaxDecl, cfg.MaxJSON = 0, 0
	name := entryName("t", selfID(), PreKey(cfg, &Source{Text: "toolchain"}))
	if body, ok := rn.get(dir, name); ok {
		var tc toolchain
		if json.Unmarshal(body, &tc) == nil && tc.Created > 0 {
			if age := time.Since(time.Unix(0, tc.Created)); age >= 0 && age < toolchainTTL {
				return tc
			}
		}
	}
	out, err := runAux(ctx, cfg, []string{"-E", "-v", "-o", os.DevNull}, "#include <cstddef>\n")
	if err != nil {
		return toolchain{}
	}
	tc := toolchain{Created: time.Now().UnixNano(), Dirs: parseSearchDirs(out.stderr), Hash: toolchainHash(out.stderr)}
	if body, err := json.Marshal(tc); err == nil {
		rn.put(dir, name, body, write)
	}
	return tc
}

// toolchainHash hashes the lines of `-v` that describe the setup (not the command line, which has the
// temporary file's name).
func toolchainHash(stderr []byte) string {
	h := sha256.New()
	in := false
	for _, line := range strings.Split(string(stderr), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "Target:"), strings.HasPrefix(line, "Selected GCC"), strings.HasPrefix(line, "Found candidate GCC"):
			io.WriteString(h, line+"\n")
		case strings.HasPrefix(line, "#include ") && strings.HasSuffix(line, "search starts here:"):
			in = true
			io.WriteString(h, line+"\n")
		case strings.HasPrefix(line, "End of search list"):
			in = false
		case in:
			io.WriteString(h, line+"\n")
		}
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// withToolchain adds the toolchain to a key.
func withToolchain(pre string, tc toolchain) string {
	if tc.Hash == "" {
		return pre
	}
	sum := sha256.Sum256([]byte(pre + "\x00toolchain\x00" + tc.Hash))
	return hex.EncodeToString(sum[:])
}

// searchDirs returns the folders where the compiler looks for an included header (its own
// include path with the flags' -I and -isystem), from `-E -v`, when there is no cached toolchain.
// The answer is kept for the life of the Runner. It is nil when the compiler does not say.
func (rn *Runner) searchDirs(ctx context.Context, cfg Config) []string {
	dirs, _ := rn.searchDirsChecked(ctx, cfg)
	dirs = slices.Clone(dirs)
	for i, dir := range dirs {
		dirs[i] = filepath.Clean(dir)
	}
	return dirs
}

// searchDirsChecked is the same discovery with a completeness requirement for input guards.
// Unlike toolchain, it also asks the compiler when persistent caching is disabled.
func (rn *Runner) searchDirsChecked(ctx context.Context, cfg Config) ([]string, error) {
	key := PreKey(cfg, &Source{Text: "search-dirs"})
	rn.mu.Lock()
	dirs, ok := rn.dirsMemo[key]
	rn.mu.Unlock()
	if ok {
		return dirs, nil
	}
	out, err := runAux(ctx, cfg, []string{"-E", "-v", "-o", os.DevNull}, "#include <cstddef>\n")
	if err != nil {
		return nil, err
	}
	dirs = rawSearchDirs(out.stderr)
	if out.exitErr != nil || out.timedOut || out.truncated ||
		!strings.Contains(string(out.stderr), "End of search list.") {
		// Older best-effort callers may still use the parsed paths; strict input guards
		// reject them, and an incomplete answer is never memoized as a complete one.
		return dirs, fmt.Errorf("incomplete compiler search path (%v)", out.exitErr)
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("compiler supplied no include search path")
	}
	rn.mu.Lock()
	if rn.dirsMemo == nil {
		rn.dirsMemo = make(map[string][]string)
	}
	rn.dirsMemo[key] = dirs
	rn.mu.Unlock()
	return dirs, nil
}

// parseSearchDirs reads the lists that `clang -v` prints:
//
//	#include "..." search starts here:
//	#include <...> search starts here:
//	 /usr/include
//	End of search list.
func parseSearchDirs(stderr []byte) []string {
	dirs := rawSearchDirs(stderr)
	for i, dir := range dirs {
		dirs[i] = filepath.Clean(dir)
	}
	return dirs
}

// Strict guards retain dot/parent spellings: cleaning them can change symlink traversal.
func rawSearchDirs(stderr []byte) []string {
	var dirs []string
	in := false
	for _, line := range strings.Split(string(stderr), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "#include ") && strings.HasSuffix(line, "search starts here:"):
			in = true
		case strings.HasPrefix(line, "End of search list"):
			in = false
		case in && strings.HasPrefix(line, " "):
			d := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "(framework directory)"))
			if d != "" {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs
}

// underDir returns the name of path relative to dir when it is inside it.
func underDir(dir, path string) (string, bool) {
	if dir == "" || !filepath.IsAbs(dir) {
		return "", false
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// searchOrder returns the folders where an include is looked up, in the order of the search as far as the
// flags and the environment tell it (-iquote, -I, -isystem, -idirafter, CPATH, CPLUS_INCLUDE_PATH:
// all of them, whether the folder exists now or not: one made later has to be seen), then the folders
// of the compiler's own list (-E -v, which leaves out the ones that do not exist). user are the
// folders that the user's flags and environment name.
func searchOrder(cfg Config, vdirs []string) (order, user []string) {
	iquote, idirs := flagDirs(cfg)
	// The package folder is the first -iquote of every run (interop.ProbeCompilerArgs), whatever the flags are.
	user = slices.Concat([]string{filepath.Clean(cfg.Dir)}, iquote, idirs)
	if cfg.Dir == "" {
		user = user[1:]
	}
	for _, name := range []string{"CPATH", "CPLUS_INCLUDE_PATH"} {
		for _, d := range filepath.SplitList(os.Getenv(name)) {
			if filepath.IsAbs(d) {
				user = append(user, d)
			}
		}
	}
	seen := make(map[string]bool)
	for _, d := range slices.Concat(user, vdirs) {
		if !seen[d] {
			seen[d] = true
			order = append(order, d)
		}
	}
	return order, user
}

// earlierShadows adds, for every header that was read and found in one of the search folders, the same
// name in the folders that come before it and in all the folders that the user's flags and environment
// name (their order is only known as far as the flags go): a header that appears there hides the one
// that was found, in a header that a header includes as well as in the package's own includes.
func earlierShadows(read, order, user []string) []string {
	seen := make(map[string]bool, len(read))
	for _, d := range read {
		seen[d] = true
	}
	var add []string
	for _, d := range read {
		best, bestLen := -1, -1
		for k, dir := range order {
			if _, ok := underDir(dir, d); ok && len(dir) > bestLen {
				best, bestLen = k, len(dir)
			}
		}
		if best < 0 {
			continue
		}
		rel, _ := underDir(order[best], d)
		for _, e := range slices.Concat(order[:best], user) {
			if p := joinRaw(e, rel); !seen[p] {
				seen[p] = true
				add = append(add, p)
			}
		}
	}
	return add
}

// systemDirs returns the folders of the search order that the user's flags and environment do not name:
// the compiler's own.
func systemDirs(order, user []string) []string {
	var sys []string
	for _, d := range order {
		if !slices.Contains(user, d) {
			sys = append(sys, d)
		}
	}
	return sys
}

// guessIncluderShadows is includerShadows for a run whose includes are not known: every folder of a
// header that was read (and the package folder) is taken to have an include of every name that a header
// was found by, and so it watches all of those names there. It costs more candidates and never fewer.
func guessIncluderShadows(read []string, cfgDir string, order, user []string) []string {
	sys := systemDirs(order, user)
	dirs := map[string]bool{cfgDir: true}
	names := map[string]bool{}
	for _, f := range read {
		skip := false
		for _, d := range sys {
			if _, ok := underDir(d, f); ok {
				skip = true
			}
		}
		for _, d := range user {
			if _, ok := underDir(d, f); ok {
				skip = false
			}
		}
		if skip {
			continue
		}
		dirs[filepath.Dir(f)] = true
		names[filepath.Base(f)] = true
		best := -1
		for _, d := range order {
			if rel, ok := underDir(d, f); ok && len(d) > best {
				best = len(d)
				names[rel] = true
			}
		}
	}
	seen := make(map[string]bool, len(read))
	for _, f := range read {
		seen[f] = true
	}
	var add []string
	for d := range dirs {
		for n := range names {
			if p := joinRaw(d, n); !seen[p] {
				seen[p] = true
				add = append(add, p)
			}
		}
	}
	slices.Sort(add)
	return add
}

// missingDeps asks the preprocessor for the headers of the text with -M -MG (a header that is not
// found is listed, by its name, instead of stopping the run): the files that exist, and the names
// that do not, relative to the package folder.
func missingDeps(ctx context.Context, cfg Config, text string) (found, missing []string) {
	out, err := runAux(ctx, cfg, []string{"-M", "-MG", "-o", os.DevNull}, text)
	if err != nil || len(out.depData) == 0 {
		return nil, nil
	}
	for _, p := range interop.ParseDeps(out.depData, out.srcPath, cfg.Dir) {
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		} else if name, ok := underDir(cfg.Dir, p); ok {
			missing = append(missing, name)
		} else {
			missing = append(missing, filepath.Base(p))
		}
	}
	return found, missing
}

// finalDeps completes the list of the headers that a run depends on: the ones the compiler read,
// the places where a header would hide one of them (the package folder and the -iquote and -I folders
// for the package's own includes; every folder before the one a header was found in, when the compiler's
// search path is known), and, when a header was not found, the places where it would be: every
// folder it is searched in, for the package's includes and for a header's own (a nested include, found with
// -MG). Without these a result that depends on a header that is missing, or on the absence of
// another, would be kept for ever.
func (rn *Runner) finalDeps(ctx context.Context, cfg Config, text string, includes, deps []string, headerErrs []Diag, tc toolchain) []string {
	read := slices.Clone(deps)
	missing := false
	var errNames []string
	for _, d := range headerErrs {
		if strings.Contains(d.Msg, "file not found") {
			missing = true
			errNames = append(errNames, d.Msg)
		}
	}
	vdirs := tc.Dirs
	if vdirs == nil && missing {
		vdirs = rn.searchDirs(ctx, cfg)
	}
	dirs, user := searchOrder(cfg, vdirs)
	info := rn.includeInfo(ctx, cfg, text, read, missing, strings.Join(errNames, "\n"))
	// When the compiler stopped at a missing header it wrote no dependency file: the files it entered
	// are the headers that were read.
	var entered []string
	for _, f := range info.entered {
		if fi, err := os.Stat(f); err == nil && fi.Mode().IsRegular() {
			entered = append(entered, f)
		}
	}
	if missing {
		read = slices.Concat(read, entered)
		deps = slices.Concat(deps, entered)
	}
	includers := includerShadows(info.directives, cfg.Dir, dirs, user)
	if !info.complete {
		includers = append(includers, guessIncluderShadows(read, cfg.Dir, dirs, user)...)
	}
	deps = slices.Concat(deps, earlierShadows(read, dirs, user), includers)
	deps = addShadows(deps, cfg, includes)
	if !missing {
		slices.Sort(deps)
		return slices.Compact(deps)
	}
	seen := make(map[string]bool, len(deps))
	for _, d := range deps {
		seen[d] = true
	}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			deps = append(deps, p)
		}
	}
	// The package's own includes, in every folder.
	for _, dir := range dirs {
		for _, inc := range includes {
			if len(inc) < 3 {
				continue
			}
			if name := inc[1 : len(inc)-1]; !filepath.IsAbs(name) {
				add(joinRaw(dir, name))
			}
		}
	}
	// The names that headers include and that are not there: in the folders of the search, the
	// package folder and the folder of every header that was read (a quoted include looks there first).
	// The names are the ones the preprocessor met, as they were spelled (`../x.h` keeps its `..`); when it
	// did not tell, they come from -M -MG, which gives a path that is cut to its base name.
	var names []string
	if info.complete {
		for _, d := range info.directives {
			if !d.entered && d.name != "" && !filepath.IsAbs(d.name) {
				names = append(names, d.name)
			}
		}
	} else {
		var found []string
		found, names = missingDeps(ctx, cfg, text)
		for _, f := range found {
			add(f)
		}
		entered = slices.Concat(entered, found)
	}
	where := slices.Concat(dirs, []string{cfg.Dir})
	for _, f := range slices.Concat(read, entered) {
		if fi, err := os.Stat(f); err == nil && fi.Mode().IsRegular() {
			where = append(where, filepath.Dir(f))
		}
	}
	for _, f := range entered {
		add(f)
	}
	slices.Sort(where)
	where = slices.Compact(where)
	slices.Sort(names)
	names = slices.Compact(names)
	for _, name := range names {
		for _, dir := range where {
			if dir != "" {
				add(joinRaw(dir, name))
			}
		}
	}
	slices.Sort(deps)
	return slices.Compact(deps)
}
