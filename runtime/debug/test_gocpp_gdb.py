# Tests of the part of gocpp-gdb.py that doesn't need gdb: the names, the Go text of strings and
# numbers, and the reader of .gp sources. Run by TestDebugScriptUnits (runtime/debugscript_test.go),
# or by hand: python3 -I runtime/debug/test_gocpp_gdb.py
#
# The script imports gdb; the stub module below lets it load in a plain Python (STUB: it then
# registers nothing).

import importlib.util
import os
import sys
import types
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def load():
    g = types.ModuleType("gdb")
    g.GOCPP_STUB = True
    g.error = RuntimeError
    g.MemoryError = type("MemoryError", (RuntimeError,), {})
    g.GdbError = type("GdbError", (Exception,), {})
    g.TYPE_CODE_STRUCT = 3
    g.TYPE_CODE_PTR = 1
    g.BreakpointEvent = type("BreakpointEvent", (), {})
    for name in ("Command", "Function", "Parameter"):
        setattr(g, name, type(name, (), {"__init__": lambda self, *a, **k: None}))
    sys.modules["gdb"] = g
    spec = importlib.util.spec_from_file_location("gocpp_gdb", os.path.join(HERE, "gocpp-gdb.py"))
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


G = load()

FRAMES = os.path.join(HERE, "..", "..", "tests", "gdb", "frames.gp")


class QuoteGo(unittest.TestCase):
    def test_quote(self):
        cases = [
            (b"", '""'),
            (b"hello", '"hello"'),
            (b'a"b\\c\n\t', '"a\\"b\\\\c\\n\\t"'),
            (b"\x00\x1f\x7f", '"\\x00\\x1f\\x7f"'),
            (b"\xff\xfe", '"\\xff\\xfe"'),
            ("héllo, 世界".encode(), '"héllo, 世界"'),
            (b"\r\a\b\f\v", '"\\r\\a\\b\\f\\v"'),
        ]
        for data, want in cases:
            self.assertEqual(G.quote_go(data), want, data)

    def test_ascii_only(self):
        self.assertEqual(G.quote_go("héllo, 世界".encode(), ascii_only=True), '"h\\u00e9llo, \\u4e16\\u754c"')
        self.assertEqual(G.quote_go("😀".encode(), ascii_only=True), '"\\U0001f600"')

    def test_limit(self):
        self.assertEqual(G.quote_go(b"abcdef", 3), '"abc"...')
        self.assertEqual(G.quote_go(b"abc", 3), '"abc"')


class GoFloat(unittest.TestCase):
    def test_float(self):
        cases = [
            (0.0, 8, "0"),
            (2.25, 8, "2.25"),
            (-1.5, 8, "-1.5"),
            (1e6, 8, "1000000.0".rstrip("0").rstrip(".")),
            (1e21, 8, "1e+21"),
            (1e-7, 8, "1e-07"),
            (float("inf"), 8, "+Inf"),
            (float("-inf"), 8, "-Inf"),
            (float("nan"), 8, "NaN"),
        ]
        for x, size, want in cases:
            self.assertEqual(G.go_float(x, size), want, x)

    def test_float32(self):
        import struct

        x = struct.unpack("f", struct.pack("f", 0.1))[0]
        self.assertEqual(G.go_float(x, 4), "0.1")
        self.assertNotEqual(G.go_float(x, 8), "0.1")


