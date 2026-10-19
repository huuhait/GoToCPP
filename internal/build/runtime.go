package build

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/fsretry"
	"github.com/huuhait/gocpp/internal/version"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The runtime (README §2.7): its files are embedded in the gocpp binary
// (package rtfiles) and extracted once into the cache folder, to
// <cache>/rt/<version>-<hash>/, where <hash> is a hash of the embedded
// files. A "dev" build changes its runtime without changing its version,
// so the version alone can't name the folder; with the hash, two gocpp
// binaries never share a folder unless their runtimes are the same.
//
// The runtime is cached next to the files as a static library, lib/<key>/libgocpprt.a (rtlib.go),
// where the key is a hash of everything the compile depends on (runtimeKey): the compiler, its
// flags and the environment variables that change what it does, and the archiver. Without an
// archiver (GOCPP_NORTLIB, or no ar), its object files are kept in obj/<key>/ and linked one by
// one.
//
// Both are written atomically, so that gocpp commands that run at the same
// time (the test harness runs many) never see half a folder: each writes a
// temporary folder next to the final one and renames it into place. When
// two do it at the same time, one rename wins, and the other one uses the
// winner's folder.

// rtHash is a hash of the embedded runtime's files: their paths and
// contents.
var rtHash = sync.OnceValue(func() string {
	h := sha256.New()
	err := fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(rtfiles.FS(), p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		panic("build: can't read the embedded runtime: " + err.Error()) // it is in the binary
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
})

// runtimeName is the name of the runtime's folder in <cache>/rt.
func runtimeName() string {
	v := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' || r == '+' {
			return r
		}
		return '_'
	}, version.Version)
	return v + "-" + rtHash()
}

// rtFile is one file of the embedded runtime.
type rtFile struct {
	path string // in rtfiles.FS, with slashes
	size int64
}

// rtFiles lists the files of the embedded runtime, to check a folder
// cheaply (checkRuntime).
var rtFiles = sync.OnceValue(func() []rtFile {
	var files []rtFile
	fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, rtFile{p, fi.Size()})
		return nil
	})
	return files
})

// ExtractRuntime returns the folder of the runtime in the cache folder
// cache (such as ~/.cache/gocpp), extracting it there first if it isn't
// there yet. The folder holds include/ (the -I folder) and src/. A folder
// that is there already is checked cheaply: each file must exist with the
// right size. One that fails the check (someone edited or removed a file)
// is replaced.
func ExtractRuntime(cache string) (string, error) {
	root := filepath.Join(cache, "rt")
	dir := filepath.Join(root, runtimeName())
	if checkRuntime(dir) == nil {
		return dir, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("extracting the runtime: %w", err)
	}
	removeStale(root)
	tmp, err := os.MkdirTemp(root, ".tmp-")
	if err != nil {
		return "", fmt.Errorf("extracting the runtime: %w", err)
	}
	defer os.RemoveAll(tmp) // after a successful rename, there is nothing left
	if err := writeRuntime(tmp); err != nil {
		return "", fmt.Errorf("extracting the runtime: %w", err)
	}
	if err := fsretry.Rename(tmp, dir); err == nil {
		return dir, nil
	}
	// Another gocpp extracted it at the same time (the rename fails when
	// dir is a folder that is not empty), or dir is broken.
	if checkRuntime(dir) == nil {
		return dir, nil
	}
	old, err := os.MkdirTemp(root, ".old-")
	if err != nil {
		return "", fmt.Errorf("extracting the runtime: %w", err)
	}
	defer os.RemoveAll(old)
	if err := fsretry.Rename(dir, filepath.Join(old, "rt")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("extracting the runtime: replacing the broken %s: %w", dir, err)
	}
	if err := fsretry.Rename(tmp, dir); err != nil && checkRuntime(dir) != nil {
		return "", fmt.Errorf("extracting the runtime: %w", err)
	}
	return dir, nil
}

// WriteRuntime writes the embedded runtime into the folder dir, which
// must exist: for a build that can't use the cache folder.
func WriteRuntime(dir string) error {
	return writeRuntime(dir)
}

func writeRuntime(dir string) error {
	return fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(rtfiles.FS(), p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

// checkRuntime checks cheaply that dir holds the embedded runtime: every
// file is there, with the right size.
func checkRuntime(dir string) error {
	for _, f := range rtFiles() {
		fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.path)))
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() || fi.Size() != f.size {
			return fmt.Errorf("%s: the file differs from gocpp's runtime", f.path)
		}
	}
	return nil
}

// staleAge is how old a temporary folder (.tmp-*, .old-*) in the cache
// must be before removeStale removes it: a gocpp that was killed
// (SIGKILL) while it wrote one leaves it behind, and one that is younger
// may still be in use.
const staleAge = time.Hour

// removeStale removes the temporary folders in dir that are older than
// staleAge.
func removeStale(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".tmp-") && !strings.HasPrefix(e.Name(), ".old-") {
			continue
		}
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > staleAge {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// runtimeObjectName returns the name of the object file of the runtime
// source src (a path of rtfiles.Sources): src/os/linux/os.cpp becomes
// src_os_linux_os.o.
func runtimeObjectName(src string) string {
	return strings.ReplaceAll(strings.TrimSuffix(src, path.Ext(src)), "/", "_") + ".o"
}

// RuntimeFolder returns the folder of the runtime in the cache folder cache (<cache>/rt/<name>),
// where a build extracts it: gocpp env prints it. The folder may not exist yet.
func RuntimeFolder(cache string) string {
	return filepath.Join(cache, "rt", runtimeName())
}
