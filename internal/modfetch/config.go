package modfetch

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/version"
)

// The environment variables of the module downloads (docs/spec/modules.md).
const (
	EnvModCache = "GOCPP_MODCACHE" // the module cache folder, or off
	EnvOffline  = "GOCPP_OFFLINE"  // 1: never use the network
	EnvModFlags = "GOCPP_MODFLAGS" // readonly or -mod=readonly: never write gocpp.sum
	EnvModProxy = "GOCPP_MODPROXY" // mirrors, "direct" and "off"
	EnvModURL   = "GOCPP_MODURL"   // where the archives of some modules are
	EnvModAuth  = "GOCPP_MODAUTH"  // host=token credentials
	EnvModCA    = "GOCPP_MODCACERT"
	EnvModTime  = "GOCPP_MODTIMEOUT" // the longest a download may take
)

// Limits are the bounds of a download and of the tree extracted from it. A
// module that exceeds one is an error, not a truncated tree.
type Limits struct {
	MaxArchive int64 // bytes of the downloaded archive
	MaxTree    int64 // bytes of all the files in it
	MaxFile    int64 // bytes of one file
	MaxFiles   int   // files and folders
}

// DefaultLimits are the limits of a Config that has none.
var DefaultLimits = Limits{
	MaxArchive: 512 << 20,
	MaxTree:    2 << 30,
	MaxFile:    256 << 20,
	MaxFiles:   200_000,
}

// A Config says where modules are cached and downloaded from. ConfigFromEnv
// fills it from the environment; tests build one by hand.
type Config struct {
	// CacheDir is the module cache folder. Empty means there is none: a
	// module that is not downloaded yet is an error.
	CacheDir string

	// Offline forbids every network access: a module that is not in the
	// module cache is an error.
	Offline bool

	// OfflineWhy is the reason in the error for a module that is not in the
	// cache when Offline is set by the program, not by GOCPP_OFFLINE (the language
	// server: nothing is downloaded or written to the project).
	OfflineWhy string

	// ReadOnly makes Fetch with Record fail for a module that gocpp.sum does
	// not list, instead of recording its hash (GOCPP_MODFLAGS=-mod=readonly).
	ReadOnly bool

	// Proxies are the places to look, in order. Empty means direct.
	Proxies []Proxy

	// URLs are the rules that say where the archives of some modules are.
	URLs []URLRule

	// Auth maps a host to the credentials sent to that host, and to no
	// other (not after a redirect either): "token" is a bearer token,
	// "user:password" is basic authentication.
	Auth map[string]string

	// CAFile is a PEM file of extra certificate authorities, for a company
	// mirror. TLS is always verified.
	CAFile string

	// Timeout bounds one download (the headers and the whole body). 0 means
	// 10 minutes.
	Timeout time.Duration

	Limits Limits

	// Client, if set, is used for every request. It replaces the client that
	// the other fields build; tests use it.
	Client *http.Client

	// Stderr gets the progress lines ("gocpp: downloading ..."); nil is
	// silent.
	Stderr io.Writer
}

// A Proxy is one element of GOCPP_MODPROXY.
type Proxy struct {
	// URL is the base URL of the mirror, or "" for "direct".
	URL string
	// Off is true for the element "off": no downloads from here on.
	Off bool
	// Any is true if this element was followed by "|": the next one is tried
	// after any error, not only "not found".
	Any bool
}

// A URLRule is one element of GOCPP_MODURL: for the modules whose path
// starts with Prefix (all modules if it is empty), either a Template for the
// URL of the archive, or a Base URL that replaces https://<host> of the
// built-in layouts of github.com, gitlab.com and codeberg.org (a mirror
// with the same URLs).
type URLRule struct {
	Prefix   string
	Template string
	Base     string
}

// DefaultTimeout is the longest a download may take.
const DefaultTimeout = 10 * time.Minute

