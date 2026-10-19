# Per-function sweeps: every exported function of math on about 100k inputs (special values, neighbours of
# the special values, random bit patterns, random values of several ranges), compared with real Go through
# the FNV hash of the result bits and 200 sampled raw results.
from lib import case

PRELUDE = r'''
var state uint64 = 0x9E3779B97F4A7C15

func next() uint64 { // splitmix64
	state += 0x9E3779B97F4A7C15
	z := state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func unit() float64 { return float64(next()>>11) / (1 << 53) }

func sgn() float64 {
	if next()&1 == 0 {
		return 1
	}
	return -1
}

func mix(h, w uint64) uint64 {
	h ^= w
	h *= 1099511628211
	h ^= h >> 29
	return h
}

func b2w(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func specials() []float64 {
	s := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 2, -2, 3, 10, math.Inf(1), math.Inf(-1), math.NaN(),
		math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64,
		2.2250738585072014e-308, 2.225073858507201e-308, math.MaxFloat32, 1e-10, 1e10, 1e15, 1e300, 1e-300,
		math.Pi, math.Pi / 2, math.Pi / 4, 3 * math.Pi / 2, 2 * math.Pi, 1e6 * math.Pi, math.E, math.Ln2, math.Ln10,
		709.782712893384, 709.79, -745.13321910194122, -745.2, -708.3964185322641, 1 << 52, 1 << 53, 1 << 62, 1 << 63,
		-1 << 63, 1 << 64, 0.7071067811865476, 0.7071067811865475, 1.4142135623730951, 0.9999999999999999, 1.0000000000000002,
		171.62437695630272, 171.7, 172, 1.0e22, 4503599627370496.5, 6.5, 7.5, 0.4999999999999999, 1e-5, 8.0e-5, 9.5367431640625e-07, 0.1, 0.2, 0.3, 1.0 / 3, 2.0 / 3,
		math.Float64frombits(0x7ff0000000000001), math.Float64frombits(0x7ff8000000000000), math.Float64frombits(0xfff8000000000000),
		math.Float64frombits(0x7ff4479c6aacc07a), math.Float64frombits(0xfff8000000000001), math.Float64frombits(0x7fffffffffffffff),
		math.Float64frombits(0xffffffffffffffff), math.Float64frombits(0xfff0000000000001)}
	for i := -40; i <= 40; i++ {
		s = append(s, float64(i), float64(i)+0.5, float64(i)/3, float64(i)*math.Pi/4)
	}
	n := len(s)
	for i := 0; i < n; i++ {
		x := s[i]
		s = append(s, math.Nextafter(x, math.Inf(1)), math.Nextafter(x, math.Inf(-1)), -x)
	}
	return s
}

func inputs() []float64 {
	xs := specials()
	for i := 0; i < 20000; i++ {
		xs = append(xs, math.Float64frombits(next()))
	}
	for i := 0; i < 30000; i++ {
		xs = append(xs, sgn()*unit())
	}
	for i := 0; i < 30000; i++ {
		xs = append(xs, sgn()*(1+unit())*math.Ldexp(1, int(next()%61)-30))
	}
	for i := 0; i < 20000; i++ {
		xs = append(xs, (unit()*2-1)*1000)
	}
	return xs
}

func pairs() ([]float64, []float64) {
	sp := specials()
	var as, bs []float64
	for i := 0; i < len(sp); i += 3 {
		for j := 0; j < len(sp); j += 3 {
			as = append(as, sp[i])
			bs = append(bs, sp[j])
		}
	}
	for i := 0; i < 30000; i++ {
		as = append(as, sgn()*unit()*20)
		bs = append(bs, sgn()*unit()*20)
	}
	for i := 0; i < 30000; i++ {
		as = append(as, sgn()*(1+unit())*math.Ldexp(1, int(next()%41)-20))
		bs = append(bs, float64(int(next()%61)-30)+[]float64{0, 0.5, 0.25, 0.1}[next()%4])
	}
	for i := 0; i < 20000; i++ {
		as = append(as, math.Float64frombits(next()))
		bs = append(bs, math.Float64frombits(next()))
	}
	for i := 0; i < 20000; i++ {
		as = append(as, (unit()*2-1)*1000)
		bs = append(bs, (unit()*2-1)*1000)
	}
	return as, bs
}
'''

