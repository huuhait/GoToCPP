package load

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// Module headers (Phase 06 step 6.1b, README §2.11, TODO Q14, Q16 and Q17).
//
// A quoted import whose first path element has a dot and that ends in a
// header extension names a header inside a module, as the import of a
// package does:
//
//	import "github.com/huuhait/app/native/x.h"       // native/x.h in the own module
//	import "github.com/ocornut/imgui/imgui.h"        // imgui.h in a required module
//
// ClassifyImports sorts it out as a local header (IsModuleHeaderPath); here
// the loader turns it into the file. The module is the one whose path is the
// longest prefix of the import path, among the module of the importing
// package (its own module) and the modules that the main module requires,
// the same rule as for the packages (find): the rest of the path is the
// file in the folder of the module (a replacement folder, vendor/<path>, or
// what Config.ModuleDir gives). The package name is the file name, as for
// every header. "./x.h" and "native/x.h" stay headers of the package
// folder; a path with a dotted first element that no module has is an
// error.
//
// The header becomes the same kind of header package as a local one, and
// the include is written with the absolute name of the file, so the
// compiler finds exactly that file whatever the package folder has
// (Include.File, Include.Target). The root folder of the module goes to the
// compiler as -iquote (Package.IncludeDirs, build.ReadPackageFlags), also in the
// probes, because the headers of a library include each other by names
// relative to the library ("imconfig.h", "backends/x.h").
//
// The cxx recipe of the module (Phase 06 step 6.1d, cxxrecipe.go): the loader
// finds the recipe block of each module that a header comes from and puts it
// in Package.CxxModules (noteCxxModule). Its include folders and defines
// reach the compiler through build.ReadPackageFlags, which checks them with the
// flag allow-list (the loader does not know the allow-list). The module-wide
// flags of a dependency's own gocpp.mod (cflags, pkg-config, ldflags) do NOT
// reach the compiler: that is the open question of how a dependency's flags are
// checked; only the project's //gocpp: directives and the explicit cxx recipes
// give flags.

// moduleHeaders resolves the module headers of the files of p, which
// ClassifyImports has classified. A header that can't be resolved gets an
// error at its import and leaves f.Info.Includes (its name stays declared,
// so that its uses report nothing more).
func (l *loader) moduleHeaders(p *Package) {
	for _, f := range p.Files {
		if f.Info == nil {
			continue
		}
		var failed map[*Include]bool
		for _, inc := range f.Info.Includes {
			if inc.Kind != LocalHeader || !IsModuleHeaderPath(inc.Path) {
				continue
			}
			file, mod, msg := l.resolveModuleHeader(p, inc.Path)
			if msg != "" {
				p.Errors.Add(diag.Diagnostic{
					Pos: l.fset.Position(inc.Pos),
					End: l.fset.Position(inc.Spec.Path.End()),
					Msg: msg,
				})
				if failed == nil {
					failed = make(map[*Include]bool)
				}
				failed[inc] = true
				continue
			}
			inc.Module, inc.File = mod, file
			l.noteCxxModule(p, mod)
			for _, dir := range l.moduleIncludeDirs(mod) {
				if !slices.Contains(p.IncludeDirs, dir) {
					p.IncludeDirs = append(p.IncludeDirs, dir)
				}
			}
		}
		if len(failed) > 0 {
			f.Info.Includes = slices.DeleteFunc(f.Info.Includes, func(inc *Include) bool { return failed[inc] })
		}
	}
}

// moduleIncludeDirs returns the folders that the compiler gets with -iquote for
// the headers of the module mod: its root folder. It is -iquote, not -I, on
// purpose: a file of the module named like a system header (string.h, time.h)
// must not hide the system's for <cstring>. Quoted includes of the library
// ("imconfig.h", "backends/x.h") find the module's files; a library that wants
// its headers found as <x.h> needs the include folders of its cxx recipe,
// which build.ReadPackageFlags adds as -I or -isystem (Package.CxxModules).
func (l *loader) moduleIncludeDirs(mod *Module) []string {
	return []string{mod.Dir}
}

