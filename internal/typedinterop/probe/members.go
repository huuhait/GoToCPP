package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/winpath"
)

// ErrMemberReceiverUnsupported means the receiver cannot be represented by this bounded query.
var ErrMemberReceiverUnsupported = errors.New("unsupported member receiver")

// MemberQuery describes an expression, not a declaration context. Type has no outer reference;
// Category supplies its value category. One explicit pointer uses ->; a named object uses dot.
// Non-type template arguments are limited to integer and boolean literals. Dependent types,
// arrays, function types, smart-pointer arrow and expression/template argument evaluation are
// deliberately unsupported. Callers must run Members off the request path.
type MemberQuery struct {
	Type     string
	Category Category
}

// MemberNames holds compiler-validated unqualified member names, not signatures or locations.
// A partial answer contains only proven names; Incomplete distinguishes it from complete-empty.
// Completeness covers the supported completion candidate domain, not every C++ expression.
type MemberNames struct {
	names      []string
	incomplete bool
}

// Incomplete reports omitted groups with unsupported argument spellings, such as named or
// dependent parameter types. A group is retained when any overload is proven usable. This
// does not promise viability for a caller's argument list or universal C++ semantic coverage.
func (m *MemberNames) Incomplete() bool {
	return m != nil && m.incomplete
}

// Names returns a copy; it does not run a compiler or revalidate the cache.
func (m *MemberNames) Names() []string {
	if m == nil {
		return nil
	}
	return slices.Clone(m.names)
}

const maxMemberOutput = 1 << 20
const maxMemberNames = 4096

var memberLiteral = regexp.MustCompile(`^(true|false|-?[0-9]+)$`)

type memberFile struct {
	Names      []string
	Incomplete bool
	Targets    map[string]string
}

