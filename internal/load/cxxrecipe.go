package load

import (
	"go/token"
	"path/filepath"

	"github.com/huuhait/gocpp/internal/diag"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// The cxx recipe of a module (Phase 06 step 6.1d, docs/spec/modules.md "C and
// C++ libraries", ADR-024).
//
// A C or C++ library is a module that is built from a small recipe in
// gocpp.mod, a brace block `cxx <module path> { ... }`. Two files can have
// it:
//
//  1. the project's own gocpp.mod (the main module's), keyed by the module
//     path that the block names: it works for a replaced, a vendored and a
//     downloaded module;
//  2. the library's own gocpp.mod, in the folder of the module, keyed by its
//     own module path: a library author can ship the recipe.
//
// The project's block overrides the library's entirely; the two are never
// merged. A module with neither is used header-only, as before.
//
// The loader only finds the block (as text lines: gocppmod.Block) of the
// modules that a package actually imports a header from, and puts it in
// Package.CxxModules. Reading the lines (gocppmod.ParseRecipe), looking at the
// files they name, the flag allow-list and building the library are the work
// of package build (ResolveRecipe, ReadPackageFlags, Builder): the loader
// never touches a source file of the library, and nothing of the library is
// executed.

// A CxxModule is a module whose headers a package imports, and the recipe that
// builds its library, if there is one.
type CxxModule struct {
	// Module is the module: its path, version and folder.
	Module *Module

	// GOOS and GOARCH are the target of the load, and Tags the extra build
	// tags (Config.Tags): the if blocks of the recipe ask for them
	// (BuildTags).
	GOOS, GOARCH string
	Tags         []string

	// Recipe is the cxx block of the module, or nil: the module has no
	// recipe, and its headers are used as they are.
	Recipe *gocppmod.Block

	// RecipeFile is the gocpp.mod file that has Recipe (for messages), and
	// FromLibrary is true when it is the library's own file, not the
	// project's.
	RecipeFile  string
	FromLibrary bool

	// Std is the `cxx` line of the library's own gocpp.mod (c++17), the
	// default C++ standard of its sources, or "".
	Std string

	// Errors are the mistakes in how the recipes of the file are written
	// (a second block for the module, a bad path in the cxx line), at their
	// lines. The lines of the recipe itself are read by gocppmod.ParseRecipe.
	Errors gocppmod.ErrorList
}

// noteCxxModule records for the package p that it uses module mod, with the
// recipe of the module, once.
func (l *loader) noteCxxModule(p *Package, mod *Module) {
	if mod == nil || mod.Path == "" {
		return
	}
	for _, c := range p.CxxModules {
		if c.Module.Dir == mod.Dir && c.Module.Path == mod.Path {
			return
		}
	}
	p.CxxModules = append(p.CxxModules, l.cxxModule(p, mod))
}

// cxxModule returns the recipe of the module mod for a package of the program
// of p (see above), read once for each module and main module.
func (l *loader) cxxModule(p *Package, mod *Module) *CxxModule {
	main := p.mainModule()
	key := mod.Dir + "\x00" + mod.Path + "\x00"
	if main != nil {
		key += main.Dir
	}
	if c, ok := l.cxxMods[key]; ok {
		return c
	}
	c := &CxxModule{Module: mod, GOOS: l.cfg.GOOS, GOARCH: l.cfg.GOARCH, Tags: l.cfg.Tags}
	l.cxxMods[key] = c
	// The library's own file: its cxx line, and a recipe if the project has
	// none.
	lib, libName := l.libraryFile(mod)
	if lib != nil && lib.Cxx != nil {
		c.Std = lib.Cxx.Std
	}
	if main != nil && main.file != nil {
		blocks, errs := main.file.Recipes()
		c.Errors = append(c.Errors, errs...)
		if b := blocks[mod.Path]; b != nil {
			c.Recipe, c.RecipeFile = b, filepath.Join(main.Dir, modFile)
			l.warnUnknownTags(c)
			return c
		}
	}
	if lib != nil && (main == nil || main.file != lib) {
		blocks, errs := lib.Recipes()
		c.Errors = append(c.Errors, errs...)
		if b := blocks[mod.Path]; b != nil {
			c.Recipe, c.RecipeFile, c.FromLibrary = b, libName, true
			l.warnUnknownTags(c)
		}
	}
	return c
}

// libraryFile returns the gocpp.mod file of the folder of the module mod and
// its name, or nil if the module has none (a plain C or C++ repository). A
// replaced module's file is read already; the file of a vendored or
// downloaded module is read here, with the errors that are not about its
// recipe left out: they are the library's own business.
func (l *loader) libraryFile(mod *Module) (*gocppmod.File, string) {
	name := filepath.Join(mod.Dir, modFile)
	if mod.file != nil {
		return mod.file, name
	}
	data, err := l.readAbs(name)
	if err != nil {
		return nil, ""
	}
	f, _ := gocppmod.Parse(name, data, nil)
	return f, name
}

// warnStrayRecipes adds a warning for each cxx block in the gocpp.mod of a
// main module (of a root package) that names a module which is neither the
// module itself nor required nor replaced there: such a recipe is never used,
// and a typo in the module path would show only as a link error. It is a
// warning, not an error, because gocpp.mod may be shared with other projects.
func (l *loader) warnStrayRecipes() {
	seen := make(map[string]bool)
	for _, root := range l.prog.Roots {
		m := root.Module
		if m == nil || m.file == nil || seen[m.Dir] {
			continue
		}
		seen[m.Dir] = true
		blocks, _ := m.file.Recipes()
		for path, b := range blocks {
			known := path == m.Path
			for _, r := range m.file.Require {
				known = known || r.Path == path
			}
			for _, r := range m.file.Replace {
				known = known || r.OldPath == path
			}
			if !known {
				l.prog.Errors.Add(diag.Diagnostic{
					Pos:      token.Position{Filename: filepath.Join(m.Dir, modFile), Line: b.Start.Line},
					Severity: diag.Warning,
					Msg: "cxx " + path + ": this module is not the main module and is neither required nor replaced here, so the recipe is never used " +
						"(a typo in the module path?)",
				})
			}
		}
	}
}