// resolveModuleHeader returns the absolute file (with / separators) and the
// module of the module header path for an import in the package from, or
// the error message.
func (l *loader) resolveModuleHeader(from *Package, path string) (file string, mod *Module, msg string) {
	if why := checkModuleHeaderPath(path); why != "" {
		return "", nil, fmt.Sprintf("invalid header path %q: %s", path, why)
	}
	if from.Std {
		return "", nil, fmt.Sprintf("header %s: a standard package can't import a module header", path)
	}
	own := -1 // the length of the path of the module of from, if path is in it
	if m := from.Module; m != nil && m.Path != "" && inModule(path, m.Path) {
		own = len(m.Path)
	}
	var modPath, found string // the module of the file, and the text of the "not found" error
	var root string
	switch req := l.findRequire(from, path); {
	case req != nil && len(req.Path) > own:
		m, why := l.requiredModule(from.mainModule(), req)
		if why != "" {
			return "", nil, fmt.Sprintf("header %s: %s", path, why)
		}
		mod, modPath, root = m, req.Path, m.Dir
		found = fmt.Sprintf("module %s@%s found (%s), but does not contain header %s", req.Path, req.Version, m.Dir, path)
	case own >= 0:
		mod, modPath, root = from.Module, from.Module.Path, from.Module.Dir
		found = "" // set below, it names the file
	case from.Module == nil:
		return "", nil, fmt.Sprintf("no required module provides header %s: "+
			"gocpp.mod file not found in %s or any parent directory", path, from.Dir)
	default:
		hint := l.replacedHint(from, path)
		if hint == "" {
			hint = fmt.Sprintf("; add the module that has it to %s with a require line, check the path for a typo, "+
				"or for a header in a folder of the package write %q", modFile, "./"+path)
		}
		return "", nil, "no required module provides header " + path + hint
	}
	rel := strings.TrimPrefix(path[len(modPath):], "/")
	name := filepath.Join(root, filepath.FromSlash(rel))
	if found == "" {
		found = fmt.Sprintf("header %s is not in module %s (%s)", path, modPath, name)
	}
	fi, err := l.statFile(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil, found
	case err != nil:
		return "", nil, fmt.Sprintf("header %s can't be read: %v", path, err)
	case fi.IsDir():
		return "", nil, fmt.Sprintf("header %s (%s) is a folder, not a file", path, name)
	}
	file = filepath.ToSlash(name)
	if strings.ContainsAny(file, "\"\n\r\x00") {
		return "", nil, fmt.Sprintf("header %s is at %s, whose name can't be written in an #include line", path, file)
	}
	return file, mod, ""
}

// checkModuleHeaderPath returns what is wrong with the path of a module
// header, besides the checks of checkHeader (a \ is "use / in header
// paths"), or "". Every element must be a plain name: the file is looked up
// by joining the elements to the folder of the module, so an element ..
// would leave the module, and one with a : is a drive letter or a stream
// name on Windows. The path can't be absolute either (its first element
// would be empty), and has no empty element ("a//b").
func checkModuleHeaderPath(path string) string {
	for elem := range strings.SplitSeq(path, "/") {
		switch {
		case elem == "":
			return "empty path element (a header path has no // and is not absolute)"
		case elem == ".":
			return `path element "." is not allowed`
		case elem == "..":
			return `path element ".." is not allowed (a module header is inside its module)`
		case strings.ContainsAny(elem, ":\\"):
			return fmt.Sprintf("path element %q has a : or a \\", elem)
		case strings.TrimRight(elem, ". ") != elem:
			return fmt.Sprintf("path element %q ends in a dot or a space", elem)
		}
	}
	return ""
}

// statFile stats the absolute name of a file, on disk or in Config.Overlay.
func (l *loader) statFile(name string) (fs.FileInfo, error) {
	if _, ok := l.cfg.Overlay[name]; ok {
		return overlayInfo{}, nil
	}
	return l.cfg.FS.Stat(name)
}

// overlayInfo is the FileInfo of a file that is only in Config.Overlay.
type overlayInfo struct{ fs.FileInfo }

func (overlayInfo) IsDir() bool { return false }
