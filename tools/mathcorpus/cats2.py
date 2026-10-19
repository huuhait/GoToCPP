# Constants, conversions, arithmetic and contraction: the language side of Go's float semantics, on amd64.
from lib import case
from cats1 import PRELUDE, HEAD

CANON = '''
// cw and cw32 are the bits of a result with every NaN as one value: which payload and sign a NaN
// result of arithmetic gets (two NaN operands, min and max) depends on the instruction order that the
// compiler picks, in Go and in C++, and is not part of the contract (ADR-071).
func cw(x float64) uint64 {
	if x != x {
		return 0x7ff8000000000001
	}
	return math.Float64bits(x)
}

func cw32(x float32) uint64 {
	if x != x {
		return 0x7fc00001
	}
	return uint64(math.Float32bits(x))
}
'''

def prog(body, decls="", imports=None):
    return HEAD + PRELUDE + "\n" + decls + "\nfunc main() {\n" + body + "\n}\n"

def plain(body, decls="", imports='\t"fmt"\n\t"math"\n'):
    return 'package main\n\nimport (\n' + imports + ')\n\n' + decls + "\nfunc main() {\n" + body + "\n}\n"

# ---- constants ----
case("math_consts_values", "The constants of package math: values printed with %v and as bits, the integer limits as typed values.",
     plain('''
	fmt.Println(math.E, math.Pi, math.Phi, math.Sqrt2, math.SqrtE, math.SqrtPi, math.SqrtPhi, math.Ln2, math.Log2E, math.Ln10, math.Log10E)
	fmt.Printf("%016x %016x %016x %016x\\n", math.Float64bits(math.E), math.Float64bits(math.Pi), math.Float64bits(math.Phi), math.Float64bits(math.Sqrt2))
	fmt.Printf("%016x %016x %016x %016x\\n", math.Float64bits(math.SqrtE), math.Float64bits(math.SqrtPi), math.Float64bits(math.SqrtPhi), math.Float64bits(math.Ln2))
	fmt.Printf("%016x %016x %016x\\n", math.Float64bits(math.Log2E), math.Float64bits(math.Ln10), math.Float64bits(math.Log10E))
	fmt.Println(math.MaxFloat32, math.SmallestNonzeroFloat32, math.MaxFloat64, math.SmallestNonzeroFloat64)
	fmt.Printf("%016x %016x %08x %08x\\n", math.Float64bits(math.MaxFloat64), math.Float64bits(math.SmallestNonzeroFloat64), math.Float32bits(math.MaxFloat32), math.Float32bits(math.SmallestNonzeroFloat32))
	fmt.Println(math.MaxInt8, math.MinInt8, math.MaxInt16, math.MinInt16, math.MaxInt32, math.MinInt32, math.MaxInt64, math.MinInt64)
	fmt.Println(math.MaxUint8, math.MaxUint16, math.MaxUint32, uint64(math.MaxUint64))
	fmt.Println(math.MaxInt, math.MinInt, uint(math.MaxUint))
	var i8 int8 = math.MaxInt8
	var i64 int64 = math.MinInt64
	var u64 uint64 = math.MaxUint64
	fmt.Println(i8, i64, u64, u64+1, i64-1, -i64)
'''))
case("math_consts_float32", "The constants of package math converted to float32: rounded to nearest, as Go does.",
     plain('''
	for _, f := range []float32{math.E, math.Pi, math.Phi, math.Sqrt2, math.Ln2, math.Ln10, math.MaxFloat32, math.SmallestNonzeroFloat32} {
		fmt.Printf("%08x %v\\n", math.Float32bits(f), f)
	}
	var a float32 = math.Pi
	var b float64 = math.Pi
	fmt.Println(float64(a) == b, float32(b) == a, float64(a)-b)
	fmt.Printf("%08x %016x\\n", math.Float32bits(a), math.Float64bits(float64(a)))
'''))
case("math_const_expr", "Constant expressions are exact (arbitrary precision) and rounded once to float64 or float32, as in Go.",
     plain('''
	const big = 1 << 100
	const third = 1.0 / 3
	const huge = math.MaxFloat64 * 2 / 4
	const tiny = math.SmallestNonzeroFloat64 * 1e300 * 1e10 / 1e310
	fmt.Println(float64(big)/(1<<98), third, huge, tiny)
	fmt.Println(math.Pi*1e300/1e300, math.Pi*math.Pi/math.Pi == math.Pi, math.E*math.E)
	fmt.Printf("%016x %016x %016x\\n", math.Float64bits(third), math.Float64bits(huge), math.Float64bits(tiny))
	const x = 0.1 + 0.2
	var y = 0.1
	y += 0.2
	fmt.Println(x, y, x == 0.3, y == 0.3)
	fmt.Printf("%016x %016x\\n", math.Float64bits(x), math.Float64bits(y))
	const f32 float32 = 0.1
	fmt.Println(f32, float64(f32), f32*3, float64(f32*3))
	const c = math.MaxInt64 / 3 * 2.5
	fmt.Println(c, math.MaxUint64/7, uint64(math.MaxUint64/7))
	const e = 1e23
	const f = 8.41e21
	fmt.Println(e, f, float32(e), float32(f))
	fmt.Printf("%016x %016x %08x\\n", math.Float64bits(e), math.Float64bits(f), math.Float32bits(float32(e)))
	var one = 1.0
	fmt.Println(one/3 == third, (one/3)*3, math.Pi*one == math.Pi)
'''))

