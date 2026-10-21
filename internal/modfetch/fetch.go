package modfetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/internal/fsretry"
	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// A Fetcher downloads modules into the module cache. It is safe for
// concurrent use, and so are several Fetchers (and processes) with the same
// cache.
type Fetcher struct {
	cfg *Config

	refMu    sync.Mutex
	refCache map[string]*RefList // by module path, for Versions and Query

	clientOnce sync.Once // the client is made at the first download: a module that is in the cache needs none
	client     *http.Client
	clientErr  error
}

// New returns a Fetcher for the configuration.
func New(cfg *Config) (*Fetcher, error) {
	if cfg.Limits == (Limits{}) {
		cfg.Limits = DefaultLimits
	}
	return &Fetcher{cfg: cfg}, nil
}

// httpClient returns the HTTP client, made from the configuration at the first call.
func (f *Fetcher) httpClient() (*http.Client, error) {
	f.clientOnce.Do(func() { f.client, f.clientErr = f.cfg.httpClient() })
	return f.client, f.clientErr
}

// Config returns the configuration of the Fetcher.
func (f *Fetcher) Config() *Config { return f.cfg }

func (f *Fetcher) progress(format string, args ...any) {
	if f.cfg.Stderr != nil {
		fmt.Fprintf(f.cfg.Stderr, "gocpp: "+format+"\n", args...)
	}
}

func (f *Fetcher) timeout() time.Duration {
	if f.cfg.Timeout > 0 {
		return f.cfg.Timeout
	}
	return DefaultTimeout
}

// A Module is a module version in the module cache.
type Module struct {
	Path, Version string

	// Dir is its folder in the cache. Its files are read-only.
	Dir string

	// Hash is the "h1:" hash of the files, ModHash that of the gocpp.mod file
	// ("" if the module has none).
	Hash, ModHash string

	// GoModHash is the hash of the go.mod file of the module, named go.mod
	// ("" if it has none): a module that comes from a Go module proxy has one
	// and no gocpp.mod, and gocpp.sum lists it as Go's go.sum does.
	GoModHash string

	// Downloaded is true if this call downloaded it; false if it was in the
	// cache already.
	Downloaded bool
}

// An OfflineError says that a module is not in the module cache and the
// network is not allowed.
type OfflineError struct {
	Path, Version string
	// Why is set when the process, not the environment, forbids downloads
	// (the language server, a module listing): the message then says why.
	Why string
}

func (e *OfflineError) Error() string {
	if e.Why != "" {
		return fmt.Sprintf("module %s@%s is not in the module cache, and %s", e.Path, e.Version, e.Why)
	}
	return fmt.Sprintf("module %s@%s is not in the module cache and downloads are off (%s=1): "+
		"run \"gocpp mod download\" with the network on, or \"gocpp mod vendor\" for a vendor folder", e.Path, e.Version, EnvOffline)
}

// A ModuleNotFoundError says that no source has the archive.
type ModuleNotFoundError struct {
	Path, Version string
	Tried         []string
}

func (e *ModuleNotFoundError) Error() string {
	return fmt.Sprintf("module %s@%s was not found; tried:\n\t%s\nCheck that the version is a tag of the repository (gocpp get %s@latest picks the newest one).",
		e.Path, e.Version, strings.Join(e.Tried, "\n\t"), e.Path)
}

// refFor returns what to download for a module version.
func refFor(repo *Repo, version string) ref {
	if c, _ := PseudoCommit(version); c != "" {
		return ref{Version: version, Commit: c}
	}
	return ref{Version: version, Tag: repo.Tag(version)}
}

