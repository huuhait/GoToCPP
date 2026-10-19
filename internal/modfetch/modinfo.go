package modfetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/fsretry"
	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// The requirements of a module version, and the versions that a module has:
// what version selection (internal/modgraph) and "gocpp get" (internal/modget)
// ask of the sources.

// A ModFileInfo is the module file of one module version.
type ModFileInfo struct {
	// Data is the text of the file; Name is gocpp.mod or go.mod, or "" (and
	// Data is nil) for a module that has none, such as a C library.
	Data []byte
	Name string

	// Hash is the hash of the file as gocpp.sum lists it (named Name).
	Hash string

	// Dir is the folder of the module in the cache, or "" when only the .mod file
	// of a module proxy was fetched.
	Dir string
}

// maxModFileSize bounds a module file.
const maxModFileSize = 16 << 20

// ReadModFileDir reads the module file of the module in dir: its gocpp.mod, or
// else its go.mod. A module without either has Name "".
func ReadModFileDir(dir string) (*ModFileInfo, error) {
	for _, name := range []string{ModFileName, GoModName} {
		p := filepath.Join(dir, name)
		if st, err := os.Lstat(p); err != nil || !st.Mode().IsRegular() {
			continue
		}
		data, err := ReadRegular(p, maxModFileSize)
		if err != nil {
			return nil, err
		}
		h, err := HashFile(p, name)
		if err != nil {
			return nil, err
		}
		return &ModFileInfo{Data: data, Name: name, Hash: h, Dir: dir}, nil
	}
	return &ModFileInfo{Dir: dir}, nil
}

// modCacheFile is the cached .mod file of a module proxy ("~mod" can't be in a
// version: no module folder has this name).
func (f *Fetcher) modCacheFile(path, version string) string { return f.ModDir(path, version) + "~mod" }

// ModFile returns the module file of the module version: from the module
// cache if the module is there, else from the .mod file of a module proxy (a
// small request), else by downloading the module (a git host has no other way:
// the file is in the archive). The hash is checked against gocpp.sum, and with
// opts.Record the lines that are missing are added to it; without Record, a
// line that is missing is an error (MissingSumError).
func (f *Fetcher) ModFile(ctx context.Context, mainDir, path, version string, opts FetchOptions) (*ModFileInfo, error) {
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
	sums, err := LoadSums(mainDir)
	if err != nil {
		return nil, err
	}
	opts.Pending.overlay(sums)
	fromTree := func(m *Module) (*ModFileInfo, error) {
		mi, err := ReadModFileDir(m.Dir)
		if err != nil {
			return nil, err
		}
		if opts.Record {
			err = f.record(sums, m, opts.Pending)
		} else if len(sums.Lookup(m.Path, m.Version, "")) == 0 {
			err = &MissingSumError{Path: path, Version: version, Name: SumFileName}
		}
		return mi, err
	}
	if m, err := f.cached(ctx, path, version, sums); err != nil {
		return nil, err
	} else if m != nil {
		return fromTree(m)
	}
	cacheFile := f.modCacheFile(path, version)
	if data, err := ReadRegular(cacheFile, maxModFileSize); err == nil {
		return f.checkProxyMod(sums, path, version, data, opts, false)
	}
	if f.cfg.Offline {
		return nil, &OfflineError{Path: path, Version: version, Why: f.cfg.OfflineWhy}
	}
	srcs, err := f.cfg.sourcesFor(repo)
	if err != nil {
		return nil, err
	}
	var tried []string
	for _, s := range srcs {
		if s.proxy == "" {
			break // a git host: the archive has the file
		}
		u := s.proxy + "/" + escSegs(escape(repo.Module)) + "/@v/" + escSegs(escape(version)) + ".mod"
		var buf bytes.Buffer
		dctx, cancel := context.WithTimeout(ctx, f.timeout())
		_, err := f.get(dctx, u, &buf, maxModFileSize, nil)
		cancel()
		if err == nil {
			return f.checkProxyMod(sums, path, version, buf.Bytes(), opts, true)
		}
		tried = append(tried, err.Error())
		if !s.next(err) {
			return nil, fmt.Errorf("%s@%s: %w", path, version, err)
		}
	}
	if len(tried) > 0 && len(tried) == len(srcs) {
		return nil, &ModuleNotFoundError{Path: path, Version: version, Tried: tried}
	}
	m, err := f.Fetch(ctx, mainDir, path, version, opts)
	if err != nil {
		return nil, err
	}
	return ReadModFileDir(m.Dir)
}