# ---- float -> int conversions (Go on amd64) ----
CONV_SPECIALS = r'''
func convSpecials() []float64 {
	s := []float64{0, math.Copysign(0, -1), 0.5, -0.5, 0.9999999999999999, -0.9999999999999999, 1, -1, 1.5, -1.5, 2.5, 126.9, 127, 127.5, 128, -128, -128.9, -129, 255, 255.9, 256, -256, 32767, 32767.9,
		32768, -32768, -32769, 65535, 65535.9, 65536, 2147483647, 2147483647.9, 2147483648, -2147483648, -2147483648.9, -2147483649, 4294967295, 4294967295.9,
		4294967296, -4294967296, 1 << 53, 1<<53 + 2, 1 << 62, 9.2233720368547748e18, 9.223372036854775807e18, 9.2233720368547758e18, -9.2233720368547758e18, -9.2233720368547768e18,
		1.8446744073709550e19, 1.8446744073709552e19, 1.8446744073709554e19, 1e19, 1e20, 1e30, -1e19, -1e20, -1e30, 1e300, -1e300, math.MaxFloat64, -math.MaxFloat64,
		math.Inf(1), math.Inf(-1), math.NaN(), math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64, 1e-300, 9.223372036854775e18, 1.5 * (1 << 63), 3 * (1 << 62), 1<<63 + 1<<11, 1<<63 + 1<<12}
	n := len(s)
	for i := 0; i < n; i++ {
		s = append(s, math.Nextafter(s[i], math.Inf(1)), math.Nextafter(s[i], math.Inf(-1)))
	}
	return s
}
'''
case("math_f2i_float64_table", "float64 to every integer type, for the special values and their neighbours: Go's amd64 results (out-of-range conversions are implementation-specific in Go; gocpp copies amd64).",
     plain('''
	for i, f := range convSpecials() {
		fmt.Printf("%d %016x | %d %d %d %d | %d %d %d %d | %d %d\\n", i, math.Float64bits(f), int(f), int64(f), int32(f), int16(f), int8(f), uint64(f), uint32(f), uint16(f), uint8(f), uint(f))
	}
''', CONV_SPECIALS))
case("math_f2i_float32_table", "float32 to every integer type, for the special values and their neighbours: Go's amd64 results.",
     plain('''
	for i, d := range convSpecials() {
		f := float32(d)
		fmt.Printf("%d %08x | %d %d %d %d | %d %d %d %d | %d %d\\n", i, math.Float32bits(f), int(f), int64(f), int32(f), int16(f), int8(f), uint64(f), uint32(f), uint16(f), uint8(f), uint(f))
	}
''', CONV_SPECIALS))
case("math_f2i_random", "float64 to integer conversion on random values of all magnitudes (including out of range): hash identical to Go's.",
     prog('''
	xs := inputs()
	var h [10]uint64
	for i := range h {
		h[i] = 14695981039346656037
	}
	for _, x := range xs {
		h[0] = mix(h[0], uint64(int64(x)))
		h[1] = mix(h[1], uint64(int32(x)))
		h[2] = mix(h[2], uint64(int16(x)))
		h[3] = mix(h[3], uint64(int8(x)))
		h[4] = mix(h[4], uint64(x))
		h[5] = mix(h[5], uint64(uint32(x)))
		h[6] = mix(h[6], uint64(uint16(x)))
		h[7] = mix(h[7], uint64(uint8(x)))
		h[8] = mix(h[8], uint64(int(x)))
		h[9] = mix(h[9], uint64(uint(x)))
	}
	for i, v := range h {
		fmt.Printf("%d %016x\\n", i, v)
	}
	var f32 [4]uint64
	for i := range f32 {
		f32[i] = 14695981039346656037
	}
	for _, x := range xs {
		f := float32(x)
		f32[0] = mix(f32[0], uint64(int64(f)))
		f32[1] = mix(f32[1], uint64(int32(f)))
		f32[2] = mix(f32[2], uint64(uint64(f)))
		f32[3] = mix(f32[3], uint64(uint32(f)))
	}
	fmt.Printf("%016x %016x %016x %016x\\n", f32[0], f32[1], f32[2], f32[3])
'''))
case("math_f2i_variables", "Conversions of float variables inside expressions and loops (not constant folded): out-of-range values included.",
     plain('''
	vals := []float64{1e18, 1e19, 2e19, -1e19, 9.3e18, math.Inf(1), math.NaN(), 3.99, -3.99}
	var sum int64
	var usum uint64
	for _, v := range vals {
		sum += int64(v)
		usum += uint64(v)
		x := v * 2
		fmt.Println(int64(x), uint64(x), int32(x), uint32(x), int64(float32(x)))
	}
	fmt.Println(sum, usum)
	var f float64 = 255.9
	var b byte = byte(f)
	var i8 = int8(f)
	var u16 = uint16(f * 300)
	fmt.Println(b, i8, u16, byte(f+1), int8(-f-1))
'''))
case("math_i2f_conversions", "Integer to float conversions: round to nearest even above 2^53, uint64 above 2^63, float32 without double rounding.",
     prog('''
	ints := []int64{0, 1, -1, 1 << 53, 1<<53 + 1, 1<<53 + 2, 1<<53 + 3, -(1<<53 + 1), 1<<62 + 1<<9, 1<<62 + 1<<9 + 1, 1<<62 + 3<<8, math.MaxInt64, math.MinInt64, math.MaxInt64 - 1, 16777217, 16777216, 16777219, -16777217, 1<<24 + 1, 1<<40 + 1<<16 + 1}
	for _, v := range ints {
		fmt.Printf("%d %016x %08x\\n", v, math.Float64bits(float64(v)), math.Float32bits(float32(v)))
	}
	us := []uint64{0, 1<<63 - 1, 1 << 63, 1<<63 + 1, 1<<63 + 1<<10, 1<<63 + 1<<10 + 1, 1<<63 + 3<<10, math.MaxUint64, math.MaxUint64 - 1, 1<<64 - 1<<11, 1<<64 - 1<<10, 1<<64 - 1<<10 - 1, 1<<53 + 1}
	for _, v := range us {
		fmt.Printf("%d %016x %08x\\n", v, math.Float64bits(float64(v)), math.Float32bits(float32(v)))
	}
	h := uint64(14695981039346656037)
	for i := 0; i < 100000; i++ {
		v := next()
		h = mix(h, math.Float64bits(float64(v)))
		h = mix(h, uint64(math.Float32bits(float32(v))))
		h = mix(h, math.Float64bits(float64(int64(v))))
		h = mix(h, uint64(math.Float32bits(float32(int64(v)))))
		w := uint32(v)
		h = mix(h, math.Float64bits(float64(w)))
		h = mix(h, uint64(math.Float32bits(float32(w))))
		h = mix(h, math.Float64bits(float64(int32(w))))
		s := int16(v)
		h = mix(h, math.Float64bits(float64(s)))
	}
	fmt.Printf("%016x\\n", h)
'''))