// Download makes sure that the module version is in the module cache,
// downloading and checking it if not, and returns it. sums, if not nil, is the
// gocpp.sum file: a module whose hash is listed there with another value is a
// SecurityError, and a download that does not match is not kept in the cache.
// Download does not record anything in gocpp.sum: Fetch does.
func (f *Fetcher) Download(ctx context.Context, path, version string, sums *Sums) (*Module, error) {
	repo, err := ParseModulePath(path)
	if err != nil {
		return nil, err
	}
	if err := repo.CheckVersion(version); err != nil {
		return nil, err
	}
	if f.cfg.CacheDir == "" {
		return nil, fmt.Errorf("no module cache folder (set %s)", EnvModCache)
	}
	if m, err := f.cached(ctx, path, version, sums); m != nil || err != nil {
		return m, err
	}
	if f.cfg.Offline {
		return nil, &OfflineError{Path: path, Version: version, Why: f.cfg.OfflineWhy}
	}
	dir := f.ModDir(path, version)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, f.cacheError(err)
	}
	unlock, err := f.lockFile(ctx, "mod-"+hashName(dir))
	if err != nil {
		return nil, err
	}
	defer unlock()
	if m, err := f.cached(ctx, path, version, sums); m != nil || err != nil {
		return m, err // another gocpp was faster
	}
	st, err := f.stage(ctx, repo, refFor(repo, version))
	if err != nil {
		return nil, err
	}
	defer st.cleanup()
	return f.publish(ctx, st, repo, version, sums)
}

// cached returns the module if it is in the cache.
func (f *Fetcher) cached(ctx context.Context, path, version string, sums *Sums) (*Module, error) {
	dir := f.ModDir(path, version)
	st, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, f.cacheError(err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("module cache: %s is not a folder (run: gocpp clean -modcache)", dir)
	}
	m := &Module{Path: path, Version: version, Dir: dir}
	m.Hash, m.ModHash, err = readSidecar(f.sidecar(path, version))
	if err != nil {
		// A cache from a gocpp that was stopped between the rename and the
		// hash file, or a hash file that was deleted: hash the files.
		if m.Hash, err = HashDir(ctx, dir, path, version); err != nil {
			return nil, fmt.Errorf("module cache: %s: %v", dir, err)
		}
		m.ModHash = modFileHash(dir)
		f.writeSidecar(path, version, m.Hash, m.ModHash)
	}
	m.GoModHash = namedFileHash(dir, GoModName)
	if err := verifySums(sums, m, true); err != nil {
		return nil, err
	}
	return m, nil
}

func modFileHash(dir string) string { return namedFileHash(dir, ModFileName) }

// namedFileHash is the hash of the file name in dir, or "" if there is none.
func namedFileHash(dir, name string) string {
	p := filepath.Join(dir, name)
	if st, err := os.Lstat(p); err != nil || !st.Mode().IsRegular() {
		return ""
	}
	h, err := HashFile(p, name)
	if err != nil {
		return ""
	}
	return h
}

