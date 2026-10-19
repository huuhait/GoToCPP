package server

import (
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// locationURI returns the URI that an answer uses for the file path of the snapshot. A document that is open is
// known to the editor by the URI it sent, and on a system whose file names are case-insensitive that URI can
// differ in case from the one of the path (the folder was opened as `c:\users\me\proj`; the disk says
// `C:\Users\Me\Proj`): the answer uses the editor's own spelling then, so that it lands on the open document.
// Every other file has the URI of its path, in the case of the disk.
func locationURI(snap *cache.Snapshot, path string) protocol.DocumentURI {
	uri := protocol.URIFromPath(path)
	if snap != nil && fspath.FoldCase() {
		if o := snap.Overlay(uri); o != nil {
			return o.URI()
		}
	}
	return uri
}
