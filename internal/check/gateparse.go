package check

import (
	"go/token"
	"regexp"
	"slices"

	"github.com/huuhait/gocpp/internal/diag"
)

// The gates of a package that was not type-checked: its load errors are
// all there is. One of them gets the gate's words: the loader rejects every
// standard package without a stub with "package X is not available yet
// (Phase 09)"; for a package of stdPackages of another phase, the message
// names that phase (weak: Phase 03, plugin: never).
//
// (Until step 10.12 this file also turned the syntax errors of a file with
// `!` forms or macros into "must calls are not supported yet (Phase 10)":
// that syntax parses always now, ADR-095.)
//
// The errors keep the order they were found in (diag.List.Rewrite).

// gateUnchecked runs the gates of the unchecked user package p.
func gateUnchecked(_ *token.FileSet, p *Package, phase int) {
	rename := func(d *diag.Diagnostic) bool {
		m := notAvailable.FindStringSubmatch(d.Msg)
		if m == nil {
			return false
		}
		row := lookupStdPackage(m[1])
		if row == nil || row.Phase <= phase || row.message(m[1]) == d.Msg {
			// At its phase, a package without a stub is still missing:
			// the loader's message stays.
			return false
		}
		d.Msg = row.message(m[1])
		return true
	}
	if !slices.ContainsFunc(p.Errors.Diagnostics(), func(d diag.Diagnostic) bool { return rename(&d) }) {
		return
	}
	p.Errors = p.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
		rename(&d)
		return []diag.Diagnostic{d}
	})
}

// notAvailable matches the loader's error for a standard package without a
// stub.
var notAvailable = regexp.MustCompile(`^package (\S+) is not available yet \(Phase 09\)$`)
