package modfetch

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// DownloadAll downloads every module that file requires and that is not
// replaced by a local folder (or only the modules named in only, if it is not
// empty), checks them against gocpp.sum in mainDir and records the ones it
// does not list. It returns the modules in the order of the require lines.
func (f *Fetcher) DownloadAll(ctx context.Context, mainDir string, file *gocppmod.File, only []string) ([]*Module, error) {
	var out []*Module
	for _, r := range file.Require {
		if len(only) > 0 && !slices.Contains(only, r.Path) {
			continue
		}
		if rep := file.FindReplace(r.Path, r.Version); rep != nil {
			continue // a replaced module is not downloaded
		}
		m, err := f.Fetch(ctx, mainDir, r.Path, r.Version, FetchOptions{Record: true})
		if err != nil {
			return out, err
		}
		out = append(out, m)
	}
	for _, p := range only {
		if file.FindRequire(p) == nil || file.FindRequire(p).Path != p {
			return out, fmt.Errorf("%s is not required in %s", p, gocppmod.FileName)
		}
	}
	return out, nil
}

// Vendor writes the folder vendor/ in mainDir for the offline builds of the module whose
// gocpp.mod is file: every required module, replaced or not, with all of its
// files, in vendor/<module path>, and vendor/modules.txt in Go's format. The
// modules that are not replaced are downloaded first (and checked against
// gocpp.sum, which gets the hashes that it lacks); the copy of each is hashed
// again against the cache. The loader then takes every module from vendor/.
//
// A folder vendor/ that gocpp did not write (no modules.txt in it) is never
// touched, since a C or C++ project often has its own third-party sources in
// a folder of that name: Vendor then fails, unless the folder is empty.
func (f *Fetcher) Vendor(ctx context.Context, mainDir string, file *gocppmod.File) error {
	vendor := filepath.Join(mainDir, "vendor")
	if ents, err := os.ReadDir(vendor); err == nil {
		if _, err := os.Stat(filepath.Join(vendor, gocppmod.VendorFile)); err != nil && len(ents) > 0 {
			return fmt.Errorf("%s exists and has no %s: it was not made by gocpp mod vendor, so gocpp does not replace it (move it away first)",
				vendor, gocppmod.VendorFile)
		}
	}
	type item struct {
		req  *gocppmod.Require
		rep  *gocppmod.Replace
		src  string
		hash string
		pkgs []string
	}
	var items []*item
	seen := map[string]bool{}
	for _, r := range file.Require {
		if seen[r.Path] {
			continue
		}
		seen[r.Path] = true
		it := &item{req: r, rep: file.FindReplace(r.Path, r.Version)}
		if it.rep != nil {
			if !it.rep.Local() {
				return fmt.Errorf("%s is replaced by %s %s, which is not a local folder: gocpp can't vendor that yet", r.Path, it.rep.NewPath, it.rep.NewVersion)
			}
			dir := filepath.FromSlash(it.rep.NewPath)
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(mainDir, dir)
			}
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				return fmt.Errorf("%s is replaced by %s, but %s is not a folder", r.Path, it.rep.NewPath, dir)
			}
			it.src = filepath.Clean(dir)
		} else {
			m, err := f.Fetch(ctx, mainDir, r.Path, r.Version, FetchOptions{Record: true})
			if err != nil {
				return err
			}
			it.src, it.hash = m.Dir, m.Hash
		}
		items = append(items, it)
	}
	slices.SortFunc(items, func(a, b *item) int { return strings.Compare(a.req.Path, b.req.Path) })

	sweepVendorTemp(mainDir)
	tmp, err := os.MkdirTemp(mainDir, ".vendor-tmp-")
	if err != nil {
		return err
	}
	defer RemoveAll(tmp)
	var txt strings.Builder
	for _, it := range items {
		dst := filepath.Join(tmp, filepath.FromSlash(it.req.Path))
		if err := copyTree(it.src, dst, it.rep != nil); err != nil {
			return fmt.Errorf("vendoring %s: %v", it.req.Path, err)
		}
		if it.rep == nil {
			got, err := HashDir(ctx, dst, it.req.Path, it.req.Version)
			if err != nil || got != it.hash {
				return fmt.Errorf("vendoring %s %s: the copy does not have the hash of the cached module (%s, want %s): nothing was changed", it.req.Path, it.req.Version, got, it.hash)
			}
		}
		it.pkgs = gpPackages(dst, it.req.Path)
		fmt.Fprintf(&txt, "# %s %s", it.req.Path, it.req.Version)
		if it.rep != nil {
			fmt.Fprintf(&txt, " => %s", it.rep.NewPath)
		}
		txt.WriteString("\n## explicit\n")
		for _, p := range it.pkgs {
			txt.WriteString(p + "\n")
		}
	}
	vm, errs := gocppmod.ParseVendor(gocppmod.VendorFile, []byte(txt.String()))
	if len(errs) > 0 {
		return fmt.Errorf("internal error: the %s that was written does not parse: %v", gocppmod.VendorFile, errs)
	}
	if problems := gocppmod.CheckVendor(file, vm); len(problems) > 0 {
		return fmt.Errorf("internal error: the %s that was written does not match %s:\n\t%s", gocppmod.VendorFile, gocppmod.FileName, strings.Join(problems, "\n\t"))
	}
	if err := os.WriteFile(filepath.Join(tmp, gocppmod.VendorFile), []byte(txt.String()), 0o644); err != nil {
		return err
	}
	// Swap the folders: the old vendor folder (made by gocpp) goes away only
	// after the new one is in place.
	old := ""
	if _, err := os.Lstat(vendor); err == nil {
		old = tmp + ".old"
		if err := os.Rename(vendor, old); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, vendor); err != nil {
		if old != "" {
			os.Rename(old, vendor)
		}
		return err
	}
	if err := os.Chmod(vendor, 0o755); err != nil {
		return err
	}
	if old != "" {
		RemoveAll(old)
	}
	return nil
}