// checkProxyMod checks the .mod file of a module proxy against gocpp.sum
// (as its go.mod line), records it, and keeps a copy in the cache if fresh.
func (f *Fetcher) checkProxyMod(sums *Sums, path, version string, data []byte, opts FetchOptions, fresh bool) (*ModFileInfo, error) {
	h, err := Hash1([]string{GoModName}, func(string) (readCloser, error) { return newBytesReader(data), nil })
	if err != nil {
		return nil, err
	}
	want := sums.Lookup(path, version, "/"+GoModName)
	switch {
	case len(want) > 0 && !slices.Contains(want, h):
		// A cached copy that does not match is never used again.
		if !fresh {
			os.Remove(f.modCacheFile(path, version))
		}
		return nil, &SecurityError{Path: path, Version: version, What: GoModName, Got: h, Want: want, SumFile: sums.Name}
	case len(want) == 0 && !opts.Record:
		return nil, &MissingSumError{Path: path, Version: version, Name: SumFileName, Line: GoModName}
	}
	if fresh {
		f.writeModCache(path, version, data)
	}
	if len(want) == 0 && opts.Pending != nil {
		opts.Pending.add(SumLine{path, version + "/" + GoModName, h})
		sums.add(SumLine{path, version + "/" + GoModName, h})
	} else if len(want) == 0 {
		if changed, err := f.saveSums(sums.Name, []SumLine{{path, version + "/" + GoModName, h}}); err != nil {
			return nil, err
		} else if changed {
			f.progress("recorded %s@%s/%s in %s", path, version, GoModName, SumFileName)
		}
	}
	return &ModFileInfo{Data: data, Name: GoModName, Hash: h}, nil
}

// writeModCache keeps the .mod file of a proxy in the cache, atomically. It is
// a cache: a failure is not an error.
func (f *Fetcher) writeModCache(path, version string, data []byte) {
	name := f.modCacheFile(path, version)
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".dl-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr != nil || cerr != nil {
		return
	}
	os.Chmod(tmp.Name(), 0o444)
	os.Link(tmp.Name(), name)
}

// ParseModFile reads a module file (gocpp.mod, go.mod or the .mod file of a
// proxy), for the requirements of a module that gocpp downloaded. Unlike the
// main module's file, any error makes the file unusable: the go command stops at
// the first one too.
func ParseModFile(name string, data []byte) (*gocppmod.File, error) {
	file, errs := gocppmod.Parse(name, data, nil)
	if len(errs) > 0 {
		return nil, errors.New(errs[0].Error())
	}
	return file, nil
}

// Versions returns the versions of the module that its source lists, in
// increasing order of precedence: the tags vX.Y.Z (or X.Y.Z) of the
// repository (of the folder of the module in it), or the list of a module
// proxy. A version v2.0.0 or higher of a module whose path has no /vN suffix is
// written with +incompatible, as Go does. Nothing is downloaded: the list is a
// small request that is made once for each module in a process.
func (f *Fetcher) Versions(ctx context.Context, path string) ([]string, error) {
	repo, err := ParseModulePath(path)
	if err != nil {
		return nil, err
	}
	rl, err := f.refList(ctx, repo)
	if err != nil {
		return nil, err
	}
	return incompatibles(repo, rl.versions(repo)), nil
}

