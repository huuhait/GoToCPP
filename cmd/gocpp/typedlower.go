package main

import (
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/lower"
)

// attachTyped gives the typed report of pkg to the lowering (lower.Interop.SetTyped, Phase 12 step
// 12.6, ADR-087). The report is the one that check made when typed interop is on (analyzeTyped in
// interop.go: collector, probes, declarations; the interop mode decides whether it runs). A
// package of the standard library, opaque mode, a build without clang++, a probe that failed and a
// build for another system than this one with headers other than the C++ library's (their types
// are the host's, not the target's) have no report: the opaque lowering then runs, which is always
// correct.
func (d *driver) attachTyped(in *lower.Interop, pkg *check.Package) {
	if in == nil || pkg.Load.Std {
		return
	}
	rep := d.check.typed.report(pkg)
	if rep == nil {
		return
	}
	in.SetTyped(rep, !isHostTarget(d.check.goos, d.check.goarch))
}
