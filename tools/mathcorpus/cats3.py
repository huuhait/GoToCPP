# Range sweeps (dense inputs where each algorithm switches branch), special-case tables, printing, and
# language-level uses of math (function values, generics, goroutines).
from lib import case
from cats1 import PRELUDE, HEAD, F
from cats2 import plain

def sweepg(gen, stmts, words, count_hint=""):
    """gen: Go statements that fill xs (a []float64)"""
    body = "func main() {\n\tvar xs []float64\n" + gen + "\n\th := uint64(14695981039346656037)\n\tfor i, x := range xs {\n"
    for s in stmts:
        body += "\t\t" + s + "\n"
    for w in words:
        body += "\t\th = mix(h, %s)\n" % w
    body += "\t\tif i%" + str(max(1, 400)) + " == 0 {\n\t\t\tfmt.Printf(\"%016x" + " %016x" * len(words) + "\\n\", math.Float64bits(x), " + ", ".join(words) + ")\n\t\t}\n\t}\n"
    body += "\tfmt.Printf(\"n=%d hash=%016x\\n\", len(xs), h)\n}\n"
    return HEAD + PRELUDE + "\n" + body

def uniform(lo, hi, n):
    return "\tfor i := 0; i < %d; i++ {\n\t\txs = append(xs, (%s)+((%s)-(%s))*unit())\n\t}\n" % (n, lo, hi, lo)

def logrange(lo_exp, hi_exp, n, signed=True):
    s = "sgn()*" if signed else ""
    return "\tfor i := 0; i < %d; i++ {\n\t\txs = append(xs, %s(1+unit())*math.Ldexp(1, %d+int(next()%%%d)))\n\t}\n" % (n, s, lo_exp, hi_exp - lo_exp + 1)