// refList is listRefs, kept for the Fetcher's lifetime.
func (f *Fetcher) refList(ctx context.Context, repo *Repo) (*RefList, error) {
	if f.cfg.Offline {
		return nil, fmt.Errorf("can't list the versions of %s while downloads are off (%s=1): name a version, such as %s@v1.2.3", repo.Module, EnvOffline, repo.Module)
	}
	f.refMu.Lock()
	defer f.refMu.Unlock()
	if rl, ok := f.refCache[repo.Module]; ok {
		return rl, nil
	}
	dctx, cancel := context.WithTimeout(ctx, f.timeout())
	defer cancel()
	rl, err := f.listRefs(dctx, repo)
	if err != nil {
		return nil, err
	}
	if f.refCache == nil {
		f.refCache = map[string]*RefList{}
	}
	f.refCache[repo.Module] = rl
	return rl, nil
}

// incompatibles rewrites the versions v2 and above of a module whose path has no
// /vN suffix as vN.x.y+incompatible.
func incompatibles(repo *Repo, vs []string) []string {
	if repo.Major != 0 {
		return vs
	}
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v
		if semverMajor(v) >= 2 && !strings.HasSuffix(v, "+incompatible") {
			out[i] = v + "+incompatible"
		}
	}
	return out
}

// A QueryOptions says what a Query may pick.
type QueryOptions struct {
	// Current is the version that the build list has now, for the queries
	// "upgrade" and "patch" ("" if the module is not in it).
	Current string

	// Allowed, if not nil, says whether a version may be used (the exclude
	// lines of the main module).
	Allowed func(version string) bool

	// KeepRetracted lets a retracted version be the answer of "latest".
	KeepRetracted bool

	// Pending, if set, collects the gocpp.sum lines that the query needs
	// (the module files that it reads) instead of writing the file.
	Pending *PendingSums
}

// A QueryResult is the version that a query names.
type QueryResult struct {
	Version string

	// Retracted is not empty when the version is retracted by its module
	// (the rationale, or "retracted by the module author").
	Retracted string

	// Module is set when the answer needed the module itself (a commit or a
	// branch, which is named by a pseudo-version): it is in the cache.
	Module *Module
}

// retractionsOf returns the retract lines of the module file of a version.
func (f *Fetcher) retractionsOf(ctx context.Context, mainDir, path, version string, pending *PendingSums) ([]*gocppmod.Retract, error) {
	mi, err := f.ModFile(ctx, mainDir, path, version, FetchOptions{Record: !f.cfg.ReadOnly, Pending: pending})
	if err != nil {
		return nil, err
	}
	if mi.Name == "" {
		return nil, nil
	}
	file, err := ParseModFile(mi.Name, mi.Data)
	if err != nil {
		return nil, fmt.Errorf("%s@%s: parsing %s: %v", path, version, mi.Name, err)
	}
	return file.Retract, nil
}

// Retractions returns the retract lines of the highest version of the module
// (exclude lines ignored), which decide what is retracted, as in Go's
// CheckRetractions; none if the module has no versions.
func (f *Fetcher) Retractions(ctx context.Context, mainDir, path string, pending *PendingSums) ([]*gocppmod.Retract, error) {
	vs, err := f.Versions(ctx, path)
	if err != nil || len(vs) == 0 {
		return nil, err
	}
	return f.retractionsOf(ctx, mainDir, path, vs[len(vs)-1], pending)
}

// Retraction returns the rationale if the module version is retracted by the
// retract lines in the module file of the latest version of the module (Go's
// rule), and "" if it is not. A rationale of a retract line without one is
// "retracted by module author".
func Retraction(rets []*gocppmod.Retract, version string) (string, bool) {
	for _, r := range rets {
		if r.Contains(version) {
			if r.Rationale == "" {
				return "retracted by module author", true
			}
			return r.Rationale, true
		}
	}
	return "", false
}

