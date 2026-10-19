package probe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// Defaults of Config.
const (
	DefaultTimeout  = 30 * time.Second
	DefaultMaxJSON  = 1 << 30 // the whole dump
	maxStderr       = 4 << 20 // kept of the compiler's messages
	probeFileName   = "gp_probe.cpp"
	probeWaitDelay  = 5 * time.Second
	probeDeclPrefix = "gp_probe_"
)

// A Config says how to run the probes of one package.
type Config struct {
	// CXX is the compiler; it must be clang++ (Family == build.Clang).
	CXX build.CXX

	// Dir is the package folder (absolute): the compiler runs there and
	// quoted includes ("./physics.h") resolve against it. Flags are the
	// package's probe flags (check.Package.ProbeFlags).
	Dir   string
	Flags []string

	// Timeout limits the compiler run (EnvTimeout, else DefaultTimeout, when 0). MaxDecl is the
	// largest declaration of the dump that is kept (DefaultMaxDecl when 0)
	// and MaxJSON the size of the whole dump (DefaultMaxJSON when 0): beyond
	// it the compiler is killed and the run fails.
	Timeout time.Duration
	MaxDecl int
	MaxJSON int64
}

// FindClang returns clang++ for the probes: the program in GOCPP_CLANG when
// it is set, else clang++ in PATH. When it isn't there, or isn't clang, the
// error wraps ErrNoClang.
func FindClang(ctx context.Context) (build.CXX, error) {
	prog := os.Getenv("GOCPP_CLANG")
	if prog == "" {
		prog = "clang++"
	}
	cxx, err := build.LookCXX(ctx, prog)
	if err != nil {
		return build.CXX{}, fmt.Errorf("%w: %v", ErrNoClang, err)
	}
	if cxx.Family != build.Clang {
		return build.CXX{}, fmt.Errorf("%w: %s is not clang", ErrNoClang, cxx.Name())
	}
	return cxx, nil
}

// A Runner runs probes and remembers the tables of the probe files it has
// run, in memory (step 12.7 builds the cache on disk from Key and DepsHash).
// The zero value is ready to use; it is safe for concurrent use.
type Runner struct {
	mu     sync.Mutex
	memo   map[string]*memoEntry
	flying map[string]*flight // probe runs in progress, by PreKey
	runs   int
}

// A flight is a compiler run that other callers of the same probe wait for.
type flight struct {
	done chan struct{}
	err  error
}

type memoEntry struct {
	table *Table
	deps  string // DepsHash of table.Deps when it was made
}

// Runs returns the number of compiler runs so far.
func (rn *Runner) Runs() int {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.runs
}

// Probe generates the probe file for set and runs it.
func (rn *Runner) Probe(ctx context.Context, cfg Config, set *collect.Set, gc GenConfig) (*Table, error) {
	t0 := time.Now()
	src := Generate(set, gc)
	gen := time.Since(t0)
	t, err := rn.Run(ctx, cfg, src)
	if t != nil {
		t.Timing.Generate = gen
		t.Timing.Total += gen
	}
	return t, err
}

// Run runs the probe file src and returns its table. A table that the
// Runner has for the same probe file, compiler, flags and environment is
// returned (Cached) when none of the headers of its run changed (by content).
func (rn *Runner) Run(ctx context.Context, cfg Config, src *Source) (*Table, error) {
	if cfg.CXX.Family != build.Clang {
		return nil, fmt.Errorf("%w: the probe compiler is %s", ErrNoClang, cfg.CXX.Name())
	}
	pre := PreKey(cfg, src)
	var fl *flight
	for {
		rn.mu.Lock()
		e := rn.memo[pre]
		if e != nil && DepsHash(e.table.Deps) == e.deps {
			rn.mu.Unlock()
			t := *e.table
			t.Cached = true
			return &t, nil
		}
		other := rn.flying[pre]
		if other == nil {
			fl = &flight{done: make(chan struct{})}
			if rn.flying == nil {
				rn.flying = make(map[string]*flight)
			}
			rn.flying[pre] = fl
			rn.mu.Unlock()
			break
		}
		rn.mu.Unlock()
		// The same probe is running (another caller): wait for it, then look at the memo
		// again, instead of starting a second compiler for the same file.
		select {
		case <-other.done:
		case <-ctx.Done():
			return nil, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), ctx.Err())
		}
		if other.err != nil && !errors.Is(other.err, context.Canceled) {
			return nil, other.err
		}
	}
	t, err := runProbe(ctx, cfg, src)
	rn.mu.Lock()
	if err == nil {
		if rn.memo == nil {
			rn.memo = make(map[string]*memoEntry)
		}
		rn.memo[pre] = &memoEntry{table: t, deps: DepsHash(t.Deps)}
		rn.runs++
	}
	delete(rn.flying, pre)
	rn.mu.Unlock()
	fl.err = err
	close(fl.done)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// EnvTimeout names the environment variable that sets the compiler run's time
