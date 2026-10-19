// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol

// This file declares URI, DocumentURI, and its methods.
//
// For the LSP definition of these types, see
// https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#uri

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol/internal/pathutil"
)

// A DocumentURI is the URI of a client editor document.
//
// According to the LSP specification:
//
//	Care should be taken to handle encoding in URIs. For
//	example, some clients (such as VS Code) may encode colons
//	in drive letters while others do not. The URIs below are
//	both valid, but clients and servers should be consistent
//	with the form they use themselves to ensure the other party
//	doesn’t interpret them as distinct URIs. Clients and
//	servers should not assume that each other are encoding the
//	same way (for example a client encoding colons in drive
//	letters cannot assume server responses will have encoded
//	colons). The same applies to casing of drive letters - one
//	party should not assume the other party will return paths
//	with drive letters cased the same as it.
//
//	file:///c:/project/readme.md
//	file:///C%3A/project/readme.md
//
// This is done during JSON unmarshalling;
// see [DocumentURI.UnmarshalText] for details.
type DocumentURI string

// A URI is an arbitrary URL (e.g. https), not necessarily a file.
type URI = string

// UnmarshalText implements decoding of DocumentURI values.
//
// In particular, it implements a systematic correction of various odd
// features of the definition of DocumentURI in the LSP spec that
// appear to be workarounds for bugs in VS Code. For example, it may
// URI-encode the URI itself, so that colon becomes %3A, and it may
// send file://foo.go URIs that have two slashes (not three) and no
// hostname.
//
// We use UnmarshalText, not UnmarshalJSON, because it is called even
// for non-addressable values such as keys and values of map[K]V,
// where there is no pointer of type *K or *V on which to call
// UnmarshalJSON. (See Go issue #28189 for more detail.)
//
// Non-empty DocumentURIs are valid "file"-scheme URIs.
// The empty DocumentURI is valid.
func (uri *DocumentURI) UnmarshalText(data []byte) (err error) {
	*uri, err = ParseDocumentURI(string(data))
	return
}

// Clean returns the cleaned uri by triggering filepath.Clean underlying.
func (uri DocumentURI) Clean() DocumentURI {
	return URIFromPath(fspath.Clean(uri.Path()))
}

// Path returns the file path for the given URI, in the one spelling that the server uses for the file
// (fspath.Canon: on Windows the clean name with an upper-case drive letter, UNC names as `\\host\share`, and
// every part that exists on disk in the case it has there, so that `file:///c:/users/me/proj` and
// `file:///C:/Users/Me/Proj` have one path). Elsewhere it is the path of the URI as it is.
//
// DocumentURI("").Path() returns the empty string.
//
// Path panics if called on a URI that is not a valid filename.
func (uri DocumentURI) Path() string {
	f := fspath.Current()
	if f.FoldCase {
		if p, ok := memoLoad(&pathMemo, uri, f); ok {
			return p
		}
	}
	gen := fspath.Generation()
	p, err := pathOf(f, uri)
	if err != nil {
		// e.g. ParseRequestURI failed.
		//
		// This can only affect DocumentURIs created by
		// direct string manipulation; all DocumentURIs
		// received from the client pass through
		// ParseRequestURI, which ensures validity.
		panic(err)
	}
	p = f.Canon(p)
	if f.FoldCase {
		memoStore(&pathMemo, uri, f, gen, p)
	}
	return p
}

// Key returns the DocumentURI that stands for the file in maps and comparisons: the URI of its Path. Two URIs of
// one file that the editor and the server spell differently have one Key. Where names are case-sensitive it
// is the URI itself.
func (uri DocumentURI) Key() DocumentURI {
	f := fspath.Current()
	if uri == "" || !f.FoldCase {
		return uri
	}
	if k, ok := memoLoad(&keyMemo, uri, f); ok {
		return DocumentURI(k)
	}
	gen := fspath.Generation()
	k := URIFromPath(uri.Path())
	memoStore(&keyMemo, uri, f, gen, string(k))
	return k
}

// Base returns the base name of the file path of the given URI.
func (uri DocumentURI) Base() string {
	return fspath.Base(uri.Path())
}

// Dir returns the URI for the directory containing the receiver.
func (uri DocumentURI) Dir() DocumentURI {
	// This function could be more efficiently implemented by avoiding any call
	// to Path(), but at least consolidates URI manipulation.
	return URIFromPath(uri.DirPath())
}

// DirPath returns the file path to the directory containing this URI, which
// must be a file URI.
func (uri DocumentURI) DirPath() string {
	return fspath.Dir(uri.Path())
}

