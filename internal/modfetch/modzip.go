package modfetch

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// Which files belong to a module (the rules of module zips).
//
// A module is the files of its folder, except:
//
//   - the folders of other modules: a folder below the root that has a
//     gocpp.mod or a go.mod file is a module of its own (as in Go, where only
//     go.mod makes a boundary: x/mod/zip does not know gocpp.mod, so gocpp
//     applies the rule itself, before it zips and hashes), with everything
//     below it;
//   - version control folders (.git, .hg, .svn, .bzr);
//   - a vendor folder of the module that gocpp mod vendor or go mod vendor made
//     (it has a modules.txt file): it is made again from the requirements. Any
//     other vendor folder belongs to the module, since C and C++ libraries
//     keep sources of their own dependencies there (Go leaves every vendor
//     folder out; the hash of a Go module that has a vendor folder with no
//     modules.txt in it is not the same as Go's).
//
// Every other file belongs to the module: the .gp and .go sources, C and C++
// sources and headers, the recipes in gocpp.mod, LICENSE files, data. A file
// that is not a regular file (a symbolic link, a device) is an error: it
// would not be in the hash everywhere the same.
//
// The rules are applied to every module that is downloaded, so the tree in the
// module cache, its hash in gocpp.sum and the zip that CreateZip makes agree,
// and a module in a repository with another module in a folder of it has the
// same hash as the module that a proxy serves.

// Limits of a module zip, as Go's (x/mod/zip).
const (
	maxZipFile    = 500 << 20 // the files of a module, in all
	maxModFile    = 16 << 20  // a gocpp.mod, a go.mod
	maxLicense    = 16 << 20  // a LICENSE file
	vendorModules = "modules.txt"
)

// vcsDirs are the folders of version control systems.
var vcsDirs = map[string]bool{".git": true, ".hg": true, ".svn": true, ".bzr": true}

// A ModuleFiles is the result of ListModuleFiles.
type ModuleFiles struct {
	// Files are the names (slash separated, relative to the root) of the
	// files that belong to the module, sorted.
	Files []string

	// Skipped are the folders that were left out and why ("module", "vcs",
	// "vendor"), sorted by name.
	Skipped []SkippedDir
}

// A SkippedDir is a folder that does not belong to the module.
type SkippedDir struct {
	Dir    string // relative name
	Reason string // "module" (it has a gocpp.mod or go.mod), "vcs" or "vendor"
}

// isModuleRoot reports whether the folder has a module file.
func isModuleRoot(dir string) bool {
	for _, n := range []string{ModFileName, GoModName} {
		if st, err := os.Lstat(filepath.Join(dir, n)); err == nil && st.Mode().IsRegular() {
			return true
		}
	}
	return false
}

// GoModName is the module file of a Go module.
const GoModName = "go.mod"

// ListModuleFiles lists the files of the module in the folder root, by the
// rules above. It does not read the files.
func ListModuleFiles(ctx context.Context, root string) (*ModuleFiles, error) {
	res := &ModuleFiles{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			switch {
			case vcsDirs[d.Name()]:
				res.Skipped = append(res.Skipped, SkippedDir{rel, "vcs"})
				return filepath.SkipDir
			case isModuleRoot(p):
				res.Skipped = append(res.Skipped, SkippedDir{rel, "module"})
				return filepath.SkipDir
			case rel == "vendor":
				if st, err := os.Lstat(filepath.Join(p, vendorModules)); err == nil && st.Mode().IsRegular() {
					res.Skipped = append(res.Skipped, SkippedDir{rel, "vendor"})
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file (a symbolic link or a device): modules have only files and folders", rel)
		}
		res.Files = append(res.Files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(res.Files)
	slices.SortFunc(res.Skipped, func(a, b SkippedDir) int { return strings.Compare(a.Dir, b.Dir) })
	return res, nil
}

// PruneModule removes what does not belong to the module from the tree in root
// (nested modules, version control and vendor folders). The tree must be
// writable: it is the staging folder of a download.
func PruneModule(ctx context.Context, root string) error {
	res, err := ListModuleFiles(ctx, root)
	if err != nil {
		return err
	}
	for _, s := range res.Skipped {
		if err := RemoveAll(filepath.Join(root, filepath.FromSlash(s.Dir))); err != nil {
			return err
		}
	}
	return nil
}

// CheckModuleName reports why a file name in a module is not allowed: the
// same checks as for the names in an archive that is downloaded (names that
// would be unsafe or ambiguous on some system).
func CheckModuleName(name string) error {
	got, err := cleanName(name)
	if err != nil {
		return err
	}
	if got != name || got == "" {
		return bad(name, "not a clean slash separated relative name")
	}
	return nil
}

// CreateZip writes the zip of the module path at version, made of the files
// of the folder dir that belong to it, to w: each file as
// path@version/<name>, as module proxies serve them (so that "go mod download"
// and gocpp read it, and the hash of the files is the one in go.sum and
// gocpp.sum). It checks the names and the sizes as a download does.
func CreateZip(ctx context.Context, w io.Writer, dir, path, version string) error {
	// A zip may be for a module whose path has no dot (a local module), which
	// CheckModulePath, the check for what can be downloaded, refuses.
	if msg := gocppmod.CheckPath(path); msg != "" {
		return fmt.Errorf("invalid module path %q: %s", path, msg)
	}
	if !gocppmod.ValidVersion(version) {
		return fmt.Errorf("invalid version %q", version)
	}
	res, err := ListModuleFiles(ctx, dir)
	if err != nil {
		return err
	}
	seen := map[string]string{}
	var total int64
	for _, name := range res.Files {
		if err := CheckModuleName(name); err != nil {
			return err
		}
		// Every prefix of the name: folders clash too (Sub/x and sub/y).
		for i := 0; i <= len(name); i++ {
			if i < len(name) && name[i] != '/' {
				continue
			}
			p := name[:i]
			key := foldName(p)
			if prev, ok := seen[key]; ok && prev != p {
				return fmt.Errorf("%s and %s differ only in case: they would clash on a case-insensitive file system", prev, p)
			}
			seen[key] = p
		}
		st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		limit := int64(maxZipFile)
		switch {
		case name == ModFileName || name == GoModName:
			limit = maxModFile
		case isLicenseName(name):
			limit = maxLicense
		}
		if st.Size() > limit {
			return fmt.Errorf("%s: file is too large (%d bytes, the limit is %d)", name, st.Size(), limit)
		}
		total += st.Size()
		if total > maxZipFile {
			return fmt.Errorf("the module has more than %d bytes of files", int64(maxZipFile))
		}
	}
	zw := zip.NewWriter(w)
	prefix := path + "@" + version + "/"
	for _, name := range res.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		fh, err := zw.CreateHeader(&zip.FileHeader{Name: prefix + name, Method: zip.Deflate})
		if err != nil {
			return err
		}
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		_, err = io.Copy(fh, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}

func isLicenseName(name string) bool {
	switch strings.ToUpper(name) {
	case "LICENSE", "LICENSE.MD", "LICENSE.TXT", "COPYING", "COPYING.MD", "COPYING.TXT":
		return true
	}
	return false
}
