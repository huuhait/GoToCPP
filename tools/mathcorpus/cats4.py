# More sweeps and workloads: float32 wrappers, complex arithmetic, identities, performance workloads (deterministic
# checksums), NaN payloads through storage.
from lib import case
from cats1 import PRELUDE, HEAD, F
from cats2 import plain, prog
from cats3 import sweepg, uniform, logrange

case("math_float32_functions", "float32(math.F(float64(x))) for the usual functions on float32 inputs (every float32 magnitude): bit-identical to Go.",
     sweepg('''	for i := 0; i < 60000; i++ {
		xs = append(xs, float64(math.Float32frombits(uint32(next()))))
	}
	for i := 0; i < 60000; i++ {
		xs = append(xs, float64(float32(sgn()*unit()*20)))
	}
''', ["f := float32(x)", "a := float32(math.Sin(float64(f)))", "b := float32(math.Exp(float64(f)))", "c := float32(math.Log(float64(f)))", "d := float32(math.Sqrt(float64(f)))", "e := float32(math.Pow(float64(f), 1.5))", "g := float32(math.Atan2(float64(f), 2))"],
            ["cw32(a)", "cw32(b)", "cw32(c)", "cw32(d)", "cw32(e)", "cw32(g)"]).replace("func main()", '''func cw32(x float32) uint64 {
	if x != x {
		return 0x7fc00001
	}
	return uint64(math.Float32bits(x))
}

func main()''', 1))
case("math_complex_arith", "complex128 and complex64 arithmetic with math: products and quotients (Go's rules for infinities and NaN, no fusion), real, imag, bits.",
     prog('''
	xs := inputs()
	h := uint64(14695981039346656037)
	for i := 0; i+3 < len(xs); i += 4 {
		a := complex(xs[i], xs[i+1])
		b := complex(xs[i+2], xs[i+3])
		for _, z := range []complex128{a * b, a / b, a + b, a - b, -a} {
			re, im := real(z), imag(z)
			if re != re {
				re = 0
			}
			if im != im {
				im = 0
			}
			h = mix(mix(h, math.Float64bits(re)), math.Float64bits(im))
			h = mix(h, b2w(z != z))
		}
		c := complex64(a)
		d := complex64(b)
		for _, z := range []complex64{c * d, c / d} {
			re, im := real(z), imag(z)
			if re != re || im != im {
				h = mix(h, 1)
				continue
			}
			h = mix(mix(h, uint64(math.Float32bits(re))), uint64(math.Float32bits(im)))
		}
	}
	fmt.Printf("%016x\\n", h)
	z := complex(1+0x1p-27, 1-0x1p-29) * complex(1-0x1p-27, 1+0x1p-31)
	fmt.Println(z, math.Hypot(real(z), imag(z)), math.Atan2(imag(z), real(z)))
	fmt.Println((1+2i)*(3-4i), (1+2i)/(3-4i), complex(math.Inf(1), 0)*2, complex(math.Inf(1), 1)/complex(math.Inf(1), 1))
'''))
case("math_trig_identities", "Identities in floating point (sin^2+cos^2, exp(log x), sqrt(x)^2, atan(tan x), asin(sin x), pow(x, 1/3) vs cbrt): the exact differences printed, as bits.",
     prog('''
	h := uint64(14695981039346656037)
	for i := 0; i < 50000; i++ {
		x := sgn() * unit() * 10
		s, c := math.Sincos(x)
		v := []float64{s*s + c*c, math.Exp(math.Log(math.Abs(x))) - math.Abs(x), math.Sqrt(math.Abs(x))*math.Sqrt(math.Abs(x)) - math.Abs(x), math.Atan(math.Tan(x)) - x,
			math.Asin(math.Sin(x/4)) - x/4, math.Pow(math.Abs(x), 1.0/3) - math.Cbrt(math.Abs(x)), math.Sinh(x)*math.Sinh(x) - math.Cosh(x)*math.Cosh(x) + 1, math.Tanh(x) - math.Sinh(x)/math.Cosh(x),
			math.Log10(math.Pow(10, x)) - x, math.Expm1(x) - (math.Exp(x) - 1), math.Gamma(math.Abs(x)+1) - math.Abs(x)*math.Gamma(math.Abs(x)), math.Erf(x) + math.Erfc(x) - 1}
		for _, e := range v {
			h = mix(h, math.Float64bits(e))
		}
		if i%5000 == 0 {
			fmt.Printf("%016x %016x %016x %016x\\n", math.Float64bits(v[0]), math.Float64bits(v[1]), math.Float64bits(v[3]), math.Float64bits(v[8]))
		}
	}
	fmt.Printf("%016x\\n", h)
'''))
case("math_gamma_integers", "Gamma(n) for n = 1..172 (factorials, exact up to 22!, the overflow at 172), Lgamma, and half integers: values and bits.",
     plain('''
	for n := 1; n <= 172; n++ {
		g := math.Gamma(float64(n))
		l, s := math.Lgamma(float64(n))
		fmt.Printf("%d %v %016x %v %d\\n", n, g, math.Float64bits(g), l, s)
	}
	for _, x := range []float64{0.5, 1.5, 2.5, -0.5, -1.5, -2.5, 10.5, 100.5, 170.5, -170.5, -171.5, -180.5} {
		l, s := math.Lgamma(x)
		fmt.Println(x, math.Gamma(x), l, s)
	}
'''))
case("math_atan2_quadrants", "Atan2 dense around the axes, the diagonals and the quadrant boundaries, with signed zeros and infinities: bit-identical to Go.",
     sweepg('''	for i := 0; i < 80000; i++ {
		xs = append(xs, sgn()*(1+unit())*math.Ldexp(1, int(next()%80)-40))
	}
''', ["y := sgn() * (1 + unit()) * math.Ldexp(1, int(next()%80)-40)", "a, b, c := math.Atan2(y, x), math.Atan2(y, -x), math.Atan2(x, x)", "d, e := math.Atan2(0, x), math.Atan2(math.Copysign(0, -1), x)", "f := math.Atan2(y, math.Inf(int(next()%2)*2-1))"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)", F + "(f)"]))
case("math_expm1_log1p_tiny", "Expm1, Log1p, Sinh, Tanh, Asinh, Atanh, Cbrt for tiny and huge arguments (the branches below 2^-28 and above 2^28 and 2^54).",
     sweepg(logrange(-1074, -20, 40000) + logrange(20, 1023, 40000) + uniform("-1", "1", 20000),
            ["a, b, c := math.Expm1(x), math.Log1p(x), math.Sinh(x)", "d, e, f, g := math.Tanh(x), math.Asinh(x), math.Atanh(x), math.Cbrt(x)"],
            [F + "(a)", F + "(b)", F + "(c)", F + "(d)", F + "(e)", F + "(f)", F + "(g)"]))
case("math_perf_sqrt_floor_abs", "A workload of 30 million Sqrt, Floor, Ceil, Trunc, Abs, FMA calls (the natives must inline in an optimized build): deterministic checksum equal to Go's.",
     plain('''
	var s, f float64
	x := 1.0
	for i := 0; i < 30000000; i++ {
		x = x*1.0000001 + 0.001
		s += math.Sqrt(x) + math.Abs(-x)
		f += math.Floor(x*0.37) - math.Ceil(x*0.11) + math.Trunc(-x*0.07) + math.FMA(x, 0.5, 1)
		if x > 1e6 {
			x = 1
		}
	}
	fmt.Printf("%016x %016x\\n", math.Float64bits(s), math.Float64bits(f))
'''))
case("math_perf_exp_log_sin", "A workload of 3 million Exp, Log, Sin, Cos, Pow, Atan calls: deterministic checksum equal to Go's.",
     plain('''
	var s float64
	x := 0.001
	for i := 0; i < 3000000; i++ {
		x += 0.00137
		if x > 700 {
			x = 0.001
		}
		s += math.Exp(x/10) + math.Log(x) + math.Sin(x) + math.Cos(x*3) + math.Pow(x, 0.25) + math.Atan(x)
	}
	fmt.Printf("%016x\\n", math.Float64bits(s))
'''))
case("math_nan_storage", "NaN payloads (signaling and quiet) survive being stored in variables, structs, arrays, slices, maps, interfaces, channels, closures and function arguments.",
     plain('''
	type box struct {
		a float64
		b float32
	}
	pats := []uint64{0x7ff0000000000001, 0x7ff8000000000000, 0xfff8000000000001, 0x7ff4479c6aacc07a, 0xffffffffffffffff}
	ch := make(chan float64, 1)
	for _, p := range pats {
		x := math.Float64frombits(p)
		bx := box{x, math.Float32frombits(uint32(p>>32) | 1)}
		arr := [2]float64{x, 1}
		sl := []float64{1, x}
		m := map[int]float64{1: x}
		var any1 any = x
		ch <- x
		fn := func(v float64) float64 { return v }
		cl := func() float64 { return x }
		r := <-ch
		fmt.Printf("%016x %016x %08x %016x %016x %016x %016x %016x %016x %016x\\n", p, math.Float64bits(bx.a), math.Float32bits(bx.b), math.Float64bits(arr[0]), math.Float64bits(sl[1]),
			math.Float64bits(m[1]), math.Float64bits(any1.(float64)), math.Float64bits(r), math.Float64bits(fn(x)), math.Float64bits(cl()))
	}
	for _, b := range []uint32{0x7f800001, 0x7fc00000, 0xffc00001, 0x7fa00000} {
		f := math.Float32frombits(b)
		var a any = f
		s := []float32{f}
		fmt.Printf("%08x %08x %08x\\n", b, math.Float32bits(a.(float32)), math.Float32bits(s[0]))
	}
'''))