// limit (a Go duration, "5m") when Config.Timeout is 0. The tests set it high:
// a clang++ run that takes 0.1 s on a quiet machine can take minutes when a
// hundred compilers run at once.
const EnvTimeout = "GOCPP_PROBE_TIMEOUT"

func effTimeout(cfg Config) time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	if d, err := time.ParseDuration(os.Getenv(EnvTimeout)); err == nil && d > 0 {
		return d
	}
	return DefaultTimeout
}

func effMaxDecl(cfg Config) int {
	if cfg.MaxDecl <= 0 {
		return DefaultMaxDecl
	}
	return cfg.MaxDecl
}

func effMaxJSON(cfg Config) int64 {
	if cfg.MaxJSON <= 0 {
		return DefaultMaxJSON
	}
	return cfg.MaxJSON
}

// PreKey returns the part of the cache key that is known before the run:
// the probe file, the compiler (path, arguments, version, identity), the
// flags, the package folder and the environment that moves the include path.
func PreKey(cfg Config, src *Source) string {
	h := sha256.New()
	w := func(parts ...string) {
		for _, p := range parts {
			io.WriteString(h, p)
			h.Write([]byte{0})
		}
	}
	// cfg.Dir is part of the key on purpose: quoted includes resolve against it,
	// and clang's messages and a header's own relative includes depend on it.
	// MaxDecl and MaxJSON are part of it too: a table made with a small limit
	// has probes that failed for the limit (TooLarge), which a larger limit would not.
	w("gocpp-probe-v2", src.Text, cfg.CXX.Path, strings.Join(cfg.CXX.Args, "\x01"), cfg.CXX.Version, cfg.Dir, strings.Join(cfg.Flags, "\x01"),
		strconv.Itoa(effMaxDecl(cfg)), strconv.FormatInt(effMaxJSON(cfg), 10))
	if id, ok := cfg.CXX.Identity(); ok {
		w(id)
	}
	// The environment that moves the include path or adds flags: the same list as the
	// runtime's object files and the kind probe (build.KeyEnv), not a copy of it.
	w(build.KeyEnv()...)
	return hex.EncodeToString(h.Sum(nil))
}

// Key returns the cache key of a table: PreKey and the contents of every
// header that the compiler read (Table.Deps), so that an edit of
// ./engine.hpp, or of a system header, makes another key.
func Key(cfg Config, src *Source, deps []string) string {
	h := sha256.New()
	io.WriteString(h, PreKey(cfg, src))
	io.WriteString(h, DepsHash(deps))
	return hex.EncodeToString(h.Sum(nil))
}

// A fileHash remembers the hash of a file by its stat, to avoid reading the
// system headers on every call. A file whose modification time is within two
// seconds of the check is always read again (it can change in the same tick).
type fileHash struct {
	size, mtime int64
	hash        string
}

var (
	hashMu    sync.Mutex
	hashCache = make(map[string]fileHash)
)

