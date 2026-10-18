package modfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// A RefList is what a repository says about its tags and branches.
type RefList struct {
	Tags     map[string]string // tag name -> commit (the commit of an annotated tag, not the tag object)
	Branches map[string]string // branch name -> commit
	Head     string            // the commit of HEAD, or ""
	HeadName string            // the branch that HEAD is, or ""

	// Versions is only set by a proxy: the versions it lists (Tags is empty).
	Versions []string
}

// maxRefsSize bounds the answer of info/refs: a big repository on GitHub
// lists tens of thousands of pull request refs.
const maxRefsSize = 64 << 20

// listRefs asks the sources, in order, for the refs of the repository: git's
// smart HTTP info/refs of the host, or the list of a proxy.
func (f *Fetcher) listRefs(ctx context.Context, repo *Repo) (*RefList, error) {
	srcs, err := f.cfg.sourcesFor(repo)
	if err != nil {
		return nil, err
	}
	var errs []string
	for _, s := range srcs {
		var rl *RefList
		var err error
		switch {
		case s.proxy != "":
			rl, err = f.proxyList(ctx, s)
		case s.refsURL() == "":
			err = s.err
			if err == nil && s.template != "" {
				err = fmt.Errorf("a %s URL template has no list of tags (name a version, such as %s@v1.2.3, or use a base URL rule)", EnvModURL, repo.Module)
			} else if err == nil {
				err = errors.New("no list of tags")
			}
		default:
			rl, err = f.gitRefs(ctx, s.refsURL())
		}
		if err == nil {
			return rl, nil
		}
		errs = append(errs, s.name+": "+err.Error())
		if !s.next(err) {
			break
		}
	}
	return nil, fmt.Errorf("can't list the versions of %s:\n\t%s", repo.Module, strings.Join(errs, "\n\t"))
}

func (f *Fetcher) proxyList(ctx context.Context, s *source) (*RefList, error) {
	var buf bytes.Buffer
	if _, err := f.get(ctx, s.listURL(), &buf, 8<<20, nil); err != nil {
		return nil, err
	}
	rl := &RefList{}
	for _, line := range strings.Fields(buf.String()) {
		if gocppmod.ValidVersion(line) {
			rl.Versions = append(rl.Versions, line)
		}
	}
	return rl, nil
}

func (f *Fetcher) gitRefs(ctx context.Context, url string) (*RefList, error) {
	var buf bytes.Buffer
	_, err := f.get(ctx, url, &buf, maxRefsSize, http.Header{"Accept": {"application/x-git-upload-pack-advertisement, */*"}})
	if err != nil {
		return nil, err
	}
	return parseRefs(buf.Bytes())
}

var hex40 = regexp.MustCompile(`^[0-9a-f]{40}$`)

// parseRefs reads the answer to git's info/refs?service=git-upload-pack: a
// "# service=" line, a flush, and the refs as pkt-lines ("<commit> <name>"; the
// first has the capabilities after a NUL, among them symref=HEAD:refs/heads/main).
func parseRefs(data []byte) (*RefList, error) {
	rl := &RefList{Tags: map[string]string{}, Branches: map[string]string{}}
	peeled := map[string]string{}
	first := true
	sawService := false
	for len(data) > 0 {
		if len(data) < 4 {
			return nil, errors.New("the list of refs is cut off")
		}
		n, err := strconv.ParseUint(string(data[:4]), 16, 16)
		if err != nil {
			return nil, errors.New("the answer is not git's smart HTTP list of refs (is the repository URL right, and does the server support smart HTTP?)")
		}
		if n == 0 { // flush
			data = data[4:]
			continue
		}
		if n < 4 || int(n) > len(data) {
			if n == 1 || n == 2 {
				return nil, errors.New("the server speaks git protocol version 2 here, which gocpp does not use")
			}
			return nil, errors.New("the list of refs is malformed (is the server speaking git's smart HTTP?)")
		}
		line := data[4:n]
		data = data[n:]
		line = bytes.TrimSuffix(line, []byte("\n"))
		if bytes.HasPrefix(line, []byte("# service=")) {
			sawService = true
			continue
		}
		if string(line) == "version 1" || string(line) == "version 2" {
			if string(line) == "version 2" {
				return nil, errors.New("the server speaks git protocol version 2 here, which gocpp does not use")
			}
			continue
		}
		caps := ""
		if i := bytes.IndexByte(line, 0); i >= 0 {
			line, caps = line[:i], string(line[i+1:])
		}
		if first {
			first = false
			for _, c := range strings.Fields(caps) {
				if v, ok := strings.CutPrefix(c, "symref=HEAD:"); ok {
					rl.HeadName = strings.TrimPrefix(v, "refs/heads/")
				}
			}
		}
		sha, name, ok := strings.Cut(string(line), " ")
		if !ok || !hex40.MatchString(sha) {
			if sawService && strings.HasPrefix(string(line), "ERR ") {
				return nil, errors.New(string(line))
			}
			return nil, errors.New("the list of refs is malformed")
		}
		switch {
		case name == "HEAD":
			rl.Head = sha
		case strings.HasPrefix(name, "refs/heads/"):
			rl.Branches[strings.TrimPrefix(name, "refs/heads/")] = sha
		case strings.HasPrefix(name, "refs/tags/"):
			t := strings.TrimPrefix(name, "refs/tags/")
			if base, ok := strings.CutSuffix(t, "^{}"); ok {
				peeled[base] = sha
			} else {
				rl.Tags[t] = sha
			}
		}
	}
	if !sawService && len(rl.Tags)+len(rl.Branches) == 0 && rl.Head == "" {
		return nil, errors.New("the answer is not git's smart HTTP list of refs")
	}
	for t, c := range peeled {
		rl.Tags[t] = c
	}
	if rl.Head == "" && rl.HeadName != "" {
		rl.Head = rl.Branches[rl.HeadName]
	}
	return rl, nil
}

// versions returns the versions of the module that the refs list: the tags
// vX.Y.Z (also X.Y.Z, as v1.2.3), in the folder of the module for a module
// that is not at the top of the repository, in increasing order.
func (rl *RefList) versions(repo *Repo) []string {
	var out []string
	add := func(v string) {
		if !gocppmod.ValidVersion(v) || strings.Contains(v, "+") {
			return
		}
		if repo.Major != 0 && semverMajor(v) != repo.Major {
			return
		}
		out = append(out, v)
	}
	for _, v := range rl.Versions {
		add(v)
	}
	for t := range rl.Tags {
		if repo.Subdir != "" {
			rest, ok := strings.CutPrefix(t, repo.Subdir+"/")
			if !ok {
				continue
			}
			t = rest
		} else if strings.Contains(t, "/") {
			continue
		}
		if _, p := PseudoCommit(t); p != "" {
			continue
		}
		if strings.HasPrefix(t, "v") {
			add(t)
		} else {
			add("v" + t)
		}
	}
	slices.SortFunc(out, CompareVersions)
	return slices.Compact(out)
}

// latest returns the highest version of vs: a release if there is one, else
// the highest prerelease. vs is sorted.
func latest(vs []string) string {
	for i := len(vs) - 1; i >= 0; i-- {
		if p, _ := parseVersion(vs[i]); p.pre == nil {
			return vs[i]
		}
	}
	if len(vs) > 0 {
		return vs[len(vs)-1]
	}
	return ""
}
