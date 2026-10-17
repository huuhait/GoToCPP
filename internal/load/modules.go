package load

import (
	"fmt"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// Required modules (Phase 06 step 6.1, docs/spec/modules.md).
//
// The main module is the one that the root packages are in. Its gocpp.mod
// file says where the imports of every package go that are not in the
// standard library and not in the package's own module: a module that it
// requires is
//
//   - in vendor/<module path> of the main module, if vendor/modules.txt
//     exists (then it must match gocpp.mod: gocppmod.CheckVendor), or else
//   - in the folder of its replace line. If a gocpp.mod file is there, it
//     must be for the same module path; a folder without one (a plain C or
//     C++ repository) is the module as it is.
//
// Nothing is downloaded by the loader itself: a required module with neither
// is asked of the download hook (moddir.go), which is a stub that says "can't
// download modules yet" until job 6.1c. There is no version selection yet, so only the main module's own
// require and replace lines count: the lines of a required module's gocpp.mod
// are read (its module line, its errors) but not followed.

// resolvedModule is where a required module is, or why it is not available.
type resolvedModule struct {
	mod *Module
	msg string
}

// vendorState is the vendor folder of a main module: its modules.txt, and
// the ways in which that file does not match gocpp.mod.
type vendorState struct {
	mods     *gocppmod.VendorModules
	problems []string
}

// vendorOf returns the vendor state of the main module main, or nil if it has
// no vendor/modules.txt file. A vendor folder without modules.txt is not
// vendoring: C and C++ projects often have a vendor folder of their own for
// third-party sources. The first call reports a malformed or inconsistent
// modules.txt in Program.Errors.
func (l *loader) vendorOf(main *Module) *vendorState {
	if vs, ok := l.vendors[main.Dir]; ok {
		return vs
	}
	name := filepath.Join(main.Dir, "vendor", gocppmod.VendorFile)
	data, err := l.readAbs(name)
	if err != nil {
		l.vendors[main.Dir] = nil
		return nil
	}
	vm, errs := gocppmod.ParseVendor(name, data)
	vs := &vendorState{mods: vm}
	for _, e := range errs {
		l.prog.Errors.Add(diag.Diagnostic{Pos: token.Position{Filename: e.Filename, Line: e.Pos.Line}, Msg: e.Err})
	}
	switch {
	case len(errs) > 0:
		vs.problems = []string{"vendor/" + gocppmod.VendorFile + " is malformed"}
	case main.file != nil:
		vs.problems = gocppmod.CheckVendor(main.file, vm)
		if len(vs.problems) > 0 {
			l.prog.Errors.Add(diag.Diagnostic{
				Pos: token.Position{Filename: name},
				Msg: "inconsistent vendoring in " + main.Dir + ":\n\t" + strings.Join(vs.problems, "\n\t") +
					"\n\n\tTo ignore the vendor folder, delete vendor/" + gocppmod.VendorFile + ".",
			})
		}
	}
	l.vendors[main.Dir] = vs
	return vs
}

// findRequire returns the require line of the main module of from with the
// longest module path that path is in, or nil.
func (l *loader) findRequire(from *Package, path string) *gocppmod.Require {
	main := from.mainModule()
	if main == nil || main.Path == "" || main.file == nil {
		return nil
	}
	return main.file.FindRequire(path)
}

// replacedHint returns the end of the "no required module provides package"
// message for a path that no require line has, but a replace line does.
func (l *loader) replacedHint(from *Package, path string) string {
	main := from.mainModule()
	if main == nil || main.file == nil {
		return ""
	}
	if r := main.file.FindReplacedPath(path); r != nil {
		return fmt.Sprintf("; %s is replaced in %s, but not required (add: require %s v0.0.0)", r.OldPath, modFile, r.OldPath)
	}
	return ""
}

// inRequired returns where the package with the import path path is, in the
// module of the require line req.
func (l *loader) inRequired(from *Package, req *gocppmod.Require, path string) (location, string) {
	mod, msg := l.requiredModule(from.mainModule(), req)
	if msg != "" {
		return location{}, fmt.Sprintf("package %s: %s", path, msg)
	}
	dir := filepath.Join(mod.Dir, filepath.FromSlash(strings.TrimPrefix(path[len(req.Path):], "/")))
	if !l.isDir(dir) {
		return location{}, fmt.Sprintf("module %s@%s found (%s), but does not contain package %s", req.Path, req.Version, mod.Dir, path)
	}
	if m := l.module(dir); !mod.Vendored && !mod.cached && (m == nil || m.Dir != mod.Dir) {
		// A gocpp.mod file inside the module: the package is in another
		// module, which is not required.
		return location{}, "no required module provides package " + path
	}
	return location{importPath: path, dir: dir, mod: mod}, ""
}

// requiredModule returns the module of the require line req of the main
// module main, or the reason why it is not available.
func (l *loader) requiredModule(main *Module, req *gocppmod.Require) (*Module, string) {
	key := main.Dir + "\x00" + req.Path + "@" + req.Version
	if r, ok := l.required[key]; ok {
		return r.mod, r.msg
	}
	mod, msg := l.resolveRequire(main, req)
	l.required[key] = resolvedModule{mod, msg}
	return mod, msg
}

func (l *loader) resolveRequire(main *Module, req *gocppmod.Require) (*Module, string) {
	if vs := l.vendorOf(main); vs != nil {
		if len(vs.problems) > 0 {
			return nil, fmt.Sprintf("module %s@%s is not available: vendor/%s does not match %s (see its errors)",
				req.Path, req.Version, gocppmod.VendorFile, modFile)
		}
		dir := filepath.Join(main.Dir, "vendor", filepath.FromSlash(req.Path))
		if !l.isDir(dir) {
			return nil, fmt.Sprintf("module %s@%s is listed in vendor/%s, but the folder vendor/%s does not exist",
				req.Path, req.Version, gocppmod.VendorFile, req.Path)
		}
		return &Module{Path: req.Path, Dir: dir, Version: req.Version, Vendored: true}, ""
	}
	rep := replacement(main.file, req)
	if rep == nil {
		// Neither vendored nor replaced: the download hook (moddir.go).
		mod, err := l.downloadedModule(req.Path, req.Version,
			fmt.Sprintf(" (replace it with a local folder: replace %s => ../dir, or vendor it)", req.Path))
		if err != nil {
			return nil, fmt.Sprintf("module %s@%s is required, but is not available: %v", req.Path, req.Version, err)
		}
		return mod, ""
	}
	if !rep.Local() {
		mod, err := l.downloadedModule(rep.NewPath, rep.NewVersion, "")
		if err != nil {
			return nil, fmt.Sprintf("module %s is replaced by %s@%s, which is not a local folder: %v",
				req.Path, rep.NewPath, rep.NewVersion, err)
		}
		// The files of the replacement are the module req.Path of the program.
		mod.Path, mod.Version = req.Path, req.Version
		return mod, ""
	}
	dir := filepath.FromSlash(rep.NewPath)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(main.Dir, dir)
	}
	dir = filepath.Clean(dir)
	switch {
	case !l.isDir(dir):
		return nil, fmt.Sprintf("module %s is replaced by %s, but the replacement folder %s does not exist", req.Path, rep.NewPath, dir)
	case !l.hasModFile(dir):
		// A plain C or C++ repository (a git clone of a library) has no
		// gocpp.mod: its folder is the module req.Path, like a vendored or
		// downloaded one.
		return &Module{Path: req.Path, Dir: dir, Version: req.Version, cached: true}, ""
	}
	mod := l.module(dir)
	switch {
	case mod == nil || mod.Dir != dir:
		return nil, fmt.Sprintf("module %s is replaced by %s, which has no %s file", req.Path, rep.NewPath, modFile)
	case mod.Path == "":
		return nil, fmt.Sprintf("module %s is replaced by %s, whose %s has no valid module line (see its errors)", req.Path, rep.NewPath, modFile)
	case mod.Path != req.Path:
		return nil, fmt.Sprintf("module %s is replaced by %s, but the %s there declares its path as %s",
			req.Path, rep.NewPath, modFile, mod.Path)
	}
	if mod.Version == "" {
		mod.Version = req.Version
	}
	return mod, ""
}

