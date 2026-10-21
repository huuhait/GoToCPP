package toolchain

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/fsretry"
	"github.com/huuhait/gocpp/internal/toolchain/xz"
)

// maxUnpacked limits what an archive may write: llvm-mingw unpacks to about 1 GB, and the hash
// already vouches for the archive, so this only bounds a mistake in a mirror's configuration
// (a hash that was pinned for something else) from filling the disk.
const maxUnpacked = 8 << 30

// fetch downloads the archive a, checks it against its pinned size and SHA-256, and unpacks it into
// dir. It holds the lock of dir, so that two builds that start at once download it once: the second
// one finds the finished folder when the lock comes to it.
func (c *Config) fetch(ctx context.Context, a Archive, dir string) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	unlock, err := lock(ctx, dir+".lock")
	if err != nil {
		return err
	}
	defer unlock()
	if _, ok := installed(dir, compilerName); ok {
		return nil // another gocpp finished it while this one waited
	}
	// A folder that is there without a compiler is not a toolchain (an interrupted older gocpp,
	// a hand-made folder): it would make the rename below fail for ever.
	if _, err := os.Lstat(dir); err == nil {
		MakeWritable(dir)
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("removing %s, which is not a finished toolchain: %w", dir, err)
		}
	}
	sweepStale(parent)
	u, err := c.archiveURL(a)
	if err != nil {
		return err
	}
	// Everything of this download goes into one temporary folder next to the final one (the same
	// file system, for the rename), which is removed whatever happens.
	tmp, err := os.MkdirTemp(parent, ".download-"+strconv.Itoa(os.Getpid())+"-") // the process number tells a dead download from a running one (sweepStale)
	if err != nil {
		return err
	}
	defer func() {
		MakeWritable(tmp)
		os.RemoveAll(tmp)
	}()
	fmt.Fprintf(c.progress(), "gocpp: downloading llvm-mingw %s (%s, %d MB) from %s\n", Release, a.Host, a.Size>>20, u)
	file := filepath.Join(tmp, "archive")
	if err := c.download(ctx, u, file, a); err != nil {
		return err
	}
	root := filepath.Join(tmp, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		return err
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	if zipName(a.Name) {
		err = unpackZip(f, a.Size, root)
	} else {
		err = unpack(f, root, strings.HasSuffix(a.Name, ".tar.xz"))
	}
	if err != nil {
		return fmt.Errorf("unpacking %s: %w", a.Name, err)
	}
	if err := os.WriteFile(filepath.Join(root, markerName), []byte("llvm-mingw "+Release+" "+a.SHA256+"\n"), 0o444); err != nil {
		return err
	}
	// A hash-matching archive can still lack the compiler (a mirror serving another release's
	// archive under this name, with its hash): refuse it before it takes the place of the folder.
	if _, ok := binFile(root, compilerName); !ok {
		return fmt.Errorf("the archive %s has no bin/%s", a.Name, compilerName)
	}
	// The one rename. A folder can't be renamed once it is read-only, so the permissions come
	// after it. A folder that is already there is a finished earlier download (the lock makes
	// that rare, a lock that failed to exclude makes it possible): keep it.
	if err := fsretry.Rename(root, dir); err != nil {
		if _, ok := installed(dir, compilerName); ok {
			return nil
		}
		return err
	}
	// Read-only, so that nothing changes the toolchain by mistake; a failure here leaves a
	// working toolchain that is writable, which is not worth failing the build for.
	_ = readOnly(dir)
	return nil
}

