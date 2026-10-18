package modfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The module cache is a folder with the extracted modules:
//
//	<cache>/github.com/ocornut/imgui@v1.91.5/    the files, read-only
//	<cache>/github.com/ocornut/imgui@v1.91.5~h1  their hash (and that of gocpp.mod)
//	<cache>/lock/                                lock files of the downloads
//
// and, next to a module while it is made, folders ".tmp-*" and files ".dl-*"
// that are removed when the download ends and, if the process died, by the
// next download in the same folder when they are a day old. Upper-case letters
// of a path are written !lower-case (as in Go's module cache), so that two
// paths that differ only in case don't clash.
//
// A module folder appears all at once, by a rename from a folder next to it,
// after its files were made read-only and its hash file written, so a gocpp that
// finds the folder can use it without a lock. A folder is never changed.

// ModDir returns the folder of the module version in the cache. The path
// and version must have been checked.
func (f *Fetcher) ModDir(path, version string) string {
	return filepath.Join(f.cfg.CacheDir, filepath.FromSlash(escape(path))+"@"+escape(version))
}

// sidecar is the hash file of a module version. "~" can't be in a version, so no
// module folder has this name (a folder of the version v1.0.0-x.h1 would have
// the name of the ".h1" file of v1.0.0-x).
func (f *Fetcher) sidecar(path, version string) string { return f.ModDir(path, version) + "~h1" }

// hashName is a short file name for a string.
func hashName(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:12])
}

// inflight serializes the downloads of one module version inside this
// process (a channel of one place per lock name, so that waiting ends with the
// context); the file lock does it between processes.
var inflight sync.Map // lock path -> chan struct{}

// errLockUnsupported is what flock returns for a file system that has no file
// locks (NFS without lockd, FUSE, 9p, SMB): gocpp goes on without the
// cross-process lock. The download stays correct, since a module folder appears
// by one rename and the loser of a race checks the winner's hash; it may be done twice.
var errLockUnsupported = errors.New("file locks are not supported here")

// flockImpl is flock; tests replace it.
var flockImpl = flock

// lockFile takes the lock with the name (a file in <cache>/lock) and returns
// the function that gives it up. It waits for another holder, until ctx ends.
func (f *Fetcher) lockFile(ctx context.Context, name string) (func(), error) {
	if f.cfg.CacheDir == "" {
		return func() {}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute) // another gocpp holds the lock at most as long as a download
	defer cancel()
	dir := filepath.Join(f.cfg.CacheDir, "lock")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, f.cacheError(err)
	}
	semAny, _ := inflight.LoadOrStore(filepath.Join(dir, name), make(chan struct{}, 1))
	sem := semAny.(chan struct{})
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for another download of the same module: %v", ctx.Err())
	}
	lf, err := os.OpenFile(filepath.Join(dir, name+".lock"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		<-sem
		return nil, f.cacheError(err)
	}
	locked := false
	for !locked {
		ok, err := flockImpl(lf)
		if errors.Is(err, errLockUnsupported) {
			break // no lock between processes on this file system
		}
		if err != nil {
			lf.Close()
			<-sem
			return nil, f.cacheError(err)
		}
		if ok {
			locked = true
			break
		}
		select {
		case <-ctx.Done():
			lf.Close()
			<-sem
			return nil, fmt.Errorf("waiting for another gocpp that downloads the same module: %v", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	return func() {
		if locked {
			funlock(lf)
		}
		lf.Close()
		<-sem
	}, nil
}

// cacheError says what is wrong with a cache folder that can't be written.
func (f *Fetcher) cacheError(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("the module cache %s is not writable: %v (set %s to a folder that is)", f.cfg.CacheDir, err, EnvModCache)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) && (pe.Err.Error() == "read-only file system") {
		return fmt.Errorf("the module cache %s is on a read-only file system (set %s to a writable folder)", f.cfg.CacheDir, EnvModCache)
	}
	return fmt.Errorf("module cache %s: %v", f.cfg.CacheDir, err)
}

// staleTemp removes the temporary files of dead downloads in dir.
func staleTemp(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), ".tmp-") && !strings.HasPrefix(e.Name(), ".dl-") {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
			RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// readSidecar returns the hash of the module folder and of its gocpp.mod
// that the .h1 file has.
func readSidecar(name string) (dirHash, modHash string, err error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) == 1 && validHash(f[0]):
			dirHash = f[0]
		case len(f) == 2 && f[0] == ModFileName && validHash(f[1]):
			modHash = f[1]
		case len(f) == 0:
		default:
			return "", "", fmt.Errorf("%s: malformed", name)
		}
	}
	if dirHash == "" {
		return "", "", fmt.Errorf("%s: no hash", name)
	}
	return dirHash, modHash, nil
}

func sidecarText(dirHash, modHash string) []byte {
	s := dirHash + "\n"
	if modHash != "" {
		s += ModFileName + " " + modHash + "\n"
	}
	return []byte(s)
}

// CleanModCache removes the whole module cache folder, including the
// folders that are read-only, as "gocpp clean -modcache" will (step 6.12).
// It refuses a folder that does not look like a module cache: one with
// entries that are neither module paths (their first element has a dot), nor
// "lock".
func CleanModCache(dir string) error {
	if dir == "" || dir == "/" || !filepath.IsAbs(dir) {
		return fmt.Errorf("refusing to clean %q", dir)
	}
	ents, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range ents {
		if n := e.Name(); n != "lock" && !strings.Contains(n, ".") {
			return fmt.Errorf("%s does not look like a module cache (it has %q): not removing it", dir, n)
		}
	}
	for _, e := range ents {
		if err := RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