// Encloses reports whether uri's path, considered as a sequence of segments,
// is a prefix of file's path.
func (uri DocumentURI) Encloses(file DocumentURI) bool {
	if fspath.FoldCase() {
		return fspath.Within(uri.Path(), file.Path()) && !fspath.Equal(uri.Path(), file.Path())
	}
	return pathutil.InDir(uri.Path(), file.Path())
}

// Location returns the Location for the specified range of this URI's file.
func (uri DocumentURI) Location(rng Range) Location {
	return Location{URI: uri, Range: rng}
}

// pathOf returns the path of the file URI in the spelling of the flavor f, without the work of fspath.Canon.
func pathOf(f fspath.Flavor, uri DocumentURI) (string, error) {
	if f.Windows {
		return windowsPathOf(f, uri)
	}
	p, err := filename(uri)
	return f.FromSlash(p), err
}

func filename(uri DocumentURI) (string, error) {
	if uri == "" {
		return "", nil
	}

	// This conservative check for the common case
	// of a simple non-empty absolute POSIX filename
	// avoids the allocation of a net.URL.
	if strings.HasPrefix(string(uri), "file:///") {
		rest := string(uri)[len("file://"):] // leave one slash
		for i := 0; i < len(rest); i++ {
			b := rest[i]
			// Reject these cases:
			if b < ' ' || b == 0x7f || // control character
				b == '%' || b == '+' || // URI escape
				b == ':' || // Windows drive letter
				b == '&' || b == '?' { // authority or query
				goto slow
			}
			// We do not reject '@' as it cannot be part of the
			// authority (e.g. user:pass@example.com) in a
			// "file:///" URL, and '@' commonly appears in file
			// paths such as GOMODCACHE/module@version/...
		}
		return rest, nil
	}
slow:

	u, err := url.ParseRequestURI(string(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != fileScheme {
		return "", fmt.Errorf("only file URIs are supported, got %q from %q", u.Scheme, uri)
	}
	// If the URI is a Windows URI, we trim the leading "/" and uppercase
	// the drive letter, which will never be case sensitive.
	if isWindowsDriveURIPath(u.Path) {
		u.Path = strings.ToUpper(string(u.Path[1])) + u.Path[2:]
	}

	return u.Path, nil
}

// windowsPathOf is pathOf for the Windows flavor: `file:///c:/x` is `C:\x`, an authority is a UNC name
// (`file://srv/share/x` is `\\srv\share\x`), `file:////srv/share/x` too.
func windowsPathOf(f fspath.Flavor, uri DocumentURI) (string, error) {
	if uri == "" {
		return "", nil
	}
	u, err := url.ParseRequestURI(string(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != fileScheme {
		return "", fmt.Errorf("only file URIs are supported, got %q from %q", u.Scheme, uri)
	}
	if u.User != nil {
		return "", fmt.Errorf("a file URI has no user name: %q", uri)
	}
	return f.FromURIParts(u.Host, u.Path), nil
}

// ParseDocumentURI interprets a string as a DocumentURI, applying VS
// Code workarounds; see [DocumentURI.UnmarshalText] for details.
// If "s" is a file name, use [URIFromPath] instead.
func ParseDocumentURI(s string) (DocumentURI, error) {
	if s == "" {
		return "", nil
	}

	if !strings.HasPrefix(s, "file://") {
		return "", fmt.Errorf("DocumentURI scheme is not 'file': %s", s)
	}

	if f := fspath.Current(); f.Windows {
		return parseWindowsURI(f, s)
	}

	// VS Code sends URLs with only two slashes,
	// which are invalid. golang/go#39789.
	if !strings.HasPrefix(s, "file:///") {
		s = "file:///" + s[len("file://"):]
	}

	// Even though the input is a URI, it may not be in canonical form. VS Code
	// in particular over-escapes :, @, etc. Unescape and re-encode to canonicalize.
	path, err := url.PathUnescape(s[len("file://"):])
	if err != nil {
		return "", err
	}

	// File URIs from Windows may have lowercase drive letters.
	// Since drive letters are guaranteed to be case insensitive,
	// we change them to uppercase to remain consistent.
	// For example, file:///c:/x/y/z becomes file:///C:/x/y/z.
	if isWindowsDriveURIPath(path) {
		path = path[:1] + strings.ToUpper(string(path[1])) + path[2:]
	}
	u := url.URL{Scheme: fileScheme, Path: path}
	return DocumentURI(u.String()), nil
}

// parseWindowsURI is ParseDocumentURI for the Windows flavor. The forms that editors and tools send for the
// same file are one: `file:///c:/x`, `file:///C%3A/x`, `file://C:/x` (two slashes, no authority: VS Code's
// bug golang/go#39789), `file://localhost/C:/x`; and a UNC name is the authority: `file://srv/share/x`,
// `file:////srv/share/x`. The result has the drive letter in upper case and the path escaped the way
// URIFromPath escapes it; the case of the names is the editor's (Path gives the case of the disk).
func parseWindowsURI(f fspath.Flavor, s string) (DocumentURI, error) {
	rest := s[len("file://"):]
	host, p := "", rest
	if !strings.HasPrefix(rest, "/") {
		i := strings.IndexByte(rest, '/')
		if i < 0 {
			i = len(rest)
		}
		h, err := url.PathUnescape(rest[:i])
		if err != nil {
			return "", err
		}
		if strings.Contains(h, "@") {
			return "", fmt.Errorf("a file URI has no user name: %s", s) // file://user@srv/share would make a path that Path cannot read
		}
		host, p = h, rest[i:]
		if len(host) == 2 && host[1] == ':' && isLetterByte(host[0]) {
			// file://C:/x: the drive letter is not a host.
			host, p = "", "/"+host+p
		} else if p == "" {
			p = "/"
		}
	}
	p, err := url.PathUnescape(p)
	if err != nil {
		return "", err
	}
	name := f.FromURIParts(host, p)
	if !f.IsAbs(name) && !strings.HasPrefix(name, `\`) {
		// A rooted name with no drive (`file:///x/y`): the URI keeps its form, as the editor sent it.
		return DocumentURI((&url.URL{Scheme: fileScheme, Path: p}).String()), nil
	}
	return uriOfName(f, name), nil
}

func isLetterByte(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// uriOfName returns the URI of the absolute name in the flavor f.
func uriOfName(f fspath.Flavor, name string) DocumentURI {
	host, p := f.URIParts(name)
	if host == "" && !strings.HasPrefix(p, "/") {
		p = "/" + p // a name that is still relative (a flavor that is not the system's): the URI has a rooted path all the same
	}
	return DocumentURI((&url.URL{Scheme: fileScheme, Host: host, Path: p}).String())
}

// URIFromPath returns DocumentURI for the supplied file path.
// Given "", it returns "".
func URIFromPath(path string) DocumentURI {
	if path == "" {
		return ""
	}
	if f := fspath.Current(); f.Windows {
		if !f.IsAbs(path) {
			if abs, err := filepath.Abs(path); err == nil && f.IsAbs(abs) {
				path = abs
			}
		}
		return uriOfName(f, path)
	}
	if plainPath(path) {
		return DocumentURI(fileScheme + "://" + path) // what the general way gives, without its work
	}
	if !IsWindowsDrivePath(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	// Check the file path again, in case it became absolute.
	if IsWindowsDrivePath(path) {
		path = "/" + strings.ToUpper(string(path[0])) + path[1:]
	}
	path = filepath.ToSlash(path)
	u := url.URL{
		Scheme: fileScheme,
		Path:   path,
	}
	return DocumentURI(u.String())
}

const fileScheme = "file"

// plainPath reports whether path is an absolute, clean path of a system with
// slashes that needs no escaping in a URI (letters, digits and - _ . ~ in
// the names): its URI is "file://" and the path. The names a language server
// answers with are nearly all so.
func plainPath(path string) bool {
	if filepath.Separator != '/' || len(path) < 2 || path[0] != '/' {
		return false
	}
	segStart := 1
	for i := 1; i <= len(path); i++ {
		if i < len(path) {
			c := path[i]
			switch {
			case c == '/':
			case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-', c == '_', c == '.', c == '~':
				continue
			default:
				return false
			}
		}
		// A name ends here: it must be a name, not "", "." or "..".
		seg := path[segStart:i]
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
		segStart = i + 1
	}
	return true
}

// IsWindowsDrivePath returns true if the file path is of the form used by
// Windows. We check if the path begins with a drive letter, followed by a ":".
// For example: C:/x/y/z.
func IsWindowsDrivePath(path string) bool {
	if len(path) < 3 {
		return false
	}
	return unicode.IsLetter(rune(path[0])) && path[1] == ':'
}

// isWindowsDriveURIPath returns true if the file URI is of the format used by
// Windows URIs. The url.Parse package does not specially handle Windows paths
// (see golang/go#6027), so we check if the URI path has a drive prefix (e.g. "/C:").
func isWindowsDriveURIPath(uri string) bool {
	if len(uri) < 4 {
		return false
	}
	return uri[0] == '/' && unicode.IsLetter(rune(uri[1])) && uri[2] == ':'
}