// replacement returns the replace line of f for the required module req: the
// one for its version, else the one for any version.
func replacement(f *gocppmod.File, req *gocppmod.Require) *gocppmod.Replace {
	if f == nil {
		return nil
	}
	return f.FindReplace(req.Path, req.Version)
}

// hasModFile reports whether the folder dir has a gocpp.mod file, on disk or
// in Config.Overlay.
func (l *loader) hasModFile(dir string) bool {
	_, err := l.readAbs(filepath.Join(dir, modFile))
	return err == nil
}

// A Dependency is a module that the main module requires, and where it is.
// It is the data of a "gocpp list -m" line.
type Dependency struct {
	Path, Version string

	// Indirect is true if the require line has the comment // indirect.
	Indirect bool

	// Replace is the replacement of the module as the replace line says it
	// ("../mathx", or "example.com/fork v1.0.0"), or "".
	Replace string

	// Vendored is true if the module comes from vendor/.
	Vendored bool

	// Dir is the folder with the packages of the module, or "" if the module
	// is not available (Err says why).
	Dir string

	// Module is the module there, or nil if it is not available. For a
	// replaced module, it is the module of the gocpp.mod file in the
	// folder; for a vendored module, it has no File.
	Module *Module

	// Err is why the module is not available: it is not replaced and not
	// vendored (gocpp can't download it yet), its replacement folder is
	// missing or is another module, or vendor/ does not match gocpp.mod.
	Err string
}

