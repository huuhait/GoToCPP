package lower

import (
	"go/ast"
	"go/constant"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Complex numbers (Phase 03 step 3.15c; the runtime is gocpp/complex.hpp):
//
//	Go                          C++
//	complex64, complex128       gp::complex64, gp::complex128 (gp::complex<float>, <double>)
//	1 + 2i (a complex128)       gp::complex128(1.0, 2.0)
//	complex64(1.5i)             gp::complex64(0.0f, 1.5f)
//	complex(r, i)               gp::complex128(r, i) (gp::complex64 for float32 parts)
//	real(c), imag(c)            gp::real(c), gp::imag(c)
//	a + b, a * b, a / b, -a     the same operators (Go's rules: complex.hpp)
//	a == b, a != b              the same operators
//	complex64(c), complex128(c) gp::complex64(c), gp::complex128(c) (ops.go: convertValue)
//	type C complex128           struct C : gp::newtype<C, gp::complex128> (named.go)
//
// go/types folds every constant expression, also real(1+2i) and complex(1, 2): a constant is
// written with its parts (complexConstant). The builtins have no effect of their own, but gc
// evaluates them in their places among the calls, like len (order.go): in real(g) + f(), g is
// read before f runs, so real(g) is then a temporary.

// complexConstant returns the C++ of the complex constant v of the complex kind: its parts,
// each the shortest literal that reads back as the same float64 (float32 for complex64, each
// part rounded to float32 as Go does).
func complexConstant(v constant.Value, kind types.BasicKind) cppir.Expr {
	f32 := kind == types.Complex64
	re := floatLiteral(constant.Real(v), f32)
	im := floatLiteral(constant.Imag(v), f32)
	return call(basicTypes[kind], re, im)
}

// complexBuiltin returns the C++ of a call of the builtin complex, real or imag whose value is
// not a constant (go/types folds a constant one). A part or a complex number of a named type
// (type F float64) is its value; the result has an unnamed type in Go too.
func (fg *funcGen) complexBuiltin(c *ast.CallExpr, name string) cppir.Expr {
	if name == "complex" {
		t := fg.g.info.TypeOf(c)
		args := make([]cppir.Expr, len(c.Args))
		f32 := underBasic(t).Kind() == types.Complex64
		for i, a := range c.Args {
			if v := fg.constValue(a); v != nil {
				args[i] = floatLiteral(v, f32) // also of a named float type: complex(F(1), f)
			} else {
				args[i] = valueOf(fg.typeOf(a), fg.expr(a))
			}
		}
		return construct(fg.g.cxxType(t, c.Pos()), args...)
	}
	arg := c.Args[0]
	return call("gp::"+name, valueOf(fg.typeOf(arg), fg.expr(arg)))
}
