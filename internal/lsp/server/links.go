package server

import (
	"cmp"
	"context"
	"encoding/json"
	"go/ast"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// textDocument/documentLink (step 11.9): a header import ("<raylib.h>",
// "./physics.h") is a link to the header file, found the way the build finds
// it (cache.Snapshot.HeaderFile: the folder of the file for a quoted include,
// the compiler's include path for a system header). A header that is not
// found (no C++ compiler yet, or no such file) has no link. The addresses
// in comments (http:// and https://) are links too, like gopls's. The links
// carry their target, so the server has no documentLink/resolve.

func (s *Server) addLinks() {
	s.requests["textDocument/documentLink"] = method{run: s.documentLink}
}

func linkCapabilities(caps *protocol.ServerCapabilities) {
	caps.DocumentLinkProvider = &protocol.DocumentLinkOptions{}
}

// urlPattern matches an address in a comment.
var urlPattern = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `\]}]*[^\s<>"'` + "`" + `\]}.,;:!?)]`)

func (s *Server) documentLink(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.DocumentLinkParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	links := []protocol.DocumentLink{}
	q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, protocol.Position{})
	if err != nil || q == nil {
		return links, err
	}
	if info := q.file.Info; info != nil {
		for _, inc := range info.Includes {
			if inc.Spec == nil || inc.Spec.Path == nil || !inc.Spec.Path.Pos().IsValid() {
				continue
			}
			path, err := q.headerFile(ctx, inc)
			if err != nil {
				return nil, err
			}
			if path == "" {
				continue
			}
			target := protocol.URI(locationURI(q.snap, path))
			links = append(links, protocol.DocumentLink{
				Range:   q.rangeOf(inc.Spec.Path.Pos(), inc.Spec.Path.End()),
				Target:  &target,
				Tooltip: "Open the header " + inc.Path,
			})
		}
	}
	for _, cg := range q.file.AST.Comments {
		for _, c := range cg.List {
			links = append(links, q.commentLinks(c)...)
		}
	}
	slices.SortFunc(links, func(a, b protocol.DocumentLink) int {
		return cmp.Or(cmp.Compare(a.Range.Start.Line, b.Range.Start.Line), cmp.Compare(a.Range.Start.Character, b.Range.Start.Character))
	})
	return links, nil
}

// commentLinks returns the links to the addresses in the comment c.
func (q *query) commentLinks(c *ast.Comment) []protocol.DocumentLink {
	if !c.Pos().IsValid() || !strings.Contains(c.Text, "://") {
		return nil
	}
	var links []protocol.DocumentLink
	start := q.tok.Offset(c.Pos())
	for _, m := range urlPattern.FindAllStringIndex(c.Text, -1) {
		u, err := url.Parse(c.Text[m[0]:m[1]])
		if err != nil || u.Host == "" {
			continue
		}
		target := protocol.URI(u.String())
		links = append(links, protocol.DocumentLink{
			Range:  protocol.Range{Start: q.mapper.Position(start + m[0]), End: q.mapper.Position(start + m[1])},
			Target: &target,
		})
	}
	return links
}