# ---- dense ranges ----
case("math_exp_dense_ranges", "Exp, Exp2, Expm1, Sinh, Cosh, Tanh over the whole range where the result is finite: the denormal tail, near overflow, near 0.",
     sweepg(uniform("-746", "710", 150000) + uniform("-1", "1", 30000) + uniform("-1e-8", "1e-8", 10000) + uniform("700", "710", 10000) + uniform("-746", "-700", 10000),
            ["a, b, c, d, e, f := math.Exp(x), math.Exp2(x), math.Expm1(x), math.Sinh(x), math.Cosh(x), math.Tanh(x)"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)", F + "(f)"]))
case("math_exp_boundaries", "Exp at the places where Go's assembly changes branch: the overflow limit, the denormal limit, +-Inf, NaN, integers, and neighbours.",
     sweepg('''	for _, c := range []float64{709.782712893384, 709.7827128933841, 709.78271289338397, 709.78271289338404, -745.1332191019411, -745.13321910194122, -745.1332191019413, -708.3964185322641, -708.39641853226420, 0, 1, -1, 0.5, 1e-300, -1e-300, 5e-324, 710, 711, -750, -1000, 1e10, -1e10, 1e300, -1e300, math.Inf(1), math.Inf(-1), math.NaN(), 88.72283905206835, -87.33654475055310} {
		xs = append(xs, c)
		for k := 0; k < 40; k++ {
			c = math.Nextafter(c, math.Inf(1))
			xs = append(xs, c)
		}
		for k := 0; k < 80; k++ {
			c = math.Nextafter(c, math.Inf(-1))
			xs = append(xs, c)
		}
	}
	for i := -800; i <= 800; i++ {
		xs = append(xs, float64(i), float64(i)+0.5, float64(i)*math.Ln2, float64(i)*math.Ln2+1e-13)
	}
''', ["a := math.Exp(x)", "b := math.Exp2(x)"], [F + "(a)", F + "(b)"]))
case("math_log_dense_ranges", "Log, Log2, Log10, Log1p: dense around 1, around sqrt(2)/2, at powers of two, huge and subnormal values (Go's amd64 Log is wrong for subnormals, and so are these).",
     sweepg(uniform("0.5", "2", 60000) + uniform("0.7071", "0.7072", 20000) + logrange(-1074, 1023, 40000, False) + uniform("-1", "1", 20000) + '''	for e := -1074; e <= 1023; e++ {
		p := math.Ldexp(1, e)
		xs = append(xs, p, math.Nextafter(p, 0), math.Nextafter(p, math.Inf(1)), p*0.7071067811865476, p*0.7071067811865475, p*1.4142135623730951)
	}
	xs = append(xs, 0.7071067811865476, 0.7071067811865475, 0.7071067811865477, 1.4142135623730951, 1.414213562373095)
''', ["a, b, c, d := math.Log(x), math.Log2(x), math.Log10(x), math.Log1p(x)"], [F + "(a)", F + "(b)", F + "(c)", F + "(d)"]))
case("math_log_subnormal_quirk", "Go on amd64 computes Log of a subnormal number wrongly (Log(5e-324) = -709.0895657128241, the true value is -744.44): gocpp has Go's bits. Also Log2, Log10 and the functions built on Log.",
     plain('''
	for _, x := range []float64{5e-324, 1e-323, 4.9e-322, 1e-310, 2.225073858507201e-308, 2.2250738585072014e-308, 2.2250738585072019e-308, 1e-300} {
		fmt.Println(x, math.Log(x), math.Log2(x), math.Log10(x), math.Log1p(x), math.Float64bits(math.Log(x)))
		fmt.Println(math.Pow(x, 0.5), math.Gamma(x))
	}
'''))
case("math_trig_large_args", "Sin, Cos, Tan, Sincos for huge arguments (Payne-Hanek reduction above 2^29) and around multiples of pi/2: bit-identical to Go.",
     sweepg(logrange(20, 1023, 60000) + uniform("-1e6", "1e6", 40000) + '''	for k := -3000; k <= 3000; k++ {
		c := float64(k) * math.Pi / 2
		xs = append(xs, c, math.Nextafter(c, 0), math.Nextafter(c, 1e300), float64(k)*1e10, float64(k)*1e15)
	}
	xs = append(xs, 1<<29, 1<<29+1, 1<<29-1, 105414357, 105414358, 1e22, 1e23, 1<<60, 1<<61, 5e-324)
''', ["s, c := math.Sincos(x)", "t := math.Tan(x)"], [F + "(s)", F + "(c)", F + "(t)"]))
case("math_atan_asin_ranges", "Atan, Asin, Acos, Asinh, Acosh, Atanh dense over their domains and near the branch points (+-1, 0.66, 2.4142, 1e-300, 1e300).",
     sweepg(uniform("-1", "1", 60000) + uniform("1", "1.0000001", 10000) + uniform("1", "1e5", 40000) + logrange(-1022, 1023, 20000) + '''	for _, c := range []float64{0.66, 0.6600000000000001, 2.414213562373095, 2.4142135623730954, 1, -1, 0.5, 1e-300, 1e300, 268435456, 268435457, 0.000000001, 1.5, 2} {
		xs = append(xs, c, math.Nextafter(c, 0), math.Nextafter(c, 1e308), -c)
	}
''', ["a, b, c := math.Atan(x), math.Asin(x), math.Acos(x)", "d, e, f := math.Asinh(x), math.Acosh(x), math.Atanh(x)"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)", F + "(f)"]))
case("math_gamma_dense", "Gamma and Lgamma on [-185, 185] densely (poles, the negative branch, overflow at 171.6, tiny arguments).",
     sweepg(uniform("-185", "185", 80000) + uniform("-4", "4", 40000) + '''	for i := -200; i <= 200; i++ {
		c := float64(i)
		xs = append(xs, c, math.Nextafter(c, 1e308), math.Nextafter(c, -1e308), c+0.5, c+1e-9)
	}
	xs = append(xs, 171.62437695630272, 171.62437695630274, 172, 1e-308, 5e-324, -1e-308, 1e308, 2.0000000000000004)
''', ["a := math.Gamma(x)", "b, s := math.Lgamma(x)"], [F + "(a)", F + "(b)", "uint64(s + 1)"]))
case("math_erf_dense", "Erf, Erfc, Erfinv, Erfcinv on dense inputs of every branch (|x| < 0.84375, < 1.25, < 1/0.35, < 28, beyond).",
     sweepg(uniform("-6", "6", 60000) + uniform("-30", "30", 30000) + uniform("-1", "1", 40000) + uniform("0", "2", 30000) + uniform("-1e-7", "1e-7", 5000) + '''	for _, c := range []float64{0.84375, 1.25, 1 / 0.35, 28, 6, 2.857142857142857, 0, 1, -1, 2, 0.5} {
		xs = append(xs, c, math.Nextafter(c, 0), math.Nextafter(c, 1e308), -c)
	}
''', ["a, b := math.Erf(x), math.Erfc(x)", "c, d := math.Erfinv(x), math.Erfcinv(x)"], [F + "(a)", F + "(b)", F + "(c)", F + "(d)"]))
case("math_bessel_dense", "J0, J1, Y0, Y1, Jn, Yn on dense inputs of every branch (x < 2, 2..8, beyond, huge) and orders -3..30.",
     sweepg(uniform("-0.5", "40", 50000) + uniform("0", "2", 20000) + uniform("2", "8", 20000) + uniform("8", "1000", 20000) + logrange(0, 60, 10000) + '''	xs = append(xs, 2, 8, 1e-10, 1e-300, 5e-324, 1<<27, 1<<28, 1<<29, 1e300)
''', ["a, b, c, d := math.J0(x), math.J1(x), math.Y0(x), math.Y1(x)", "n := int(next()%34) - 3", "e, f := math.Jn(n, x), math.Yn(n, x)"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)", F + "(f)"]))
case("math_sqrt_exact", "Sqrt of perfect squares, their neighbours, subnormals and huge numbers: correctly rounded, equal to Go's.",
     sweepg('''	for i := 0; i < 100000; i++ {
		v := float64(next() >> uint(11+next()%40))
		xs = append(xs, v*v, math.Nextafter(v*v, 0), math.Nextafter(v*v, 1e308), v*v+1)
	}
	for e := -1074; e <= 1023; e += 1 {
		xs = append(xs, math.Ldexp(1, e), math.Ldexp(3, e), math.Ldexp(1.9999999999999998, e))
	}
''', ["a := math.Sqrt(x)", "b := math.Cbrt(x)", "c := math.Hypot(x, 3)"], [F + "(a)", F + "(b)", F + "(c)"]))
case("math_rounding_halves", "Floor, Ceil, Trunc, Round, RoundToEven on halves, near-integers, and numbers around 2^52 and 2^53.",
     sweepg('''	for i := -2000; i <= 2000; i++ {
		c := float64(i) / 4
		xs = append(xs, c, math.Nextafter(c, 1e308), math.Nextafter(c, -1e308))
	}
	for _, c := range []float64{1 << 51, 1 << 52, 1 << 53, 1 << 54, 1<<52 + 0.5, 4503599627370495.5, 4503599627370494.5, 9007199254740993, 0.49999999999999994, 0.5000000000000001, 1.5, 2.5, 1e15 + 0.5, 1e16 + 1} {
		for k := -3; k <= 3; k++ {
			xs = append(xs, c+float64(k), -(c + float64(k)), c+float64(k)+0.5)
		}
	}
	xs = append(xs, uniformInts()...)
''', ["a, b, c, d, e := math.Floor(x), math.Ceil(x), math.Trunc(x), math.Round(x), math.RoundToEven(x)"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)"]).replace("func main()", '''func uniformInts() []float64 {
	var r []float64
	for i := 0; i < 20000; i++ {
		v := float64(int64(next()>>uint(1+next()%62))) * sgn()
		r = append(r, v, v+0.5, v-0.5)
	}
	return r
}

func main()''', 1))
case("math_pow_special_table", "Pow on the special cases of its documentation (zeros, infinities, NaN, negative bases, integer and half exponents, overflow, underflow): values and bits.",
     plain('''
	vals := []float64{0, math.Copysign(0, -1), 1, -1, 2, -2, 0.5, -0.5, 3, -3, 10, 1.5, 1e-5, 1e10, 1e300, 2.5, 1023, 1024, -1074, -1075, 0.1, math.Inf(1), math.Inf(-1), math.NaN(), 1 << 62, 1 << 63, 1 << 64, 9007199254740993, 1.0000000000000002, 0.9999999999999999}
	for _, x := range vals {
		for _, y := range vals {
			fmt.Printf("%v %v %v %016x\\n", x, y, math.Pow(x, y), math.Float64bits(math.Pow(x, y)))
		}
	}
'''))
case("math_pow_int_exponents", "Pow with integer and half-integer exponents -300..300 and bases of every scale: the repeated-squaring path, bit-identical to Go.",
     sweepg('''	for i := 0; i < 100000; i++ {
		xs = append(xs, sgn()*(1+unit())*math.Ldexp(1, int(next()%61)-30))
	}
''', ["n := float64(int(next()%601) - 300)", "a := math.Pow(x, n)", "b := math.Pow(math.Abs(x), n/2)", "c := math.Pow(x, float64(int(next()%7)-3))", "d := math.Pow10(int(n))"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)"]))
case("math_mod_remainder_big", "Mod and Remainder with huge ratios (x/y up to 2^2000), subnormal divisors, and exact multiples: bit-identical to Go.",
     sweepg('''	for i := 0; i < 60000; i++ {
		xs = append(xs, sgn()*(1+unit())*math.Ldexp(1, int(next()%2046)-1022))
	}
''', ["y := sgn() * (1 + unit()) * math.Ldexp(1, int(next()%2046)-1022)", "a, b := math.Mod(x, y), math.Remainder(x, y)", "c, d := math.Mod(x, 3), math.Remainder(x, 0.1)", "e, f := math.Mod(x*y, y), math.Remainder(x*y, x)"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)", F + "(f)"]))
case("math_hypot_scales", "Hypot at every scale: tiny, huge, mixed, with zeros, infinities and NaN (Inf wins over NaN).",
     sweepg('''	for i := 0; i < 80000; i++ {
		xs = append(xs, sgn()*(1+unit())*math.Ldexp(1, int(next()%2046)-1022))
	}
''', ["y := sgn() * (1 + unit()) * math.Ldexp(1, int(next()%2046)-1022)", "a, b, c := math.Hypot(x, y), math.Hypot(x, x), math.Hypot(x, 0)", "d := math.Hypot(math.Inf(1), x)", "e := math.Hypot(math.NaN(), x)"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)"]))
case("math_frexp_ldexp_subnormal", "Frexp, Ldexp, Logb, Ilogb, Modf around the subnormal range and the overflow limit: round trips and bits.",
     sweepg('''	for e := -1080; e <= 1030; e++ {
		for _, m := range []float64{1, 1.5, 1.9999999999999998, 1.0000000000000002} {
			xs = append(xs, math.Ldexp(m, e), -math.Ldexp(m, e))
		}
	}
	for i := 0; i < 40000; i++ {
		xs = append(xs, math.Float64frombits(next()>>uint(next()%64)))
	}
''', ["f, e := math.Frexp(x)", "r := math.Ldexp(f, e)", "l := math.Logb(x)", "ib := math.Ilogb(x)", "n := int(next()%400) - 200", "s := math.Ldexp(x, n)", "ip, fp := math.Modf(x)"],
            [F + "(f)", "uint64(e)", F + "(r)", F + "(l)", "uint64(ib)", F + "(s)", F + "(ip)", F + "(fp)"]))
case("math_nextafter_walks", "Nextafter and Nextafter32 walking across zero, the subnormal boundary, powers of two and the largest finite number, in both directions.",
     sweepg('''	for _, c := range []float64{0, math.Copysign(0, -1), 5e-324, 2.2250738585072014e-308, 1, -1, 2, 0.5, math.MaxFloat64, -math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN()} {
		a, b := c, c
		for k := 0; k < 300; k++ {
			a = math.Nextafter(a, math.Inf(1))
			b = math.Nextafter(b, math.Inf(-1))
			xs = append(xs, a, b)
		}
		xs = append(xs, math.Nextafter(c, c), math.Nextafter(c, math.NaN()), math.Nextafter(math.NaN(), c))
	}
''', ["f := float32(x)", "a := math.Nextafter32(f, float32(math.Inf(1)))", "b := math.Nextafter32(f, float32(math.Inf(-1)))", "c := math.Nextafter32(f, f)"],
            ["uint64(math.Float32bits(a))", "uint64(math.Float32bits(b))", "uint64(math.Float32bits(c))"]))
case("math_nan_payloads", "NaN payloads and signs through Abs, Copysign, Floor, Ceil, Trunc, Sqrt, Max, Min, Float32 conversions and arithmetic: the bits are Go's on amd64.",
     plain('''
	nans := []uint64{0x7ff0000000000001, 0x7ff8000000000000, 0xfff8000000000000, 0x7ff4479c6aacc07a, 0xfff8000000000001, 0x7fffffffffffffff, 0xffffffffffffffff, 0xfff0000000000001, 0x7ff8000000000001}
	for _, b := range nans {
		x := math.Float64frombits(b)
		fmt.Printf("%016x abs=%016x neg=%016x cs=%016x fl=%016x ce=%016x tr=%016x sq=%016x\\n", b, math.Float64bits(math.Abs(x)), math.Float64bits(-x), math.Float64bits(math.Copysign(x, -1)),
			math.Float64bits(math.Floor(x)), math.Float64bits(math.Ceil(x)), math.Float64bits(math.Trunc(x)), math.Float64bits(math.Sqrt(x)))
		fmt.Printf("  add=%016x mul=%016x max=%016x min=%016x f32=%08x rte=%016x rnd=%016x\\n", math.Float64bits(x+1), math.Float64bits(x*2), math.Float64bits(math.Max(x, 1)), math.Float64bits(math.Min(1, x)),
			math.Float32bits(float32(x)), math.Float64bits(math.RoundToEven(x)), math.Float64bits(math.Round(x)))
		fmt.Printf("  exp=%016x log=%016x sin=%016x pow=%016x mod=%016x hyp=%016x fma=%016x\\n", math.Float64bits(math.Exp(x)), math.Float64bits(math.Log(x)), math.Float64bits(math.Sin(x)),
			math.Float64bits(math.Pow(x, 2)), math.Float64bits(math.Mod(x, 2)), math.Float64bits(math.Hypot(x, 1)), math.Float64bits(math.FMA(x, 2, 1)))
	}
	fmt.Printf("%016x %016x %016x\\n", math.Float64bits(math.Sqrt(-1)), math.Float64bits(math.Inf(1)-math.Inf(1)), math.Float64bits(math.Log(-1)))
'''))

# ---- special-case tables, in the spirit of Go's tests ----
SPECIAL_VALUES = '''[]float64{0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 2, -2, 10, -10, math.Pi, math.Pi / 2, -math.Pi / 2, 1e-300, 5e-324, 1e300, math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(), 709.7, 710, -745.2, -746, 171.5, 172, 0.9999999999999999, 1.0000000000000002}'''
def table(name, desc, funcs):
    body = "\tvals := " + SPECIAL_VALUES + "\n"
    for f in funcs:
        body += '\tfmt.Println("%s")\n\tfor _, x := range vals {\n\t\tfmt.Printf("  %%v -> %%v (%%016x)\\n", x, math.%s(x), math.Float64bits(math.%s(x)))\n\t}\n' % (f, f, f)
    case(name, desc, 'package main\n\nimport (\n\t"fmt"\n\t"math"\n)\n\nfunc main() {\n' + body + "}\n")
table("math_special_trig", "Special values through Sin, Cos, Tan, Asin, Acos, Atan: results and bits, as Go prints them.", ["Sin", "Cos", "Tan", "Asin", "Acos", "Atan"])
table("math_special_hyperbolic", "Special values through Sinh, Cosh, Tanh, Asinh, Acosh, Atanh.", ["Sinh", "Cosh", "Tanh", "Asinh", "Acosh", "Atanh"])
table("math_special_explog", "Special values through Exp, Exp2, Expm1, Log, Log2, Log10, Log1p, Logb.", ["Exp", "Exp2", "Expm1", "Log", "Log2", "Log10", "Log1p", "Logb"])
table("math_special_root", "Special values through Sqrt and Cbrt.", ["Sqrt", "Cbrt"])
table("math_special_rounding", "Special values through Floor, Ceil, Trunc, Round, RoundToEven, Abs.", ["Floor", "Ceil", "Trunc", "Round", "RoundToEven", "Abs"])
table("math_special_gamma", "Special values through Gamma, Erf, Erfc, Erfinv, Erfcinv, J0, J1, Y0, Y1.", ["Gamma", "Erf", "Erfc", "Erfinv", "Erfcinv", "J0", "J1", "Y0", "Y1"])
case("math_special_two_args", "Special values through Atan2, Hypot, Mod, Remainder, Dim, Max, Min, Copysign, Nextafter (all pairs of 14 values).",
     'package main\n\nimport (\n\t"fmt"\n\t"math"\n)\n\nfunc main() {\n\tvals := []float64{0, math.Copysign(0, -1), 1, -1, 2.5, -2.5, 1e300, 5e-324, math.Inf(1), math.Inf(-1), math.NaN(), 3, 0.5, 1 << 60}\n' +
     ''.join('\tfmt.Println("%s")\n\tfor _, x := range vals {\n\t\tfor _, y := range vals {\n\t\t\tfmt.Printf("  %%v %%v -> %%v (%%016x)\\n", x, y, math.%s(x, y), math.Float64bits(math.%s(x, y)))\n\t\t}\n\t}\n' % (f, f, f) for f in ["Atan2", "Hypot", "Mod", "Remainder", "Dim", "Max", "Min", "Copysign", "Nextafter"]) + "}\n")
case("math_special_classify", "Inf, NaN, IsInf, IsNaN, Signbit, Copysign, Frexp, Modf, Ilogb, Logb on special values.",
     'package main\n\nimport (\n\t"fmt"\n\t"math"\n)\n\nfunc main() {\n\tvals := ' + SPECIAL_VALUES + '''
	for _, x := range vals {
		f, e := math.Frexp(x)
		ip, fp := math.Modf(x)
		fmt.Println(x, math.IsNaN(x), math.IsInf(x, 0), math.IsInf(x, 1), math.IsInf(x, -1), math.Signbit(x), f, e, ip, fp, math.Ilogb(x), math.Logb(x))
	}
}
''')

# ---- printing through fmt ----
case("math_print_verbs_sweep", "Results of math functions printed with %v %g %e %f %G %E %.3f %10.4f %-12.2e %+g: hash of the text and samples, identical to Go.",
     HEAD.replace('"math"\n', '"math"\n') + PRELUDE + '''
func main() {
	xs := inputs()
	h := uint64(14695981039346656037)
	verbs := []string{"%v", "%g", "%e", "%f", "%G", "%E", "%.3f", "%10.4f", "%-12.2e", "%+g", "%.0f", "%8.3g", "%+.5e", "% f"}
	for i, x := range xs {
		y := math.Sin(x) * math.Exp(float64(i%40-20))
		v := verbs[i%len(verbs)]
		s := fmt.Sprintf(v, y)
		t := fmt.Sprint(y, float32(y), x)
		for j := 0; j < len(s); j++ {
			h = mix(h, uint64(s[j]))
		}
		for j := 0; j < len(t); j++ {
			h = mix(h, uint64(t[j]))
		}
		if i%1000 == 0 {
			fmt.Println(v, s, t)
		}
	}
	fmt.Printf("%016x\\n", h)
}
''')
case("math_print_special_floats", "Inf, NaN and -0 with every float verb, width, precision and flag: the text is Go's.",
     plain('''
	vals := []float64{math.Inf(1), math.Inf(-1), math.NaN(), 0, math.Copysign(0, -1), 1e21, 1e20, 1e-5, 1e-4, 123456789, 0.000001234, math.MaxFloat64, math.SmallestNonzeroFloat64}
	for _, v := range vals {
		fmt.Printf("%v|%g|%e|%f|%G|%E|%F|%8.2f|%-8.2f|%+.1f|% .1f|%08.2f|%.0f|%10v|%-10v|%x|%X|%b\\n", v, v, v, v, v, v, v, v, v, v, v, v, v, v, v, v, v, v)
		f := float32(v)
		fmt.Printf("%v|%g|%e|%f|%6.2f\\n", f, f, f, f, f)
	}
	fmt.Println(vals, []float32{1.5, float32(math.Inf(1)), float32(math.NaN())})
	fmt.Printf("%v %.3v %10.2v %6.0v\\n", math.Pi, math.Pi, math.Pi, math.Pi)
'''))
case("math_print_shortest", "Println of floats: the shortest representation that round-trips, for random bit patterns, float32 and float64, as Go prints it.",
     HEAD + PRELUDE + '''
func main() {
	h := uint64(14695981039346656037)
	for i := 0; i < 60000; i++ {
		var x float64
		switch i % 4 {
		case 0:
			x = math.Float64frombits(next())
		case 1:
			x = float64(int64(next()>>20)) / 1000
		case 2:
			x = math.Sqrt(float64(i))
		default:
			x = math.Ldexp(unit(), int(next()%200)-100)
		}
		s := fmt.Sprint(x) + fmt.Sprint(float32(x)) + fmt.Sprintf("%v%g", -x, x*3)
		for j := 0; j < len(s); j++ {
			h = mix(h, uint64(s[j]))
		}
		if i%3000 == 0 {
			fmt.Println(x, float32(x), -x)
		}
	}
	fmt.Printf("%016x\\n", h)
}
''')
case("math_print_function_results", "Println of the results of Sin, Cos, Exp, Log, Sqrt, Pow, Atan2, Gamma for 20 inputs: shortest round-trip text identical to Go.",
     plain('''
	for _, x := range []float64{0.1, 0.5, 1, 2, 3, 10, 100, 1e-3, 123.456, 7.5, 0.7, 5, 20, 0.3333333333333333, 1e5, 1e-10, 2.5, 4.25, 9, 15} {
		fmt.Println(x, math.Sin(x), math.Cos(x), math.Tan(x), math.Exp(x), math.Log(x), math.Sqrt(x), math.Pow(x, 1.5), math.Atan2(x, 3), math.Gamma(x), math.Cbrt(x), math.Sinh(x))
	}
'''))

# ---- language-level uses of math ----
case("math_function_values", "math functions as function values: assigned to variables, stored in slices and maps, passed as arguments, returned from functions.",
     plain('''
	f := math.Sqrt
	g := math.Sin
	fs := []func(float64) float64{math.Sqrt, math.Floor, math.Exp, math.Log, math.Abs, math.Cbrt, math.Trunc, math.Ceil, math.RoundToEven, math.Sin}
	for _, fn := range fs {
		fmt.Printf("%016x %016x\\n", math.Float64bits(fn(2.5)), math.Float64bits(fn(-0.5)))
	}
	m := map[string]func(float64, float64) float64{"pow": math.Pow, "hypot": math.Hypot, "max": math.Max, "mod": math.Mod}
	for _, k := range []string{"hypot", "max", "mod", "pow"} {
		fmt.Println(k, m[k](7, 3))
	}
	apply := func(fn func(float64) float64, v float64) float64 { return fn(v) }
	fmt.Println(f(16), g(0), apply(math.Floor, -2.5), apply(f, 2))
	bits := math.Float64bits
	fb := math.Float64frombits
	fmt.Println(bits(1.5), fb(0x4000000000000000))
	var fma = math.FMA
	fmt.Println(fma(2, 3, 4))
'''))
case("math_generic_functions", "math in generic functions (Phase 07): a type-parameter constraint over float32 and float64 calling math through conversions.",
     plain('''
	fmt.Println(norm(3.0, 4.0), norm(float32(1.5), float32(2.5)), sumSqrt([]float64{1, 4, 9}), sumSqrt([]float32{2, 8}))
	fmt.Println(clamp(5.5, 0.0, 3.0), clamp(float32(-1), 0, 1), roundTo(3.14159, 2), roundTo(float32(2.71828), 3))
''', '''
type Float interface{ ~float32 | ~float64 }

func norm[T Float](x, y T) T { return T(math.Sqrt(float64(x*x + y*y))) }

func sumSqrt[T Float](xs []T) T {
	var s T
	for _, x := range xs {
		s += T(math.Sqrt(float64(x)))
	}
	return s
}

func clamp[T Float](x, lo, hi T) T { return min(max(x, lo), hi) }

func roundTo[T Float](x T, digits int) T {
	p := math.Pow10(digits)
	return T(math.Round(float64(x)*p) / p)
}
'''))
case("math_goroutines", "math from several goroutines at once (the first call of Exp reads the CPU features once): results identical to a sequential run.",
     HEAD + PRELUDE + '''
func work(seed int, out chan<- uint64) {
	h := uint64(14695981039346656037)
	for i := 0; i < 20000; i++ {
		x := float64(seed*20000+i) * 0.001
		h = mix(h, math.Float64bits(math.Exp(x/100)+math.Log(x+1)+math.Sin(x)+math.Sqrt(x)+math.Pow(x, 0.3)))
	}
	out <- h
}

func main() {
	out := make(chan uint64, 8)
	for g := 0; g < 8; g++ {
		go work(g, out)
	}
	var res [8]uint64
	for g := 0; g < 8; g++ {
		res[g] = <-out
	}
	// sort the results: arrival order is not fixed
	for i := range res {
		for j := i + 1; j < len(res); j++ {
			if res[j] < res[i] {
				res[i], res[j] = res[j], res[i]
			}
		}
	}
	for _, r := range res {
		fmt.Printf("%016x\\n", r)
	}
}
''')
case("math_import_forms", "Import forms: an alias, local names that look like math functions (exp, log, sqrt, floor), and package-level variables initialized from math.",
     'package main\n\nimport (\n\t"fmt"\n\tm "math"\n)\n\nvar (\n\ttau = 2 * m.Pi\n\troot2 = m.Sqrt(2)\n\ttable = [...]float64{m.Sin(1), m.Cos(1), m.Exp(1), m.Log(2)}\n\tbig = m.Pow(2, 100)\n)\n\n' + '''
func exp(x float64) float64 { return m.Exp(x) * 2 }

func main() {
	log := m.Log
	sqrt := func(x float64) float64 { return m.Sqrt(x) + 1 }
	floor := 3.0
	abs := -2.5
	fmt.Println(tau, root2, table, big, exp(1), log(10), sqrt(4), floor, m.Abs(abs))
	fmt.Printf("%016x %016x %016x\\n", m.Float64bits(tau), m.Float64bits(root2), m.Float64bits(big))
	const pi2 = m.Pi / 2
	var x float32 = m.Pi
	fmt.Println(pi2, x, m.MaxInt32, m.MaxUint32, uint32(m.MaxUint32))
}
''')
case("math_integer_overflow_constants", "Integer limits from math in arithmetic: wrapping at MaxInt64 and MaxUint64, conversions of MinInt64, shifts, and constant expressions with them.",
     plain('''
	var a int64 = math.MaxInt64
	var b int64 = math.MinInt64
	var c uint64 = math.MaxUint64
	var d int32 = math.MaxInt32
	var e int8 = math.MinInt8
	fmt.Println(a+1, b-1, -b, c+1, c*2, d+1, e-1, -e, a*2, b/-1, uint64(b), int64(c), uint8(c), int8(a))
	fmt.Println(math.MaxInt64/2+1, uint64(math.MaxUint64)>>1 == math.MaxInt64, math.MinInt64 == -math.MaxInt64-1, float64(math.MaxInt64), float64(math.MinInt64), float64(uint64(math.MaxUint64)))
	fv := 0.5
	u32 := uint32(math.MaxUint32)
	fmt.Println(float32(math.MaxInt32), int64(float64(math.MaxInt32)+fv), u32+1)
'''))