// ConfigFromEnv reads the Config from the environment (getenv is os.Getenv
// or a test's). stderr gets the progress lines. It returns an error for a
// variable that is malformed, so a wrong mirror is never silently ignored.
func ConfigFromEnv(getenv func(string) string, stderr io.Writer) (*Config, error) {
	c := &Config{Stderr: stderr, Limits: DefaultLimits}
	c.CacheDir = getenv(EnvModCache)
	switch {
	case c.CacheDir == "off":
		c.CacheDir = "" // no module cache: every module must be vendored or replaced
	case c.CacheDir != "" && !filepath.IsAbs(c.CacheDir):
		return nil, fmt.Errorf("%s=%q: the module cache folder must be an absolute path (or off)", EnvModCache, c.CacheDir)
	case c.CacheDir == "":
		// os.UserCacheDir fails when neither XDG_CACHE_HOME nor HOME is set: then
		// there is no cache, and a download says so.
		if base, err := os.UserCacheDir(); err == nil && base != "" {
			c.CacheDir = filepath.Join(base, "gocpp", "mod")
		}
	}
	c.Offline = envOn(getenv(EnvOffline))
	for _, f := range strings.Fields(getenv(EnvModFlags)) {
		switch strings.TrimPrefix(f, "-mod=") {
		case "readonly":
			c.ReadOnly = true
		case "mod":
			c.ReadOnly = false
		case "vendor":
			return nil, fmt.Errorf("%s: -mod=vendor is not needed: a project with a vendor/modules.txt file is vendored", EnvModFlags)
		default:
			return nil, fmt.Errorf("%s: unknown flag %q (want -mod=readonly or -mod=mod)", EnvModFlags, f)
		}
	}
	var err error
	if c.Proxies, err = parseProxies(getenv(EnvModProxy)); err != nil {
		return nil, err
	}
	if c.URLs, err = parseURLRules(getenv(EnvModURL)); err != nil {
		return nil, err
	}
	if c.Auth, err = parseAuth(getenv(EnvModAuth)); err != nil {
		return nil, err
	}
	c.CAFile = getenv(EnvModCA)
	if s := getenv(EnvModTime); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("%s=%q: want a duration such as 90s or 15m", EnvModTime, s)
		}
		c.Timeout = d
	}
	return c, nil
}

func envOn(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// checkBase reports why s is not a usable base URL: HTTPS only (the
// certificate is always verified), no user name in it, no fragment, and no query
// unless allowQuery (a template for an archive URL may have one).
func checkBase(env, s string, allowQuery bool) (string, error) {
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return "", fmt.Errorf("%s: invalid URL %q: %v", env, redact(s), err)
	case u.Scheme != "https":
		return "", fmt.Errorf("%s: %q must be an https:// URL (gocpp always verifies TLS and does not download over plain http)", env, redact(s))
	case u.Host == "":
		return "", fmt.Errorf("%s: %q has no host", env, redact(s))
	case u.User != nil:
		return "", fmt.Errorf("%s: %q has a user name in it: use %s for credentials", env, redact(s), EnvModAuth)
	case u.Fragment != "" || strings.Contains(s, "#"):
		return "", fmt.Errorf("%s: %q has a # fragment", env, redact(s))
	case !allowQuery && (u.RawQuery != "" || strings.Contains(s, "?")):
		return "", fmt.Errorf("%s: %q has a query (?...), which a base URL can't have", env, redact(s))
	}
	return strings.TrimRight(s, "/"), nil
}

// redactList hides the credentials and queries of the URLs in a list of mirrors,
// for a message.
func redactList(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' })
	for _, p := range parts {
		if r := redact(p); r != p {
			s = strings.ReplaceAll(s, p, r)
		}
	}
	return s
}

func parseProxies(s string) ([]Proxy, error) {
	orig := redactList(s)
	var out []Proxy
	for s != "" {
		i := strings.IndexAny(s, ",|")
		elem, sep := s, byte(0)
		if i >= 0 {
			elem, sep = s[:i], s[i]
			s = s[i+1:]
		} else {
			s = ""
		}
		var p Proxy
		switch elem {
		case "":
			return nil, fmt.Errorf("%s: an empty element in %q: name a mirror, direct or off", EnvModProxy, orig)
		case "direct":
		case "off":
			p.Off = true
		default:
			if strings.HasPrefix(elem, "file://") {
				base, err := checkFileBase(elem)
				if err != nil {
					return nil, err
				}
				p.URL = base
				break
			}
			base, err := checkBase(EnvModProxy, elem, false)
			if err != nil {
				return nil, err
			}
			p.URL = base
		}
		p.Any = sep == '|'
		out = append(out, p)
		if sep != 0 && s == "" {
			return nil, fmt.Errorf("%s: an empty element at the end of %q: name a mirror, direct or off", EnvModProxy, orig)
		}
	}
	return out, nil
}