// A ModuleList is the main module of a folder and the modules that it
// requires.
type ModuleList struct {
	// Main is the module of the folder, or nil if it is in no module.
	Main *Module

	// Deps are the require lines of gocpp.mod, in file order, each with
	// where its module is.
	Deps []*Dependency

	// Errors are the errors of gocpp.mod and of the gocpp.mod files of
	// the replaced modules, and the inconsistencies of vendor/modules.txt.
	Errors *diag.List
}

// ListModules reads the module of the folder dir (found by walking up to a
// gocpp.mod file) and resolves each module that it requires, to the
// replacement folder or the vendor folder, with the checks that Load makes
// when a package imports it: the folder exists and has a gocpp.mod file for
// the same module path, and vendor/modules.txt matches gocpp.mod. Config
// gives the folder dir is relative to (Dir), the overlay and the file
// system; nothing else of it is used. It never fails: Errors and the Err of
// each dependency say what is wrong.
func ListModules(cfg Config, dir string) *ModuleList {
	l := newLoader(cfg)
	ml := &ModuleList{Errors: l.prog.Errors}
	ml.Main = l.module(l.abs(dir))
	if ml.Main == nil || ml.Main.Path == "" || ml.Main.file == nil {
		return ml
	}
	l.vendorOf(ml.Main)
	for _, r := range ml.Main.file.Require {
		d := &Dependency{Path: r.Path, Version: r.Version, Indirect: r.Indirect}
		if rep := replacement(ml.Main.file, r); rep != nil {
			d.Replace = rep.NewPath
			if rep.NewVersion != "" {
				d.Replace += " " + rep.NewVersion
			}
		}
		if mod, msg := l.requiredModule(ml.Main, r); msg != "" {
			d.Err = msg
		} else {
			d.Dir, d.Module, d.Vendored = mod.Dir, mod, mod.Vendored
		}
		ml.Deps = append(ml.Deps, d)
	}
	return ml
}

// vendoredFolder returns, for a folder dir under vendor/ of the main module
// mod when vendoring is on, the vendored module that has it and the import
// path of the package there: vendor/example.com/lib/sub is the package
// example.com/lib/sub of the module example.com/lib, not
// example.com/app/vendor/example.com/lib/sub. The language server loads a
// package by its folder, so it asks for this. It returns nil when dir is not
// in a vendored module of a consistent vendor folder.
func (l *loader) vendoredFolder(mod *Module, dir string) (*Module, string) {
	if mod == nil || mod.Path == "" || mod.file == nil {
		return nil, ""
	}
	rel, err := filepath.Rel(mod.Dir, dir)
	if err != nil {
		return nil, ""
	}
	rest, ok := strings.CutPrefix(filepath.ToSlash(rel), "vendor/")
	if !ok {
		return nil, ""
	}
	req := mod.file.FindRequire(rest)
	if req == nil || l.vendorOf(mod) == nil {
		return nil, ""
	}
	m, msg := l.requiredModule(mod, req)
	if msg != "" || !m.Vendored {
		return nil, ""
	}
	return m, rest
}
