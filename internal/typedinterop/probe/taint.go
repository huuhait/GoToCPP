package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/interop"
)

// Macros whose value depends on where they are used (step 12.8, review of ADR-097).
//
// A constant is probed in the probe file, which is not the program's file: `#define WHERE __LINE__`
// is the number of the probe's line, `__FILE__` is the path of a temporary file, `__COUNTER__` counts
// the probe's expansions and `__DATE__` is the time of this run (and the cache would keep it).
// None of those is the value in the program. VolatileMacros lists the macros that the headers of a
// probe file define whose body, directly or through other macros, names one of these builtins, so
// that the caller refuses them.

// volatileBuiltins are the predefined macros that change with the place or the time of the
// expansion. (__cplusplus, __VERSION__ and the like are the same in the probe and the build when the
// compiler is the same; when it is not, the guard of ADR-097 §7 catches the difference.)
var volatileBuiltins = []string{"__LINE__", "__FILE__", "__FILE_NAME__", "__COUNTER__", "__DATE__", "__TIME__", "__TIMESTAMP__", "__BASE_FILE__", "__INCLUDE_LEVEL__"}

var macroIdentRx = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// stripLiterals removes the string and character literals of a macro body, so that a name inside
// one ("__LINE__ is a macro") is not taken for a use.
func stripLiterals(body string) string {
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '"' && c != '\'' {
			b.WriteByte(c)
			continue
		}
		j := i + 1
		for j < len(body) && body[j] != c {
			if body[j] == '\\' {
				j++
			}
			j++
		}
		b.WriteByte(' ')
		i = j
	}
	return b.String()
}

// volatileIn reads the output of -E -dM (`#define NAME body` and `#define NAME(args) body` lines) and
// returns the macros that depend on the place or the time, each with the builtin it names.
func volatileIn(dM []byte) map[string]string {
	bodies := make(map[string][]string) // macro -> the names its body mentions
	for line := range bytes.SplitSeq(dM, []byte("\n")) {
		rest, ok := bytes.CutPrefix(line, []byte("#define "))
		if !ok {
			continue
		}
		s := string(rest)
		end := strings.IndexAny(s, " (")
		if end < 0 {
			end = len(s)
		}
		name, body := s[:end], ""
		if end < len(s) {
			body = s[end:]
			if body[0] == '(' { // a function-like macro: the body follows the parameter list
				if i := strings.IndexByte(body, ')'); i >= 0 {
					body = body[i+1:]
				}
			}
		}
		bodies[name] = macroIdentRx.FindAllString(stripLiterals(body), -1)
	}
	out := make(map[string]string)
	state := make(map[string]int) // 1: being visited, 2: done
	var visit func(name string) string
	visit = func(name string) string {
		if w, ok := out[name]; ok {
			return w
		}
		if state[name] != 0 {
			return ""
		}
		state[name] = 1
		defer func() { state[name] = 2 }()
		for _, id := range bodies[name] {
			if slices.Contains(volatileBuiltins, id) {
				out[name] = id
				return id
			}
			if _, isMacro := bodies[id]; isMacro {
				if w := visit(id); w != "" {
					out[name] = w
					return w
				}
			}
		}
		return ""
	}
	for name := range bodies {
		visit(name)
	}
	return out
}

// VolatileMacros returns the macros that src's headers define whose value depends on where or when
// they are expanded (see above), with the builtin that makes them so. The answer is cached like the
// probe results: under the key of the preamble and the contents of the headers it read.
func (rn *Runner) VolatileMacros(ctx context.Context, cfg Config, src *Source) (map[string]string, error) {
	dir, write := rn.diskFor(cfg)
	tc := rn.toolchain(ctx, cfg, dir, write)
	pre := withToolchain(PreKey(cfg, &Source{Text: src.Preamble}), tc)
	name := entryName("t", selfID(), pre)
	if v := rn.findVariant(dir, name, "t"); v != nil {
		var m map[string]string
		if json.Unmarshal(v.Part, &m) == nil {
			return m, nil
		}
	}
	start := time.Now()
	var dM bytes.Buffer
	out, err := runAuxTo(ctx, cfg, []string{"-E", "-dM"}, src.Preamble, func(string) io.Writer { return &dM })
	if err != nil {
		return nil, err
	}
	if len(out.depData) == 0 || out.timedOut || dM.Len() == 0 {
		return nil, nil // the run did not work: no answer, the caller treats every macro as unknown
	}
	m := volatileIn(dM.Bytes())
	deps := rn.finalDeps(ctx, cfg, src.Preamble, src.Includes, interop.ParseDeps(out.depData, out.srcPath, cfg.Dir), nil, tc)
	if racyDeps(deps, start) {
		return m, nil
	}
	body, err := json.Marshal(m)
	if err != nil {
		return m, nil
	}
	_, write = rn.diskFor(cfg)
	rn.addVariant(dir, name, "t", cfg.CXX.Name()+" volatile macros", variant{Deps: deps, DepsHash: DepsHash(deps), Part: body}, write && len(deps) > 0)
	return m, nil
}