# ---- no contraction ----
case("math_fma_contraction_float64", "a*b+c is two roundings (no fused multiply-add), as in gc on amd64: values where fusing changes the result (the operands come from a function the C++ compiler cannot fold).",
     prog('''
	// x*x - 1 for x = 1+2^-27 differs when fused; the operands are not compile-time constants
	e := next() & 0
	x := 1 + math.Ldexp(1, -27+int(e))
	c := -(1 + math.Ldexp(1, -26+int(e)))
	fmt.Printf("%016x %016x\\n", math.Float64bits(x*x+c), math.Float64bits(math.FMA(x, x, c)))
	a, b := 0.1+float64(e), 10.0+float64(e)
	fmt.Printf("%016x %016x\\n", math.Float64bits(a*b-1), math.Float64bits(math.FMA(a, b, -1)))
	h := uint64(14695981039346656037)
	diff := 0
	const n = 200000
	ps := make([]float64, n)
	qs := make([]float64, n)
	rs := make([]float64, n)
	for i := 0; i < n; i++ {
		ps[i] = math.Float64frombits(next()&^(0x7ff<<52) | uint64(1023-4+int(next()%8))<<52)
		qs[i] = math.Float64frombits(next()&^(0x7ff<<52) | uint64(1023-4+int(next()%8))<<52)
		rs[i] = -(ps[i] * qs[i])
	}
	for i := 0; i < n; i++ {
		p, q, r := ps[i], qs[i], rs[i]
		s := p*q + r
		f := math.FMA(p, q, r)
		if s != f {
			diff++
		}
		z := p*q - -r
		w := r + p*q
		h = mix(mix(mix(h, math.Float64bits(s)), math.Float64bits(z)), math.Float64bits(w))
	}
	fmt.Printf("%016x %v\\n", h, diff > 0)
	var u, v, w2 float64 = 3.0000000000000004, 3.0000000000000004, -9.000000000000003
	u += float64(e)
	fmt.Printf("%016x %016x\\n", math.Float64bits(u*v+w2), math.Float64bits(math.FMA(u, v, w2)))
	fmt.Printf("%016x\\n", math.Float64bits(u*v-(-w2)))
'''))
case("math_fma_contraction_forms", "No fusion in any expression shape: sums of products, Horner polynomials, dot products, a += b*c, compound assignments, expressions through function calls.",
     prog('''
	xs := inputs()
	h := uint64(14695981039346656037)
	for i := 0; i+3 < len(xs); i += 4 {
		a, b, c, d := xs[i], xs[i+1], xs[i+2], xs[i+3]
		if a-a != 0 || b-b != 0 || c-c != 0 || d-d != 0 {
			continue // NaN and Inf: the payload of a NaN result is not the point here
		}
		h = mix(h, math.Float64bits(a*b+c*d))
		h = mix(h, math.Float64bits(a*b-c*d))
		h = mix(h, math.Float64bits(((a*b+c)*d+a)*b+c))
		acc := a
		acc += b * c
		acc -= c * d
		h = mix(h, math.Float64bits(acc))
		h = mix(h, math.Float64bits(a*b+c*d+a*c+b*d))
		h = mix(h, math.Float64bits(float64(float32(a))*float64(float32(b))+c))
		f, g, k := float32(a), float32(b), float32(c)
		h = mix(h, uint64(math.Float32bits(f*g+k)))
		h = mix(h, uint64(math.Float32bits(f*g-k*f)))
	}
	fmt.Printf("%016x\\n", h)
	// a loop that a compiler could vectorize and fuse
	s := 0.0
	for i := 1; i <= 100000; i++ {
		x := float64(i) * 0.001
		s += x * x * 1.0000001
	}
	fmt.Printf("%016x %v\\n", math.Float64bits(s), s)
	// complex multiplication
	z1 := complex(1+0x1p-27, 1-0x1p-29)
	z2 := complex(1-0x1p-27, 1+0x1p-31)
	m := z1 * z2
	fmt.Printf("%016x %016x\\n", math.Float64bits(real(m)), math.Float64bits(imag(m)))
'''))