// Query resolves a version query for a module: the query is "latest",
// "upgrade", "patch", "none" (answered as "none"), a version (itself), a
// prefix v1 or v1.2, a comparison (<v1.5.0, <=, >, >=: the highest version
// that satisfies it), or a branch, a tag or a commit (a pseudo-version). The
// answer is a version of the list of the module, or a pseudo-version.
// Nothing is recorded in gocpp.sum; the module of a commit is downloaded.
func (f *Fetcher) Query(ctx context.Context, mainDir, path, query string, o QueryOptions) (*QueryResult, error) {
	repo, err := ParseModulePath(path)
	if err != nil {
		return nil, err
	}
	if query == "none" {
		return &QueryResult{Version: "none"}, nil
	}
	allowed := func(v string) bool { return o.Allowed == nil || o.Allowed(v) }
	if gocppmod.ValidVersion(query) {
		query = gocppmod.CanonicalVersion(query) // v1.2.3+meta is v1.2.3
		if err := repo.CheckVersion(query); err != nil {
			return nil, err
		}
		// v2 and above of a path with no /vN is +incompatible, as the list of
		// versions says it, so that a later "get" of the module does not flip it
		// to another name of the same tree (and a second sum line).
		if repo.Major == 0 && semverMajor(query) >= 2 && !strings.HasSuffix(query, "+incompatible") {
			query += "+incompatible"
		}
		if !allowed(query) {
			return nil, fmt.Errorf("%s@%s: excluded by an exclude line in %s", path, query, gocppmod.FileName)
		}
		return &QueryResult{Version: gocppmod.CanonicalVersion(query)}, nil
	}
	if f.cfg.Offline {
		return nil, fmt.Errorf("can't resolve %s@%s while downloads are off (%s=1): name a version, such as %s@v1.2.3", path, nonEmpty(query, "latest"), EnvOffline, path)
	}
	rl, err := f.refList(ctx, repo)
	if err != nil {
		return nil, err
	}
	all := incompatibles(repo, rl.versions(repo))
	var vs []string
	for _, v := range all {
		if allowed(v) {
			vs = append(vs, v)
		}
	}
	if query == "" {
		query = "latest"
	}
	// matchesMajor: the version is one that the module path can have (v0 and v1
	// for a path with no /vN, else vN).
	matchesMajor := func(v string) bool {
		if repo.Major == 0 {
			// vN.x.y+incompatible is a version that a path with no /vN can have.
			return semverMajor(v) <= 1 || strings.HasSuffix(v, "+incompatible")
		}
		return semverMajor(v) == repo.Major
	}
	// pick chooses among the versions that pred accepts, as Go's queryProxy does:
	// +incompatible versions count only if preferInc says so (the current version is
	// one, or the query names a major version that the path can't have) or if the
	// highest compatible version has no module file of its own (a library that
	// was never a Go module: its v2 and v3 tags are its releases); releases come
	// before prereleases; lower picks the lowest (the queries > and >=).
	pick := func(pred func(string) bool, preferInc, lower bool, what string) (*QueryResult, error) {
		var cands []string
		needInc := preferInc
		lastCompat := ""
		for _, v := range vs {
			if !pred(v) {
				continue
			}
			if !needInc {
				if !strings.HasSuffix(v, "+incompatible") {
					lastCompat = v
				} else if lastCompat != "" {
					has, err := f.versionHasModFile(ctx, mainDir, path, lastCompat, o.Pending)
					if err != nil {
						return nil, err
					}
					if has {
						break
					}
					needInc = true
				}
			}
			cands = append(cands, v)
		}
		if len(cands) == 0 {
			return nil, &NoMatchError{Path: path, Query: what, Versions: all}
		}
		// The retract lines of the highest version of all decide.
		var rets []*gocppmod.Retract
		if !o.KeepRetracted {
			top := all[len(all)-1] // exclude lines are ignored here, as in Go
			var err error
			if rets, err = f.retractionsOf(ctx, mainDir, path, top, o.Pending); err != nil {
				return nil, err
			}
		}
		var live, prerel []string
		for _, v := range cands {
			if _, bad := Retraction(rets, v); bad {
				continue
			}
			if gocppmod.IsPrerelease(v) {
				prerel = append(prerel, v)
			} else {
				live = append(live, v)
			}
		}
		for _, l := range [][]string{live, prerel} {
			if len(l) > 0 {
				if lower {
					return &QueryResult{Version: l[0]}, nil
				}
				return &QueryResult{Version: l[len(l)-1]}, nil
			}
		}
		return nil, &NoMatchError{Path: path, Query: what, Versions: all, Retracted: true}
	}
	preferInc := strings.HasSuffix(o.Current, "+incompatible")
	switch {
	case query == "latest" || query == "upgrade" || query == "patch":
		if query == "patch" && (o.Current == "" || !gocppmod.ValidVersion(o.Current)) {
			return nil, fmt.Errorf("%s@patch: the module is not required yet, so there is no version to patch", path)
		}
		pred := func(string) bool { return true }
		cur := o.Current != "" && gocppmod.ValidVersion(o.Current)
		switch {
		case query == "upgrade" && cur:
			pred = func(v string) bool { return gocppmod.CompareVersions(v, o.Current) >= 0 }
		case query == "patch":
			pred = func(v string) bool {
				return semverMajorMinor(v) == semverMajorMinor(o.Current) && gocppmod.CompareVersions(v, o.Current) >= 0
			}
		}
		if len(vs) == 0 {
			if query != "latest" && cur {
				return &QueryResult{Version: o.Current}, nil
			}
			return f.revQuery(ctx, mainDir, repo, rl, "latest")
		}
		res, err := pick(pred, preferInc, false, query)
		if err == nil && query != "latest" && cur && res.Version != o.Current {
			// As in Go, "upgrade" and "patch" never move from a pseudo-version to a
			// chronologically older version.
			if older, oerr := f.olderThanPseudo(ctx, mainDir, repo, rl, res.Version, o.Current); oerr != nil {
				return nil, oerr
			} else if older {
				if !allowed(o.Current) {
					return nil, fmt.Errorf("%s@%s: excluded by an exclude line in %s", path, o.Current, gocppmod.FileName)
				}
				return &QueryResult{Version: o.Current}, nil
			}
		}
		var nm *NoMatchError
		if errors.As(err, &nm) && query != "latest" && cur && !nm.Retracted {
			// "upgrade" and "patch" stay on the current version.
			return &QueryResult{Version: o.Current}, nil
		}
		return res, err
	case prefixQuery.MatchString(query):
		bound := prefixBound(query)
		return pick(func(v string) bool {
			// v1.2 matches v1.2.x, but not the prereleases v1.2.0-rc.1 that are below v1.2.0.
			return strings.HasPrefix(v, query+".") && gocppmod.CompareVersions(v, bound) >= 0
		}, preferInc || !matchesMajor(query), false, query)
	}
	for _, op := range []string{"<=", ">=", "<", ">"} {
		rest, ok := strings.CutPrefix(query, op)
		if !ok {
			continue
		}
		isPrefix := prefixQuery.MatchString(rest)
		if !gocppmod.ValidVersion(rest) && !isPrefix {
			return nil, fmt.Errorf("%s@%s: invalid semantic version %q in range %q", path, query, rest, query)
		}
		if isPrefix && (op == "<=" || op == ">") {
			// Refuse to say whether <=v1.2 allows v1.2.3 (v1.2 might mean v1.2.3).
			return nil, fmt.Errorf("%s@%s: ambiguous semantic version %q in range %q", path, query, rest, query)
		}
		bound := rest
		if isPrefix {
			bound = prefixBound(rest)
		}
		return pick(func(v string) bool {
			c := gocppmod.CompareVersions(v, bound)
			return op == "<=" && c <= 0 || op == ">=" && c >= 0 || op == "<" && c < 0 || op == ">" && c > 0
		}, preferInc || !matchesMajor(rest), op[0] == '>', query)
	}
	return f.revQuery(ctx, mainDir, repo, rl, query)
}