// copyTree copies the files of src into dst (made if needed). It refuses
// anything that is not a regular file or a folder. skipVendor is for the folder
// of a replace line (a checkout): it leaves out a vendor folder at the top (the
// replacement may have its own) and every .git.
func copyTree(src, dst string, skipVendor bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if skipVendor && d.IsDir() && rel == "vendor" {
			return filepath.SkipDir
		}
		if skipVendor && d.Name() == ".git" { // a checkout's repository is not part of the module
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			st, err := d.Info()
			if err != nil {
				return err
			}
			mode := fs.FileMode(0o644)
			if st.Mode()&0o100 != 0 {
				mode = 0o755
			}
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		}
		return fmt.Errorf("%s is not a regular file or a folder (links are not copied)", p)
	})
}

// gpPackages lists the packages of a module folder as import paths for
// modules.txt: the folders with a .gp file (not only _test.gp files), not
// below a folder that starts with . or _ or is called testdata or vendor, and not
// in a module of its own (a folder with a gocpp.mod file below the top).
func gpPackages(root, modPath string) []string {
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel != "." {
			n := d.Name()
			if strings.HasPrefix(n, ".") || strings.HasPrefix(n, "_") || n == "testdata" || n == "vendor" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, gocppmod.FileName)); err == nil {
				return filepath.SkipDir
			}
		}
		ents, err := os.ReadDir(p)
		if err != nil {
			return nil
		}
		for _, e := range ents {
			n := e.Name()
			if e.Type().IsRegular() && strings.HasSuffix(n, ".gp") && !strings.HasSuffix(n, "_test.gp") && !strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "_") {
				if rel == "." {
					out = append(out, modPath)
				} else {
					out = append(out, modPath+"/"+filepath.ToSlash(rel))
				}
				break
			}
		}
		return nil
	})
	slices.Sort(out)
	return out
}

// A VerifyResult is the state of one module for Verify.
type VerifyResult struct {
	Path, Version string
	// Where is "module cache" or "vendor".
	Where string
	// Problem is "" if the module is as gocpp.sum says, else why not.
	Problem string
	// Skipped is true if there was nothing to check (the module is not in the module cache).
	Skipped bool
}

// Verify hashes again every required module that is in the module cache (and
// every one in vendor/ if the folder is vendored) and compares it with gocpp.sum
// of mainDir and the hash file of the cache. A replaced module is a local folder,
// which is not checked. It returns one result for each module and place.
func (f *Fetcher) Verify(ctx context.Context, mainDir string, file *gocppmod.File) ([]VerifyResult, error) {
	sums, err := LoadSums(mainDir)
	if err != nil {
		return nil, err
	}
	var out []VerifyResult
	vendorText, verr := os.ReadFile(filepath.Join(mainDir, "vendor", gocppmod.VendorFile))
	vendored := verr == nil
	_ = vendorText
	check := func(r *gocppmod.Require, where, dir, recorded string) VerifyResult {
		res := VerifyResult{Path: r.Path, Version: r.Version, Where: where}
		got, err := HashDir(ctx, dir, r.Path, r.Version)
		want := sums.Lookup(r.Path, r.Version, "")
		switch {
		case err != nil:
			res.Problem = fmt.Sprintf("can't read %s: %v", dir, err)
		case recorded != "" && recorded != got:
			res.Problem = fmt.Sprintf("dir has been modified (%s): it has the hash %s, but it was %s when it was downloaded", dir, got, recorded)
		case len(want) == 0:
			res.Problem = fmt.Sprintf("has no entry in %s (the module has the hash %s)", SumFileName, got)
		case !slices.Contains(want, got):
			res.Problem = fmt.Sprintf("dir has been modified (%s): it has the hash %s, but %s says %s", dir, got, SumFileName, want[0])
		}
		return res
	}
	for _, r := range file.Require {
		if file.FindReplace(r.Path, r.Version) != nil {
			continue
		}
		if vendored {
			dir := filepath.Join(mainDir, "vendor", filepath.FromSlash(r.Path))
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				out = append(out, check(r, "vendor", dir, ""))
			} else {
				out = append(out, VerifyResult{Path: r.Path, Version: r.Version, Where: "vendor", Problem: "is not in vendor/"})
			}
			continue
		}
		if CheckModulePath(r.Path) != nil || f.cfg.CacheDir == "" {
			continue
		}
		dir := f.ModDir(r.Path, r.Version)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			out = append(out, VerifyResult{Path: r.Path, Version: r.Version, Where: "module cache", Skipped: true})
			continue
		}
		recorded, _, _ := readSidecar(f.sidecar(r.Path, r.Version))
		out = append(out, check(r, "module cache", dir, recorded))
	}
	return out, nil
}

// sweepVendorTemp removes the folders that a gocpp mod vendor that was killed
// left in the project, when they are a day old.
func sweepVendorTemp(mainDir string) {
	ents, err := os.ReadDir(mainDir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), ".vendor-tmp-") {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
			RemoveAll(filepath.Join(mainDir, e.Name()))
		}
	}
}