// DepsHash returns a hash of the contents of the files (a missing file has
// a hash of its own: its appearance changes the result).
func DepsHash(deps []string) string {
	sorted := slices.Clone(deps)
	sort.Strings(sorted)
	h := sha256.New()
	for _, d := range sorted {
		io.WriteString(h, d)
		h.Write([]byte{0})
		io.WriteString(h, hashFile(d))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashFile(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "missing"
	}
	fresh := time.Since(st.ModTime()) > 2*time.Second
	hashMu.Lock()
	c, ok := hashCache[path]
	hashMu.Unlock()
	if ok && fresh && c.size == st.Size() && c.mtime == st.ModTime().UnixNano() {
		return c.hash
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	sum := sha256.Sum256(data)
	hs := hex.EncodeToString(sum[:8])
	// The racy-git rule: a file that was modified less than 2 s before it was
	// read can change again in the same clock tick without its size or modification
	// time changing, so its hash is not kept (and the file is read again next time).
	if fresh {
		hashMu.Lock()
		hashCache[path] = fileHash{st.Size(), st.ModTime().UnixNano(), hs}
		hashMu.Unlock()
	}
	return hs
}

// ---------------------------------------------------------------- the run

// limitWriter keeps the first max bytes written to it and counts the rest.
type limitWriter struct {
	buf       bytes.Buffer
	max       int
	truncated bool
	total     int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	w.total += len(p)
	if room := w.max - w.buf.Len(); room > 0 {
		if len(p) > room {
			w.buf.Write(p[:room])
			w.truncated = true
		} else {
			w.buf.Write(p)
		}
	} else {
		w.truncated = true
	}
	return len(p), nil
}

// countReader counts the bytes read and cancels the run beyond a limit.
type countReader struct {
	r      io.Reader
	n      int64
	max    int64
	cancel context.CancelFunc
	over   bool
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.max && !c.over {
		c.over = true
		c.cancel()
	}
	return n, err
}

// found is what the dump has for one probe declaration.
type found struct {
	typ     string // the canonical type (desugared, else as written)
	wrapped bool   // the type was the helper's wrapper, which the probe line puts around it
	value   *big.Int
	str     *string
	tooBig  bool
	written string
}

type probeName struct {
	id   int
	role Role
}

var probeNameRx = regexp.MustCompile(`^gp_probe_(\d+)(_alt|_v|_s)?$`)

func parseProbeName(name string) (probeName, bool) {
	m := probeNameRx.FindStringSubmatch(name)
	if m == nil {
		return probeName{}, false
	}
	id, err := strconv.Atoi(m[1])
	if err != nil {
		return probeName{}, false
	}
	role := RoleMain
	switch m[2] {
	case "_alt":
		role = RoleAlt
	case "_v":
		role = RoleValue
	case "_s":
		role = RoleString
	}
	return probeName{id, role}, true
}

func runProbe(ctx context.Context, cfg Config, src *Source) (*Table, error) {
	start := time.Now()
	timeout := effTimeout(cfg)
	maxJSON := effMaxJSON(cfg)
	tmp, err := os.MkdirTemp("", "gocpp-typed-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	tmp, err = filepath.Abs(tmp)
	if err != nil {
		return nil, err
	}
	srcPath := filepath.Join(tmp, probeFileName)
	depPath := filepath.Join(tmp, "gp_probe.d")
	if err := os.WriteFile(srcPath, []byte(src.Text), 0o644); err != nil {
		return nil, err
	}
	mode := []string{"-fsyntax-only", "-Xclang", "-ast-dump=json", "-Xclang", "-ast-dump-filter=" + probeDeclPrefix}
	args, err := interop.ProbeCompilerArgs(cfg.CXX, mode, cfg.Dir, cfg.Flags, depPath, srcPath)
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stderr := &limitWriter{max: maxStderr}
	var cmd *exec.Cmd
	var stdout io.ReadCloser
	cmd, err = build.Start(rctx, func() *exec.Cmd {
		c := cfg.CXX.Command(rctx, args...)
		c.Dir = cfg.Dir
		c.Env = interop.ProbeEnv(tmp, cfg.Dir)
		c.Stderr = stderr
		c.WaitDelay = probeWaitDelay
		build.InGroup(c)
		stdout, _ = c.StdoutPipe()
		return c
	})
	if err != nil {
		return nil, fmt.Errorf("%s didn't start: %w", cfg.CXX.Name(), err)
	}
	// Whatever happens below, the process group dies with the run.
	defer build.KillGroup(cmd)

	t := &Table{Source: src, Results: make([]Result, src.NRefs), Compiler: compilerID(cfg.CXX)}
	founds := make(map[probeName]*found)
	cr := &countReader{r: stdout, max: maxJSON, cancel: cancel}
	var parse time.Duration
	n, scanErr := ScanDecls(cr, cfg.MaxDecl, func(d RawDecl) error {
		t.Stats.Decls++
		if d.Size > t.Stats.LargestDecl {
			t.Stats.LargestDecl = d.Size
		}
		pn, ok := parseProbeName(d.Name)
		if !ok {
			return nil
		}
		if d.TooLarge {
			t.Stats.TooLarge++
			founds[pn] = &found{tooBig: true}
			return nil
		}
		p0 := time.Now()
		f, err := decodeDecl(d.Raw)
		parse += time.Since(p0)
		if err != nil {
			return fmt.Errorf("%s: %w", d.Name, err)
		}
		founds[pn] = f
		return nil
	})
	_ = n
	t.Stats.JSONBytes = cr.n
	abort := scanErr != nil && !cr.over
	// Drain what is left (the compiler must be able to exit). A reader error does not
	// kill the compiler yet: if it crashed in the middle of the dump (a truncated
	// object), the signal and its messages are the report, not the JSON error. The
	// run's timeout and the size limit bound the wait.
	io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	build.KillGroup(cmd)
	switch {
	case cr.over:
		return nil, fmt.Errorf("the AST dump of the probes is larger than %d bytes: probing stopped", maxJSON)
	case ctx.Err() != nil:
		return nil, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), ctx.Err())
	case rctx.Err() != nil:
		return nil, fmt.Errorf("%s was stopped after %v (probe timeout)", cfg.CXX.Name(), timeout)
	case cmd.ProcessState == nil:
		return nil, fmt.Errorf("%s didn't run: %w", cfg.CXX.Name(), waitErr)
	case !cmd.ProcessState.Exited():
		return nil, fmt.Errorf("%s: %v%s", cfg.CXX.Name(), cmd.ProcessState, excerpt(stderr.buf.Bytes()))
	case abort:
		return nil, fmt.Errorf("reading the AST dump: %w%s", scanErr, excerpt(stderr.buf.Bytes()))
	}
	t.Stats.ExitCode = cmd.ProcessState.ExitCode()
	t.Stats.StderrBytes = stderr.total
	t.Stats.StderrTruncated = stderr.truncated
	t.Stderr = stderr.buf.String()
	if t.Stats.ExitCode > 1 && t.Stats.Decls == 0 {
		return nil, fmt.Errorf("%s failed (exit %d)%s", cfg.CXX.Name(), t.Stats.ExitCode, excerpt(stderr.buf.Bytes()))
	}
	if data, err := os.ReadFile(depPath); err == nil {
		t.Deps = interop.ParseDeps(data, srcPath, cfg.Dir)
		slices.Sort(t.Deps)
	}
	if err := assemble(t, src, founds, ParseDiags(stderr.buf.Bytes(), srcPath)); err != nil {
		return nil, err
	}
	t.Deps = addShadows(t.Deps, cfg, src)
	t.Key = Key(cfg, src, t.Deps)
	t.Timing.Parse = parse
	t.Timing.CPU = cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
	t.Timing.Compile = time.Since(start)
	t.Timing.Total = t.Timing.Compile
	return t, nil
}

// addShadows adds to the headers the compiler read the places where a header
// that the probe file includes would hide the one that was found: the same
// name in the package folder (a quoted include), in the -iquote folders and
// in the -I folders that come before it. A file that is not there has a hash
// of its own, so its appearance changes the key. The limit is the one of the
// kind probe's cache: the top-level includes only, not those of the headers.
func addShadows(deps []string, cfg Config, src *Source) []string {
	var iquote, idirs []string
	for i := 0; i < len(cfg.Flags); i++ {
		f := cfg.Flags[i]
		var dst *[]string
		var val string
		switch {
		case f == "-I" || f == "-iquote":
			if i+1 < len(cfg.Flags) {
				val = cfg.Flags[i+1]
				i++
			}
			dst = &idirs
			if f == "-iquote" {
				dst = &iquote
			}
		case strings.HasPrefix(f, "-I") && len(f) > 2:
			dst, val = &idirs, f[2:]
		}
		if dst != nil && val != "" {
			if !filepath.IsAbs(val) {
				val = filepath.Join(cfg.Dir, val)
			}
			*dst = append(*dst, val)
		}
	}
	seen := make(map[string]bool, len(deps))
	for _, d := range deps {
		seen[d] = true
	}
	add := func(p string) {
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			deps = append(deps, p)
		}
	}
	for _, inc := range src.Includes {
		if len(inc) < 3 {
			continue
		}
		name := inc[1 : len(inc)-1]
		switch inc[0] {
		case '"':
			if filepath.IsAbs(name) {
				continue
			}
			if cfg.Dir != "" {
				add(filepath.Join(cfg.Dir, name))
			}
			for _, d := range iquote {
				add(filepath.Join(d, name))
			}
			fallthrough
		case '<':
			if filepath.IsAbs(name) {
				continue
			}
			for _, d := range idirs {
				add(filepath.Join(d, name))
			}
		}
	}
	slices.Sort(deps)
	return deps
}