// olderThanPseudo reports whether the commit of the release version is older
// than the pseudo-version current (false if current is not a pseudo-version or a
// time is not known). The time of a release is the time of its commit: the
// proxy's .info says it, and a git host's archive carries it (so the archive of
// the tag is downloaded and thrown away; the one "get" keeps comes after).
func (f *Fetcher) olderThanPseudo(ctx context.Context, mainDir string, repo *Repo, rl *RefList, version, current string) (bool, error) {
	_, stamp := PseudoCommit(current)
	if stamp == "" {
		return false, nil
	}
	cur, err := time.Parse("20060102150405", stamp)
	if err != nil {
		return false, nil
	}
	var rel time.Time
	if rl.Versions != nil {
		if _, rel, err = f.proxyInfo(ctx, repo, version); err != nil {
			return false, err
		}
	} else {
		st, err := f.stage(ctx, repo, refFor(repo, strings.TrimSuffix(version, "+incompatible")))
		if err != nil {
			return false, err
		}
		rel = st.info.Time
		st.cleanup()
	}
	return !rel.IsZero() && rel.Before(cur), nil
}

// versionHasModFile reports whether the module version has a module file of
// its own: a gocpp.mod or a go.mod in its archive, or a .mod file from a proxy
// that is not the one that a proxy makes up for a module with none
// ("module <path>").
func (f *Fetcher) versionHasModFile(ctx context.Context, mainDir, path, version string, pending *PendingSums) (bool, error) {
	mi, err := f.ModFile(ctx, mainDir, path, version, FetchOptions{Record: !f.cfg.ReadOnly, Pending: pending})
	if err != nil {
		return false, err
	}
	if mi.Name == "" {
		return false, nil
	}
	// A gocpp.mod is never made up by a proxy; a go.mod that is only the module line may be.
	return mi.Name == ModFileName || !bytes.Equal(mi.Data, []byte("module "+path+"\n")), nil
}