HEAD = 'package main\n\nimport (\n\t"fmt"\n\t"math"\n)\n'

def sweep1(stmts, words, extra_imports=""):
    """a program that runs stmts (using x) over inputs() and hashes the words"""
    body = "func main() {\n\txs := inputs()\n\th := uint64(14695981039346656037)\n\tfor i, x := range xs {\n"
    for s in stmts:
        body += "\t\t" + s + "\n"
    for w in words:
        body += "\t\th = mix(h, %s)\n" % w
    body += "\t\tif i%500 == 0 {\n\t\t\tfmt.Printf(\"%016x" + " %016x" * len(words) + "\\n\", math.Float64bits(x), " + ", ".join(words) + ")\n\t\t}\n\t}\n"
    body += "\tfmt.Printf(\"n=%d hash=%016x\\n\", len(xs), h)\n}\n"
    return HEAD + PRELUDE + "\n" + body

def sweep2(stmts, words):
    body = "func main() {\n\tas, bs := pairs()\n\th := uint64(14695981039346656037)\n\tfor i := range as {\n\t\tx, y := as[i], bs[i]\n"
    for s in stmts:
        body += "\t\t" + s + "\n"
    for w in words:
        body += "\t\th = mix(h, %s)\n" % w
    body += "\t\tif i%500 == 0 {\n\t\t\tfmt.Printf(\"%016x %016x" + " %016x" * len(words) + "\\n\", math.Float64bits(x), math.Float64bits(y), " + ", ".join(words) + ")\n\t\t}\n\t}\n"
    body += "\tfmt.Printf(\"n=%d hash=%016x\\n\", len(as), h)\n}\n"
    return HEAD + PRELUDE + "\n" + body

F = "math.Float64bits"

F1 = ["Abs", "Acos", "Acosh", "Asin", "Asinh", "Atan", "Atanh", "Cbrt", "Ceil", "Cos", "Cosh", "Erf", "Erfc",
      "Erfcinv", "Erfinv", "Exp", "Exp2", "Expm1", "Floor", "Gamma", "J0", "J1", "Log", "Log10", "Log1p", "Log2",
      "Logb", "Round", "RoundToEven", "Sin", "Sinh", "Sqrt", "Tan", "Tanh", "Trunc", "Y0", "Y1"]

for f in F1:
    case("math_%s_sweep" % f.lower(), "math.%s on about 100k inputs: bit-identical to Go (hash of the result bits and 200 samples)." % f,
         sweep1(["y := math.%s(x)" % f], [F + "(y)"]))

# two-argument functions
F2 = ["Atan2", "Copysign", "Dim", "Hypot", "Max", "Min", "Mod", "Nextafter", "Pow", "Remainder"]
for f in F2:
    case("math_%s_sweep" % f.lower(), "math.%s on about 100k pairs: bit-identical to Go (hash of the result bits and 200 samples)." % f,
         sweep2(["z := math.%s(x, y)" % f], [F + "(z)"]))

# the rest: other signatures
case("math_fma_sweep", "math.FMA on about 100k triples: bit-identical to Go.",
     sweep2(["p := x * y", "if p != p { p = 0 } // the NaN that * makes has a sign and payload that depend on the compiler (ADR-071)", "z := math.FMA(x, y, p-y)", "z2 := math.FMA(x, y, -p)", "z3 := math.FMA(y, x, 1)"], [F + "(z)", F + "(z2)", F + "(z3)"]))
case("math_modf_sweep", "math.Modf: integer and fractional parts, bit-identical to Go.",
     sweep1(["a, b := math.Modf(x)"], [F + "(a)", F + "(b)"]))
case("math_frexp_sweep", "math.Frexp: fraction and exponent, bit-identical to Go.",
     sweep1(["a, e := math.Frexp(x)"], [F + "(a)", "uint64(e)"]))