func excerpt(out []byte) string {
	s := strings.TrimSpace(string(out))
	if s == "" {
		return ""
	}
	if len(s) > 600 {
		s = s[:600] + "..."
	}
	return ": " + s
}

// ---------------------------------------------------------------- the dump

// A node is the part of a node of clang's JSON dump that the reader uses.
type node struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Type *struct {
		QualType          string `json:"qualType"`
		DesugaredQualType string `json:"desugaredQualType"`
	} `json:"type"`
	Value json.RawMessage `json:"value"`
	Inner []node          `json:"inner"`
}

var gpCRx = regexp.MustCompile(`^gocpp_probe_internal_c<(.*)>$`)

// decodeDecl reads one probe declaration.
func decodeDecl(raw []byte) (*found, error) {
	var n node
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, err
	}
	f := &found{}
	if n.Type != nil {
		t := n.Type.DesugaredQualType
		if t == "" {
			t = n.Type.QualType
		}
		f.written = normalizeType(n.Type.QualType)
		if m := gpCRx.FindStringSubmatch(t); m != nil {
			f.typ = normalizeType(strings.TrimSpace(m[1]))
			f.wrapped = true
		} else {
			f.typ = normalizeType(t)
		}
	}
	// A string constant: the VarDecl gp_probe_N_s = str_of((E)) of type const char *,
	// where E is the string literal itself. A string literal deeper in the tree (the
	// "" of __builtin_nanf("") in NAN) is not the value.
	if n.Type != nil && strings.HasPrefix(n.Kind, "VarDecl") && isCharPtr(n.Type.QualType) {
		if lit := directLiteral(&n); lit != nil {
			if s, err := unquoteC(lit.Value); err == nil {
				f.str = &s
			}
		}
	}
	// A constant: the value of the ConstantExpr of the template argument.
	var visit func(n *node)
	visit = func(n *node) {
		if n.Kind == "ConstantExpr" && f.value == nil && len(n.Value) > 0 {
			var s string
			if json.Unmarshal(n.Value, &s) == nil {
				if v, ok := new(big.Int).SetString(s, 10); ok {
					f.value = v
				}
			}
		}
		for i := range n.Inner {
			visit(&n.Inner[i])
		}
	}
	visit(&n)
	return f, nil
}