// prefixBound is the version a prefix query stands for: v1 is v1.0.0, v1.2 is v1.2.0.
func prefixBound(q string) string {
	if strings.Count(q, ".") == 0 {
		return q + ".0.0"
	}
	return q + ".0"
}

func semverMajorMinor(v string) string {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}

// A NoMatchError says that no version matches a query.
type NoMatchError struct {
	Path, Query string
	Versions    []string
	Retracted   bool
}

func (e *NoMatchError) Error() string {
	tags := "none"
	if len(e.Versions) > 0 {
		vs := e.Versions
		if len(vs) > 8 {
			vs = vs[len(vs)-8:]
		}
		tags = quoteAll(vs)
	}
	why := "no version of %s matches %s (versions: %s)"
	if e.Retracted {
		why = "every version of %s that matches %s is retracted (versions: %s)"
	}
	return fmt.Sprintf("%s@%s: "+why, e.Path, e.Query, e.Path, e.Query, tags)
}

// revQuery resolves a revision (a branch, a tag, a commit, or "latest" of a
// repository without version tags) to a pseudo-version, from the refs of a git
// host, or from the .info of a module proxy.
func (f *Fetcher) revQuery(ctx context.Context, mainDir string, repo *Repo, rl *RefList, query string) (*QueryResult, error) {
	if rl.Versions != nil || (len(rl.Tags) == 0 && len(rl.Branches) == 0 && rl.Head == "") {
		// A proxy: ask it, a revision is a thing that its .info answers.
		v, _, err := f.proxyInfo(ctx, repo, query)
		if err != nil {
			return nil, err
		}
		return &QueryResult{Version: v}, nil
	}
	var m *Module
	var err error
	switch {
	case query == "latest":
		if rl.Head == "" {
			return nil, fmt.Errorf("%s has no version tags and no default branch: name a commit (%s@<commit>)", repo.Module, repo.Module)
		}
		m, err = f.getCommit(ctx, mainDir, repo, rl.Head, false)
	default:
		if c, ok := rl.Branches[query]; ok {
			m, err = f.getCommit(ctx, mainDir, repo, c, false)
		} else if c, ok := rl.Tags[query]; ok {
			m, err = f.getCommit(ctx, mainDir, repo, c, false)
		} else if hexQuery.MatchString(query) {
			q := strings.ToLower(query)
			full := q
		find:
			for _, set := range []map[string]string{rl.Branches, rl.Tags} {
				for _, c := range set {
					if strings.HasPrefix(c, q) {
						full = c
						break find
					}
				}
			}
			m, err = f.getCommit(ctx, mainDir, repo, full, false)
		} else {
			return nil, fmt.Errorf("%s@%s: no such version, tag, branch or commit (tags: %s)", repo.Module, query, tagList(rl, repo))
		}
	}
	if err != nil {
		return nil, err
	}
	return &QueryResult{Version: m.Version, Module: m}, nil
}

