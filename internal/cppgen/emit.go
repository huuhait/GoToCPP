package cppgen

import (
	"errors"
	"fmt"
	"go/token"
	"io"
	"path"
	"path/filepath"

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

// EmitPackage writes the C++ of the type-checked main package pkg to w: one C++20 translation unit
// for the whole program (the unit layout; EmitProgram writes the split layout that gocpp build
// compiles: one file per .gp file, a pkg.hpp for each package, and several packages). fset holds the positions of pkg's files
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

// A ProgramPackage is a type-checked package of a program for EmitProgram, with what the
// C++ compiler's probes found out about its C/C++ names (lower.ProbeInterop; nil for a package
// without header imports).
type ProgramPackage struct {
	Pkg     *check.Package
	Interop *lower.Interop
}

// A File is one generated file.
type File struct {
	Path string // relative to the root of the generated files, with slashes: gpkg/<import path>/pkg.hpp
	Data []byte
}

// A ProgramFiles are the generated files of a program.
type ProgramFiles struct {
	// Files are all the files, in the order of the packages (each one after the packages it
	// imports): for each package its header, its units, pkg_desc.cpp and pkg_init.cpp.
	Files []File

	// Packages say what the build needs to know of each package, in the same order.
	Packages []PackageFiles
}

// PackageFiles describes the files of one package of a program.
type PackageFiles struct {
	ImportPath string
	Compile    []string // the translation units of the package (paths of Files)
	HeaderC    bool     // pkg.hpp includes the package's C/C++ headers: its flags are public
}

// EmitProgram writes the C++ of the packages of a program in the split layout (Phase 06 README
// §2.2): for each package a header, pkg.hpp, a translation unit for each .gp file that has
// functions, pkg_desc.cpp and pkg_init.cpp, in the folder gpkg/<import path>. The packages
// must be in the order of their imports (each one after the packages it imports), the last one
// is the main package, and all of them are checked and without errors. root is the absolute
// folder that the files will be written in, for the #line directives of their generated lines
// ("" for gocpp emit; the files of a standard package name $GOCPP/gen/<path> instead, so that their
// text does not depend on the program: the object cache reuses their objects across programs). The files are in the order of the packages, in the order header, units,
// pkg_desc.cpp, pkg_init.cpp, so that equal programs give equal bytes.
func EmitProgram(fset *token.FileSet, pkgs []ProgramPackage, root string, cfg Config) (*ProgramFiles, error) {
	in := make([]lower.PackageInput, len(pkgs))
	for i, p := range pkgs {
		switch {
		case !p.Pkg.Checked():
			return nil, fmt.Errorf("cppgen: package %s was not type-checked", p.Pkg.Load.ImportPath)
		case p.Pkg.Errors.HasErrors():
			return nil, fmt.Errorf("cppgen: package %s has errors", p.Pkg.Load.ImportPath)
		}
		in[i] = lower.PackageInput{Pkg: p.Pkg, Interop: p.Interop}
	}
	prog := lower.NewProgram(fset, in, lower.Options{AllForwarders: cfg.AllForwarders})
	pfs, err := prog.Lower()
	if err != nil {
		var le *lower.Error
		if errors.As(err, &le) {
			return nil, &Error{Pos: le.Pos, Msg: le.Msg, User: le.User}
		}
		return nil, err
	}
	out := &ProgramFiles{}
	for i, pf := range pfs {
		info := PackageFiles{ImportPath: pf.ImportPath, HeaderC: pf.HeaderC}
		std := pkgs[i].Pkg.Load.Std
		add := func(rel string, f *cppir.File, compile bool) {
			outPath := ""
			switch {
			case root != "" && std:
				// The files of a standard package are the same in every program, and the object
				// cache keys a compile by the text of the unit: the folder of this program must
				// not be in it. The placeholder is the one of the .gp paths ($GOCPP/stdlib).
				outPath = "$GOCPP/gen/" + rel
			case root != "":
				outPath = filepath.Join(root, filepath.FromSlash(rel))
			}
			out.Files = append(out.Files, File{Path: rel, Data: cppir.Print(f, outPath)})
			if compile {
				info.Compile = append(info.Compile, rel)
			}
		}
		add(lower.HeaderPath(pf.ImportPath), pf.Header, false)
		for _, u := range pf.Units {
			add(path.Join(pf.Dir, u.Name), u.File, true)
		}
		if pf.Desc != nil {
			add(path.Join(pf.Dir, "pkg_desc.cpp"), pf.Desc, true)
		}
		if pf.Init != nil {
			add(path.Join(pf.Dir, "pkg_init.cpp"), pf.Init, true)
		}
		out.Packages = append(out.Packages, info)
	}
	return out, nil
}