class Types(unittest.TestCase):
    def test_go_type(self):
        cases = [
            ("long", "int"),
            ("gp::string", "string"),
            ("gp::slice<long>", "[]int"),
            ("gp::slice<gp::slice<long> >", "[][]int"),
            ("gp::map<gp::string, long>", "map[string]int"),
            ("gp::map<gp::string, gp::slice<long> >", "map[string][]int"),
            ("gp::ptr<gpkg_main::Point>", "*main.Point"),
            ("gp::bptr<gpkg_main::Point>", "*main.Point"),
            ("gp::local_ptr<gpkg_main::Point>", "*main.Point"),
            ("gp::chan<long>", "chan int"),
            ("gp::array<long, 3>", "[3]int"),
            ("gp::array<long, 2l>", "[2]int"),
            ("gp::array<unsigned char, 16ul>", "[16]uint8"),
            ("gp::func<long(long, long)>", "func(int, int) int"),
            ("gp::func<void()>", "func()"),
            ("gpkg_main::Point", "main.Point"),
            ("gp_anon_1a2b", "struct{...}"),
            ("gp::any", "any"),
            ("gp::error", "error"),
            ("const gp::string &", "string"),
            ("unsigned char", "uint8"),
            ("gp::complex128", "complex128"),
        ]
        for cxx, want in cases:
            self.assertEqual(G.go_type(cxx), want, cxx)

    def test_data_model(self):
        # Windows (LLP64): a long is 32 bits, gocpp's int is long long, and the interface kinds
        # read their value as long long. The model of the system the tests run on comes back at the end.
        try:
            G.apply_data_model(4)
            cases = [
                ("long", "int32"),
                ("unsigned long", "uint32"),
                ("long long", "int"),
                ("unsigned long long", "uint"),
                ("gp::slice<long long>", "[]int"),
                ("int64_t", "int"),
            ]
            for cxx, want in cases:
                self.assertEqual(G.go_type(cxx), want, cxx)
            self.assertEqual(G.KIND_TYPES["Int"], "long long")
            self.assertEqual(G.KIND_TYPES["Uintptr"], "unsigned long long")
            self.assertEqual(G.KIND_TYPES["Int64"], "long long")
        finally:
            G.apply_data_model(8)
        self.assertEqual(G.go_type("long"), "int")
        self.assertEqual(G.go_type("long long"), "int64")
        self.assertEqual(G.KIND_TYPES["Int"], "long")

    def test_split_args(self):
        self.assertEqual(G.split_args("a, b<c, d>, e(f, g)"), ["a", "b<c, d>", "e(f, g)"])

    def test_package_name(self):
        self.assertEqual(G.package_name("gpkg_main"), "main")
        self.assertEqual(G.package_name("gpkg_github_dcom__huuhait__game__engine"), "engine")

    def test_field_name(self):
        self.assertEqual(G.go_field_name("new_"), "new")
        self.assertEqual(G.go_field_name("class_"), "class")
        self.assertEqual(G.go_field_name("name_"), "name_")


class Names(unittest.TestCase):
    def test_demangle(self):
        cases = [
            ("gpkg_main::main", "main.main"),
            ("gpkg_main::fact", "main.fact"),
            ("gpkg_main::T::Inc", "main.T.Inc"),
            ("gpkg_main::gp_var_init", "main.init"),
            ("gpkg_main::gp_init_1", "main.init.0"),
            ("gpkg_main::gp_inst_1a2b3c_Max<long>", "main.Max[...]"),
            ("gpkg_main::main()::$_0::operator()<1>", "main.main.func1"),
            ("gp::run_main", None),
            ("operator()<>", None),
        ]
        for name, want in cases:
            self.assertEqual(G.demangle_cxx(name), want, name)

    def test_lambda_and_glue(self):
        self.assertTrue(G.is_lambda_name("operator()<>(void)"))
        self.assertTrue(G.is_lambda_name("gpkg_main::main()::$_0::operator()<1>()"))
        self.assertFalse(G.is_lambda_name("gpkg_main::main"))
        self.assertTrue(G.is_glue_name("gp::func<long (long)>::operator()"))
        self.assertTrue(G.is_glue_name("gp::detail::call_deferred"))
        self.assertFalse(G.is_glue_name("gpkg_main::main"))
        # the helper frames of a deferred call, of a goroutine and of its thread
        for name in (
            "std::__invoke_impl<void, gpkg_main::main()::<lambda()>&>",
            "std::__invoke<gpkg_main::main()::<lambda()>&>",
            "std::__apply_impl<gpkg_main::main()::<lambda()>&, std::tuple<> >",
            "std::apply<gpkg_main::main()::<lambda()>&, std::tuple<> >",
            "gp::defer_frame::gp_node_of<(gp::defer_frame::gp_mode)0, gpkg_main::main()::<lambda()> >::gp_do",
            "gp::defer_frame::gp_run",
            "gp::detail::call_task<gpkg_main::main()::<lambda()> >::run",
            "gp::detail::(anonymous namespace)::run_goroutine",
            "gp::detail::(anonymous namespace)::worker_main",
            "gp::os::(anonymous namespace)::thread_main",
            "start_thread",
            "__clone3",
        ):
            self.assertTrue(G.is_glue_name(name), name)
        # but not the natives a Go program calls, nor a frame that merely contains a word
        for name in ("std::string::append", "gp::detail::select_run", "gp::run_main", "main", "gp::defer_frame::gp_push_call"):
            self.assertFalse(G.is_glue_name(name), name)


