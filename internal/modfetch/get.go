package modfetch

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

var (
	hexQuery    = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	prefixQuery = regexp.MustCompile(`^v[0-9]+(\.[0-9]+)?$`)
)

// Get is "gocpp get path@query": it resolves the query to a version, makes
// sure that the module is in the cache and in gocpp.sum of mainDir (the first
// download of a module is trusted), and returns the module. The query is
//
//   - "" or "latest": the highest release tag (vX.Y.Z or X.Y.Z; a prerelease if there is no
//     release), or the pseudo-version of the head of the default branch if the
//     repository has no version tags;
//   - a version (v1.2.3, or a pseudo-version): itself;
//   - v1 or v1.2: the highest version of that line;
//   - a branch or a tag name that is not a version, or a commit (7 to 40 hex digits):
//     the pseudo-version of that commit, v0.0.0-yyyymmddhhmmss-commit12. The time is the time
//     of the commit, as the archive of the host says.
//
// Tags and branches come from the smart HTTP listing of the repository (or the
// list of a proxy, which has only versions).
func (f *Fetcher) Get(ctx context.Context, mainDir, path, query string) (*Module, error) {
	repo, err := ParseModulePath(path)
	if err != nil {
		return nil, err
	}
	opts := FetchOptions{Record: true}
	if gocppmod.ValidVersion(query) {
		return f.Fetch(ctx, mainDir, path, query, opts)
	}
	if f.cfg.Offline {
		return nil, fmt.Errorf("can't resolve %s@%s while downloads are off (%s=1): name a version, such as %s@v1.2.3", path, nonEmpty(query, "latest"), EnvOffline, path)
	}
	dctx, cancel := context.WithTimeout(ctx, f.timeout())
	rl, err := f.listRefs(dctx, repo)
	cancel()
	if err != nil {
		return nil, err
	}
	pick := func(vs []string, what string) (*Module, error) {
		v := latest(vs)
		if v == "" {
			return nil, fmt.Errorf("%s: no version of %s matches %s (tags: %s)", path, path, what, tagList(rl, repo))
		}
		return f.Fetch(ctx, mainDir, path, v, opts)
	}
	switch {
	case query == "" || query == "latest":
		vs := rl.versions(repo)
		if len(vs) > 0 {
			return f.Fetch(ctx, mainDir, path, latest(vs), opts)
		}
		if rl.Head == "" {
			return nil, fmt.Errorf("%s has no version tags and no default branch: name a commit (%s@<commit>)", path, path)
		}
		return f.getCommit(ctx, mainDir, repo, rl.Head)
	case prefixQuery.MatchString(query):
		var vs []string
		for _, v := range rl.versions(repo) {
			if strings.HasPrefix(v+".", query+".") || strings.HasPrefix(v, query+"-") {
				vs = append(vs, v)
			}
		}
		return pick(vs, query)
	}
	if rl.Versions != nil || (len(rl.Tags) == 0 && len(rl.Branches) == 0) {
		return nil, fmt.Errorf("%s@%s: the source lists versions only (a proxy): name a version, such as %s@v1.2.3", path, query, path)
	}
	if c, ok := rl.Branches[query]; ok {
		return f.getCommit(ctx, mainDir, repo, c)
	}
	if c, ok := rl.Tags[query]; ok {
		return f.getCommit(ctx, mainDir, repo, c)
	}
	if hexQuery.MatchString(query) {
		q := strings.ToLower(query)
		for _, set := range []map[string]string{rl.Branches, rl.Tags} {
			for _, c := range set {
				if strings.HasPrefix(c, q) {
					return f.getCommit(ctx, mainDir, repo, c)
				}
			}
		}
		return f.getCommit(ctx, mainDir, repo, q)
	}
	return nil, fmt.Errorf("%s@%s: no such version, tag, branch or commit (tags: %s)", path, query, tagList(rl, repo))
}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// tagList returns some of the tags of the repository, for a message.
func tagList(rl *RefList, repo *Repo) string {
	vs := rl.versions(repo)
	if len(vs) == 0 {
		var all []string
		for t := range rl.Tags {
			all = append(all, t)
		}
		sort.Strings(all)
		if len(all) > 8 {
			all = all[len(all)-8:]
		}
		if len(all) == 0 {
			return "none"
		}
		return quoteAll(all) + " (none is a version vX.Y.Z)"
	}
	if len(vs) > 8 {
		vs = vs[len(vs)-8:]
	}
	return quoteAll(vs)
}

// quoteAll quotes tag names for a message: they come from the server.
func quoteAll(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(q, ", ")
}

// getCommit downloads the archive of a commit and names the module by its
// pseudo-version.
func (f *Fetcher) getCommit(ctx context.Context, mainDir string, repo *Repo, commit string) (*Module, error) {
	commit = strings.ToLower(commit)
	if f.cfg.CacheDir == "" {
		return nil, fmt.Errorf("no module cache folder (set %s)", EnvModCache)
	}
	sums, err := LoadSums(mainDir)
	if err != nil {
		return nil, err
	}
	st, err := f.stage(ctx, repo, ref{Commit: commit})
	if err != nil {
		return nil, err
	}
	defer st.cleanup()
	full := st.info.Commit
	if full == "" {
		full = commit
	}
	if len(full) < 12 {
		return nil, fmt.Errorf("%s: the archive does not say which commit it is: give at least 12 hex digits of it", repo.Module)
	}
	if st.info.Time.IsZero() || st.info.Time.Year() < 1971 {
		return nil, fmt.Errorf("%s: the archive has no time for the commit, so it has no pseudo-version", repo.Module)
	}
	version := PseudoVersion(repo.Major, st.info.Time.UTC().Format("20060102150405"), full)
	if err := repo.CheckVersion(version); err != nil {
		return nil, err
	}
	unlock, err := f.lockFile(ctx, "mod-"+hashName(f.ModDir(repo.Module, version)))
	if err != nil {
		return nil, err
	}
	defer unlock()
	m, err := f.cached(ctx, repo.Module, version, sums)
	if err != nil {
		return nil, err
	}
	if m == nil {
		if m, err = f.publish(ctx, st, repo, version, sums); err != nil {
			return nil, err
		}
	}
	return m, f.record(sums, m)
}