case("math_sincos_sweep", "math.Sincos: sine and cosine, bit-identical to Go (and to Sin, Cos).",
     sweep1(["s, c := math.Sincos(x)", "if math.Float64bits(s) != math.Float64bits(math.Sin(x)) && !math.IsNaN(s) { panic(\"Sincos != Sin\") }",
             "if math.Float64bits(c) != math.Float64bits(math.Cos(x)) && !math.IsNaN(c) { panic(\"Sincos != Cos\") }"],
            [F + "(s)", F + "(c)"]))
case("math_lgamma_sweep", "math.Lgamma: value and sign, bit-identical to Go.",
     sweep1(["a, sg := math.Lgamma(x)"], [F + "(a)", "uint64(sg + 1)"]))
case("math_ilogb_sweep", "math.Ilogb, bit-identical to Go.",
     sweep1(["a := math.Ilogb(x)"], ["uint64(a)"]))
case("math_isnan_isinf_sweep", "math.IsNaN, math.IsInf (all three signs), math.Signbit: identical to Go.",
     sweep1(["a, b, c, d, e := math.IsNaN(x), math.IsInf(x, 0), math.IsInf(x, 1), math.IsInf(x, -1), math.Signbit(x)"],
            ["b2w(a) | b2w(b)<<1 | b2w(c)<<2 | b2w(d)<<3 | b2w(e)<<4"]))
case("math_float64bits_sweep", "math.Float64bits and Float64frombits round trips, with NaN payloads, bit-identical to Go.",
     sweep1(["w := math.Float64bits(x)", "r := math.Float64frombits(w ^ 1)"], ["w", F + "(r)"]))
case("math_float32bits_sweep", "math.Float32bits and Float32frombits on float32(x), bit-identical to Go.",
     sweep1(["f := float32(x)", "w := math.Float32bits(f)", "r := math.Float32frombits(w + 1)"], ["uint64(w)", "uint64(math.Float32bits(r))"]))
case("math_nextafter32_sweep", "math.Nextafter32 on float32 pairs, bit-identical to Go.",
     sweep2(["z := math.Nextafter32(float32(x), float32(y))"], ["uint64(math.Float32bits(z))"]))
case("math_ldexp_sweep", "math.Ldexp(x, n) for n in about [-1200, 1200], bit-identical to Go.",
     sweep1(["n := int(next()%2401) - 1200", "z := math.Ldexp(x, n)", "z2 := math.Ldexp(x, n/20)"], [F + "(z)", F + "(z2)"]))
case("math_jn_yn_sweep", "math.Jn and math.Yn (orders -5..25), bit-identical to Go.",
     sweep1(["n := int(next()%31) - 5", "a := math.Jn(n, x)", "b := math.Yn(n, x)"], [F + "(a)", F + "(b)"]))
case("math_pow10_table", "math.Pow10 for n in [-500, 500], bit-identical to Go.", HEAD + '''
func main() {
	h := uint64(14695981039346656037)
	for n := -500; n <= 500; n++ {
		b := math.Float64bits(math.Pow10(n))
		h = (h ^ b) * 1099511628211
		if n%25 == 0 {
			fmt.Printf("%d %016x\\n", n, b)
		}
	}
	fmt.Printf("hash=%016x\\n", h)
}
''')
case("math_inf_nan_sign", "math.Inf(sign), math.NaN() and the signs of zero: the bit patterns are Go's.", HEAD + '''
func main() {
	fmt.Printf("%016x %016x %016x %016x\\n", math.Float64bits(math.Inf(1)), math.Float64bits(math.Inf(-1)), math.Float64bits(math.Inf(0)), math.Float64bits(math.NaN()))
	fmt.Println(math.IsInf(math.Inf(2), 1), math.IsInf(math.Inf(-3), -1), math.IsNaN(math.NaN()), math.NaN() == math.NaN())
	var z float64
	nz := math.Copysign(z, -1)
	fmt.Println(z, nz, 1/z, 1/nz, math.Signbit(nz), math.Signbit(z), nz == z)
	fmt.Printf("%016x %016x\\n", math.Float64bits(nz), math.Float64bits(-z))
}
''')