class Fake:
    """A stand-in for a gdb.Value that holds integers (and fields)."""

    def __init__(self, **fields):
        self.fields = fields
        self.is_optimized_out = False

    def __getitem__(self, k):
        return self.fields[k]


class Robust(unittest.TestCase):
    """Garbage in the program's memory must give text, not a Python exception."""

    def test_string_with_negative_length(self):
        for n in (-1, -(1 << 63), 0):
            v = Fake(gp_len_=n, gp_data_=0x1000)
            self.assertEqual(G.fmt_string(v, None, 0), '""', n)

    def test_fmt_catches_everything(self):
        class Bad(Fake):
            @property
            def type(self):
                raise OverflowError("int too big")

        self.assertEqual(G.fmt(Bad()), "<unreadable: OverflowError: int too big>")
        class Unreadable(Fake):
            @property
            def type(self):
                raise sys.modules["gdb"].MemoryError("Cannot access memory")

        mem = Unreadable()
        self.assertEqual(G.fmt(mem), "<unreadable: Cannot access memory>")

    def test_c_string(self):
        mem = {}

        def read(addr, n):
            out = bytearray()
            for a in range(addr, addr + n):
                if a not in mem:
                    raise sys.modules["gdb"].MemoryError("Cannot access memory at address 0x%x" % a)
                out.append(mem[a])
            return bytes(out)

        old = G.read_bytes
        G.read_bytes = read
        try:
            def put(addr, data):
                for i, b in enumerate(data):
                    mem[addr + i] = b

            put(0x100, b"main.T\0" + bytes(100))
            self.assertEqual(G.c_string(0x100), "main.T")
            put(0x200, b"\xff\xfe\x80ok\0" + bytes(100))  # not UTF-8: replaced, no exception
            self.assertEqual(G.c_string(0x200), "\ufffd\ufffd\ufffdok")
            put(0x3f0, b"x" * 80)  # no NUL, then unreadable memory
            self.assertEqual(G.c_string(0x3f0), "x" * 80)
            self.assertEqual(G.c_string(0x9000), "")  # unreadable
            put(0x1000, b"y" * 2000)  # no NUL for ever: capped
            self.assertEqual(len(G.c_string(0x1000)), G.NAME_CAP)
            put(0x5000, "héllo\0".encode() + bytes(100))
            self.assertEqual(G.c_string(0x5000), "héllo")
        finally:
            G.read_bytes = old

    def test_budget(self):
        G._budget[0] = 5
        old = G.fmt
        G.fmt = lambda e, depth=0: str(e)
        try:
            self.assertEqual(G._fmt_list([1, 2, 3], 3, 0), "[1 2 3]")
            self.assertEqual(G._fmt_list([4, 5, 6], 3, 0), "[4 5 ...]")
            self.assertEqual(G._fmt_list([7, 8], 2, 0), "[...]")
        finally:
            G.fmt = old

    def test_sort_key_numbers(self):
        keys = ["100", "-2.5", "10.5", "9.5", "NaN", "-Inf", "+Inf", "1e3", "-7"]
        self.assertEqual(sorted(keys, key=G._sort_key), ["NaN", "-Inf", "-7", "-2.5", "9.5", "10.5", "100", "1e3", "+Inf"])
        self.assertEqual(sorted(["b", "a", "10", "9"], key=G._sort_key), ["9", "10", "a", "b"])
        big = ["18446744073709551615", "18446744073709551614"]
        self.assertEqual(sorted(big, key=G._sort_key), big[::-1])


