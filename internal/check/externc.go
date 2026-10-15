package check

import (
	"go/token"
	"path"
	"path/filepath"
	"strconv"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
)

// The extern "C" rule (Phase 04 README §2.9, step 4.14).
//
// The build includes a C header that has no __cplusplus guard of its own
// inside extern "C" { }, so that the C functions it declares get C linkage
// and link with the C code that defines them (a .c file of the package, a
// C library). Which headers those are, Check decides here, for each header
// import of a package, before the namespace probe runs, and the probes and
// the lowerer use the decision (Package.ExternC):
//   - //gocpp:extern-c <hdr> wraps the header, whatever it is (also a .hpp
//     file, or a system header such as <lua.h>), and //gocpp:no-extern-c
//     <hdr> never wraps it;
//   - else a local .h header is wrapped when it is a C header: the C
//     compiler parses it as C (Config.CHeader, interop.CheckCHeaderFor by
//     default, with the package's C flags), and its own text has no
//     __cplusplus (the guard). A local .hh, .hpp, .hxx, .h++ or .inl
//     header and a system header are not wrapped.
//
// A directive names a header as it is imported: <lua.h> as is, a local
// header by its path, with ./ and .. cleaned away (./legacy.h, legacy.h
// and sub/../legacy.h name the same header). Both directives on one header
// are an error, and so is a directive for a header that no file of the
// package imports (a typo would do nothing): the errors are at the
// directives, and the default rules apply then.

// headerKey returns the key of a header in the directives: the system
// header <lua.h> as it is written, a local header by its cleaned path.
func headerKey(kind load.HeaderKind, p string) string {
	if kind == load.LocalHeader {
		return path.Clean(p)
	}
	return p
}

// readExternC returns what the //gocpp:extern-c (true) and no-extern-c
// (false) directives of lp say for each header (by headerKey), and the
// errors of these directives, in source order.
func readExternC(fset *token.FileSet, lp *load.Package) (map[string]bool, []diag.Diagnostic) {
	imported := make(map[string]bool)
	for _, f := range lp.Files {
		if f.Info == nil {
			continue
		}
		for _, inc := range f.Info.Includes {
			imported[headerKey(inc.Kind, inc.Path)] = true
		}
	}
	var errs []diag.Diagnostic
	fail := func(d load.Directive, msg string) {
		errs = append(errs, diag.Diagnostic{
			Pos: fset.Position(d.Pos),
			End: fset.Position(d.Pos + token.Pos(len("//gocpp:")+len(d.Name))),
			Msg: msg,
		})
	}
	want := make(map[string]bool)
	first := make(map[string]load.Directive) // the first directive of each header
	bad := make(map[string]bool)             // headers with both directives
	for _, f := range lp.Files {
		for _, d := range f.Directives {
			if d.Name != "extern-c" && d.Name != "no-extern-c" || len(d.Args) != 1 {
				continue
			}
			hdr := d.Args[0]
			kind := load.LocalHeader
			if hdr[0] == '<' {
				kind = load.SystemHeader
			}
			key := headerKey(kind, hdr)
			wrap := d.Name == "extern-c"
			switch prev, seen := first[key]; {
			case !imported[key]:
				fail(d, "//gocpp:"+d.Name+" names header "+hdr+", which no file of the package imports")
			case seen && prev.Name != d.Name:
				pos := fset.Position(prev.Pos)
				fail(d, "header "+hdr+" has both //gocpp:extern-c and //gocpp:no-extern-c (the other is at "+
					filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line)+")")
				bad[key] = true
			case !seen:
				first[key] = d
				want[key] = wrap
			}
		}
	}
	for key := range bad {
		delete(want, key)
	}
	return want, errs
}

// externCDefault reports whether the header import inc may be wrapped by
// the default rules, if it is a C header: only a local .h header.
func externCDefault(inc *load.Include) bool {
	return inc.Kind == load.LocalHeader && path.Ext(inc.Path) == ".h"
}

// wrapExternC returns the include text t, marked with interop.ExternC
// when wrap says so.
func wrapExternC(t string, wrap map[string]bool) string {
	if wrap[t] {
		return interop.ExternC(t)
	}
	return t
}
