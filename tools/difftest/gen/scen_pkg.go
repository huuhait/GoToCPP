package gen

import (
	"fmt"
	"strconv"
	"strings"
)

// Scenarios that use the standard packages that gocpp ports from Go's source (stdlib/PORTED.txt:
// strings, strconv, unicode, unicode/utf8, bytes, math, math/bits, errors; and sync, time). The
// data is random, the calls are the same in every run: no clock, no zone database, no scheduling
// dependence, only the functions whose results the packages' documentation fixes (math: no
// transcendental functions, whose last bit differs between assembly and Go code).

func init() {
	register("pkgstrings", 9, []string{"pkgstrings", "strings"}, scenPkgStrings)
	register("pkgstrconv", 9, []string{"pkgstrconv", "strings"}, scenPkgStrconv)
	register("pkgunicode", 8, []string{"pkgunicode"}, scenPkgUnicode)
	register("pkgbytes", 7, []string{"pkgbytes", "strings"}, scenPkgBytes)
	register("pkgmath", 8, []string{"pkgmath", "floats"}, scenPkgMath)
	register("pkgerrors", 7, []string{"pkgerrors", "errors", "ifaces"}, scenPkgErrors)
	register("pkgsync", 6, []string{"pkgsync", "goroutines"}, scenPkgSync)
	register("pkgtime", 8, []string{"pkgtime"}, scenPkgTime)
}

func (g *Gen) pick(list ...string) string { return list[g.rn(len(list))] }

func scenPkgStrings(g *Gen) {
	g.importPkg("strings")
	for i := g.between(2, 4); i > 0; i-- {
		g.tpl(`{
	s, t, u := @S@, @T@, @U@
	fmt.Printf("%q %q %v %v %v %v\n", strings.ToUpper(s), strings.ToLower(s), strings.Contains(s, t), strings.HasPrefix(s, t), strings.HasSuffix(s, t), strings.EqualFold(s, t))
	fmt.Println(strings.Index(s, t), strings.LastIndex(s, t), strings.Count(s, t), strings.IndexByte(s, 'a'), strings.IndexRune(s, 'é'), strings.IndexAny(s, t), strings.Compare(s, t))
	fmt.Printf("%q %q %q\n", strings.Split(s, t), strings.SplitN(s+t+s, t, @N@), strings.SplitAfter(s, t))
	fmt.Printf("%q %q %q %q\n", strings.Fields(s+" "+t), strings.TrimSpace(s), strings.Trim(s, t), strings.TrimLeft(s, t))
	fmt.Printf("%q %q %q\n", strings.TrimRight(s, t), strings.TrimPrefix(s, t), strings.TrimSuffix(s, t))
	fmt.Printf("%q %q %q\n", strings.Replace(s, t, u, @N@), strings.ReplaceAll(s, t, u), strings.Repeat(s, @R@))
	fmt.Printf("%q %q\n", strings.ToValidUTF8(s, u), strings.Join([]string{s, t, u}, @S@))
	before, after, found := strings.Cut(s, t)
	fmt.Printf("%q %q %v\n", before, after, found)
	fmt.Printf("%q %q\n", strings.Map(func(r rune) rune {
		if r == 'a' {
			return -1
		}
		return r + 1
	}, s), strings.ToTitle(s))
	var b strings.Builder
	for i, r := range s {
		b.WriteRune(r)
		b.WriteByte(byte('0' + i%10))
	}
	fmt.Printf("%q %d\n", b.String(), b.Len())
	fmt.Println(strings.NewReplacer("a", "1", "é", "ee", "b", "2").Replace(s+t))
	fmt.Printf("%q %q\n", strings.FieldsFunc(s+" "+t, func(r rune) bool { return r == ' ' || r == 'o' }), strings.TrimFunc(s, func(r rune) bool { return r < 'a' }))
}`, "S", g.strRaw(), "T", g.strRaw(), "U", g.strRaw(), "N", strconv.Itoa(g.between(-1, 3)), "R", strconv.Itoa(g.between(0, 3)))
	}
	g.stmts(g.between(0, 3), 1)
	g.showAll()
}