class Decls(unittest.TestCase):
    def test_classify(self):
        cases = [
            ("x", "\tx := 20", "box"),
            ("x", "\tx := -1", "box"),
            ("s", '\ts := "hi" // comment', "box"),
            ("a", "\ta := []int{1}", "box"),
            ("m", "\tm := map[string]int{}", "box"),
            ("m", "\tm := make(map[string]int)", "box"),
            ("t", "\tt := T{1}", "box"),
            ("t", "\tt := pkg.T{1}", "box"),
            ("f", "\tf := func() {}", "box"),
            ("err", "\tvar err error", "box"),
            ("arr", "\tvar arr [1000000]int", "box"),
            ("n", "\tvar n int = 3", "box"),
            ("p", "\tp := &T{1}", "ptr?"),
            ("p", "\tp := new(T)", "ptr?"),
            ("p", "\tvar p *T", "ptr?"),
            ("p", "\tp := &q", None),
            ("p", "\tp := f()", None),
            ("p", "\tp := q", None),
            ("p", "\tp, q := f()", None),
            ("x", "\ty := 20", None),
            ("x", "\tx = 20", None),
            ("i", "\tfor i := 0; i < n; i++ {", "box"),
        ]
        for name, text, want in cases:
            self.assertEqual(G.classify_decl(name, text), want, text)

    def test_find_decl_kind(self):
        src = G.GoSource("package main\n\nfunc main() {\n\tcount := 20\n\tinc := func() {\n\t\tcount++\n\t}\n\tinc()\n}\n")
        self.assertEqual(G.find_decl_kind(src, "count", 6), "box")
        self.assertIsNone(G.find_decl_kind(src, "other", 6))
        self.assertEqual(G.find_decl_kind(src, "inc", 6), "box")  # (a func literal)

    def test_go_source_limits(self):
        import tempfile

        with tempfile.TemporaryDirectory() as d:
            ok = os.path.join(d, "a.gp")
            with open(ok, "w") as f:
                f.write("package a\n\nfunc f() {}\n")
            self.assertEqual(G.go_source(ok).package, "a")
            other = os.path.join(d, "a.txt")
            with open(other, "w") as f:
                f.write("package a\n")
            self.assertIsNone(G.go_source(other))
            big = os.path.join(d, "big.gp")
            with open(big, "w") as f:
                f.write("package a\n" + "// x\n" * (G.MAX_SOURCE // 5))
            self.assertIsNone(G.go_source(big))
            self.assertIsNone(G.go_source(os.path.join(d, "nofile.gp")))
            os.mkfifo(os.path.join(d, "pipe.gp"))
            self.assertIsNone(G.go_source(os.path.join(d, "pipe.gp")))  # not a regular file: not opened

    def test_go_cxx_name(self):
        self.assertEqual(G.go_cxx_name("main.main"), "gpkg_main::main")
        self.assertEqual(G.go_cxx_name("main.T.Get"), "gpkg_main::T::Get")
        self.assertEqual(G.go_cxx_name("main.(*T).Inc"), "gpkg_main::T::Inc")
        self.assertIsNone(G.go_cxx_name("main"))
        self.assertIsNone(G.go_cxx_name("main.a.b.c"))

    def test_skip_rules_have_no_backslash(self):
        # gdb takes a backslash in a skip rule as its own escape and eats it.
        for rule in G.SKIP_RULES + G.runtime_rules("/c/gocpp/rt/dev-1a2b") + G.runtime_rules(None):
            self.assertNotIn("\\", rule)

    def test_runtime_rules_are_anchored(self):
        rules = G.runtime_rules("/home/me/.cache/gocpp/rt/dev-1a2b")
        self.assertIn("skip -gfi /home/me/.cache/gocpp/rt/dev-1a2b/src/*.cpp", rules)
        self.assertIn("skip -gfi /home/me/.cache/gocpp/rt/dev-1a2b/include/gocpp/[!f]*.hpp", rules)
        # a glob character in the folder name matches itself only
        self.assertIn("skip -gfi /a[[]b/src/*.cpp", G.runtime_rules("/a[b"))
        # without the folder (or with a name gdb's command line would split): by the cache place
        for root in (None, "/has space/rt/x"):
            self.assertIn("skip -gfi */gocpp/rt/*/src/*.cpp", G.runtime_rules(root))
        for rule in rules + G.runtime_rules(None):
            self.assertNotIn("*/rt/*/src", rule)

    def test_go_cxx_name_init(self):
        self.assertEqual(G.go_cxx_name("main.init"), "gpkg_main::gp_var_init")
        self.assertEqual(G.go_cxx_name("main.init.0"), "gpkg_main::gp_init_1")


class Source(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with open(FRAMES, encoding="utf-8") as f:
            cls.text = f.read()
        cls.src = G.GoSource(cls.text)
        cls.tags = {}
        for i, line in enumerate(cls.text.split("\n"), 1):
            if "// @" in line:
                cls.tags[line.rsplit("@", 1)[1].strip()] = i

    def name(self, tag):
        return self.src.name_at(self.tags[tag])

    def test_names(self):
        self.assertEqual(self.name("get"), "T.Get")
        self.assertEqual(self.name("inc"), "(*T).Inc")
        self.assertEqual(self.name("inner"), "outer.func1.1")
        self.assertEqual(self.name("middle"), "outer.func1")
        self.assertEqual(self.name("defer"), "deferred.func1")
        self.assertEqual(self.name("fact"), "fact")
        self.assertEqual(self.name("loop"), "main")

    def test_package_variable_literal(self):
        # var global = func() int {...}(): a literal of main.init, not a function named int.
        self.assertEqual(self.name("global"), "init.func1")
        self.assertIsNone(self.src.declared("int"))
        self.assertEqual(self.src.package, "main")

    def test_go_name_of(self):
        n = G.go_name_of("gpkg_main::T::Inc", self.src, 15, self.tags["inc"])
        self.assertEqual(n, "main.(*T).Inc")
        n = G.go_name_of("gpkg_main::T::Get", self.src, 11, self.tags["get"])
        self.assertEqual(n, "main.T.Get")
        # g++ names every literal operator()<>; the .gp line says which one.
        n = G.go_name_of("operator()<>", self.src, 0, self.tags["inner"])
        self.assertEqual(n, "main.outer.func1.1")
        n = G.go_name_of("operator()<>", self.src, 0, self.tags["global"])
        self.assertEqual(n, "main.init.func1")
        self.assertEqual(G.go_name_of("gp::run_main", self.src, 0, 0), None)

    def test_tokens(self):
        src = G.GoSource('package p\n\nfunc a() {\n\ts := "func x() {"\n\t// func b() {\n\t_ = s\n}\n\nfunc (t *T[K]) M() {}\n')
        self.assertEqual(src.package, "p")
        self.assertEqual([f.name for f in src.funcs], ["a", "(*T).M"])


class ClosureSignature(unittest.TestCase):
    """The Go type of a func literal that stays a C++ lambda: the debug info has no signature for
    its closure object, so gp locals reads it from the literal in the .gp source."""

    def source_of(self, literal, pkg="main", before=""):
        return "package %s\n\n%sfunc f() {\n\tx := %s\n\t_ = x\n}\n" % (pkg, before, literal)

    def sig(self, literal, pkg="main", before=""):
        src = G.GoSource(self.source_of(literal, pkg, before))
        line = 4 + before.count("\n")
        lit = src.literal_for(line, "x")
        self.assertIsNotNone(lit, literal)
        return lit.sig

    def test_signatures(self):
        cases = [
            ("func() {}", "func()"),
            ("func(x int) int { return x }", "func(int) int"),
            (
                "func(a, b int, s string) (int, error) { return 0, nil }",
                "func(int, int, string) (int, error)",
            ),
            ("func(int, string) {}", "func(int, string)"),
            ("func(n int) (r int, err error) { return }", "func(int) (int, error)"),
            ("func(xs []int, m map[string][]int) {}", "func([]int, map[string][]int)"),
            ("func(p *T) *T { return p }", "func(*main.T) *main.T"),
            ("func(w fmt.Stringer) {}", "func(fmt.Stringer)"),
            ("func(f func(int) string) func() {}", "func(func(int) string) func()"),
            ("func(vs ...int) {}", "func(...int)"),
            ("func(a [3]int) {}", "func([3]int)"),
            ("func(a any, e error) bool { return true }", "func(any, error) bool"),
            ("func(\n\ta int,\n\tb string,\n) {}", "func(int, string)"),
            # the names of a func type's parameters are not part of the type
            ("func(f func(x, y int) bool) {}", "func(func(int, int) bool)"),
            (
                "func(f func(a string) (n int, err error)) {}",
                "func(func(string) (int, error))",
            ),
            (
                "func(c chan<- string, d <-chan int, e chan int) {}",
                "func(chan<- string, <-chan int, chan int)",
            ),
            ("func(v interface{}) {}", "func(interface{})"),
            # a channel of receive-only channels keeps the space (Go writes chan (<-chan int))
            ("func(c chan (<-chan int)) {}", "func(chan (<-chan int))"),
            ("func(c chan<- chan int) {}", "func(chan<- chan int)"),
            # the type arguments of an instance have no space after the comma
            ("func(p Pair[K, string]) {}", "func(main.Pair[main.K,string])"),
            ("func(p Pair[int, []string]) {}", "func(main.Pair[int,[]string])"),
            ("func(m map[string]Pair[int, int]) {}", "func(map[string]main.Pair[int,int])"),
        ]
        for literal, want in cases:
            self.assertEqual(self.sig(literal), want, literal)

    def test_unrenderable_types_are_not_guessed(self):
        # a struct or interface with members is not spelled: the printer shows func(...)
        for literal in (
            "func(s struct{ a int }) {}",
            "func(i interface{ M() }) {}",
            "func() struct{ n int } { return struct{ n int }{} }",
        ):
            self.assertIsNone(self.sig(literal), literal)

    def test_an_array_length_that_is_a_name_is_not_guessed(self):
        # Go spells the value of the constant ([4]string), which the source does not say here
        for literal in (
            "func(a [N]string) {}",
            "func(a [N + 1]string) {}",
            "func(a map[string][N]int) {}",
            "func() [N]int { return [N]int{} }",
        ):
            self.assertIsNone(self.sig(literal), literal)

    def test_slices_and_arrays_with_a_number_are_spelled(self):
        self.assertEqual(self.sig("func(a [4]string, b [][2]int) {}"), "func([4]string, [][2]int)")

    def test_type_parameters_are_not_package_types(self):
        src = G.GoSource(
            "package main\n\n"
            "func g[T any, K comparable](v T, k K) {\n"
            "\tx := func(a T, b []K) K { var z K; return z }\n"
            "\t_ = x\n"
            "}\n"
        )
        self.assertEqual(src.literal_for(4, "x").sig, "func(T, []K) K")
        src = G.GoSource(
            "package main\n\n"
            "func (s *Stack[E]) Push(v E) {\n"
            "\tx := func(e E) T { return T{} }\n"
            "\t_ = x\n"
            "}\n"
        )
        self.assertEqual(src.literal_for(4, "x").sig, "func(E) main.T")

    def test_package_name(self):
        self.assertEqual(self.sig("func(p *T) {}", "shapes"), "func(*shapes.T)")

    def test_literal_of_a_line(self):
        src = G.GoSource(
            "package main\n\n"
            "func main() {\n"
            "\ta := func() {}\n"
            "\tb := func(n int) int {\n"
            "\t\treturn n\n"
            "\t}\n"
            "\t_, _ = a, b\n"
            "}\n"
        )
        self.assertEqual(src.literal_for(4, "a").sig, "func()")
        self.assertEqual(src.literal_for(5, "b").sig, "func(int) int")
        self.assertIsNone(src.literal_for(3, "a"))  # (before the first literal)
        self.assertIsNone(src.literal_for(6, "a"))
        self.assertIsNone(src.literal_for(8, "a"))
        self.assertIsNone(src.literal_for(4, "b"))  # (not the variable that the line declares)

    def test_two_literals_in_a_line_match_by_position(self):
        src = G.GoSource(
            "package main\n\n"
            "func main() {\n"
            "\tx, y := func(a int) int { return a }, func(s string) {}\n"
            "\t_, _ = x, y\n"
            "}\n"
        )
        self.assertEqual(src.literal_for(4, "x").sig, "func(int) int")
        self.assertEqual(src.literal_for(4, "y").sig, "func(string)")

    def test_a_line_that_does_not_say_is_not_guessed(self):
        # three names, two literals; one name, two literals (a literal in a literal is one: its
        # owner's)
        text = (
            "package main\n\n"
            "func main() {\n"
            "\tx, y, z := func() {}, 1, func(n int) {}\n"
            "\tw := f(func() {}, func(n int) {})\n"
            "\tq := func() func() { return func() {} }\n"
            "}\n"
        )
        src = G.GoSource(text)
        for name in ("x", "y", "z"):
            self.assertIsNone(src.literal_for(4, name), name)
        self.assertIsNone(src.literal_for(5, "w"))
        self.assertEqual(src.literal_for(6, "q").sig, "func() func()")


class ClosureType(unittest.TestCase):
    """The closure type of a func literal has no name; only a frame of a .gp file says that an
    unnamed type is one (a C++ program's unnamed structs are not closures)."""

    class Field:
        def __init__(self, name, base=False, member=True):
            self.name = name
            self.is_base_class = base
            if member:
                self.bitpos = 0

    class Type:
        def __init__(self, text, fields=(), code=3, tag=None, name=None):
            self.text = text
            self._fields = list(fields)
            self.code = code
            self.tag = tag
            self.name = name

        def fields(self):
            return self._fields

        def __str__(self):
            return self.text

    def check(self, t, in_go):
        old = G._in_go_frame
        G._in_go_frame = lambda: in_go
        try:
            return G.is_closure_type(t)
        finally:
            G._in_go_frame = old

    def test_unnamed_struct_in_a_go_frame(self):
        F, T = self.Field, self.Type
        self.assertTrue(self.check(T("struct {...}"), True))  # no captures
        self.assertTrue(self.check(T("struct {...}", [F("__k"), F("__err")]), True))
        self.assertTrue(self.check(T("class {...}", [F("count")]), True))  # clang++

    def test_the_same_types_outside_a_go_frame(self):
        F, T = self.Field, self.Type
        self.assertFalse(self.check(T("struct {...}"), False))
        self.assertFalse(self.check(T("struct {...}", [F("__k")]), False))
        self.assertFalse(self.check(T("class {...}", [F("count")]), False))

    def test_other_types(self):
        F, T = self.Field, self.Type
        self.assertFalse(self.check(T("struct {...}", [F("x")]), True))  # g++: captures are __x
        self.assertFalse(self.check(T("struct {...}", [F("gp::string", base=True)]), True))
        self.assertFalse(self.check(T("gp_anon_1", tag="gp_anon_1"), True))
        self.assertFalse(self.check(T("gpkg_main::T", tag="gpkg_main::T"), True))
        self.assertFalse(self.check(T("int", code=8), True))


class PanicStops(unittest.TestCase):
    """gp catch panic selects the Go frame at a stop that it made: its flag is set (the condition
    of one of its stops was true) and one of its stops is in the event's list."""

    class Frame:
        def __init__(self):
            self.selected = False

        def select(self):
            self.selected = True

    def setUp(self):
        self.state = G._panic_state()
        self.state["numbers"][:] = [3, 4]
        self.state["pending"] = False
        self.runtime_frame = self.Frame()
        self.go_frame = self.Frame()
        self.saved_go_frame_above = G.go_frame_above
        G.go_frame_above = lambda frame: self.go_frame
        G.gdb.selected_frame = lambda: self.runtime_frame
        G.gdb.execute = lambda *args, **kwargs: None

    def tearDown(self):
        G.go_frame_above = self.saved_go_frame_above
        del G.gdb.selected_frame
        del G.gdb.execute
        self.state["numbers"][:] = []
        self.state["pending"] = False

    def stop(self, *numbers):
        event = G.gdb.BreakpointEvent()
        event.breakpoints = [types.SimpleNamespace(number=n) for n in numbers]
        G._select_go_frame(event)

    def test_a_stop_of_gp_catch_panic_selects_the_go_frame(self):
        self.state["pending"] = True
        self.stop(3)
        self.assertTrue(self.go_frame.selected)

    def test_a_user_breakpoint_at_the_same_place_does_not_stop_it(self):
        self.state["pending"] = True
        self.stop(1, 3)
        self.assertTrue(self.go_frame.selected)

    def test_a_stop_of_the_users_only_keeps_its_frame(self):
        self.state["pending"] = False  # (the condition of gp catch panic's was false there)
        self.stop(1, 3)
        self.assertFalse(self.go_frame.selected)

    def test_a_flag_left_by_a_hit_that_did_not_stop_is_not_enough(self):
        # an ignore count or commands that continue: the condition was true, the stop never came
        self.state["pending"] = True
        self.stop(1)
        self.assertFalse(self.go_frame.selected)

    def test_the_flag_is_used_once(self):
        self.state["pending"] = True
        self.stop(1)
        self.stop(3)
        self.assertFalse(self.go_frame.selected)

    def test_other_events_do_nothing(self):
        self.state["pending"] = True
        G._select_go_frame(object())
        self.assertFalse(self.go_frame.selected)

    def test_a_second_copy_of_the_script_shares_the_state(self):
        # auto-load and a manual source: two copies, one gdb module
        path = os.path.join(HERE, "gocpp-gdb.py")
        spec = importlib.util.spec_from_file_location("gocpp_gdb2", path)
        other = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(other)
        self.assertIs(other.gdb, G.gdb)
        self.assertIs(other._panic_state(), self.state)
        other._panic_state()["pending"] = True
        self.stop(4)
        self.assertTrue(self.go_frame.selected)


if __name__ == "__main__":
    unittest.main()
