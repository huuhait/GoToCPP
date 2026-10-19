package typedinterop

import (
	"context"
	"go/token"
	"regexp"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The answers for check's second pass (step 12.9, ADR-098): which embedded C++ class has which
// member name.
//
// One probe per class and name: decltype((std::declval<C&>().name)), an access that is not a
// use. The compiler says what the name is by what it says about the access, which is the C++
// compiler's own lookup (bases, using-declarations, anonymous unions, the library's internal base
// classes that the declaration extraction does not follow):
//
//	compiles                                              a data member, a static member, an enumerator
//	"reference to non-static member function must be called"   a member function, any overloads or a template
//	"no member named 'n' in 'C'"                          none
//	"'n' is a private member of 'B'"                      there, not for its users (also from a private base)
//	"'n' is a protected member of 'B'"
//	"member 'n' found in multiple base classes ..."       ambiguous in C++ (different bases), or
//	"non-static member 'n' found in multiple base-class subobjects of type 'B'"   (a non-virtual diamond)
//	"attempt to use a deleted function"                   a function declared = delete
//	"cannot refer to type member 'n' in 'C' with '.'"     a nested type
//	any message with a note "expanded from macro 'n'"     the name is a macro
//
// Anything else is MemberUnknown: the hook says nothing and the selector stays as the first
// pass left it. A class that has an operator-> and no member of the name is MemberUnknown too
// (typed code generation writes x->name for it: ADR-087), which is why every class is probed for
// operator-> once. The answers are probe results, so the probe cache of step 12.7 (ADR-088)
// holds them: a warm build asks the compiler nothing.

// MembersHook returns the check.CppMembers hook of typed interop. opts has the compiler, the
// runner (the cache of the compiler runs; share it with the Analyze of the same build) and the
// limits; nothing else of it is used. When there is no clang the hook returns the error
// ErrNoClang, and check goes on without the second pass.
func MembersHook(opts Options) check.CppMembers {
	return func(ctx context.Context, p *check.Package, qs []check.MemberQuery) ([]check.MemberAnswer, error) {
		return AnswerMembers(ctx, p, qs, opts)
	}
}

// AnswerMembers probes the queries about the checked package p (the first pass's result) and
// returns one answer for each.
func AnswerMembers(ctx context.Context, p *check.Package, qs []check.MemberQuery, opts Options) ([]check.MemberAnswer, error) {
	out := make([]check.MemberAnswer, len(qs))
	if len(qs) == 0 {
		return out, nil
	}
	cxx := opts.Clang
	if cxx.Path == "" {
		var err error
		if cxx, err = probe.FindClang(ctx); err != nil {
			return nil, err
		}
	}
	includes := UnitIncludes(p)
	if len(includes) == 0 {
		return out, nil
	}
	set, member, arrow := collect.MemberSet(p, token.NewFileSet(), qs)
	if len(set.Refs) == 0 {
		return out, nil
	}
	kinds, incErrs, err := probeKinds(ctx, cxx, p.Load.Dir, p.ProbeFlags, includes, set)
	if err != nil {
		return nil, err
	}
	if len(incErrs) > 0 {
		return out, nil // a header that cannot be included: the compiler decides
	}
	rn := opts.Runner
	if rn == nil {
		rn = new(probe.Runner)
	}
	cfg := probe.Config{CXX: cxx, Dir: p.Load.Dir, Flags: p.ProbeFlags, Timeout: opts.Timeout, MaxDecl: opts.MaxDecl, MaxJSON: opts.MaxJSON}
	tab, err := rn.Probe(ctx, cfg, set, probe.GenConfig{Includes: includes, Kinds: kinds, StringToCStr: opts.StringToCStr})
	if err != nil {
		return nil, err
	}
	if len(tab.HeaderErrors) > 0 {
		return out, nil // the headers do not compile for the probe: nothing it says is certain
	}
	for i, q := range qs {
		if member[i] == nil {
			continue
		}
		res := tab.Result(member[i].ID)
		if res == nil {
			continue
		}
		a := ClassifyMember(res, q.Name)
		if a.Status == check.MemberMissing {
			if ar := tab.Result(arrow[i].ID); ar != nil && ClassifyMember(ar, "operator->").Status == check.MemberFound {
				a = check.MemberAnswer{Status: check.MemberUnknown, Why: "the class has an operator-> (the pointee may have the member): " + a.Why}
			}
		}
		out[i] = a
	}
	return out, nil
}

var (
	reNoMember  = regexp.MustCompile(`^no member named '([^']+)' in `)
	reMacroNote = regexp.MustCompile(`^expanded from macro '([^']+)'`)
)

// ClassifyMember turns the probe result of an access obj.name into the answer for check. name is the
// member name that was asked.
func ClassifyMember(r *probe.Result, name string) check.MemberAnswer {
	if r.Skipped != "" {
		return check.MemberAnswer{Why: "not probed: " + r.Skipped}
	}
	msg, notes := r.Error, r.Notes
	switch {
	case r.OK && r.Recovered == "":
		return check.MemberAnswer{Status: check.MemberFound, Why: "compiles"}
	case r.OK:
		// The compiler recovered from the error and kept a typed expression (an access to a private
		// member is one).
		msg, notes = r.Recovered, r.RecoveredNotes
	}
	if macroNamed(notes, name) {
		return check.MemberAnswer{Status: check.MemberMacro, Why: msg}
	}
	status := check.MemberUnknown
	switch {
	case strings.HasPrefix(msg, "reference to non-static member function must be called"):
		status = check.MemberFound
	case reNoMember.MatchString(msg):
		if m := reNoMember.FindStringSubmatch(msg); m[1] == name {
			status = check.MemberMissing
		}
	case strings.Contains(msg, "' is a private member of '"):
		status = check.MemberPrivate
	case strings.Contains(msg, "' is a protected member of '"):
		status = check.MemberProtected
	case strings.Contains(msg, "found in multiple base classes"), strings.Contains(msg, "found in multiple base-class subobjects"):
		status = check.MemberAmbiguous
	case strings.HasPrefix(msg, "attempt to use a deleted function"):
		status = check.MemberDeleted
	case strings.HasPrefix(msg, "cannot refer to type member"):
		status = check.MemberNotValue
	}
	return check.MemberAnswer{Status: status, Why: msg}
}

// macroNamed reports whether one of the notes says that the text of name was expanded from the macro
// of that name.
func macroNamed(notes []probe.Diag, name string) bool {
	for _, n := range notes {
		if m := reMacroNote.FindStringSubmatch(n.Msg); m != nil && m[1] == name {
			return true
		}
	}
	return false
}