// download gets url into the file and checks its size and SHA-256 against a, before anything is
// unpacked. An error leaves nothing that a later run would believe.
func (c *Config) download(ctx context.Context, url, file string, a Archive) error {
	// The transfer ends when the server sends nothing for the idle time (the header timeout of the
	// client covers only the wait for the first byte of the answer).
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	idle := c.IdleTimeout
	if idle <= 0 {
		idle = 60 * time.Second
	}
	timer := time.AfterFunc(idle, cancel)
	defer timer.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := *c.client()
	client.CheckRedirect = httpsRedirects
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	body := &idleReader{r: resp.Body, timer: timer, idle: idle}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s", url, resp.Status)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != a.Size {
		return fmt.Errorf("downloading %s: the server says %d bytes, the pinned archive has %d", url, resp.ContentLength, a.Size)
	}
	out, err := os.Create(file)
	if err != nil {
		return err
	}
	h := sha256.New()
	// One byte more than the pinned size, to see an archive that is too long.
	n, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(body, a.Size+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil:
		return fmt.Errorf("downloading %s: %w", url, err)
	case n != a.Size:
		return fmt.Errorf("downloading %s: got %d bytes, want %d (the download is cut off, or it is not the pinned archive)", url, n, a.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != a.SHA256 {
		return fmt.Errorf("downloading %s: SHA-256 mismatch: got %s, pinned %s (nothing was unpacked)", url, got, a.SHA256)
	}
	return nil
}

// httpsRedirects is the redirect policy of the download: a redirect may go to another host (GitHub
// serves its release assets from a storage host; the SHA-256 vouches for the contents), but only
// over https, so that a redirect can't take the download off the encrypted channel.
func httpsRedirects(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("redirect to %s: only https is accepted", req.URL.Redacted())
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// idleReader restarts the idle timer at every read that returns data.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
	idle  time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.timer.Reset(r.idle)
	}
	return n, err
}

// sweepStale removes what a gocpp that was killed in the middle of a download left in the toolchain folder: the
// .download-<pid>-* folders (about a gigabyte each once unpacked). A folder whose process is gone is the leavings
// of a dead download and goes at once; one whose process lives is a running download and stays; one that
// carries no process number (an older gocpp) goes when it is a day old.
func sweepStale(parent string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		rest, ok := strings.CutPrefix(e.Name(), ".download-")
		if !ok {
			continue
		}
		switch pid, hasPID := downloadPID(rest); {
		case hasPID && processAlive(pid):
			continue
		case !hasPID:
			if fi, err := e.Info(); err != nil || time.Since(fi.ModTime()) < 24*time.Hour {
				continue
			}
		}
		p := filepath.Join(parent, e.Name())
		MakeWritable(p)
		os.RemoveAll(p)
	}
}

// downloadPID reads the process number from the name of a download folder after ".download-": "1234-567".
func downloadPID(rest string) (int, bool) {
	num, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	pid, err := strconv.Atoi(num)
	return pid, err == nil && pid > 0
}

// compilerName is the program that tells a finished toolchain.
const compilerName = "x86_64-w64-mingw32-clang++"

// unpack writes the tar archive (xz-compressed when compressed is set) into root, without its
// top-level folder. It writes files, folders and relative symbolic links that stay inside the
// archive; anything else (an absolute or ".." path, a link that leaves the tree, a hard link, a
// device) is an error. The writes go through os.Root, so even a symbolic link that an earlier
// entry made can't take a later file out of root.
func unpack(r io.Reader, root string, compressed bool) error {
	if compressed {
		z, err := xz.NewReader(r)
		if err != nil {
			return err
		}
		r = z
	}
	rt, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer rt.Close()
	tr := tar.NewReader(r)
	var total int64
	top := ""
	links := linkSet{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name, first, err := stripTop(h.Name)
		if err != nil {
			return err
		}
		if top == "" {
			top = first
		} else if first != top {
			return fmt.Errorf("%s: the archive has more than one top-level folder (%s, %s)", h.Name, top, first)
		}
		if name == "" {
			continue // the top-level folder itself
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := rt.MkdirAll(name, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += h.Size
			if total > maxUnpacked {
				return errors.New("the archive unpacks to more than 8 GiB")
			}
			if err := rt.MkdirAll(path.Dir(name), 0o755); err != nil {
				return err
			}
			mode := fs.FileMode(0o644)
			if h.Mode&0o111 != 0 {
				mode = 0o755
			}
			f, err := rt.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return fmt.Errorf("%s: %w", h.Name, err)
			}
		case tar.TypeSymlink:
			if err := links.add(name, h.Linkname); err != nil {
				return fmt.Errorf("%s: %w", h.Name, err)
			}
			if err := rt.MkdirAll(path.Dir(name), 0o755); err != nil {
				return err
			}
			// The target has '/' in the archive; Windows wants its own separator in a relative target.
			if err := rt.Symlink(filepath.FromSlash(h.Linkname), name); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: entry type %q is not accepted (only files, folders and relative links)", h.Name, h.Typeflag)
		}
	}
	if top == "" {
		return errors.New("the archive is empty")
	}
	// The tar reader stops at the end-of-archive blocks, before the end of the compressed stream:
	// read the rest, so that the checks of the last block, the index and the footer run (a cut-off
	// or damaged trailer is an error), and so that bytes after the tar's end are not ignored
	// silently when the compression says they are broken.
	if z, ok := r.(*xz.Reader); ok {
		if _, err := io.Copy(io.Discard, z); err != nil {
			return err
		}
	}
	return nil
}

