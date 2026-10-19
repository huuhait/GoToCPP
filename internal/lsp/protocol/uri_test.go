// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package protocol_test

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// TestURIFromPath tests the conversion between URIs and filenames. The test cases
// include Windows-style URIs and filepaths, but we avoid having OS-specific
// tests by using only forward slashes, assuming that the standard library
// functions filepath.ToSlash and filepath.FromSlash do not need testing.
func TestURIFromPath(t *testing.T) {
	for _, test := range []struct {
		path, wantFile string
		wantURI        protocol.DocumentURI
	}{
		{
			path:     ``,
			wantFile: ``,
			wantURI:  protocol.DocumentURI(""),
		},
		{
			path:     `C:/Windows/System32`,
			wantFile: `C:/Windows/System32`,
			wantURI:  protocol.DocumentURI("file:///C:/Windows/System32"),
		},
		{
			path:     `C:/Go/src/bob.go`,
			wantFile: `C:/Go/src/bob.go`,
			wantURI:  protocol.DocumentURI("file:///C:/Go/src/bob.go"),
		},
		{
			path:     `c:/Go/src/bob.go`,
			wantFile: `C:/Go/src/bob.go`,
			wantURI:  protocol.DocumentURI("file:///C:/Go/src/bob.go"),
		},
		{
			path:     `/path/to/dir`,
			wantFile: `/path/to/dir`,
			wantURI:  protocol.DocumentURI("file:///path/to/dir"),
		},
		{
			path:     `/a/b/c/src/bob.go`,
			wantFile: `/a/b/c/src/bob.go`,
			wantURI:  protocol.DocumentURI("file:///a/b/c/src/bob.go"),
		},
		{
			path:     `c:/Go/src/bob george/george/george.go`,
			wantFile: `C:/Go/src/bob george/george/george.go`,
			wantURI:  protocol.DocumentURI("file:///C:/Go/src/bob%20george/george/george.go"),
		},
	} {
		got := protocol.URIFromPath(test.path)
		if got != test.wantURI {
			t.Errorf("URIFromPath(%q): got %q, expected %q", test.path, got, test.wantURI)
		}
		gotFilename := got.Path()
		if gotFilename != test.wantFile {
			t.Errorf("Filename(%q): got %q, expected %q", got, gotFilename, test.wantFile)
		}
	}
}

func TestParseDocumentURI(t *testing.T) {
	for _, test := range []struct {
		input    string
		want     string // string(DocumentURI) on success or error.Error() on failure
		wantPath string // expected DocumentURI.Path on success
	}{
		{
			input:    `file:///c:/Go/src/bob%20george/george/george.go`,
			want:     "file:///C:/Go/src/bob%20george/george/george.go",
			wantPath: `C:/Go/src/bob george/george/george.go`,
		},
		{
			input:    `file:///C%3A/Go/src/bob%20george/george/george.go`,
			want:     "file:///C:/Go/src/bob%20george/george/george.go",
			wantPath: `C:/Go/src/bob george/george/george.go`,
		},
		{
			input:    `file:///path/to/%25p%25ercent%25/per%25cent.go`,
			want:     `file:///path/to/%25p%25ercent%25/per%25cent.go`,
			wantPath: `/path/to/%p%ercent%/per%cent.go`,
		},
		{
			input:    `file:///C%3A/`,
			want:     `file:///C:/`,
			wantPath: `C:/`,
		},
		{
			input:    `file:///`,
			want:     `file:///`,
			wantPath: `/`,
		},
		{
			input:    `file://wsl%24/Ubuntu/home/wdcui/repo/VMEnclaves/cvm-runtime`,
			want:     `file:///wsl$/Ubuntu/home/wdcui/repo/VMEnclaves/cvm-runtime`,
			wantPath: `/wsl$/Ubuntu/home/wdcui/repo/VMEnclaves/cvm-runtime`,
		},
		{
			input:    "",
			want:     "",
			wantPath: "",
		},
		// Errors:
		{
			input: "https://go.dev/",
			want:  "DocumentURI scheme is not 'file': https://go.dev/",
		},
	} {
		uri, err := protocol.ParseDocumentURI(test.input)
		var got string
		if err != nil {
			got = err.Error()
		} else {
			got = string(uri)
		}
		if got != test.want {
			t.Errorf("ParseDocumentURI(%q): got %q, want %q", test.input, got, test.want)
		}
		if err == nil && uri.Path() != test.wantPath {
			t.Errorf("DocumentURI(%s).Path = %q, want %q", uri,
				uri.Path(), test.wantPath)
		}
	}
}

// TestURIFromPlainPath: the short way for plain paths gives the URI of the
// general way, and the paths that need work (dots, double slashes, escapes,
// a trailing slash) do not take it.
func TestURIFromPlainPath(t *testing.T) {
	for _, path := range []string{"/", "/a", "/a/b.gp", "/home/x/proj-1/pkg_2/f00.gp", "/a/.hidden/b~c.gp", "/a/b..c/d", "/a/b/", "/a//b", "/a/./b", "/a/../b", "/a b/c", "/a/é/c", "/a/b%20c", "/a/b#c", "/a/b?c"} {
		u := url.URL{Scheme: "file", Path: path}
		want := protocol.DocumentURI(u.String())
		if abs, err := filepath.Abs(path); err == nil {
			u.Path = filepath.ToSlash(abs)
			want = protocol.DocumentURI(u.String())
		}
		if got := protocol.URIFromPath(path); got != want {
			t.Errorf("URIFromPath(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestLocationListJSON: a LocationList encodes to the bytes of the []Location
// it holds, also for names that need escapes and for an empty list.
func TestLocationListJSON(t *testing.T) {
	locs := []protocol.Location{
		{URI: "file:///a/b.gp", Range: protocol.Range{Start: protocol.Position{Line: 1, Character: 2}, End: protocol.Position{Line: 3, Character: 4000000}}},
		{URI: "file:///a/é<&>\"\\\n.gp"},
		{},
	}
	for _, in := range [][]protocol.Location{locs, {}, nil} {
		want, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		got, err := protocol.LocationList(in).MarshalJSON() // not through json.Marshal, which would escape < > & again
		if err != nil {
			t.Fatal(err)
		}
		if in == nil {
			want = []byte("[]") // the list answers "[]", never null
		}
		if string(got) != string(want) {
			t.Errorf("got  %s\nwant %s", got, want)
		}
	}
}

// TestLocationListEscapes: each character that encoding/json escapes in a
// string is escaped in the same way by the hand-made encoding, one URI per
// character so that a missing case is found.
func TestLocationListEscapes(t *testing.T) {
	for _, c := range []string{"<", ">", "&", "\"", "\\", "\n", "\r", "\t", "\x00", "\x1f", "\x7f", "\u2028", "\u2029", "é", "\xff"} {
		in := []protocol.Location{{URI: protocol.DocumentURI("file:///a/" + c + "b.gp")}}
		want, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		got, err := protocol.LocationList(in).MarshalJSON() // not through json.Marshal, which would escape < > & again
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("character %q:\n got  %s\n want %s", c, got, want)
		}
	}
}