// checkFileBase checks a file:// element of GOCPP_MODPROXY: a folder on this
// machine that has the files of a module proxy (<module>/@v/list,
// <version>.info, .mod and .zip, as the go command's GOPROXY=file:// reads
// them). It is the only element that is not https: nothing is sent anywhere,
// and a folder you name yourself is as trusted as a vendor folder (the hashes
// in gocpp.sum are checked all the same).
func checkFileBase(s string) (string, error) {
	u, err := url.Parse(s)
	switch {
	case err != nil:
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err // the *url.Error repeats the URL
		}
		return "", fmt.Errorf("%s: invalid URL %q: %v", EnvModProxy, redact(s), err)
	case u.User != nil:
		return "", fmt.Errorf("%s: %q has a user name in it: a file:// URL names a folder", EnvModProxy, redact(s))
	case u.Host != "" && u.Host != "localhost":
		return "", fmt.Errorf("%s: %q: a file:// URL must have no host (file:///folder)", EnvModProxy, redact(s))
	case u.RawQuery != "" || strings.Contains(s, "?") || u.Fragment != "" || strings.Contains(s, "#"):
		return "", fmt.Errorf("%s: %q has a query or a fragment", EnvModProxy, redact(s))
	case !strings.HasPrefix(u.Path, "/"):
		return "", fmt.Errorf("%s: %q: the folder must be an absolute path", EnvModProxy, redact(s))
	}
	return "file://" + strings.TrimRight(u.EscapedPath(), "/"), nil
}

// fileURLPath returns the file name that a file:// URL stands for.
func fileURLPath(u *url.URL) string {
	p := u.Path
	// file:///C:/dir on Windows.
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' && (p[1] >= 'A' && p[1] <= 'Z' || p[1] >= 'a' && p[1] <= 'z') {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

var templateFields = map[string]bool{"path": true, "root": true, "host": true, "repo": true, "name": true, "version": true, "ref": true, "subdir": true}

func parseURLRules(s string) ([]URLRule, error) {
	var out []URLRule
	for _, entry := range strings.Fields(s) {
		var r URLRule
		val := entry
		if eq := strings.IndexByte(entry, '='); eq >= 0 {
			if sch := strings.Index(entry, "://"); sch < 0 || eq < sch {
				r.Prefix, val = entry[:eq], entry[eq+1:]
				if r.Prefix == "" || CheckModulePathPrefix(r.Prefix) != nil {
					return nil, fmt.Errorf("%s: invalid module path prefix %q in %q", EnvModURL, r.Prefix, redact(entry))
				}
			}
		}
		if strings.Contains(val, "{") {
			// Validate the fields on a copy with the fields replaced.
			probe := val
			for {
				i := strings.IndexByte(probe, '{')
				if i < 0 {
					break
				}
				j := strings.IndexByte(probe[i:], '}')
				if j < 0 {
					return nil, fmt.Errorf("%s: unclosed { in %q", EnvModURL, redact(val))
				}
				name := probe[i+1 : i+j]
				if !templateFields[name] {
					return nil, fmt.Errorf("%s: unknown field {%s} in %q (fields: {path} {root} {host} {repo} {name} {subdir} {version} {ref})", EnvModURL, name, redact(val))
				}
				probe = probe[:i] + "x" + probe[i+j+1:]
			}
			if _, err := checkBase(EnvModURL, probe, true); err != nil {
				return nil, err
			}
			r.Template = val
		} else {
			base, err := checkBase(EnvModURL, val, false)
			if err != nil {
				return nil, err
			}
			r.Base = base
		}
		out = append(out, r)
	}
	return out, nil
}

// CheckModulePathPrefix reports whether p is a prefix of module paths.
func CheckModulePathPrefix(p string) error {
	for _, e := range strings.Split(p, "/") {
		if e == "" || e == "." || e == ".." {
			return errors.New("empty or dot element")
		}
		for i := 0; i < len(e); i++ {
			c := e[i]
			if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~') {
				return errors.New("invalid character")
			}
		}
	}
	return nil
}

func parseAuth(s string) (map[string]string, error) {
	var m map[string]string
	for _, entry := range strings.Fields(s) {
		host, cred, ok := strings.Cut(entry, "=")
		if !ok || host == "" || cred == "" || strings.ContainsAny(host, "/@") {
			// Never print the entry: it has a secret in it.
			return nil, fmt.Errorf("%s: malformed entry (want host=token or host=user:password)", EnvModAuth)
		}
		if m == nil {
			m = make(map[string]string)
		}
		m[strings.ToLower(host)] = cred
	}
	return m, nil
}

// redact returns a URL without its user name, password and query, for
// messages.
func redact(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		if i := strings.IndexAny(s, "?@"); i >= 0 {
			return s[:i] + "..."
		}
		return s
	}
	u.User = nil
	if u.RawQuery != "" {
		u.RawQuery = "..."
	}
	return u.String()
}

// userAgent is the User-Agent header of every request.
func userAgent() string { return "gocpp/" + version.Version + " (modfetch)" }

// httpClient returns the client that downloads use.
func (c *Config) httpClient() (*http.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if c.CAFile != "" {
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", EnvModCA, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s: %s has no certificate", EnvModCA, c.CAFile)
		}
	}
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{Transport: tr}, nil
}