func scenPkgStrconv(g *Gen) {
	g.importPkg("strconv")
	nums := []string{"0", "-0", "12", "+7", "-129", "255", "256", "9223372036854775807", "9223372036854775808",
		"-9223372036854775809", "0x1f", "0b101", "0o17", "1_000", "  1", "1 ", "", "abc", "1e3", "٣", "-", "+", "00012",
		"18446744073709551615", "18446744073709551616", "-1", "0x", "0xFFFFFFFF", "0b102", "٠"}
	floats := []string{"1.5", "0.1", "1e3", "-0", "inf", "-Inf", "nan", "1e400", "1e-400", "0x1p-2", "1_0.5", "1.5e", "", ".5", "5.",
		"1e+", "0.000001", "123456789012345678901234567890", "4.9e-324", "1.7976931348623157e308", "1.7976931348623159e308",
		"3.4028235e38", "3.4028236e38", "0x1.8p3", "+1.25", "1e", "- 1", "Infinity", "+inf"}
	for i := g.between(2, 4); i > 0; i-- {
		base := g.pick("0", "2", "8", "10", "16", "36", "10", "10", "1", "37")
		bits := g.pick("0", "8", "16", "32", "64", "64", "65", "-1")
		g.tpl(`{
	s := @S@
	n, err := strconv.Atoi(@NUM@)
	fmt.Println(n, err)
	i64, err := strconv.ParseInt(@NUM@, @BASE@, @BITS@)
	fmt.Println(i64, err)
	u64, err := strconv.ParseUint(@NUM@, @BASE@, @BITS@)
	fmt.Println(u64, err)
	f, err := strconv.ParseFloat(@FSTR@, @FBITS@)
	fmt.Println(f, err, strconv.FormatFloat(f, 'g', -1, 64))
	bv, err := strconv.ParseBool(@BOOL@)
	fmt.Println(bv, err)
	fmt.Println(strconv.Quote(s), strconv.QuoteToASCII(s), strconv.QuoteRune('é'), strconv.QuoteToGraphic(s), strconv.QuoteRuneToASCII(0x1F600))
	us, err := strconv.Unquote(strconv.Quote(s))
	fmt.Printf("%q %v %v\n", us, us == s, err)
	_, err = strconv.Unquote(@UNQ@)
	fmt.Println(err)
	fmt.Println(strconv.FormatInt(@I@, @FB@), strconv.FormatUint(@UU@, @FB@), strconv.Itoa(@I@))
	fmt.Println(strconv.FormatFloat(@FL@, 'e', @PREC@, 64), strconv.FormatFloat(@FL@, 'f', @PREC@, 32), strconv.FormatFloat(@FL@, 'g', -1, 64), strconv.FormatFloat(@FL@, 'G', 3, 64), strconv.FormatFloat(@FL@, 'b', -1, 64), strconv.FormatFloat(@FL@, 'x', -1, 64))
	buf := strconv.AppendInt(nil, @I@, 10)
	buf = strconv.AppendQuote(buf, s)
	buf = strconv.AppendBool(buf, true)
	buf = strconv.AppendFloat(buf, @FL@, 'f', -1, 64)
	fmt.Printf("%s\n", buf)
}`, "S", g.strRaw(), "NUM", strconv.Quote(g.pick(nums...)), "BASE", base, "BITS", bits,
			"FSTR", strconv.Quote(g.pick(floats...)), "FBITS", g.pick("32", "64", "64"),
			"BOOL", strconv.Quote(g.pick("1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False", "tRue", "yes", "")),
			"UNQ", strconv.Quote(g.pick(`"a"`, `'a'`, "`a`", `"a`, `a`, `"\x"`, `"é"`, `'ab'`, `"\400"`, ``, `"`, `'\''`, `"\"`)),
			"I", g.intRaw(intTs["int64"]), "UU", g.intRaw(intTs["uint64"]), "FB", g.pick("2", "8", "10", "16", "36", "10"),
			"FL", g.pick(g.floatRaw(), g.floatRaw(), "0.1", "3.141592653589793", "1e21", "1e-7", "123456789.125", "5e-324", "1.7976931348623157e308", "100", "2.5", "0.000123"),
			"PREC", g.pick("-1", "0", "1", "3", "10", "20"))
	}
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenPkgUnicode(g *Gen) {
	g.importPkg("unicode")
	g.importPkg("unicode/utf8")
	runes := []string{"'a'", "'Z'", "'é'", "'ß'", "'日'", "'😀'", "'١'", "'\\u00a0'", "'\\u2028'", "'ǅ'", "'Σ'", "'ς'", "'\\t'", "'\\x00'", "'_'",
		"'$'", "'€'", "'\\u0301'", "'ⅷ'", "'½'", "'İ'", "'ı'", "'ǆ'", "'K'", "-1", "0xD800", "0x10FFFF", "0x110000", "' '", "'\\u200b'", "'Ⓐ'"}
	var picked []string
	for i := g.between(8, 14); i > 0; i-- {
		picked = append(picked, g.pick(runes...))
	}
	g.tpl(`for _, r := range []rune{@RUNES@} {
	fmt.Println(string(r), r, unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsNumber(r), unicode.IsSpace(r), unicode.IsUpper(r), unicode.IsLower(r), unicode.IsTitle(r), unicode.IsPunct(r), unicode.IsSymbol(r), unicode.IsControl(r), unicode.IsPrint(r), unicode.IsGraphic(r), unicode.IsMark(r))
	fmt.Println(unicode.ToUpper(r), unicode.ToLower(r), unicode.ToTitle(r), unicode.SimpleFold(r), unicode.Is(unicode.Han, r), unicode.In(r, unicode.Latin, unicode.Greek), utf8.RuneLen(r), utf8.ValidRune(r))
	var buf [4]byte
	n := utf8.EncodeRune(buf[:], r)
	rr, sz := utf8.DecodeRune(buf[:n])
	fmt.Println(buf[:n], n, rr, sz, utf8.AppendRune(nil, r))
}`, "RUNES", strings.Join(picked, ", "))
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenPkgBytes(g *Gen) {
	g.importPkg("bytes")
	g.importPkg("unicode/utf8")
	for i := g.between(1, 3); i > 0; i-- {
		g.tpl(`{
	s := @S@
	b1, b2 := []byte(s), []byte(@T@)
	fmt.Println(bytes.Equal(b1, b2), bytes.Compare(b1, b2), bytes.Contains(b1, b2), bytes.Index(b1, b2), bytes.Count(b1, b2), bytes.HasPrefix(b1, b2), bytes.LastIndex(b1, b2))
	fmt.Printf("%q %q %q %q\n", bytes.ToUpper(b1), bytes.TrimSpace(b1), bytes.Fields(b1), bytes.Split(b1, b2))
	fmt.Printf("%q %q %q\n", bytes.Repeat(b1, @N@), bytes.ReplaceAll(b1, b2, []byte("-")), bytes.Title(b1))
	fmt.Println(utf8.RuneCount(b1), utf8.Valid(b1), bytes.Runes(b1), bytes.EqualFold(b1, b2), bytes.IndexByte(b1, 'a'), bytes.ContainsRune(b1, 'é'))
	var buf bytes.Buffer
	buf.WriteString(s)
	buf.WriteByte('!')
	buf.WriteRune('é')
	fmt.Fprintf(&buf, "%d", 42)
	fmt.Printf("%q %d %s\n", buf.String(), buf.Len(), buf.Bytes()[:1])
	c, err := buf.ReadByte()
	fmt.Println(c, err)
	fmt.Printf("%q\n", buf.Next(@N@))
	buf.Truncate(buf.Len() / 2)
	fmt.Printf("%q %d\n", buf.Bytes(), buf.Len())
	rr, sz, err := buf.ReadRune()
	fmt.Println(rr, sz, err)
	line, err := bytes.NewBufferString(s + "\n" + s).ReadString('\n')
	fmt.Printf("%q %v\n", line, err)
	buf.Reset()
	fmt.Println(buf.Len(), buf.String() == "")
}`, "S", g.strRaw(), "T", g.strRaw(), "N", strconv.Itoa(g.between(0, 3)))
	}
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenPkgMath(g *Gen) {
	g.importPkg("math")
	g.importPkg("math/bits")
	fl := func() string {
		return g.pick(g.floatRaw(), g.floatRaw(), g.floatRaw(), "0.5", "-0.5", "1.5", "2.5", "-2.5", "1e300", "-1e300", "4503599627370496.5", "0", "3")
	}
	g.tpl(`{
	var zero float64
	inf, nan, negzero := 1/zero, zero/zero, -zero
	for _, x := range []float64{@X1@, @X2@, @X3@, inf, -inf, nan, negzero} {
		y := float64(@Y@)
		fmt.Println(x, y, math.Floor(x), math.Ceil(x), math.Trunc(x), math.Round(x), math.RoundToEven(x), math.Abs(x), math.Sqrt(x))
		fmt.Println(math.Mod(x, y), math.Remainder(x, y), math.Max(x, y), math.Min(x, y), math.Copysign(x, y), math.Signbit(x), math.IsInf(x, 0), math.IsNaN(x), math.Dim(x, y))
		fr, ex := math.Frexp(x)
		ip, fp := math.Modf(x)
		fmt.Println(fr, ex, ip, fp, math.Ldexp(x, @E@), math.Float64bits(x), math.Float32bits(float32(x)), math.Nextafter(x, y), math.FMA(x, y, 1))
		fmt.Println(math.Float64frombits(math.Float64bits(x)+1), math.Cbrt(8), math.Pow(2, @E@), math.Pow10(@E@))
	}
	fmt.Println(math.MaxInt8, math.MinInt64, uint64(math.MaxUint64), math.MaxFloat32, math.SmallestNonzeroFloat64, math.Pi, math.MaxInt32+1)
}`, "X1", fl(), "X2", fl(), "X3", fl(), "Y", fl(), "E", strconv.Itoa(g.between(-5, 12)))
	t := g.pick("uint8", "uint16", "uint32", "uint64", "uint")
	g.tpl(`{
	var x, y @T@ = @A@, @B@
	fmt.Println(bits.OnesCount(uint(x)), bits.LeadingZeros64(uint64(x)), bits.TrailingZeros32(uint32(x)), bits.Len(uint(x)), bits.Len8(uint8(x)), bits.Reverse16(uint16(x)), bits.ReverseBytes32(uint32(x)))
	fmt.Println(bits.RotateLeft8(uint8(x), @R@), bits.RotateLeft32(uint32(x), @R@), bits.RotateLeft64(uint64(x), @R@), bits.OnesCount8(uint8(y)), bits.LeadingZeros16(uint16(y)))
	s, c := bits.Add64(uint64(x), uint64(y), 1)
	d, bo := bits.Sub64(uint64(x), uint64(y), 1)
	hi, lo := bits.Mul64(uint64(x), uint64(y))
	q, r := bits.Div64(0, uint64(x), uint64(y)|1)
	fmt.Println(s, c, d, bo, hi, lo, q, r)
	s32, c32 := bits.Add32(uint32(x), uint32(y), 0)
	hi32, lo32 := bits.Mul32(uint32(x), uint32(y))
	fmt.Println(s32, c32, hi32, lo32, bits.UintSize)
}`, "T", t, "A", g.intRaw(intTs[t]), "B", g.intRaw(intTs[t]), "R", strconv.Itoa(g.between(-70, 70)))
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenPkgErrors(g *Gen) {
	g.importPkg("errors")
	pe := g.panicErrType()
	pt := g.topName("PtrErr")
	g.topf("type %s struct{ code int }\nfunc (e *%s) Error() string { return fmt.Sprint(\"ptrerr \", e.code) }\nfunc (e *%s) Is(target error) bool { t, ok := target.(*%s); return ok && e != nil && t != nil && t.code/10 == e.code/10 }\n", pt, pt, pt, pt)
	wt := g.topName("WrapErr")
	g.topf("type %s struct{ msg string; err error }\nfunc (e %s) Error() string { return e.msg + \": \" + fmt.Sprint(e.err) }\nfunc (e %s) Unwrap() error { return e.err }\n", wt, wt, wt)
	g.tpl(`{
	e1 := errors.New("e1")
	e1b := errors.New("e1")
	e2 := fmt.Errorf("wrap2: %w", e1)
	e3 := fmt.Errorf("wrap3: %w + %w", e2, e1b)
	fmt.Println(e1 == e1b, errors.Is(e1, e1b), errors.Is(e3, e1), errors.Is(e3, e1b), errors.Unwrap(e2) == e1, errors.Unwrap(e3) == nil, errors.Is(nil, e1), errors.Is(e1, nil))
	j := errors.Join(e1, nil, e1b)
	fmt.Printf("%v|%q|%v\n", j, j.Error(), errors.Join(nil, nil) == nil)
	fmt.Println(errors.Is(j, e1b), errors.Unwrap(j) == nil)
	p1, p2 := &@PT@{@C1@}, &@PT@{@C2@}
	w := @WT@{"outer", fmt.Errorf("mid: %w", p1)}
	fmt.Println(errors.Is(w, p2), errors.Is(w, p1), errors.Is(p1, p2), w)
	var target *@PT@
	fmt.Println(errors.As(w, &target), target == p1, errors.As(e1, &target))
	var pv @PE@
	err := error(fmt.Errorf("deep: %w", @PE@{@C1@, "m"}))
	fmt.Println(errors.As(err, &pv), pv, errors.As(w, &pv))
	var wt @WT@
	fmt.Println(errors.As(w, &wt), wt.msg)
	fmt.Println(fmt.Errorf("bad %w", 5), fmt.Errorf("no verb"), fmt.Errorf("two %w %w", e1, nil))
	fmt.Println(fmt.Sprintf("%w", e1), fmt.Sprintf("%v %s %q %x", e2, e2, e1, e1))
	var nilp *@PT@
	var ne error = nilp
	fmt.Println(ne == nil, nilp == nil, errors.Is(ne, p1) == false)
	fmt.Println(errors.ErrUnsupported)
}`, "PT", pt, "PE", pe, "WT", wt, "C1", strconv.Itoa(g.between(10, 19)), "C2", strconv.Itoa(g.between(10, 29)))
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenPkgSync(g *Gen) {
	g.importPkg("sync")
	n, k := g.between(2, 8), g.between(1, 9)
	g.tpl(`{
	var mu sync.Mutex
	var wg sync.WaitGroup
	total := 0
	for i := 1; i <= @N@; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			total += i * @K@
		}()
	}
	wg.Wait()
	fmt.Println(total)
	var once sync.Once
	calls := 0
	for i := 0; i < @N@; i++ {
		once.Do(func() { calls++ })
	}
	fmt.Println(calls)
	var rw sync.RWMutex
	rw.RLock()
	rw.RLock()
	fmt.Println(rw.TryLock(), rw.TryRLock())
	rw.RUnlock()
	rw.RUnlock()
	rw.RUnlock()
	rw.Lock()
	fmt.Println(rw.TryRLock())
	rw.Unlock()
	fmt.Println(mu.TryLock(), mu.TryLock())
	mu.Unlock()
	var sm sync.Map
	sm.Store("a", 1)
	v, ok := sm.Load("a")
	act, loaded := sm.LoadOrStore("a", 2)
	act2, loaded2 := sm.LoadOrStore("b", 3)
	sm.Delete("a")
	_, ok2 := sm.Load("a")
	sum := 0
	sm.Range(func(k, v any) bool { sum += v.(int); return true })
	prev, was := sm.Swap("b", 10)
	fmt.Println(v, ok, act, loaded, act2, loaded2, ok2, sum, prev, was)
	fmt.Println(sm.CompareAndSwap("b", 10, 11), sm.CompareAndSwap("b", 10, 12))
	val, ok3 := sm.LoadAndDelete("b")
	fmt.Println(val, ok3)
	done := make(chan struct{})
	var mu2 sync.Mutex
	cond := sync.NewCond(&mu2)
	ready := false
	go func() {
		mu2.Lock()
		for !ready {
			cond.Wait()
		}
		mu2.Unlock()
		close(done)
	}()
	mu2.Lock()
	ready = true
	cond.Broadcast()
	mu2.Unlock()
	<-done
	fmt.Println("cond done")
}`, "N", strconv.Itoa(n), "K", strconv.Itoa(k))
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}

func scenPkgTime(g *Gen) {
	g.importPkg("time")
	date := func() string {
		return fmt.Sprintf("time.Date(%d, time.Month(%d), %d, %d, %d, %d, %s, time.UTC)", g.between(1, 2400), g.between(-2, 14), g.between(-3, 35), g.between(-3, 30), g.between(-5, 70), g.between(-5, 70), g.pick("0", "1", "999999999", "500000000", "123456789", "-1", "1000000000"))
	}
	for i := g.between(1, 3); i > 0; i-- {
		g.tpl(`{
	d := time.Duration(@DMS@) * time.Millisecond
	fmt.Println(d, d.Hours(), d.Minutes(), d.Seconds(), d.Round(time.Second), d.Truncate(time.Minute), d.Milliseconds(), d.Microseconds(), d.Abs(), d.String())
	t, t2 := @D1@, @D2@
	fmt.Println(t, t.Weekday(), t.YearDay(), t.Unix(), t.UnixNano(), t.UnixMilli())
	fmt.Println(t.Format(time.RFC3339Nano), t.Format("Mon Jan _2 15:04:05 2006"), t.Format(time.RFC1123), t.Format("2006-01-02 03:04:05.000 PM -0700 MST"), t.Format(time.Kitchen))
	fmt.Println(t.Add(d), t.Sub(t2), t2.Sub(t), t.AddDate(0, @M@, @DD@), t.Truncate(time.Hour), t.Round(time.Minute), t.Before(t2), t.After(t2), t.Equal(t), t.Compare(t2), t.IsZero())
	y, mo, dd := t.Date()
	h, mi, s := t.Clock()
	_, wk := t.ISOWeek()
	fmt.Println(y, mo, dd, h, mi, s, wk, t.Year(), t.Month(), t.Day(), t.Hour(), t.Nanosecond(), time.Time{}, time.Time{}.IsZero(), time.Time{}.Unix())
	fz := time.FixedZone("X", @OFF@)
	tz := t.In(fz)
	fmt.Println(tz, tz.Location(), tz.Equal(t), tz.Format(time.RFC822Z), t.UTC() == t)
	name, off := tz.Zone()
	fmt.Println(name, off)
	p, err := time.Parse(time.RFC3339, @PARSE@)
	fmt.Println(p, err)
	pd, err := time.ParseDuration(@PDUR@)
	fmt.Println(pd, err)
	u := time.Unix(@SEC@, @NSEC@).UTC()
	fmt.Println(u, u.Unix(), u.Nanosecond(), time.UnixMilli(@SEC@).UTC())
	fmt.Println(time.Duration(1<<63-1), time.Duration(-1<<63), time.Duration(0), time.Duration(999), time.Duration(1001), time.Hour+30*time.Minute+5*time.Second+7*time.Millisecond)
	fmt.Println(time.March, time.Saturday, time.Month(13), time.Weekday(8))
}`, "DMS", g.pick(g.intRaw(intTs["int32"]), "0", "1500", "-1500", "3600000", "86400000", "123456789"),
			"D1", date(), "D2", date(), "M", strconv.Itoa(g.between(-14, 14)), "DD", strconv.Itoa(g.between(-40, 40)),
			"OFF", strconv.Itoa(g.between(-12, 14)*3600+g.between(0, 2)*1800),
			"PARSE", strconv.Quote(g.pick("2006-01-02T15:04:05Z", "2006-01-02T15:04:05+07:00", "2006-01-02T15:04:05.999999999Z", "2006-13-02T15:04:05Z", "2006-02-30T15:04:05Z", "2006-01-02", "", "2006-01-02T24:00:00Z", "2006-01-02T15:04:05-00:30", "0000-01-01T00:00:00Z", "9999-12-31T23:59:59Z", "2006-01-02T15:04:60Z")),
			"PDUR", strconv.Quote(g.pick("1h30m", "1.5s", "-2m3.5s", "100", "1d", "", "0", "1us", "1µs", "9223372036854775807ns", "9223372036854775808ns", ".5h", "1h-1m", "+3ms", "1e3s")),
			"SEC", g.pick("0", "1", "-1", "1700000000", "-62135596800", "253402300799", "4102444800", "951782400"),
			"NSEC", g.pick("0", "1", "999999999", "1000000000", "-1", "-1000000001"))
	}
	g.stmts(g.between(0, 2), 1)
	g.showAll()
}
