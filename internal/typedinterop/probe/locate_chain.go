package probe

import (
	"context"
	"errors"
	"fmt"

	"github.com/huuhait/gocpp/internal/build"
)

// The declarations of one function or variable (step 12.13e). A lookup of a name gives the latest
// declaration of each entity, and the dump of a redeclaration does not say whether another one in
// its chain has the body: `int f(int a) { ... }` followed by `int f(int a);` is a definition that
// the lookup shows as a declaration. The filtered AST dump prints every declaration of the name, each with its
// own body, so one run of it finds the definition. It is asked only for the names whose latest
// declaration is such a redeclaration, and the runner caches it like the others.

// LocateChain returns every declaration that the compiler has of the function or variable
// scope::name after the includes, in the order of the dump, each with its Defined flag. Like Locate
// it runs the compiler and must run off the request path.
func (rn *Runner) LocateChain(ctx context.Context, cfg Config, includes []string,
	scope, name string,
) ([]Declaration, error) {
	if cfg.CXX.Family != build.Clang {
		return nil, ErrNoClang
	}
	if !validLocateName(LocateName{Scope: scope, Name: name}) {
		return nil, errors.New("not a name that can be looked up")
	}
	text, err := tagSource(includes)
	if err != nil {
		return nil, err
	}
	qualified := name
	if scope != "" {
		qualified = scope + "::" + name
	}
	key := "c\x00" + qualified
	produce := func(set *sourceSet, answers map[string]roundAnswer) ([]*auxOut, error) {
		reader, out, err := rn.filteredDump(ctx, cfg, text, set, qualified)
		if err != nil {
			return nil, err
		}
		answer := roundAnswer{Failed: true}
		r := &renderer{set: set}
		var decls []Declaration
		complete := true
		for _, group := range reader.groups {
			if group.name != qualified {
				continue
			}
			t := newTree(group.nodes)
			if group.droppedTop || len(t.tops) != 1 {
				complete = false
				continue
			}
			decl, ok := r.render(t, t.tops[0], scope, group.partial)
			if !ok || t.nodes[t.tops[0]].hdr.name != name {
				complete = false
				continue
			}
			decls = append(decls, decl)
		}
		if complete && len(decls) > 0 {
			answer = roundAnswer{Decls: decls}
		}
		answers[key] = answer
		return []*auxOut{out}, nil
	}
	version := fmt.Sprintf("%s-chain/%s", locateKeyVersion, qualified)
	answers, _, err := rn.cachedRound(ctx, cfg, includes, version, text, produce)
	if err != nil {
		return nil, err
	}
	answer := answers[key]
	if answer.Failed {
		return nil, errors.New("the declarations were not found exactly")
	}
	return answer.Decls, nil
}
