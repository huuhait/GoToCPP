package lower

import (
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// anonTables returns the method tables (the method sets of T and *T, as expressions for td_struct)
// of the anonymous struct type st with embedded fields, whose promoted methods go into its
// descriptor; the thunks and tables are written with the descriptors (descDecls). Both are nil for
// a struct without methods.
func (g *gen) anonTables(st *types.Struct, pos token.Pos) (vmt, pmt cppir.Expr) {
	si := g.structOf(st, pos)
	if si == nil {
		return nil, nil
	}
	dt := g.dtable()
	nd := dt.anon[si]
	if nd == nil {
		nd = &namedDesc{ty: st, si: si}
		if dt.anon == nil {
			dt.anon = make(map[*structInfo]*namedDesc)
		}
		dt.anon[si] = nd
		dt.anons = append(dt.anons, nd)
		var decls []cppir.Decl
		nd.vmt, nd.pmt = g.tables(nd, pos, &decls)
		nd.decls = decls
	}
	return nd.vmt, nd.pmt
}

// needForwarders says that the struct of t (a struct type or a pointer to one, named or not) needs
// its forwarding members (promote.go): its method tables call them.
func (g *gen) needForwarders(t types.Type) {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok && g.tdecls().named[n.Obj()] == nil {
		return // a named type without a C++ struct (type P *[]int, type F func()): no methods to forward
	}
	if si := g.structOf(t, token.NoPos); si != nil {
		// (The owner of the struct writes its forwarders: a struct shared by several packages
		// is defined once, from its owner's table, whoever asks for its descriptor.)
		o := si.owner
		if o.fwdNeed == nil {
			o.fwdNeed = make(map[*structInfo]bool)
		}
		o.fwdNeed[si] = true
	}
}