// writeSidecar writes the hash file of a module in the cache, unless there is
// one. It is a cache of the hash: a failure is not an error.
func (f *Fetcher) writeSidecar(path, version, hash, modHash string) {
	name := f.sidecar(path, version)
	tmp, err := os.CreateTemp(filepath.Dir(name), ".dl-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(sidecarText(hash, modHash))
	if cerr := tmp.Close(); werr != nil || cerr != nil {
		return
	}
	os.Chmod(tmp.Name(), 0o444)
	os.Link(tmp.Name(), name) // fails if there is a file: that one stays
}

// verifySums compares the hashes of a module with gocpp.sum.
func verifySums(sums *Sums, m *Module, fromCache bool) error {
	if sums == nil {
		return nil
	}
	cached := ""
	if fromCache {
		cached = m.Dir
	}
	if want := sums.Lookup(m.Path, m.Version, ""); len(want) > 0 && !slices.Contains(want, m.Hash) {
		return &SecurityError{Path: m.Path, Version: m.Version, Got: m.Hash, Want: want, Cached: cached, SumFile: sums.Name}
	}
	if want := sums.Lookup(m.Path, m.Version, "/"+ModFileName); len(want) > 0 && !slices.Contains(want, m.ModHash) {
		got := m.ModHash
		if got == "" {
			got = "(the module has no " + ModFileName + " file)"
		}
		return &SecurityError{Path: m.Path, Version: m.Version, What: ModFileName, Got: got, Want: want, Cached: cached, SumFile: sums.Name}
	}
	// A module with no go.mod file in its archive (a C library, a Go module from
	// before modules) has a go.mod line for the file that the module proxy made up
	// ("module path"): it is checked when the .mod file is fetched, not here.
	if want := sums.Lookup(m.Path, m.Version, "/"+GoModName); len(want) > 0 && m.GoModHash != "" && !slices.Contains(want, m.GoModHash) {
		got := m.GoModHash
		return &SecurityError{Path: m.Path, Version: m.Version, What: GoModName, Got: got, Want: want, Cached: cached, SumFile: sums.Name}
	}
	return nil
}

// A staged is a module extracted next to its place in the cache.
type staged struct {
	root string // the module's folder, writable, in the cache folder of its path
	info *ExtractInfo
	src  string // the name of the source that had the archive
	tmps []string
}

func (st *staged) cleanup() {
	for _, p := range st.tmps {
		RemoveAll(p)
	}
	if st.root != "" {
		RemoveAll(st.root)
	}
}

// stage downloads and extracts the archive of r into a folder in the parent
// folder of the module (so that the last step is a rename in one folder), and
// checks its structure. The caller publishes it, or cleans up.
func (f *Fetcher) stage(ctx context.Context, repo *Repo, r ref) (res *staged, err error) {
	parent := filepath.Join(f.cfg.CacheDir, filepath.FromSlash(escape(repo.Module)))
	parent = filepath.Dir(parent)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, f.cacheError(err)
	}
	staleTemp(parent)
	st := &staged{}
	defer func() {
		if err != nil {
			st.cleanup()
		}
	}()
	dl, err := os.CreateTemp(parent, ".dl-*")
	if err != nil {
		return nil, f.cacheError(err)
	}
	st.tmps = append(st.tmps, dl.Name())
	what := r.Version
	if what == "" {
		what = r.Commit
	}
	f.progress("downloading %s %s", repo.Module, what)
	dctx, cancel := context.WithTimeout(ctx, f.timeout())
	st.src, err = f.download(dctx, repo, r, dl)
	cancel()
	if cerr := dl.Close(); err == nil && cerr != nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(parent, ".tmp-")
	if err != nil {
		return nil, f.cacheError(err)
	}
	st.tmps = append(st.tmps, tmp)
	subdirDone := false
	info, err := Extract(ctx, dl.Name(), tmp, f.cfg.Limits)
	if err != nil {
		return nil, fmt.Errorf("%s %s (from %s): %w", repo.Module, what, st.src, err)
	}
	st.info = info
	root := info.Root
	// A module proxy's zip (Go's layout) has all its files in
	// <module path>@<version>/. An archive of a mirror may instead have one folder
	// like a git host's; a folder with a @ in its name that is not the right one,
	// or a Go layout with another prefix, is refused.
	pr := filepath.Join(tmp, filepath.FromSlash(repo.Module+"@"+r.Version))
	fromProxyZip := r.Version != "" && info.Zip && strings.HasPrefix(st.src, "proxy ")
	switch top := filepath.Base(root); {
	case fromProxyZip && top == repo.Module+"@"+r.Version:
		subdirDone = true // the files are in the top folder
	case fromProxyZip && (strings.Contains(top, "@") || top == strings.SplitN(repo.Module, "/", 2)[0]):
		if fi, err := os.Lstat(pr); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("%s %s (from %s): the zip does not have its files in %s@%s/", repo.Module, what, st.src, repo.Module, r.Version)
		}
		root = pr
		subdirDone = true
	case r.Version != "" && top == strings.SplitN(repo.Module, "/", 2)[0]:
		if fi, err := os.Lstat(pr); err == nil && fi.IsDir() {
			root = pr
			subdirDone = true
		}
	}
	if repo.Subdir != "" && !subdirDone {
		root = filepath.Join(root, filepath.FromSlash(repo.Subdir))
		if fi, err := os.Lstat(root); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("%s %s: the archive has no folder %s (the module is in a folder of the repository)", repo.Module, what, repo.Subdir)
		}
	}
	if ents, err := os.ReadDir(root); err != nil || len(ents) == 0 {
		return nil, fmt.Errorf("%s %s: the module is empty", repo.Module, what)
	}
	if r.Commit != "" && info.Commit != "" && !strings.HasPrefix(info.Commit, r.Commit) {
		return nil, fmt.Errorf("%s: the archive from %s is for the commit %s, not %s", repo.Module, st.src, info.Commit, r.Commit)
	}
	// Move the module's folder next to its final place, so that sealing
	// it and the last rename stay in one folder.
	mod := tmp + ".mod"
	if err := fsretry.Rename(root, mod); err != nil {
		return nil, err
	}
	st.root = mod
	RemoveAll(tmp)
	st.tmps = st.tmps[:len(st.tmps)-1]
	// Nested modules, version control and vendor folders are not the module's
	// files (modzip.go): the tree, its hash and the zip of the same module agree.
	if err := PruneModule(ctx, mod); err != nil {
		return nil, fmt.Errorf("%s %s: %v", repo.Module, what, err)
	}
	// The module's gocpp.mod (else its go.mod) must name the module that was asked for.
	modName := ModFileName
	if _, err := os.Lstat(filepath.Join(mod, ModFileName)); err != nil {
		modName = GoModName
	}
	if data, err := os.ReadFile(filepath.Join(mod, modName)); err == nil {
		file, errs := gocppmod.Parse(modName, data, nil)
		switch {
		case file == nil || file.Module == nil:
			msg := "it has no module line"
			if len(errs) > 0 {
				msg = errs[0].Error()
			}
			return nil, fmt.Errorf("%s %s: its %s has no valid module line (%s)", repo.Module, what, modName, msg)
		case file.Module.Path != repo.Module:
			return nil, fmt.Errorf("%s %s: its %s declares the module path %s, not %s (a fork needs a replace line)",
				repo.Module, what, modName, file.Module.Path, repo.Module)
		}
	}
	return st, nil
}

