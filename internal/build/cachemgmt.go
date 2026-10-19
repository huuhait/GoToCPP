package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The folders of the cache (Phase 06 step 6.6, docs/spec/build.md). The cache folder is
// $XDG_CACHE_HOME/gocpp (~/.cache/gocpp on Linux); each kind of thing has a folder in it, and
// all of it can be removed at any time: gocpp makes it again, and a build that runs while it is
// removed fails or goes without the cache, never with a wrong result (every file is written to a
// temporary name and renamed, and read back with its hash or its stat).

// A CacheFolder is one folder of the cache folder.
type CacheFolder struct {
	Name string // the name of the folder
	What string // what is in it
}

// CacheFolders are the folders that `gocpp clean -cache` removes, in the order it lists them. The
// module cache (mod, `gocpp clean -modcache`: it is the user's downloads and needs the network to
// make again) and the language server's cache are not among them.
var CacheFolders = []CacheFolder{
	{"obj", "the object cache: object files of the generated C++ and of C and C++ sources, with the lists of headers they read"},
	{"rt", "the runtime: its sources, its libraries (one per compiler and flags) or object files, and precompiled headers"},
	{"gen", "the generated C++ of the programs that were built"},
	{"stamp", "the records of finished builds (the up-to-date check)"},
	{"cxxid", "what the C++ compilers are (their versions)"},
	{"probe", "the compiler's answers about C and C++ headers"},
}

// CacheUsage is the size of one folder of the cache.
type CacheUsage struct {
	Name  string
	Files int
	Bytes int64
}

// UsageOf returns the number of files and bytes of the folder dir (0 if it is not there).
func UsageOf(dir string) (files int, bytes int64) {
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			files++
			bytes += fi.Size()
		}
		return nil
	})
	return files, bytes
}

// CacheUsageOf returns the usage of each folder of CacheFolders in the cache folder cache, in order.
func CacheUsageOf(cache string) []CacheUsage {
	var list []CacheUsage
	for _, f := range CacheFolders {
		files, size := UsageOf(filepath.Join(cache, f.Name))
		list = append(list, CacheUsage{f.Name, files, size})
	}
	return list
}

// CleanCache removes the folders names (of CacheFolders) of the cache folder cache, or all of them
// for no names, and returns what each held. keep, if not nil, is asked about each entry of a
// folder (a program's folder in gen): it returns true for one that is in use and stays. With dry, nothing
// is removed.
func CleanCache(cache string, names []string, dry bool, keep func(folder, entry string) bool) ([]CacheUsage, error) {
	if cache == "" || !filepath.IsAbs(cache) || cache == string(filepath.Separator) {
		return nil, &os.PathError{Op: "clean", Path: cache, Err: fs.ErrInvalid}
	}
	var removed []CacheUsage
	for _, f := range CacheFolders {
		if len(names) > 0 && !contains(names, f.Name) {
			continue
		}
		dir := filepath.Join(cache, f.Name)
		// A folder of the cache that is a symbolic link is removed as a link: what it points at is
		// not the cache's (a user may have moved a folder to another disk, or a link may point
		// anywhere), and nothing of it is read or removed.
		fi, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			if !dry {
				if err := os.Remove(dir); err != nil {
					return removed, err
				}
			}
			removed = append(removed, CacheUsage{Name: f.Name})
			continue
		}
		files, size := UsageOf(dir)
		if keep != nil {
			entries, _ := os.ReadDir(dir)
			kept := false
			for _, e := range entries {
				if keep(f.Name, e.Name()) {
					kept = true
					continue
				}
				if !dry {
					if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
						return removed, err
					}
				}
			}
			if !kept && !dry {
				os.Remove(dir)
			}
			removed = append(removed, CacheUsage{f.Name, files, size})
			continue
		}
		if !dry {
			if err := os.RemoveAll(dir); err != nil {
				return removed, err
			}
		}
		removed = append(removed, CacheUsage{f.Name, files, size})
	}
	return removed, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Limits of the records of finished builds and of the compiler detection: TrimOld removes the
// files of a folder that were not used for maxAge (a stamp is written again when it is used, so
// the time of a stamp is the time of its last build).
const (
	stampMaxAge = 60 * 24 * time.Hour
	cxxidMaxAge = 180 * 24 * time.Hour
)

// TrimOld removes the files of the folder dir (not its subfolders) that were last modified more than
// maxAge before now, and temporary files (.tmp-*) that a gocpp that was killed left, older than a
// minute. A failure is ignored.
func TrimOld(dir string, maxAge time.Duration, now time.Time) {
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		return // a link to a folder is not the cache's: nothing in it is read or removed
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		age := now.Sub(fi.ModTime())
		if age > maxAge || strings.HasPrefix(e.Name(), ".tmp-") && age > time.Minute {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// TrimStamps removes the stamps of builds that were not made for two months, at most once a day (the
// time of the folder's marker file says when).
func TrimStamps(cache string) {
	trimEvery(filepath.Join(cache, "stamp"), stampMaxAge)
	trimEvery(filepath.Join(cache, "cxxid"), cxxidMaxAge)
}

func trimEvery(dir string, maxAge time.Duration) {
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		return
	}
	marker := filepath.Join(dir, ".trimmed")
	if fi, err := os.Stat(marker); err == nil && time.Since(fi.ModTime()) < 24*time.Hour {
		return
	}
	if f, err := os.Create(marker); err == nil {
		f.Close()
	} else {
		return
	}
	TrimOld(dir, maxAge, time.Now())
}
