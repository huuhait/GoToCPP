from lib import case
from cats1 import fl_lines, FLAGS

FFLAGS = ["", "+", "-", " ", "0", "#", "10", "-10", "010", "+010", "10.3", ".0", ".1", ".3", ".10", "#.0", "#10.3", "+.2", " 10.2", "-#10.2", "20.15", ".20", "0-10"]

SPECIALS = """	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
"""
FV = {
    "basic": ["0.0", "1.0", "-1.0", "3.14159265358979", "2.5", "100.0", "0.5", "123456.789"],
    "round": ["2.675", "1.005", "0.125", "0.375", "2.5", "3.5", "-2.5", "0.045", "1e22", "999999.5", "0.9999995"],
    "big": ["1e6", "1e20", "1e21", "1e22", "123456789.0", "12345678901234567890.0", "1e100", "1.7976931348623157e308", "float64(1<<53)"],
    "small": ["1e-4", "1e-5", "0.0001234", "1e-7", "5e-324", "2.2250738585072014e-308", "1.5e-10", "0.1", "0.2+0.1"],
    "special": ["inf", "ninf", "nan", "nzero", "0.0"],
}
VERBS = [("v", "v"), ("g", "g"), ("G", "G"), ("e", "e"), ("E", "E"), ("f", "f"), ("F", "F"), ("b", "b"), ("x", "x"), ("X", "X")]
names = {"v":"v","g":"g","G":"gu","e":"e","E":"eu","f":"f","F":"fu","b":"b","x":"x","X":"xu"}
for verb, _ in VERBS:
    for grp, vals in FV.items():
        if verb in "bxX" and grp in ("round",):
            vals = FV["basic"] if grp != "round" else vals
        fl = FFLAGS
        if verb in "bxX":
            fl = ["", "+", "-", " ", "0", "#", "30", "-30", "030", "+030", ".0", ".1", ".3", ".10", "#.3", "#30"]
        pre = SPECIALS if "inf" in " ".join(vals) else ""
        case("fmt_float_%s_%s" % (names[verb], grp),
             "Go's %%%s of float64 values (%s) with flags, width and precision, as strconv.AppendFloat and fmt.fmtFloat do." % (verb, grp),
             "", pre + fl_lines(verb, vals, fl))

# float32
F32 = ["float32(0.1)", "float32(3.14159274)", "float32(16777216.0)", "float32(1e-7)", "float32(1e20)", "float32(2.5)", "float32(-0.0)", "float32(1)/3"]
for verb in "vgefxb":
    case("fmt_float32_%s" % verb, "Go's %%%s of float32 values: the shortest digits are those of a float32 (0.1, not 0.10000000149011612)." % verb,
         "", fl_lines(verb, F32, ["", "+", "10", "-10", ".3", "#", "010.2"]))

case("fmt_float_named",
     "Named float types print as their underlying float; the error forms name the type; %v of Stringer floats.",
     "type Celsius float64\n\ntype Ratio float32\n",
     '''	fmt.Printf("%v %.1f %g %e %6.2f|%-8.3v|\\n", Celsius(21.5), Celsius(21.55), Celsius(1e21), Celsius(1234.5), Celsius(3.14159), Celsius(2.71828))
	fmt.Printf("%v %.2f %g %s %d\\n", Ratio(0.1), Ratio(0.125), Ratio(1)/3, Ratio(2), Ratio(3))
	fmt.Println(Celsius(1.5), Ratio(0.1), []Celsius{1, 2.5}, map[string]Ratio{"a": 0.5})
	fmt.Printf("%T %T %#v %#v\\n", Celsius(1), Ratio(1), Celsius(2), Ratio(0.25))
''')

# complex
CV = ["complex(1, 2)", "complex(-1.5, 0.25)", "complex(0, 0)", "complex(1e21, 1e-7)", "complex(inf, nan)", "complex(2.675, -2.675)"]
for verb in "vgefxb":
    case("fmt_complex_%s" % verb, "Go's %%%s of complex128: both parts with the verb, the imaginary part always signed." % verb,
         "", SPECIALS + fl_lines(verb, CV, ["", "+", "-", " ", "10", "-10", "010", ".2", "10.3", "#", "+.1"]))
case("fmt_complex64",
     "complex64 prints its parts as float32; named complex types; the error forms.",
     "type Z complex128\n",
     '''	fmt.Println(complex64(complex(0.1, 0.2)), complex(float32(1), 2), Z(complex(3, -4)))
	fmt.Printf("%v %.2f %g %e %8.1f|%-12v|\\n", complex64(complex(0.1, 0.2)), complex64(complex(1.005, 2.675)), complex64(complex(1e10, 1e-10)), complex64(1+2i), complex64(3+4i), complex64(5i))
	fmt.Printf("%d %s %t %q\\n", complex(1, 2), complex(1, 2), complex(1, 2), complex(1, 2))
	fmt.Printf("%T %T %T %#v\\n", complex64(1), complex128(1), Z(1), Z(1+2i))
''')