// stripTop splits an archive path into its first element and the rest, both clean and relative:
// "llvm/bin/clang" is ("bin/clang", "llvm"). The path may not be absolute, hold "..", or be
// empty.
func stripTop(p string) (rest, first string, err error) {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.ContainsRune(p, 0) {
		return "", "", fmt.Errorf("%q is not a relative path of the archive", p)
	}
	for _, e := range strings.Split(strings.TrimSuffix(p, "/"), "/") {
		if e == ".." {
			return "", "", fmt.Errorf("%q has a .. element", p)
		}
	}
	c := path.Clean(p)
	first, rest, _ = strings.Cut(c, "/")
	if first == "." {
		return "", "", fmt.Errorf("%q has no top-level folder", p)
	}
	return rest, first, nil
}

// A linkSet holds the symbolic links an archive has made so far, by their physical names (the
// names of the folders on the way resolved), so that a link is checked as the file system will
// follow it: a link whose target runs through an earlier link (t/d/e -> .., then
// t/d/e/f -> ../..) is judged by where it really goes, not by its spelling.
type linkSet map[string]string

// add checks the link at name (relative to the root) with the target, and records it. The target
// must be relative, and followed from the link's physical folder through the links so far it must
// stay inside the root.
func (l linkSet) add(name, target string) error {
	if target == "" || strings.HasPrefix(target, "/") || strings.Contains(target, "\\") {
		return fmt.Errorf("the link target %q is not a relative path", target)
	}
	dir, err := l.resolve(path.Dir(name))
	if err != nil {
		return err
	}
	if _, err := l.resolve(path.Join(dir, target)); err != nil {
		return fmt.Errorf("the link target %q %w", target, err)
	}
	l[path.Join(dir, path.Base(name))] = target
	return nil
}

// resolve returns the physical form of the relative path p: every link on the way followed. It is
// an error to leave the root by "..", or to follow more than 40 links.
func (l linkSet) resolve(p string) (string, error) {
	parts := strings.Split(p, "/")
	var cur []string
	hops := 0
	for len(parts) > 0 {
		c := parts[0]
		parts = parts[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			if len(cur) == 0 {
				return "", errors.New("leaves the toolchain's folder")
			}
			cur = cur[:len(cur)-1]
			continue
		}
		cur = append(cur, c)
		if target, ok := l[strings.Join(cur, "/")]; ok {
			if hops++; hops > 40 {
				return "", errors.New("goes through too many links")
			}
			cur = cur[:len(cur)-1]
			parts = append(strings.Split(target, "/"), parts...)
		}
	}
	return strings.Join(cur, "/"), nil
}

// readOnly takes the write permission from every file and folder of root (the folders last, so
// that the walk can still enter them).
func readOnly(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			dirs = append(dirs, p)
		case d.Type()&fs.ModeSymlink == 0:
			fi, err := d.Info()
			if err != nil {
				return err
			}
			return os.Chmod(p, fi.Mode().Perm()&^0o222)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil // the read-only attribute of a folder means nothing on Windows but trouble when it is removed
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i], 0o555); err != nil {
			return err
		}
	}
	return nil
}