func isCharPtr(t string) bool {
	t = strings.TrimSuffix(strings.TrimSpace(t), "const")
	return strings.TrimSpace(t) == "const char *"
}

// directLiteral returns the string literal that is the argument of the call
// that initializes the variable v (str_of((E)) with E a literal), looking
// through parentheses, casts and cleanups, or nil.
func directLiteral(v *node) *node {
	skip := func(n *node) *node {
		for n != nil && len(n.Inner) == 1 && (n.Kind == "ParenExpr" || n.Kind == "ImplicitCastExpr" ||
			n.Kind == "ExprWithCleanups" || n.Kind == "ConstantExpr" || n.Kind == "MaterializeTemporaryExpr") {
			n = &n.Inner[0]
		}
		return n
	}
	if len(v.Inner) != 1 {
		return nil
	}
	c := skip(&v.Inner[0])
	if c.Kind == "StringLiteral" { // an initializer that is the literal itself
		return c
	}
	if c.Kind != "CallExpr" || len(c.Inner) != 2 {
		return nil
	}
	if a := skip(&c.Inner[1]); a.Kind == "StringLiteral" {
		return a
	}
	return nil
}

// atRx matches the place that clang prints for a type without a name:
// "(unnamed struct at /abs/dir/edge.hpp:25:3)", "(lambda at /abs/x.cpp:3:9)".
var atRx = regexp.MustCompile(`\((unnamed (?:struct|union|enum)|anonymous (?:struct|union|enum)|lambda) at [^()]*\)`)

// normalizeType drops the file position of an unnamed type, so that the same
// header gives the same text from any folder: "(unnamed struct)".
func normalizeType(s string) string {
	if !strings.Contains(s, " at ") {
		return s
	}
	return atRx.ReplaceAllString(s, "($1)")
}