# strings
SV = {
    "ascii": ['""', '"hello"', '"hello, world"', '"a b\\tc"', '"line1\\nline2"', '"quote\\"d"', '"back\\\\slash"', '"`tick`"'],
    "unicode": ['"héllo"', '"日本語"', '"😀 smile"', '"\\u00a0nbsp"', '"zero\\u200bwidth"'],
    "controls": ['"\\u2028sep"', '"bell\\a"', '"\\x7f"', '"\\x00nul"', '"\\ufeffbom"', '"\\x1b[0m"', '"\\u0085next"'],
    "invalid": ['"\\xff"', '"ab\\xc3"', '"\\xed\\xa0\\x80"', '"\\xf4\\x90\\x80\\x80"', '"a\\xffb"'],
}
for verb in "svqxX":
    for grp, vals in SV.items():
        if verb in "sv" and grp in ("invalid", "controls"):
            continue
        fl = ["", "+", "-", " ", "#", "10", "-10", "010", ".3", ".0", "10.3", "-10.3", "#10", "#.2", "+#", "x" if False else ""]
        fl = [f for f in fl if f is not None]
        fl = list(dict.fromkeys(fl))
        if verb in "xX":
            fl = ["", "+", "-", " ", "#", "# ", "10", "-10", ".3", ".0", "20.2", " 12", "#12", "020"]
        if grp == "invalid" and verb in "svxX":
            if verb == "v":
                continue
            if verb in "xX":
                pass
        case("fmt_string_%s_%s" % (verb, grp),
             "Go's %%%s of strings (%s) with flags, width and precision (runes, not bytes)." % (verb, grp),
             "", fl_lines(verb, vals, fl))

case("fmt_string_sharp_v",
     "%#v of strings quotes them with Go syntax; %+q escapes non-ASCII; %#q uses backquotes when it can.",
     "type Name string\n",
     '''	fmt.Printf("%#v %#v %#v %#v\\n", "hi", "tab\\there", "é😀", Name("n"))
	fmt.Printf("%+q %+q %q %#q %#q %#q\\n", "日本", "é", "日本", "raw `string", "plain", "tab\\tin")
	fmt.Printf("%#v %v %q\\n", []string{"a", "b"}, []string{"a b", "c"}, []string{"a", "b"})
	fmt.Printf("%#v\\n", map[string]string{"k": "v"})
	fmt.Printf("%5.2q|%-8q|%08q|\\n", "abcdef", "ab", "ab")
''')

case("fmt_string_error_verbs",
     "Verbs that don't fit a string print Go's error form %!d(string=x), also inside composite values.",
     "type S string\n",
     '''	fmt.Printf("%d %t %c %f %e %b %o %U %p\\n", "x", "x", "x", "x", "x", "x", "x", "x", "x")
	fmt.Printf("%d %s\\n", S("x"), S("y"))
	fmt.Printf("%d %d\\n", []string{"a"}, map[string]string{"k": "v"})
	fmt.Printf("%z %!\\n", "x")
''')

# bytes
BV = ['[]byte(nil)', '[]byte{}', '[]byte("hello")', '[]byte{0, 1, 2, 254, 255}', '[]byte("日本語")', '[]byte("a b")', '[]byte("tab\\t")']
for verb in "svqxXd":
    fl = ["", "+", "-", " ", "#", "10", "-10", ".3", "10.2", "# ", "#10", "020"]
    BV2 = [v for v in BV if not (verb == "s" and "254" in v)]
    case("fmt_bytes_%s" % verb, "Go's %%%s of byte slices: %%s %%q %%x %%X treat them as strings, %%v and %%d as lists of numbers." % verb,
         "", fl_lines(verb, BV2, fl))
case("fmt_bytes_arrays_named",
     "Byte arrays and named byte types: %s %x %q %v work on them as on slices; %T and %#v name the types.",
     "type B []byte\n\ntype Hash [4]byte\n\ntype Octet byte\n",
     '''	h := Hash{1, 2, 255, 16}
	fmt.Printf("%v %x %X %s|%q %d\\n", h, h, h, Hash{72, 105, 33, 33}, Hash{72, 105, 0, 1}, h)
	fmt.Printf("%v %s %x %q\\n", B("abc"), B("abc"), B("abc"), B("abc"))
	fmt.Printf("%v %c %d %q\\n", Octet(65), Octet(65), Octet(65), Octet(65))
	fmt.Printf("%v %s\\n", []Octet{72, 105}, []Octet{72, 105})
	fmt.Printf("%T %T %T %T\\n", []byte(nil), []uint8{}, h, B(nil))
	fmt.Printf("%#v %#v %#v %#v\\n", []byte("ab"), []uint8(nil), h, B("x"))
	fmt.Printf("%#v %#v\\n", [2][]byte{[]byte("a"), nil}, struct{ P []byte }{[]byte("x")})
	var a any = []byte("xyz")
	fmt.Printf("%s %v %x %T\\n", a, a, a, a)
	fmt.Println([]byte("ab"), [2]byte{1, 2}, [][]byte{[]byte("a"), []byte("b")})
	fmt.Print([]byte("ab"), "\\n")
''')

# bools
case("fmt_bool_verbs", "Go's %t and %v of bool with width and flags; the error forms for other verbs.",
     "type Flag bool\n",
     fl_lines("t", ["true", "false"], ["", "5", "-5", "05", "+", "#", ".1", "8.3"]) + fl_lines("v", ["true", "false"], ["", "5", "-6"]) + '''	fmt.Printf("%d %s %x %c %q\\n", true, false, true, true, false)
	fmt.Printf("%v %t %d\\n", Flag(true), Flag(false), Flag(true))
	fmt.Printf("%v %v %t\\n", []bool{true, false}, map[bool]int{true: 1, false: 0}, [2]bool{})
	fmt.Printf("%#v %#v %T\\n", true, Flag(false), Flag(true))
''')
