package protocol

import (
	"sync"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/lsp/fspath"
)

// On a system whose file names fold case, DocumentURI.Path and Key parse the URI and ask fspath.Canon, about 10 µs
// and 17 allocations each, and the server calls them for every document of every request. The answers are
// remembered, with the generation of fspath (Invalidate makes them old), as Canon's own are.
type uriMemoKey struct {
	uri DocumentURI
	f   fspath.Flavor
}

type uriMemoValue struct {
	gen uint64
	s   string
}

var (
	pathMemo, keyMemo sync.Map // uriMemoKey -> uriMemoValue
	uriMemoCount      atomic.Int64
)

const maxURIMemo = 100_000

func memoLoad(m *sync.Map, uri DocumentURI, f fspath.Flavor) (string, bool) {
	if v, ok := m.Load(uriMemoKey{uri, f}); ok && v.(uriMemoValue).gen == fspath.Generation() {
		return v.(uriMemoValue).s, true
	}
	return "", false
}

// memoStore remembers s, unless fspath was invalidated since gen (the answer may be old).
func memoStore(m *sync.Map, uri DocumentURI, f fspath.Flavor, gen uint64, s string) {
	if fspath.Generation() != gen {
		return
	}
	if uriMemoCount.Add(1) > maxURIMemo {
		pathMemo.Clear()
		keyMemo.Clear()
		uriMemoCount.Store(1)
	}
	m.Store(uriMemoKey{uri, f}, uriMemoValue{gen, s})
}