// unquoteC reads the value of a StringLiteral node: clang prints it as C
// text, "\"My\\nApp\"" with C escapes.
func unquoteC(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("not a string literal: %s", s)
	}
	s = s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch e := s[i]; e {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte(7)
		case 'b':
			b.WriteByte(8)
		case 'f':
			b.WriteByte(12)
		case 'v':
			b.WriteByte(11)
		case '"', '\\', '\'', '?':
			b.WriteByte(e)
		case 'x':
			j := i + 1
			for j < len(s) && isHex(s[j]) {
				j++
			}
			v, _ := strconv.ParseUint(s[i+1:j], 16, 8)
			b.WriteByte(byte(v))
			i = j - 1
		case '0', '1', '2', '3', '4', '5', '6', '7':
			j := i
			for j < len(s) && j < i+3 && s[j] >= '0' && s[j] <= '7' {
				j++
			}
			v, _ := strconv.ParseUint(s[i:j], 8, 16)
			b.WriteByte(byte(v))
			i = j - 1
		default:
			b.WriteByte('\\')
			b.WriteByte(e)
		}
	}
	return b.String(), nil
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// ---------------------------------------------------------------- diagnostics

var (
	diagRx   = regexp.MustCompile(`^(.+?):(\d+):(\d+): (fatal error|error|warning|note): (.*)$`)
	driverRx = regexp.MustCompile(`^(clang[^:\s]*): (fatal error|error): (.*)$`)
)

// A DiagGroup is an error with the notes that follow it.
type DiagGroup struct {
	Err   Diag
	Notes []Diag
}

// ParseDiags returns the errors of the compiler output with their notes. A
// warning, an include stack line ("In file included from ...") and the rest
// are dropped. probe is the probe file's path as the compiler got it (kept
// for the caller, who attributes the groups).
func ParseDiags(out []byte, probe string) []DiagGroup {
	var groups []DiagGroup
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if m := diagRx.FindStringSubmatch(line); m != nil {
			l, _ := strconv.Atoi(m[2])
			c, _ := strconv.Atoi(m[3])
			d := Diag{File: m[1], Line: l, Col: c, Severity: m[4], Msg: normalizeType(m[5])}
			if isProbeFile(d.File, probe) {
				d.File = probeFileName // not the temporary folder: the same text on every run
			}
			switch d.Severity {
			case "error", "fatal error":
				groups = append(groups, DiagGroup{Err: d})
			case "note":
				if len(groups) > 0 {
					g := &groups[len(groups)-1]
					g.Notes = append(g.Notes, d)
				}
			}
			continue
		}
		if m := driverRx.FindStringSubmatch(line); m != nil {
			groups = append(groups, DiagGroup{Err: Diag{Severity: m[2], Msg: normalizeType(m[3])}})
		}
	}
	return groups
}

func isProbeFile(file, probe string) bool {
	return file == probe || filepath.Base(file) == probeFileName
}

// attribution is what the compiler said about the lines of one reference.
type attribution struct {
	main, alt, value *DiagGroup
}

