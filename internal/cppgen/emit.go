package cppgen

import (
	"errors"
	"fmt"
	"go/token"
	"io"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/lower"
)

// A Config says how EmitPackage writes the C++.
type Config struct {
	// OutputPath is the absolute path of the file that the C++ goes to, or "". When it is
	// set, the code that has no place in the .gp source (the main wrapper, gp_init_funcs, the
	// closing lines of gp_var_init) gets #line directives with its own lines in that file,
	// so that a debugger and the line table don't show it at unrelated .gp lines. gocpp emit,
	// which prints the C++, leaves it empty.
	OutputPath string

	// Interop is what the C++ compiler's probes found out about the C/C++ names of the package
	// (lower.ProbeInterop, Phase 04): a package with header imports needs it. nil for a
	// package without.
	Interop *lower.Interop

	// AllForwarders writes the forwarding members of promoted methods for every struct (lower.Options),
	// not only for those whose method tables need them. The tests that call them from C++ set it.
	AllForwarders bool
}

// An Error is a problem that stops EmitPackage: mostly a construct that the emitter can't
// translate yet ("not supported yet: switch statement"). The phase gates of internal/check
// reject the features of later phases with a message for the user, so an Error is a gocpp
// bug or a feature that this phase hasn't built yet, and the driver reports it as an internal
// error.
type Error struct {
	Pos token.Position // where in the .gp source; may be invalid
	Msg string         // without the position and the "cppgen: " prefix

	// User says that the error is in the user's code, not gocpp's (lower.Error.User): a
	// C/C++ name that the imported headers don't declare, which only the compiler's probes
	// find out. The driver reports it as an error of the program, not as an internal error.
	User bool
}

func (e *Error) Error() string {
	if e.Pos.IsValid() {
		return e.Pos.String() + ": cppgen: " + e.Msg
	}
	return "cppgen: " + e.Msg
}

// EmitPackage writes the C++ of the type-checked package pkg to w: one C++20 translation unit
// for the whole program (Phases 02 to 05 build a program from one package, main; Phase 06
// writes one file per .gp file and a pkg.hpp). fset holds the positions of pkg's files
// (check.Result.Fset); their names must be absolute, as the loader makes them, because they
// go into the #line directives. lower.Package says what the translation unit holds.
//
// It compiles with the driver's flags and the runtime (-I runtime/include and the runtime's
// .cpp files, rtfiles.Sources). EmitPackage writes nothing to w if it fails: it returns an
// *Error for a construct that it can't translate, and another error if pkg is not a checked
// main package without errors.
func EmitPackage(w io.Writer, fset *token.FileSet, pkg *check.Package, cfg Config) error {
	switch {
	case !pkg.Checked():
		return fmt.Errorf("cppgen: package %s was not type-checked", pkg.Load.ImportPath)
	case pkg.Errors.HasErrors():
		return fmt.Errorf("cppgen: package %s has errors", pkg.Load.ImportPath)
	}
	f, err := lower.Package(fset, pkg, cfg.Interop, lower.Options{AllForwarders: cfg.AllForwarders})
	if err != nil {
		var le *lower.Error
		if errors.As(err, &le) {
			return &Error{Pos: le.Pos, Msg: le.Msg, User: le.User}
		}
		return err
	}
	_, err = w.Write(cppir.Print(f, cfg.OutputPath))
	return err
}