// proxyInfo asks the module proxies for the version that a revision names, and
// its time:
// "<module>/@v/<rev>.info", or "<module>/@latest".
func (f *Fetcher) proxyInfo(ctx context.Context, repo *Repo, rev string) (string, time.Time, error) {
	srcs, err := f.cfg.sourcesFor(repo)
	if err != nil {
		return "", time.Time{}, err
	}
	var errs []string
	for _, s := range srcs {
		if s.proxy == "" {
			continue
		}
		base := s.proxy + "/" + escSegs(escape(repo.Module))
		u := base + "/@v/" + escSegs(escape(rev)) + ".info"
		if rev == "latest" {
			u = base + "/@latest"
		}
		var buf bytes.Buffer
		dctx, cancel := context.WithTimeout(ctx, f.timeout())
		_, err := f.get(dctx, u, &buf, 1<<20, nil)
		cancel()
		if err != nil {
			errs = append(errs, err.Error())
			if s.next(err) {
				continue
			}
			return "", time.Time{}, err
		}
		var info struct {
			Version string
			Time    time.Time
		}
		if err := json.Unmarshal(buf.Bytes(), &info); err != nil || !gocppmod.ValidVersion(info.Version) {
			return "", time.Time{}, fmt.Errorf("%s: the answer is not a version info", redact(u))
		}
		if err := repo.CheckVersion(info.Version); err != nil {
			return "", time.Time{}, err
		}
		return gocppmod.CanonicalVersion(info.Version), info.Time, nil
	}
	if len(errs) == 0 {
		return "", time.Time{}, fmt.Errorf("%s@%s: no tags or branches to look in, and no module proxy to ask", repo.Module, rev)
	}
	return "", time.Time{}, fmt.Errorf("%s@%s: unknown revision:\n\t%s", repo.Module, rev, strings.Join(errs, "\n\t"))
}

// A PendingSums is a set of gocpp.sum lines that are waiting to be written.
type PendingSums struct {
	mu    sync.Mutex
	lines []SumLine
}

// overlay makes the pending lines count as listed in s (in memory). A nil
// PendingSums has none.
func (p *PendingSums) overlay(s *Sums) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, l := range p.lines {
		s.add(l)
	}
}

func (p *PendingSums) add(l SumLine) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !slices.Contains(p.lines, l) {
		p.lines = append(p.lines, l)
	}
}

// Lines returns the pending lines.
func (p *PendingSums) Lines() []SumLine {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.lines)
}

// NoteModule adds the hash lines of a module that is in the cache and that
// gocpp.sum in mainDir does not list to pending (it was fetched with no
// recording, such as the module of a commit).
func (f *Fetcher) NoteModule(mainDir string, m *Module, pending *PendingSums) error {
	sums, err := LoadSums(mainDir)
	if err != nil {
		return err
	}
	pending.overlay(sums)
	return f.record(sums, m, pending)
}