// assemble fills the table from the dump and the diagnostics.
func assemble(t *Table, src *Source, founds map[probeName]*found, groups []DiagGroup) error {
	attr := make(map[int]*attribution)
	get := func(id int) *attribution {
		a := attr[id]
		if a == nil {
			a = &attribution{}
			attr[id] = a
		}
		return a
	}
	for gi := range groups {
		g := &groups[gi]
		// The probe line is the error's own, else the line of a note in the probe
		// file ("in instantiation of ... requested here": an error in a header or in
		// a helper that a probe line caused).
		var l Line
		ok := false
		cands := []Diag{g.Err}
		cands = append(cands, g.Notes...)
		for _, d := range cands {
			if isProbeFile(d.File, "") {
				if l, ok = src.Lines[d.Line]; ok {
					break
				}
			}
		}
		if !ok {
			if isProbeFile(g.Err.File, "") && g.Err.Line >= src.HelperFrom && g.Err.Line <= src.HelperTo {
				return fmt.Errorf("%s:%d: %s: a header of the package collides with a name of the probe helpers (gocpp_probe_internal_*)",
					probeFileName, g.Err.Line, g.Err.Msg)
			}
			t.HeaderErrors = append(t.HeaderErrors, g.Err)
			continue
		}
		a := get(l.Ref)
		switch l.Role {
		case RoleMain, RoleAlias:
			if a.main == nil {
				a.main = g
			}
		case RoleAlt:
			if a.alt == nil {
				a.alt = g
			}
		case RoleValue, RoleString:
			// The value probes of a reference that is not a number are expected
			// to fail (a string has no value, a number has no string): keep the
			// error of the value probe.
			if l.Role == RoleValue && a.value == nil {
				a.value = g
			}
		}
	}
	forms := make(map[probeName]Form, len(src.Lines))
	for _, l := range src.Lines {
		if l.Role == RoleMain || l.Role == RoleAlt {
			forms[probeName{l.Ref, l.Role}] = l.Form
		}
	}
	for id := range t.Results {
		r := &t.Results[id]
		r.ID = id
		r.Key = src.Keys[id]
		if why, ok := src.Skipped[id]; ok {
			r.Skipped = why
			continue
		}
		main := founds[probeName{id, RoleMain}]
		alt := founds[probeName{id, RoleAlt}]
		formOf := func(role Role) Form {
			if f, ok := forms[probeName{id, role}]; ok {
				return f
			}
			return FormExpr
		}
		switch {
		case main != nil && !main.tooBig && main.wrapped:
			r.OK, r.Form = true, formOf(RoleMain)
			r.splitType(main.typ, r.Form == FormExpr)
		case alt != nil && !alt.tooBig && alt.wrapped:
			r.OK, r.Alt, r.Form = true, true, formOf(RoleAlt)
			r.splitType(alt.typ, r.Form == FormExpr)
		case main != nil && main.tooBig:
			r.Error = "the probe's AST is larger than the limit"
		case main != nil && !main.wrapped, alt != nil && !alt.wrapped:
			r.Error = "the probe's type is not the helper's wrapper: a header of the package declares a name that collides with the probe helpers"
		default:
			setError(r, attr[id], src)
		}
		if r.OK {
			if a := attr[id]; a != nil && a.alt != nil && !r.Alt && alt == nil {
				r.AltError = a.alt.Err.Msg
			}
			setValue(r, founds[probeName{id, RoleValue}], founds[probeName{id, RoleString}], attr[id])
		}
	}
	slices.SortFunc(t.HeaderErrors, func(a, b Diag) int { return strings.Compare(a.File+a.Msg, b.File+b.Msg) })
	return nil
}

// setError fills the error of a reference whose probes all failed.
func setError(r *Result, a *attribution, src *Source) {
	if a != nil && a.main != nil {
		g := a.main
		r.Error = g.Err.Msg
		e := g.Err
		r.ErrorAt = &e
		r.Notes = g.Notes
		if strings.Contains(r.Error, "gp_decl_") {
			r.Error = "depends on a local variable whose C/C++ type could not be probed"
		}
		if a.alt != nil {
			r.AltError = a.alt.Err.Msg
		}
		return
	}
	if a != nil && a.alt != nil {
		r.Error = a.alt.Err.Msg
		e := a.alt.Err
		r.ErrorAt = &e
		r.Notes = a.alt.Notes
		return
	}
	r.Error = "no result for the probe (the compiler printed no error for it)"
}

// setValue reads the constant of a reference from its value probes.
func setValue(r *Result, v, s *found, a *attribution) {
	// A string literal wins: the integer probe of a string fails by design.
	if s != nil && s.str != nil && !s.tooBig {
		r.Value = &Const{Kind: "string", Str: *s.str}
		return
	}
	if v == nil || v.value == nil || v.tooBig {
		if a != nil && a.value != nil {
			r.ValueError = a.value.Err.Msg
			// The helper calls a function that is not constexpr when the value does not fit.
			for _, n := range a.value.Notes {
				if strings.Contains(n.Msg, "not_exact") {
					b := strings.TrimSpace(strings.TrimPrefix(r.Base, "const "))
					r.ValueError = "the value of a " + b + " is not exactly representable (float64 or signed 128-bit integer)"
					break
				}
			}
		}
		return
	}
	switch {
	case isFloatType(r.Base):
		bits := v.value.Uint64()
		r.Value = &Const{Kind: "float", Float: math.Float64frombits(bits)}
	case r.Base == "bool" || r.Base == "const bool":
		r.Value = &Const{Kind: "bool", Int: v.value}
	default:
		r.Value = &Const{Kind: "int", Int: v.value}
	}
}

func isFloatType(t string) bool {
	t = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "const "), " const"))
	return t == "float" || t == "double" || t == "long double"
}