// Members asks clang about one exact receiver. Completion discovers candidate names, then
// bounded requires-expressions prove unqualified lookup, access and availability without
// guessing from direct declaration lists. Unmodelled overload groups are marked Incomplete.
// It shares the Runner's bounded serialized store, variants, compiler identity and flights.
func (rn *Runner) Members(ctx context.Context, cfg Config, includes []string,
	query MemberQuery,
) (*MemberNames, error) {
	text, prefix, err := memberSource(includes, query)
	if err != nil {
		return nil, err
	}
	if cfg.CXX.Family != build.Clang {
		return nil, ErrNoClang
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, write := rn.diskFor(cfg)
	tc := rn.toolchain(ctx, cfg, dir, write)
	pre := withToolchain(PreKey(cfg, &Source{Text: "member-names-v2\n" + text}), tc)
	name := entryName("m", selfID(), pre)
	lookup := func() *MemberNames {
		v := rn.findVariant(dir, name, "m")
		if v == nil {
			return nil
		}
		var file memberFile
		if json.Unmarshal(v.Part, &file) != nil || file.Names == nil ||
			file.Targets == nil || !sameIndexTargets(file.Targets) {
			return nil
		}
		return &MemberNames{names: file.Names, incomplete: file.Incomplete}
	}
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if found := lookup(); found != nil {
			return found, nil
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
		pending := &flight{done: make(chan struct{})}
		if rn.flying == nil {
			rn.flying = make(map[string]*flight)
		}
		rn.flying[name] = pending
		rn.mu.Unlock()

		result, err := func() (*MemberNames, error) {
			if dir != "" {
				release, _ := acquireLock(ctx, dir, name, effTimeout(cfg))
				if release != nil {
					defer release()
				}
			}
			if found := lookup(); found != nil {
				return found, nil
			}
			return rn.makeMembers(ctx, cfg, includes, text, prefix, dir, name, write, tc)
		}()
		rn.mu.Lock()
		delete(rn.flying, name)
		pending.err = err
		close(pending.done)
		rn.mu.Unlock()
		return result, err
	}
	return nil, errors.New("the member query kept being taken by other callers")
}

func memberSource(includes []string, query MemberQuery) (string, string, error) {
	unsupported := func() (string, string, error) {
		return "", "", ErrMemberReceiverUnsupported
	}
	if len(query.Type) == 0 || len(query.Type) > 4096 || unsafeName(query.Type) != "" {
		return unsupported()
	}
	typ, err := ctype.Parse(query.Type)
	if err != nil || !memberTypeSafe(typ, 0) {
		return unsupported()
	}
	object := typ
	op := "."
	if object.Kind == ctype.Pointer {
		object = object.Elem
		op = "->"
	}
	if object.Kind != ctype.Named {
		return unsupported()
	}
	ref := ""
	switch query.Category {
	case CatLValue:
		ref = "&"
	case CatXValue:
		ref = "&&"
	case CatPRValue:
	default:
		return unsupported()
	}
	var source strings.Builder
	for _, include := range includes {
		if err := interop.ValidateInclude(include); err != nil {
			return "", "", err
		}
		header, externC := interop.SplitExternC(include)
		if externC {
			source.WriteString("extern \"C\" {\n")
		}
		fmt.Fprintf(&source, "#include %s\n", winpath.IncludeSpelling(header))
		if externC {
			source.WriteString("}\n")
		}
		if source.Len() > 65536 {
			return unsupported()
		}
	}
	fmt.Fprintf(&source, "using gocpp_member_type = %s;\n", typ.String())
	fmt.Fprintf(&source, "gocpp_member_type%s gocpp_member_receiver();\n", ref)
	size := "gocpp_member_receiver()"
	if op == "->" {
		size = "*" + size
	}
	fmt.Fprintf(&source, "static_assert(sizeof(%s) > 0);\n", size)
	source.WriteString("void gocpp_member_query() {\n")
	prefix := source.String() + "gocpp_member_receiver()" + op
	// The complete version validates the whole header and receiver before the completion run,
	// which intentionally stops parsing at the cursor and may ignore later diagnostics.
	return source.String() + "}\n", prefix, nil
}

func memberTypeSafe(typ *ctype.Type, depth int) bool {
	if typ == nil || depth > 16 || typ.Pack || typ.Restrict || typ.Tag != "" {
		return false
	}
	switch typ.Kind {
	case ctype.Builtin:
		return true
	case ctype.Pointer:
		return memberTypeSafe(typ.Elem, depth+1)
	case ctype.Named:
		for _, segment := range typ.Path {
			if !identRx.MatchString(segment.Name) || strings.HasPrefix(segment.Name, "gocpp_") {
				return false
			}
			for _, arg := range segment.Args {
				if arg.Pack || arg.Type != nil && !memberTypeSafe(arg.Type, depth+1) ||
					arg.Type == nil && !memberLiteral.MatchString(arg.Expr) {
					return false
				}
			}
		}
		return len(typ.Path) > 0
	}
	return false
}

func (rn *Runner) makeMembers(ctx context.Context, cfg Config, includes []string,
	text, prefix, dir, name string, write bool, tc toolchain,
) (*MemberNames, error) {
	start := time.Now()
	// Bind exactly the inputs used by discovery, before either result-producing run. The same
	// guards as the header index distinguish required reads from missing shadow candidates.
	_, discovery, err := indexOutput(ctx, cfg, []string{"-E", "-o", os.DevNull}, text)
	if err != nil {
		return nil, err
	}
	if discovery.truncated {
		return nil, errors.New("truncated member dependency diagnostics")
	}
	read := interop.ParseDeps(discovery.depData, discovery.srcPath, cfg.Dir)
	slices.Sort(read)
	read = slices.Compact(read)
	metadata := new(Runner)
	deps := metadata.finalDeps(ctx, cfg, text, includes, read, nil, tc)
	info := metadata.includeInfo(ctx, cfg, text, read, false, "")
	if !info.complete || !info.successful || len(read) == 0 || racyDeps(deps, start) {
		return nil, errors.New("member query inputs could not be validated")
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
	_, checked, err := indexOutput(ctx, cfg, []string{"-fsyntax-only"}, text)
	if err != nil {
		return nil, err
	}
	if checked.truncated {
		return nil, errors.New("truncated member validation diagnostics")
	}
	output := &limitWriter{max: maxMemberOutput}
	completed, err := runAuxMode(ctx, cfg, func(path string) []string {
		line := strings.Count(prefix, "\n") + 1
		column := len(prefix) - strings.LastIndex(prefix, "\n")
		return []string{"-fsyntax-only", "-Xclang",
			fmt.Sprintf("-code-completion-at=%s:%d:%d", path, line, column)}
	}, prefix+"\n}\n", func(string) io.Writer { return output })
	if err != nil {
		return nil, err
	}
	if completed.timedOut {
		return nil, timeoutError("member completion")
	}
	if completed.exitErr != nil {
		return nil, fmt.Errorf("member completion failed: %w", completed.exitErr)
	}
	if completed.truncated || output.truncated {
		return nil, errors.New("truncated member completion output")
	}
	names, incomplete, validated, err := validateMemberNames(ctx, cfg, text, output.buf.String(), strings.HasSuffix(prefix, "->"))
	if err != nil {
		return nil, err
	}
	for _, result := range []*auxOut{checked, completed, validated} {
		if result == nil {
			continue
		}
		actual := interop.ParseDeps(result.depData, result.srcPath, cfg.Dir)
		slices.Sort(actual)
		if !slices.Equal(slices.Compact(actual), read) {
			return nil, errors.New("member query dependency resolution changed")
		}
	}
	if !required.unchanged() || !resolution.unchanged() || racyDeps(deps, start) ||
		DepsHash(deps) != depsHash {
		return nil, errors.New("member query inputs changed during production")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(memberFile{Names: names, Incomplete: incomplete, Targets: resolution.targets})
	if err != nil {
		return nil, err
	}
	rn.addVariant(dir, name, "m", "member names "+cfg.CXX.Name(),
		variant{Deps: deps, DepsHash: depsHash, Part: body}, write)
	return &MemberNames{names: names, incomplete: incomplete}, nil
}

type memberCandidate struct {
	name string
	expr string // empty means the completion's argument spelling is unsupported
}

func parseMemberNames(output string) ([]string, error) {
	names, _, err := memberCandidates(output)
	return names, err
}

func memberCandidates(output string) ([]string, []memberCandidate, error) {
	if !strings.HasSuffix(output, "\n") {
		return nil, nil, errors.New("missing or truncated member completion output")
	}
	names := make(map[string]bool)
	var candidates []memberCandidate
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		line, ok := strings.CutPrefix(line, "COMPLETION: ")
		name, detail, found := strings.Cut(line, " : ")
		if !ok || !found || name == "" || detail == "" {
			return nil, nil, errors.New("unrecognized member completion output")
		}
		name = strings.TrimSuffix(name, " (InBase)")
		if !identRx.MatchString(name) || strings.HasPrefix(name, "__") ||
			strings.HasPrefix(name, "gocpp_") || !strings.HasPrefix(detail, "[#") {
			continue // inaccessible annotations, operators, types and internal names
		}
		if len(name) > 256 || !names[name] && len(names) >= maxMemberNames ||
			len(candidates) >= 4*maxMemberNames {
			return nil, nil, errors.New("too many or oversized member candidates")
		}
		name = strings.Clone(name)
		names[name] = true
		candidates = append(candidates, memberCandidate{name, memberExpression(name, detail)})
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	slices.Sort(result)
	return result, candidates, nil
}

// A completion is only a candidate. Validate an unqualified expression with its actual printed
// argument types; never add a base qualifier to make an ambiguous member usable. Unknown type
// aliases, method-template arguments and more elaborate declarators stay explicitly incomplete.
func memberExpression(name, detail string) string {
	for strings.HasPrefix(detail, "[#") {
		_, rest, ok := strings.Cut(detail, "#]")
		if !ok {
			return ""
		}
		detail = rest
	}
	tail, ok := strings.CutPrefix(detail, name)
	if !ok {
		return ""
	}
	if tail == "" {
		return name
	}
	if !strings.HasPrefix(tail, "(") {
		return ""
	}
	tail = tail[1:]
	var args []string
	for !strings.HasPrefix(tail, ")") {
		if len(args) >= 32 || !strings.HasPrefix(tail, "<#") {
			return ""
		}
		argument, rest, ok := strings.Cut(tail[2:], "#>")
		if !ok {
			return ""
		}
		typ := memberArgument(argument)
		if typ == "" {
			return ""
		}
		args = append(args, "gocpp_member_value<"+typ+">()")
		tail = rest
		if strings.HasPrefix(tail, ", ") {
			tail = tail[2:]
		} else if !strings.HasPrefix(tail, ")") {
			return ""
		}
	}
	return name + "(" + strings.Join(args, ", ") + ")"
}

func memberArgument(text string) string {
	if unsafeName(text) != "" || len(text) > 4096 {
		return ""
	}
	typ, err := ctype.Parse(text)
	if err != nil {
		// Clang's placeholder may include a parameter name. Removing it is accepted only if
		// the remaining complete declarator parses; no textual type substitutions are made.
		end := len(text)
		for end > 0 && (isIndexIdentStart(text[end-1]) || text[end-1] >= '0' && text[end-1] <= '9') {
			end--
		}
		if end == len(text) || !identRx.MatchString(text[end:]) {
			return ""
		}
		typ, err = ctype.Parse(strings.TrimSpace(text[:end]))
		if err != nil {
			return ""
		}
	}
	leaf := typ
	for depth := 0; depth < 16; depth++ {
		if leaf == nil || leaf.Pack || leaf.Restrict {
			return ""
		}
		switch leaf.Kind {
		case ctype.Pointer, ctype.LRef, ctype.RRef:
			leaf = leaf.Elem
		case ctype.Builtin:
			if leaf.Name == "void" && leaf == typ {
				return ""
			}
			return typ.String()
		default:
			return "" // no invented meaning for unqualified aliases or dependent types
		}
	}
	return ""
}

func validateMemberNames(ctx context.Context, cfg Config, text, output string, indirect bool,
) ([]string, bool, *auxOut, error) {
	names, candidates, err := memberCandidates(output)
	if err != nil {
		return nil, false, nil, err
	}
	unknown := make(map[string]bool)
	var checks []memberCandidate
	var source strings.Builder
	source.WriteString(text)
	source.WriteString("template<class T> T gocpp_member_value();\n")
	source.WriteString("using gocpp_member_expression = decltype(gocpp_member_receiver());\n")
	lines := make(map[int]int)
	line := strings.Count(source.String(), "\n") + 1
	op := "."
	if indirect {
		op = "->"
	}
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, false, nil, err
		}
		if candidate.expr == "" {
			unknown[candidate.name] = true
			continue
		}
		id := len(checks)
		fmt.Fprintf(&source, "template<class T> constexpr bool gocpp_member_valid_%d = requires { gocpp_member_value<T>()%s%s; };\n",
			id, op, candidate.expr)
		lines[line+1] = id
		line += 2
		fmt.Fprintf(&source, "static_assert(gocpp_member_valid_%d<gocpp_member_expression>, \"gocpp_member_invalid\");\n", id)
		checks = append(checks, candidate)
		if source.Len() > maxMemberOutput {
			return nil, false, nil, errors.New("member validation exceeds its source budget")
		}
	}
	valid := make(map[string]bool)
	var out *auxOut
	if len(checks) > 0 {
		sentinel := line
		source.WriteString("static_assert(false, \"gocpp_member_validation_complete\");\n")
		out, err = runAux(ctx, cfg, []string{"-fsyntax-only", "-ferror-limit=0"}, source.String())
		if err != nil {
			return nil, false, nil, err
		}
		var status *exec.ExitError
		if out.timedOut {
			return nil, false, nil, timeoutError("member validation")
		}
		if out.truncated || !errors.As(out.exitErr, &status) || status.ExitCode() != 1 {
			return nil, false, nil, errors.New("incomplete member validation")
		}
		invalid := make(map[int]bool)
		complete := false
		for _, group := range ParseDiags(out.stderr, out.srcPath) {
			diag := group.Err
			if diag.Line == sentinel && isProbeFile(diag.File, out.srcPath) &&
				strings.Contains(diag.Msg, "gocpp_member_validation_complete") {
				complete = true
				continue
			}
			id, ok := lines[diag.Line]
			if !ok || !isProbeFile(diag.File, out.srcPath) ||
				!strings.Contains(diag.Msg, "gocpp_member_invalid") {
				return nil, false, nil, errors.New("unexpected member validation diagnostic")
			}
			invalid[id] = true
		}
		if !complete {
			return nil, false, nil, errors.New("member validation did not reach its sentinel")
		}
		for id, candidate := range checks {
			if !invalid[id] {
				valid[candidate.name] = true
			}
		}
	}
	result := make([]string, 0, len(valid))
	incomplete := false
	for _, name := range names {
		if valid[name] {
			result = append(result, name)
		} else if unknown[name] {
			incomplete = true
		}
	}
	return result, incomplete, out, nil
}