// SavePending writes the pending lines into the gocpp.sum file of mainDir,
// merged with the file as it is on disk now (another gocpp may have written
// it), atomically. It reports whether the file changed.
func (f *Fetcher) SavePending(mainDir string, pending *PendingSums) (bool, error) {
	return f.saveSums(filepath.Join(mainDir, SumFileName), pending.Lines())
}

// RetractionOf says whether the module version is retracted by the module's
// own retract lines (those of its highest version), and why. It asks the
// source for the versions of the module: with no list (a module that can't be
// listed, or downloads that are off) it answers "not retracted".
func (f *Fetcher) RetractionOf(ctx context.Context, mainDir, path, version string, pending *PendingSums) (string, bool, error) {
	if f.cfg.Offline {
		return "", false, nil
	}
	vs, err := f.Versions(ctx, path)
	if err != nil || len(vs) == 0 {
		return "", false, nil
	}
	rets, err := f.retractionsOf(ctx, mainDir, path, vs[len(vs)-1], pending)
	if err != nil {
		return "", false, err
	}
	why, bad := Retraction(rets, version)
	return why, bad, nil
}

// Lock takes a named lock of the module cache (shared by every gocpp that uses
// the cache) and returns the function that gives it up. It waits for another
// holder until ctx ends. Without a module cache it does nothing.
func (f *Fetcher) Lock(ctx context.Context, name string) (func(), error) {
	return f.lockFile(ctx, name)
}

// LockName returns the lock name for a folder (a project) of the given kind.
func LockName(kind, dir string) string { return kind + "-" + hashName(dir) }

// WriteModFile writes the module file of the folder (gocpp.mod), atomically
// and with the mode that it had.
func WriteModFile(dir string, f *gocppmod.File) error {
	name := filepath.Join(dir, gocppmod.FileName)
	if err := CheckWritable(name); err != nil {
		return err
	}
	data, err := FormatFile(name, f)
	if err != nil {
		return err
	}
	return writeFileAtomic(name, data, ".gocpp.mod-*")
}

// WriteFileAtomic replaces the file with the data by one rename, keeping its
// mode.
func WriteFileAtomic(name string, data []byte) error {
	return writeFileAtomic(name, data, ".gocpp-*")
}

func writeFileAtomic(name string, data []byte, pattern string) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(name); err == nil {
		mode = st.Mode().Perm()
	}
	sweepStaleTemps(filepath.Dir(name))
	tmp, err := os.CreateTemp(filepath.Dir(name), pattern)
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	// The data is on disk before the rename makes it the file: a crash leaves the
	// old file or the new one, never an empty one.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := fsretry.Rename(tmp.Name(), name); err != nil {
		return err
	}
	syncDir(filepath.Dir(name))
	return nil
}

// syncDir makes a rename in the folder durable, as far as the system lets it.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}

// LockProject takes an exclusive lock on the folder of a project (gocpp.mod is in
// it), which every gocpp that changes the project's files respects, also when there
// is no module cache. It waits until ctx ends and returns the function that
// releases it. Where the system has no file locks the lock is only for this process.
func LockProject(ctx context.Context, dir string) (func(), error) {
	semAny, _ := inflight.LoadOrStore("project:"+dir, make(chan struct{}, 1))
	sem := semAny.(chan struct{})
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for another gocpp that changes %s: %v", dir, ctx.Err())
	}
	d, err := os.Open(dir)
	if err != nil {
		<-sem
		return nil, err
	}
	locked := false
	for !locked {
		ok, err := flockImpl(d)
		if errors.Is(err, errLockUnsupported) {
			break
		}
		if err != nil {
			d.Close()
			<-sem
			return nil, err
		}
		if ok {
			locked = true
			break
		}
		select {
		case <-ctx.Done():
			d.Close()
			<-sem
			return nil, fmt.Errorf("waiting for another gocpp that changes %s: %v", dir, ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	return func() {
		if locked {
			funlock(d)
		}
		d.Close()
		<-sem
	}, nil
}