// download gets the archive of r from the first source that has it.
func (f *Fetcher) download(ctx context.Context, repo *Repo, r ref, dst *os.File) (string, error) {
	srcs, err := f.cfg.sourcesFor(repo)
	if err != nil {
		return "", err
	}
	var tried, errs []string
	for _, s := range srcs {
		urls, err := s.archiveURLs(r)
		if err != nil {
			errs = append(errs, s.name+": "+err.Error())
			if s.next(err) {
				continue
			}
			return "", err
		}
		for _, u := range urls {
			if err := dst.Truncate(0); err != nil {
				return "", err
			}
			if _, err := dst.Seek(0, io.SeekStart); err != nil {
				return "", err
			}
			_, err := f.get(ctx, u, dst, f.cfg.Limits.MaxArchive, nil)
			if err == nil {
				return s.name, nil
			}
			var nf *NotFoundError
			if errors.As(err, &nf) {
				tried = append(tried, err.Error())
				continue
			}
			tried = append(tried, err.Error())
			if s.next(err) {
				break
			}
			return "", fmt.Errorf("downloading %s %s: %w", repo.Module, r.Version, err)
		}
	}
	if len(tried) == 0 && len(errs) > 0 {
		return "", errors.New(strings.Join(errs, "\n"))
	}
	return "", &ModuleNotFoundError{Path: repo.Module, Version: firstNonEmpty(r.Version, r.Commit), Tried: append(tried, errs...)}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// publish hashes a staged module, checks the hashes against gocpp.sum, makes
// it read-only and moves it into the cache. The caller holds the lock of the
// module, or does not mind the race.
func (f *Fetcher) publish(ctx context.Context, st *staged, repo *Repo, version string, sums *Sums) (*Module, error) {
	path := repo.Module
	m := &Module{Path: path, Version: version, Dir: f.ModDir(path, version), Downloaded: true}
	var err error
	if m.Hash, err = HashDir(ctx, st.root, path, version); err != nil {
		return nil, fmt.Errorf("%s %s: %v", path, version, err)
	}
	m.ModHash = modFileHash(st.root)
	m.GoModHash = namedFileHash(st.root, GoModName)
	if err := verifySums(sums, m, false); err != nil {
		return nil, err
	}
	if err := Seal(st.root); err != nil {
		return nil, err
	}
	err = fsretry.Rename(st.root, m.Dir)
	if err != nil {
		// Another gocpp was faster, if the folder is there now with the same hash.
		if other, oerr := f.cached(ctx, path, version, nil); oerr == nil && other != nil {
			if other.Hash == m.Hash {
				other.Downloaded = false
				return other, nil
			}
			return nil, fmt.Errorf("the module cache already has %s %s with the hash %s, but this download has %s: "+
				"the host served different files to two downloads; nothing was changed", path, version, other.Hash, m.Hash)
		}
		return nil, f.cacheError(err)
	}
	st.root = ""
	f.writeSidecar(path, version, m.Hash, m.ModHash)
	return m, nil
}

// FetchOptions are the options of Fetch.
type FetchOptions struct {
	// Record adds the hash of a module that gocpp.sum does not list to
	// gocpp.sum (the first download is trusted). Without it, such a module
	// is an error and nothing is downloaded.
	Record bool

	// Pending, if set (with Record), collects the lines to add instead of
	// writing gocpp.sum: the caller writes them with SavePending when
	// everything else has worked (gocpp get changes gocpp.mod and gocpp.sum
	// together). The lines of Pending count as listed for the next Fetch with
	// the same Pending.
	Pending *PendingSums

	// NoArchive is for ModFile: a module file that can be read only by
	// downloading the whole module (a git host has no smaller file) is not
	// read: the error is ErrArchiveNeeded. A module that is in the cache, or
	// whose module file a module proxy serves alone, is read as usual.
	NoArchive bool
}

// ErrArchiveNeeded is the error of ModFile with NoArchive for a module that is
// not in the module cache and has no source for its module file alone.
var ErrArchiveNeeded = errors.New("the module file is only in the module's archive, which is not downloaded for this")

// A MissingSumError says that gocpp.sum has no line for a module and may
// not get one.
type MissingSumError struct {
	Path, Version, Name string
	Line                string // "" for the line of the module, "gocpp.mod" for the line of its module file
}

func (e *MissingSumError) Error() string {
	what := e.Path + " " + e.Version
	if e.Line != "" {
		what += "/" + e.Line
	}
	return fmt.Sprintf("%s: missing entry for %s (%s is read-only here: %s=-mod=readonly, or the language server); "+
		"run \"gocpp mod download %s\" to add it, after checking that you trust the module",
		e.Name, what, SumFileName, EnvModFlags, e.Path)
}

// Fetch makes sure that the module version is in the cache, checks it against
// the gocpp.sum file in mainDir (the main module's folder), and, with
// opts.Record, adds the missing hash lines to it.
func (f *Fetcher) Fetch(ctx context.Context, mainDir, path, version string, opts FetchOptions) (*Module, error) {
	sums, err := LoadSums(mainDir)
	if err != nil {
		return nil, err
	}
	opts.Pending.overlay(sums)
	if !opts.Record && !f.cfg.Offline && len(sums.Lookup(path, version, "")) == 0 {
		// Before anything is downloaded. (Offline, the module may be in the cache:
		// "not in the cache" is the better message, so the check is after Download.)
		return nil, &MissingSumError{Path: path, Version: version, Name: SumFileName}
	}
	m, err := f.Download(ctx, path, version, sums)
	if err != nil {
		return nil, err
	}
	if !opts.Record {
		// Read-only: nothing is written, and a module whose gocpp.mod line is
		// missing is refused like one whose module line is.
		if len(sums.Lookup(path, version, "")) == 0 {
			return nil, &MissingSumError{Path: path, Version: version, Name: SumFileName}
		}
		if m.ModHash != "" && len(sums.Lookup(path, version, "/"+ModFileName)) == 0 {
			return nil, &MissingSumError{Path: path, Version: version, Name: SumFileName, Line: ModFileName}
		}
		if m.GoModHash != "" && len(sums.Lookup(path, version, "/"+GoModName)) == 0 {
			return nil, &MissingSumError{Path: path, Version: version, Name: SumFileName, Line: GoModName}
		}
		return m, nil
	}
	return m, f.record(sums, m, opts.Pending)
}

// record adds the hash lines of m that gocpp.sum does not have (to pending,
// if it is not nil).
func (f *Fetcher) record(sums *Sums, m *Module, pending *PendingSums) error {
	var add []SumLine
	if len(sums.Lookup(m.Path, m.Version, "")) == 0 {
		add = append(add, SumLine{m.Path, m.Version, m.Hash})
	}
	if m.ModHash != "" && len(sums.Lookup(m.Path, m.Version, "/"+ModFileName)) == 0 {
		add = append(add, SumLine{m.Path, m.Version + "/" + ModFileName, m.ModHash})
	}
	if m.GoModHash != "" && len(sums.Lookup(m.Path, m.Version, "/"+GoModName)) == 0 {
		add = append(add, SumLine{m.Path, m.Version + "/" + GoModName, m.GoModHash})
	}
	if len(add) == 0 {
		return nil
	}
	if pending != nil {
		for _, l := range add {
			pending.add(l)
			sums.add(l)
		}
		return nil
	}
	changed, err := f.saveSums(sums.Name, add)
	if changed {
		f.progress("recorded %s@%s in %s (use GOCPP_MODFLAGS=-mod=readonly in CI to refuse modules that it does not list)", m.Path, m.Version, SumFileName)
	}
	return err
}

var forceOffline atomic.Int32

// SetOffline(true) makes Dir (the loader's entry) refuse every download and
// never write gocpp.sum in this process, whatever the environment says; the
// language server uses it, since an editor must not start downloads or change
// the user's project. Calls nest: each SetOffline(true) needs its own
// SetOffline(false).
func SetOffline(off bool) {
	if off {
		forceOffline.Add(1)
	} else if forceOffline.Add(-1) < 0 {
		forceOffline.Store(0)
	}
}

// Dir returns the folder of the module version in the module cache, for the
// loader: it downloads the module if it is not there yet and the environment
// allows it (GOCPP_OFFLINE, GOCPP_MODFLAGS, GOCPP_MODCACHE ... in
// docs/spec/modules.md), and checks it against, or adds it to, the gocpp.sum
// of the main module in mainDir. The error text is for a user.
func Dir(mainDir, path, version string) (string, error) {
	return dir(mainDir, path, version, "")
}

// LookupDir is Dir that never downloads and never writes gocpp.sum: for a
// module that is not in the module cache it is an error (listing the modules
// of a project).
func LookupDir(mainDir, path, version string) (string, error) {
	return dir(mainDir, path, version, "listing modules does not download them: run \"gocpp mod download\" in the project")
}

func dir(mainDir, path, version, noDownload string) (string, error) {
	cfg, err := ConfigFromEnv(os.Getenv, os.Stderr)
	if err != nil {
		return "", err
	}
	record := !cfg.ReadOnly
	if noDownload == "" && forceOffline.Load() > 0 {
		noDownload = "the language server does not download modules: run \"gocpp mod download\" in the project"
	}
	if noDownload != "" {
		cfg.Offline, cfg.OfflineWhy, record = true, noDownload, false
	}
	f, err := New(cfg)
	if err != nil {
		return "", err
	}
	m, err := f.Fetch(context.Background(), mainDir, path, version, FetchOptions{Record: record})
	if err != nil {
		return "", err
	}
	return m.Dir, nil
}
