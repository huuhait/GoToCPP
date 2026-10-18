from lib import case

FLAGS = ["", "+", "-", " ", "0", "#", "8", "-8", "08", "+8", "+08", " 8", "8.3", ".3", ".0", "#8", "#08", "-#8", "+.5", "0-8", "12.8", "-12.8", "#.4", "+#"]

def fl_lines(verb, vals, flags=FLAGS, tmpl='\tfmt.Printf("[%%%s%s]\\n", %s)\n'):
    out = ""
    for v in vals:
        out += "\t// %s\n" % v if False else ""
        for f in flags:
            out += tmpl % (f, verb, v)
    return out

# ---- signed integers ----
INTV = {
    "int": ["0", "1", "-1", "42", "-42", "1234567", "-1234567"],
    "int8": ["int8(0)", "int8(127)", "int8(-128)", "int8(-5)"],
    "int16": ["int16(32767)", "int16(-32768)", "int16(300)"],
    "int32": ["int32(2147483647)", "int32(-2147483648)", "int32(65)"],
    "int64": ["int64(9223372036854775807)", "int64(-9223372036854775808)", "int64(1) << 40"],
}
for verb in "dboOxX":
    for tname, vals in INTV.items():
        if verb in "oOX" and tname in ("int8", "int16", "int32"):
            continue
        flags = FLAGS
        case("fmt_int_%s_%s" % ({"d":"dec","b":"bin","o":"oct","O":"octo","x":"hex","X":"hexu"}[verb], tname),
             "Go's %%%s of %s values with every flag, width and precision: the integer formatting of fmt/format.go." % (verb, tname),
             "", fl_lines(verb, vals, flags))

for verb, name in (("c", "char"), ("q", "quoted"), ("U", "unicode"), ("v", "v")):
    vals = ["65", "0x263A", "0x1F600", "0", "10", "-1", "0x10FFFF", "0x110000", "0xD800", "127", "0xFFFD", "'\\''", "'\"'", "'\\\\'", "7", "0x80", "0x200B"]
    if verb == "c":
        vals = ["65", "0x263A", "0x1F600", "-1", "0x10FFFF", "0x110000", "0xD800", "0xFFFD", "'\\''", "'\"'", "'\\\\'", "0x200B"]
    if verb == "v":
        vals = ["0", "42", "-42", "int8(-7)", "int64(1)<<62"]
    flags = ["", "+", "-", "#", "8", "-8", "08", ".3", "#8", "+#", "-#8", " ", "3.8"] if verb != "v" else FLAGS
    case("fmt_int_%s_values" % name,
         "Go's %%%s of integers: code points valid and invalid (surrogates, beyond U+10FFFF, negative), with flags." % verb,
         "", fl_lines(verb, vals, flags))

# ---- unsigned ----
UINTV = {
    "uint": ["uint(0)", "uint(1)", "uint(42)", "uint(1234567)"],
    "uint8": ["uint8(0)", "uint8(255)", "uint8(65)"],
    "uint16": ["uint16(65535)", "uint16(300)"],
    "uint32": ["uint32(4294967295)", "uint32(65)"],
    "uint64": ["uint64(18446744073709551615)", "uint64(1)<<40"],
    "uintptr": ["uintptr(0)", "uintptr(4096)"],
}
for verb in "dxXoc":
    for tname, vals in UINTV.items():
        if verb in "oc" and tname not in ("uint", "uint8", "uint64"):
            continue
        flags = FLAGS if verb != "c" else ["", "8", "-8"]
        if verb == "c":
            vals = [v for v in vals if "(0)" not in v]
        case("fmt_uint_%s_%s" % (verb, tname),
             "Go's %%%s of %s values with flags, width and precision." % (verb, tname),
             "", fl_lines(verb, vals, flags))

case("fmt_uint_v_sharp",
     "%#v of unsigned integers is hex (0x41), of signed ones decimal: the sharpV flag; named types print the same.",
     "type Count uint16\n\ntype Level int8\n",
     '''	fmt.Printf("%#v %#v %#v %#v %#v\\n", uint(1), uint8(255), uint64(1<<40), uintptr(7), uint16(3))
	fmt.Printf("%#v %#v %#v %#v\\n", 1, int8(-3), int64(5), int32(9))
	fmt.Printf("%#v %v %d %#v\\n", Count(9), Count(9), Count(9), Level(-4))
	fmt.Printf("%#v %#v\\n", []uint8{1, 2}, [3]uint16{1, 2, 3})
	fmt.Printf("%#v\\n", map[uint8]uint32{1: 2})
''')
