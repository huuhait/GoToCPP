package main

import (
	"context"
	"runtime"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// membersHook is check.Config.CppMembers for typed interop (Phase 12 step 12.9, ADR-098): the
// second pass for promoted C++ members asks the C++ compiler which embedded class has which
// member name. A build for another system than this one is asked only for the headers of the C++
// standard library (their classes are the same on both; other headers are the host's, not the
// target's), as attachTyped does. Without clang++ the hook returns ErrNoClang and check goes on
// without the second pass: opaque mode, which is always correct.
func (c *checkCommand) membersHook() check.CppMembers {
	rn := new(probe.Runner)
	return func(ctx context.Context, p *check.Package, qs []check.MemberQuery) ([]check.MemberAnswer, error) {
		if cross := c.goos != runtime.GOOS || c.goarch != runtime.GOARCH; cross && !stdHeadersOnly(p) {
			return nil, nil
		}
		return typedinterop.AnswerMembers(ctx, p, qs, typedinterop.Options{Runner: rn})
	}
}