# ---- arithmetic edge cases ----
case("math_arith_edge", "Float arithmetic on special values: + - * / on all pairs, division by zero (Inf, NaN, no panic), negative zero.",
     prog('''
	sp := specials()
	h := [6]uint64{}
	for i := range h {
		h[i] = 14695981039346656037
	}
	for i := 0; i < len(sp); i += 2 {
		for j := 0; j < len(sp); j += 2 {
			x, y := sp[i], sp[j]
			h[0] = mix(h[0], cw(x+y))
			h[1] = mix(h[1], cw(x-y))
			h[2] = mix(h[2], cw(x*y))
			h[3] = mix(h[3], cw(x/y))
			f, g := float32(x), float32(y)
			h[4] = mix(h[4], cw32(f+g))
			h[5] = mix(h[5], cw32(f/g))
		}
	}
	for i, v := range h {
		fmt.Printf("%d %016x\\n", i, v)
	}
	var z, one float64 = 0, 1
	fmt.Println(one/z, -one/z, z/z, -z, one/(-z), z*-1, math.IsNaN(z/z))
	var fz, fone float32 = 0, 1
	fmt.Println(fone/fz, -fone/fz, fz/fz, -fz)
''', CANON))
case("math_nan_compare", "Comparisons with NaN and signed zeros: all six operators on all pairs of special values, identical to Go.",
     prog('''
	sp := specials()
	var h uint64 = 14695981039346656037
	for i := 0; i < len(sp); i++ {
		for j := 0; j < len(sp); j += 3 {
			x, y := sp[i], sp[j]
			m := b2w(x == y) | b2w(x != y)<<1 | b2w(x < y)<<2 | b2w(x <= y)<<3 | b2w(x > y)<<4 | b2w(x >= y)<<5
			f, g := float32(x), float32(y)
			m |= (b2w(f == g) | b2w(f != g)<<1 | b2w(f < g)<<2 | b2w(f <= g)<<3 | b2w(f > g)<<4 | b2w(f >= g)<<5) << 6
			h = mix(h, m)
		}
	}
	fmt.Printf("%016x\\n", h)
	nan := math.NaN()
	fmt.Println(nan == nan, nan != nan, nan < 1, nan > 1, !(nan < 1), !(nan >= 1))
	m := map[float64]int{}
	m[nan] = 1
	m[nan] = 2
	m[0] = 3
	m[math.Copysign(0, -1)] = 4
	fmt.Println(len(m), m[0], m[nan])
'''))
case("math_minmax_builtin", "The built-in min and max on floats: NaN wins, -0 is less than +0, as in Go.",
     prog('''
	sp := specials()
	var h uint64 = 14695981039346656037
	for i := 0; i < len(sp); i++ {
		for j := 0; j < len(sp); j += 2 {
			x, y := sp[i], sp[j]
			h = mix(mix(h, cw(min(x, y))), cw(max(x, y)))
			f, g := float32(x), float32(y)
			h = mix(mix(h, cw32(min(f, g))), cw32(max(f, g)))
			h = mix(h, cw(min(x, y, 1)))
			h = mix(h, cw(max(x, y, 0.5, -3)))
		}
	}
	fmt.Printf("%016x\\n", h)
	nz := math.Copysign(0, -1)
	fmt.Println(min(0.0, nz), max(0.0, nz), min(nz, 0.0), max(nz, 0.0), min(math.NaN(), 1), max(1, math.NaN()), min(math.Inf(-1), math.NaN()))
	fmt.Println(math.Signbit(min(0.0, nz)), math.Signbit(max(0.0, nz)), math.Signbit(min(nz, 0.0)), math.Signbit(max(nz, 0.0)))
	fmt.Println(math.Max(0, nz), math.Min(0, nz), math.Signbit(math.Max(nz, nz)), math.Signbit(math.Min(0, 0)))
''', CANON))
case("math_float32_arith", "float32 arithmetic rounds every operation to float32 (no excess precision): sums, products, divisions, chains.",
     prog('''
	xs := inputs()
	var h uint64 = 14695981039346656037
	for i := 0; i+2 < len(xs); i += 3 {
		a, b, c := float32(xs[i]), float32(xs[i+1]), float32(xs[i+2])
		h = mix(h, cw32(a+b))
		h = mix(h, cw32(a*b))
		h = mix(h, cw32(a/b))
		h = mix(h, cw32(a-b))
		h = mix(h, cw32((a*b+c)*a-c/b))
		h = mix(h, cw32(float32(math.Sqrt(float64(a)))))
	}
	fmt.Printf("%016x\\n", h)
	var s float32
	for i := 1; i <= 1000000; i++ {
		s += 1 / float32(i)
	}
	var d float64
	for i := 1; i <= 1000000; i++ {
		d += 1 / float64(i)
	}
	fmt.Printf("%08x %016x %v %v\\n", math.Float32bits(s), math.Float64bits(d), s, d)
''', CANON))
case("math_float32_conv", "float32(float64) and float64(float32): rounding to nearest even, subnormals, overflow to Inf, NaN payloads.",
     prog('''
	xs := inputs()
	var h uint64 = 14695981039346656037
	for _, x := range xs {
		f := float32(x)
		h = mix(h, uint64(math.Float32bits(f)))
		h = mix(h, math.Float64bits(float64(f)))
	}
	fmt.Printf("%016x\\n", h)
	for _, x := range []float64{3.4028235677973366e38, 3.4028234663852886e38, 3.4028235677973362e38, 1.401298464324817e-45, 7.006492321624085e-46, 7.006492321624086e-46, 1e-46, 1.1754943508222875e-38, 1.1754942106924411e-38,
		math.Float64frombits(0x7ff0000000000001), math.Float64frombits(0xfff8000000000001), math.Float64frombits(0x7ffffffffffffff0)} {
		f := float32(x)
		fmt.Printf("%016x %08x %016x\\n", math.Float64bits(x), math.Float32bits(f), math.Float64bits(float64(f)))
	}
	for _, b := range []uint32{0x7f800001, 0x7fc00000, 0xffc00001, 0x7fffffff, 0x00000001, 0x007fffff, 0x80000001} {
		f := math.Float32frombits(b)
		fmt.Printf("%08x %016x %08x\\n", b, math.Float64bits(float64(f)), math.Float32bits(f*1))
	}
'''))
case("math_negative_zero", "Negative zero: -x of a variable, 0 * -1, x - x, Copysign, Signbit, printing, and division by it.",
     plain('''
	var z float64
	var f32 float32
	nz := -z
	fmt.Println(nz, -f32, z*-1, 0*nz, nz+nz, nz-z, z-z, nz+z, 1/nz, 1/z)
	fmt.Printf("%v %g %e %f %.2f %+v %08.3f %x\\n", nz, nz, nz, nz, nz, nz, nz, nz)
	fmt.Println(math.Signbit(nz), math.Signbit(z*-1), math.Signbit(float64(-f32)), math.Copysign(3, nz), math.Copysign(nz, 1))
	fmt.Printf("%016x %016x %016x\\n", math.Float64bits(nz), math.Float64bits(nz*nz), math.Float64bits(math.Abs(nz)))
	fmt.Println(math.Sqrt(nz), math.Floor(nz), math.Ceil(-0.3), math.Trunc(-0.3), math.Round(-0.3), math.RoundToEven(-0.3), math.Mod(nz, 3), math.Max(nz, z), math.Min(nz, z))
'''))
case("math_series_sums", "Long float sums and products: the order of operations is Go's (no reassociation, no vectorized reductions).",
     plain('''
	s, sgn := 0.0, 1.0
	for i := 0; i < 5000000; i++ {
		s += sgn / float64(2*i+1)
		sgn = -sgn
	}
	fmt.Printf("%016x %v\\n", math.Float64bits(4*s), 4*s)
	h := 0.0
	for i := 1; i <= 3000000; i++ {
		h += 1 / float64(i)
	}
	fmt.Printf("%016x %v\\n", math.Float64bits(h), h)
	p := 1.0
	for i := 1; i <= 200; i++ {
		p *= 1 + 1/float64(i*i)
	}
	fmt.Printf("%016x %v\\n", math.Float64bits(p), p)
	var a [1000]float64
	for i := range a {
		a[i] = math.Sin(float64(i)) * 1e8
	}
	t := 0.0
	for k := 0; k < 100; k++ {
		for _, v := range a {
			t += v
		}
	}
	fmt.Printf("%016x %v\\n", math.Float64bits(t), t)
'''))
