package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The calls of C/C++ functions that return the typedef HRESULT are Go errors (ADR-137). The check
// wraps each such call in error(call) before it is done with the package (check.Config.HResults),
// so the program has a conversion to error where the call is, and this is where the C++ makes the
// value that the type says:
//
//	hr := swapChain.ResizeBuffers(...)   auto hr = gp::hresult_error(swapChain->ResizeBuffers(...));
//	swapChain.Present(1, 0)              swapChain->Present(1, 0);           (the result is dropped)
//	n := int32(swapChain.Present(1, 0))  int32_t n = gp::cgo::as<int32_t>(swapChain->Present(1, 0));
//
// The call is the argument table's: out-parameters are copied back before the result is looked at,
// as always. A call the program drops, or converts to a number to get the code, is not wrapped.

// hresultError returns the C++ of the conversion c to the type to when it is error(call) of a call
// that returns an HRESULT: gp::hresult_error(call), the value that is nil for a code that is not
// negative.
func (fg *funcGen) hresultError(c *ast.CallExpr, to types.Type) (cppir.Expr, bool) {
	g := fg.g
	if !types.Identical(to, errorType) {
		return nil, false
	}
	// The conversion that check wrote around the call, or one that the program wrote around a call that
	// the probe says returns an HRESULT. (A package with generic functions is lowered without the
	// typed view, but check wrapped its calls all the same.)
	if !g.pkg.IsHResultCall(c) {
		inner, ok := ast.Unparen(c.Args[0]).(*ast.CallExpr)
		if !ok || !g.typedOn() || !g.interop.typed.rep.ReturnsHResult(g.pkg, inner) {
			return nil, false
		}
	}
	return call("gp::hresult_error", fg.expr(c.Args[0])), true
}

// errorType is Go's predeclared error.
var errorType = types.Universe.Lookup("error").Type()
