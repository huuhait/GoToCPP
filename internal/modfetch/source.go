package modfetch

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// A source is one place that a module can come from: a git host (direct), a
// mirror with the layout of a git host, a rule's URL template, or a module
// proxy.
type source struct {
	name string // for messages: "github.com", "mirror https://m.example", "proxy https://p.example"
	any  bool   // after an error that is not "not found", try the next source

	proxy    string // base URL of a proxy ("" if not a proxy)
	base     string // base URL of a git host or its mirror
	layout   string // "github", "gitlab" or "gitea": how a git host names archives
	template string // a rule's template for the archive URL

	err  error // the source can't be used (the "direct" element for an unknown host)
	repo *Repo
}

// A ref names what to download: a tag (Tag) or a commit (Commit, full or
// abbreviated hex).
type ref struct {
	Version string // the module version the download is for ("" while resolving a commit)
	Tag     string
	Commit  string
}

func (r ref) name() string {
	if r.Commit != "" {
		return r.Commit
	}
	return r.Tag
}

// layoutOf returns the layout of the archives of a host.
func layoutOf(host string) string {
	switch host {
	case "github.com":
		return "github"
	case "gitlab.com":
		return "gitlab"
	}
	return "gitea" // codeberg.org, Forgejo, Gitea and the others with /archive/<ref>.tar.gz
}

// sourcesFor returns the places to try for a module, in order, as GOCPP_MODPROXY
// and GOCPP_MODURL say.
func (c *Config) sourcesFor(repo *Repo) ([]*source, error) {
	proxies := c.Proxies
	if len(proxies) == 0 {
		proxies = []Proxy{{}}
	}
	var out []*source
	for _, p := range proxies {
		switch {
		case p.Off:
			if len(out) == 0 {
				return nil, fmt.Errorf("downloads are turned off (%s=off)", EnvModProxy)
			}
			return out, nil
		case p.URL != "":
			out = append(out, &source{name: "proxy " + p.URL, proxy: p.URL, any: p.Any, repo: repo})
		default:
			s, err := c.directSource(repo)
			if err != nil {
				if len(proxies) > 1 {
					// The error is for the "direct" element only; the
					// others may still work.
					out = append(out, &source{name: "direct", any: p.Any, repo: repo, err: err})
					continue
				}
				return nil, err
			}
			s.any = p.Any
			out = append(out, s)
		}
	}
	return out, nil
}

func (c *Config) directSource(repo *Repo) (*source, error) {
	for _, r := range c.URLs {
		if r.Prefix != "" && repo.Module != r.Prefix && !strings.HasPrefix(repo.Module, r.Prefix+"/") {
			continue
		}
		if r.Template != "" {
			return &source{name: "GOCPP_MODURL " + redact(r.Template), template: r.Template, repo: repo}, nil
		}
		return &source{name: "mirror " + r.Base, base: r.Base, layout: layoutOf(repo.Host), repo: repo}, nil
	}
	if !gitHosts[repo.Host] {
		return nil, fmt.Errorf("don't know where to download %s from: %s is not github.com, gitlab.com or codeberg.org (set %s or %s)",
			repo.Module, repo.Host, EnvModURL, EnvModProxy)
	}
	return &source{name: repo.Host, base: "https://" + repo.Host, layout: layoutOf(repo.Host), repo: repo}, nil
}

// escSegs escapes every element of a slash separated path for a URL.
func escSegs(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// repoPath returns owner/repo of the repository for a URL, without ".git".
func (s *source) repoPath() string {
	return escSegs(strings.TrimSuffix(s.repo.GitName(), ".git"))
}

// archiveURLs returns the URLs to try for the archive of r, in order. A
// version tag is tried as v1.2.3 and as 1.2.3 (many C and C++ libraries tag
// without the v).
func (s *source) archiveURLs(r ref) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	repo := s.repo
	refs := []string{r.name()}
	if r.Commit == "" {
		if alt := strings.TrimPrefix(strings.TrimSuffix(r.Version, "+incompatible"), "v"); alt != strings.TrimSuffix(r.Version, "+incompatible") {
			t := alt
			if repo.Subdir != "" {
				t = repo.Subdir + "/" + alt
			}
			refs = append(refs, t)
		}
	}
	var out []string
	for i, name := range refs {
		isTag := r.Commit == ""
		switch {
		case s.proxy != "":
			if i > 0 {
				continue
			}
			v := r.Version
			if v == "" {
				v = r.Commit
			}
			p := s.proxy + "/" + escSegs(escape(repo.Module)) + "/@v/" + url.PathEscape(escape(v))
			out = append(out, p+".tar.gz", p+".zip")
		case s.template != "":
			out = append(out, expand(s.template, repo, r, name))
		case s.layout == "github":
			if isTag {
				out = append(out, s.base+"/"+s.repoPath()+"/archive/refs/tags/"+escSegs(name)+".tar.gz")
			} else {
				out = append(out, s.base+"/"+s.repoPath()+"/archive/"+url.PathEscape(name)+".tar.gz")
			}
		case s.layout == "gitlab":
			file := strings.ReplaceAll(name, "/", "-")
			last := repo.Root[strings.LastIndexByte(strings.TrimSuffix(repo.Root, ".git"), '/')+1:]
			out = append(out, s.base+"/"+s.repoPath()+"/-/archive/"+escSegs(name)+"/"+url.PathEscape(strings.TrimSuffix(last, ".git")+"-"+file)+".tar.gz")
		default:
			out = append(out, s.base+"/"+s.repoPath()+"/archive/"+escSegs(name)+".tar.gz")
		}
	}
	return out, nil
}

// refsURL returns the URL of the list of refs of the repository (git's smart
// HTTP "info/refs"), or "" if the source has none.
func (s *source) refsURL() string {
	switch {
	case s.proxy != "" || s.err != nil:
		return ""
	case s.template != "":
		return "" // the template says where archives are, not where the repository is: nothing is asked of the module's own host
	}
	return s.base + "/" + s.repoPath() + ".git/info/refs?service=git-upload-pack"
}

// listURL is the URL of the list of versions of a proxy.
func (s *source) listURL() string {
	if s.proxy == "" {
		return ""
	}
	return s.proxy + "/" + escSegs(escape(s.repo.Module)) + "/@v/list"
}

func expand(tmpl string, repo *Repo, r ref, name string) string {
	root := strings.TrimSuffix(repo.Root, ".git")
	last := root[strings.LastIndexByte(root, '/')+1:]
	v := r.Version
	if v == "" {
		v = r.Commit
	}
	return strings.NewReplacer(
		"{path}", escSegs(repo.Module),
		"{root}", escSegs(root),
		"{host}", url.PathEscape(repo.Host),
		"{repo}", escSegs(strings.TrimPrefix(strings.TrimPrefix(root, repo.Host), "/")),
		"{name}", url.PathEscape(last),
		"{subdir}", escSegs(repo.Subdir),
		"{version}", url.PathEscape(v),
		"{ref}", escSegs(name),
	).Replace(tmpl)
}

// next reports whether the next source is tried after the error err of this
// one: after "not found", and after any error if the list said "|".
func (s *source) next(err error) bool {
	var nf *NotFoundError
	return s.any || s.err != nil || errors.As(err, &nf)
}
