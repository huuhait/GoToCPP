# gocpp-gdb.py: GDB support for gocpp programs (Phase 13, step 13.2).
#
# What it gives you:
#   - pretty printers for the runtime's values: gp::string, gp::slice, gp::array, gp::map,
#     gp::ptr (and cptr, weak_ptr), gp::iface and every interface struct (gp::any, gp::error),
#     gp::chan, gp::func, named types over them (gp::newtype), and the structs that gocpp
#     writes for Go structs (printed as {X:1 Y:2}, with their Go field names);
#   - Go names in backtraces (main.f, main.(*T).Inc, main.main.func1) through a frame filter
#     that reads the .gp source, and the commands "bt" (as before), "gp bt";
#   - "gp goroutines" and "info goroutines": one line for each goroutine (thread), with its id,
#     its state and its top Go frame; "gp locals", "gp print", "gp whatis", "gp skip-runtime".
#
# It is loaded by the .debug_gdb_scripts section of every gocpp program (runtime/src/
# debug_scripts.cpp embeds this file), or by hand: "source gocpp-gdb.py". Everything is read
# from memory: the printers never run code in the program, so they work on a core file, and
# in optimized builds they print what the debug info still has (<optimized out> for the rest).
#
# Settings:  set gocpp print-style go|tree   go (default): one line of Go text, as the README
#                                            shows; tree: a short summary and children, for
#                                            IDEs that show a variable tree (VS Code).
#            set gocpp names go|cpp          go (default): Go names in backtraces.
#
# The part before "gdb glue" doesn't use gdb: runtime/debug/test_gocpp_gdb.py tests it.

import os
import re
import stat
import struct

import gdb

# ---------------------------------------------------------------------------------------------
# Names: C++ type names to Go type names, namespaces to packages.
# ---------------------------------------------------------------------------------------------

GO_BASIC = {
    "bool": "bool",
    "char": "int8",
    "signed char": "int8",
    "unsigned char": "uint8",
    "short": "int16",
    "unsigned short": "uint16",
    "int": "int32",
    "unsigned int": "uint32",
    # gocpp's int and int64 are both int64_t (long): the debug info can't tell them apart.
    "long": "int",
    "unsigned long": "uint",
    "long long": "int64",
    "unsigned long long": "uint64",
    "float": "float32",
    "double": "float64",
    "int8_t": "int8",
    "uint8_t": "uint8",
    "int16_t": "int16",
    "uint16_t": "uint16",
    "int32_t": "int32",
    "uint32_t": "uint32",
    "int64_t": "int",
    "uint64_t": "uint",
    "intptr_t": "int",
    "uintptr_t": "uintptr",
    "std::uintptr_t": "uintptr",
    "void": "",
    "gp::string": "string",
    "gp::any": "any",
    "gp::error": "error",
    "gp::complex64": "complex64",
    "gp::complex128": "complex128",
    "gp::complex<float>": "complex64",
    "gp::complex<double>": "complex128",
}

# The C++ words that gocpp escapes with a trailing "_" (docs/spec/cpp-mapping.md, "Names inside a
# package"): a field named new_ is the Go field new. (The macros of the C headers, EOF_ and the
# like, are not listed: they are rare as field names.)
CXX_KEYWORDS = frozenset(
    """alignas alignof and and_eq asm auto bitand bitor bool break case catch char char8_t
    char16_t char32_t class compl concept const consteval constexpr constinit const_cast
    continue co_await co_return co_yield decltype default delete do double dynamic_cast else
    enum explicit export extern false float for friend goto if inline int long mutable
    namespace new noexcept not not_eq nullptr operator or or_eq private protected public
    register reinterpret_cast requires return short signed sizeof static static_assert
    static_cast struct switch template this thread_local throw true try typedef typeid
    typename union unsigned using virtual void volatile wchar_t while xor xor_eq""".split()
)

NS_CODES = {"_": "/", "d": ".", "h": "-", "u": "_", "t": "~", "p": "+"}


def unescape_namespace(ns):
    """The import path of the C++ namespace ns (gpkg_ and the escaped path), or None."""
    if not ns.startswith("gpkg_"):
        return None
    s = ns[5:]
    out = []
    i = 0
    while i < len(s):
        c = s[i]
        if c != "_":
            out.append(c)
            i += 1
            continue
        n = s[i + 1 : i + 2]
        if n in NS_CODES:
            out.append(NS_CODES[n])
            i += 2
        elif n == "x" and re.fullmatch(r"[0-9a-fA-F]{2}", s[i + 2 : i + 4]):
            out.append(chr(int(s[i + 2 : i + 4], 16)))
            i += 4
        else:
            return None
    return "".join(out)


def package_name(ns):
    """The name that a package shows in Go names: main, or the last element of the import path
    (the package may have chosen another name; the debug info doesn't say)."""
    if ns == "gpkg_main":
        return "main"
    path = unescape_namespace(ns)
    if path is None:
        return ns
    return path.rsplit("/", 1)[-1]


def go_field_name(name):
    """The Go name of a C++ field name."""
    if name.endswith("_") and name[:-1] in CXX_KEYWORDS:
        return name[:-1]
    return name


def split_args(s):
    """Splits the text of a template argument list at the commas outside <>, () and []."""
    args = []
    depth = 0
    cur = []
    for c in s:
        if c in "<([":
            depth += 1
        elif c in ">)]":
            depth -= 1
        if c == "," and depth == 0:
            args.append("".join(cur).strip())
            cur = []
        else:
            cur.append(c)
    last = "".join(cur).strip()
    if last or args:
        args.append(last)
    return args


def _angle(name, prefix):
    """The argument text of prefix<...> if name has the form, else None."""
    if name.startswith(prefix + "<") and name.endswith(">"):
        return name[len(prefix) + 1 : -1]
    return None


def go_type(name):
    """The Go spelling of the C++ type that gdb names name: gp::slice<long> is []int,
    gp::map<gp::string, long> is map[string]int, gpkg_main::Point is main.Point."""
    name = name.strip()
    name = re.sub(r"^(const|volatile)\s+", "", name)
    name = re.sub(r"\s*(const|volatile)$", "", name)
    if name.endswith("&"):
        return go_type(name[:-1])
    if name in GO_BASIC:
        return GO_BASIC[name]
    for prefix, fmt in (("gp::slice", "[]%s"), ("gp::ptr", "*%s"), ("gp::cptr", "*%s"), ("gp::weak_ptr", "weak.Pointer[%s]")):
        a = _angle(name, prefix)
        if a is not None:
            return fmt % go_type(a)
    a = _angle(name, "gp::chan")
    if a is not None:
        return "chan " + go_type(a)
    a = _angle(name, "gp::array")
    if a is not None:
        parts = split_args(a)
        if len(parts) == 2:
            return "[%s]%s" % (parts[1].rstrip("uUlL"), go_type(parts[0]))  # clang writes 2l
    a = _angle(name, "gp::map")
    if a is not None:
        parts = split_args(a)
        if len(parts) == 2:
            return "map[%s]%s" % (go_type(parts[0]), go_type(parts[1]))
    a = _angle(name, "gp::func")
    if a is not None:
        return go_func_type(a)
    m = re.fullmatch(r"(gpkg_\w+)::(\w+)", name)
    if m:
        return "%s.%s" % (package_name(m.group(1)), m.group(2))
    if name.startswith("gp_anon_i"):
        return "interface{...}"
    if name.startswith("gp_anon_"):
        return "struct{...}"
    m = re.fullmatch(r"(.*)\s*\*", name)
    if m:
        return "*" + go_type(m.group(1))
    return name


def go_func_type(sig):
    """The Go type of the C++ signature R(A, B) (the argument of gp::func)."""
    sig = sig.strip()
    # Find the parameter list: the last top-level "(" ... ")" .
    depth = 0
    close = len(sig) - 1
    while close >= 0 and sig[close] == " ":
        close -= 1
    if close < 0 or sig[close] != ")":
        return "func(...)"
    i = close
    while i >= 0:
        if sig[i] == ")":
            depth += 1
        elif sig[i] == "(":
            depth -= 1
            if depth == 0:
                break
        i -= 1
    if i < 0:
        return "func(...)"
    ret = sig[:i].strip()
    params = [go_type(p) for p in split_args(sig[i + 1 : close]) if p]
    text = "func(%s)" % ", ".join(params)
    if ret and ret != "void":
        tup = _angle(ret, "std::tuple")
        if tup is not None:
            text += " (%s)" % ", ".join(go_type(r) for r in split_args(tup))
        else:
            text += " " + go_type(ret)
    return text


# ---------------------------------------------------------------------------------------------
# Go text: strings and numbers.
# ---------------------------------------------------------------------------------------------


def quote_go(data, limit=None, ascii_only=False):
    """data (bytes) as a Go string literal, like strconv.Quote: with limit, only the first
    limit bytes, and "..." after the closing quote when there are more. Bytes that aren't UTF-8
    come out as \\xNN."""
    cut = limit is not None and len(data) > limit
    if cut:
        data = data[:limit]
    text = data.decode("utf-8", errors="surrogateescape")
    out = ['"']
    for ch in text:
        o = ord(ch)
        if ch == '"':
            out.append('\\"')
        elif ch == "\\":
            out.append("\\\\")
        elif ch == "\n":
            out.append("\\n")
        elif ch == "\t":
            out.append("\\t")
        elif ch == "\r":
            out.append("\\r")
        elif ch == "\a":
            out.append("\\a")
        elif ch == "\b":
            out.append("\\b")
        elif ch == "\f":
            out.append("\\f")
        elif ch == "\v":
            out.append("\\v")
        elif 0xDC80 <= o <= 0xDCFF:
            out.append("\\x%02x" % (o - 0xDC00))
        elif o < 0x20 or o == 0x7F:
            out.append("\\x%02x" % o)
        elif o < 0x7F:
            out.append(ch)
        elif ascii_only or not ch.isprintable():
            out.append("\\u%04x" % o if o <= 0xFFFF else "\\U%08x" % o)
        else:
            out.append(ch)
    out.append('"')
    return "".join(out) + ("..." if cut else "")


def go_float(x, size=8):
    """A float in Go's %v form: the shortest text that reads back to the same value."""
    if x != x:
        return "NaN"
    if x in (float("inf"), float("-inf")):
        return "+Inf" if x > 0 else "-Inf"
    if size == 4:
        for p in range(1, 10):
            s = "%.*g" % (p, x)
            if struct.unpack("f", struct.pack("f", float(s)))[0] == x:
                break
    else:
        s = repr(x)
    if s.endswith(".0"):
        s = s[:-2]
    m = re.fullmatch(r"(-?[0-9.]+)e([+-])0*([0-9]+)", s)
    if m:  # Go writes at least two exponent digits: 1e+06, 1e-07
        s = "%se%s%02d" % (m.group(1), m.group(2), int(m.group(3)))
    return s


# ---------------------------------------------------------------------------------------------
# The Go source: names of the functions that the frames belong to.
#
# gocpp writes #line directives, so a frame of the program says "main.gp:22", but g++ names a
# func literal's frame "operator()<>(void)", and no compiler says that main.(*T).Inc was a
# method of *T. The .gp file does. GoSource finds, for every line of the file, the Go function
# that holds it: main.f, main.T.M, main.(*T).M, main.f.func1, main.f.func1.2 (nested literals).
# ---------------------------------------------------------------------------------------------

_TOKEN_RX = re.compile(
    r"""
      (?P<nl>\n)
    | (?P<space>[ \t\r\f\v]+)
    | (?P<lcomment>//[^\n]*)
    | (?P<bcomment>/\*.*?\*/)
    | (?P<raw>`[^`]*`)
    | (?P<str>"(?:\\.|[^"\\\n])*")
    | (?P<rune>'(?:\\.|[^'\\\n])*')
    | (?P<ident>[^\W\d]\w*)
    | (?P<num>\d[\w.]*)
    | (?P<op>\.\.\.|<-|[{}()\[\],;.*&|^<>=+\-!~%/:?@#$])
    """,
    re.X | re.S,
)

# After these tokens a newline ends a statement (Go's automatic semicolons).
_SEMI_AFTER = re.compile(r"^(?:[^\W\d]\w*|\d[\w.]*|\"|`|'|\)|\]|\})")


def go_tokens(text):
    """(kind, text, line) for the tokens of Go source: no comments or white space. A malformed
    rest of the file ends the list."""
    toks = []
    line = 1
    pos = 0
    while pos < len(text):
        m = _TOKEN_RX.match(text, pos)
        if not m:
            pos += 1
            continue
        pos = m.end()
        kind = m.lastgroup
        s = m.group()
        if kind == "nl":
            toks.append(("nl", s, line))
            line += 1
            continue
        if kind in ("space", "lcomment"):
            continue
        if kind == "bcomment":
            line += s.count("\n")
            continue
        toks.append((kind, s, line))
        line += s.count("\n")  # a raw string can span lines
    return toks


class GoFunc:
    """A function of a .gp file: a declaration or a func literal."""

    __slots__ = ("name", "start", "end", "literal", "parent")

    def __init__(self, name, start, literal=False, parent=None):
        self.name = name  # without the package: f, T.M, (*T).M, f.func1, f.func1.1
        self.start = start  # the line of the "func" keyword
        self.end = start  # the line of the closing brace of the body
        self.literal = literal
        self.parent = parent

    @property
    def plain_name(self):
        """The name that the C++ name has: T.M for (*T).M."""
        return self.name.replace("(*", "").replace(")", "")


class GoSource:
    """The functions of one .gp file, found by scanning its tokens: the .gp file knows what the
    debug info doesn't (that main.(*T).Inc has a pointer receiver; which func literal a frame
    is)."""

    def __init__(self, text):
        self.package = "main"
        self.funcs = []  # every function and literal, in source order
        self.lines = text.split("\n")  # for line_text
        self._scan(self._statements(go_tokens(text)))

    @staticmethod
    def _statements(toks):
        """The tokens with Go's automatic semicolons: a newline after an identifier, a literal,
        or ) ] } ends a statement ("semi" token); the other newlines are dropped."""
        out = []
        prev = None
        for t in toks:
            if t[0] == "nl":
                if prev is not None and (prev[0] in ("ident", "num", "str", "raw", "rune") or prev[1] in (")", "]", "}")):
                    out.append(("semi", "\n", t[2]))
                prev = None
                continue
            out.append(t)
            prev = t
        return out

    def _scan(self, toks):
        n = len(toks)
        for i in range(n - 1):
            if toks[i][1] == "package" and toks[i][0] == "ident" and toks[i + 1][0] == "ident":
                self.package = toks[i + 1][1]
                break
        depth = 0
        stack = []  # (GoFunc, depth inside its body)
        counts = {}  # id(GoFunc) -> the number of literals in it so far
        top_literals = 0
        i = 0
        while i < n:
            kind, s, line = toks[i]
            if kind == "op" and s == "{":
                depth += 1
            elif kind == "op" and s == "}":
                depth -= 1
                while stack and depth < stack[-1][1]:
                    stack.pop()[0].end = line
            elif kind == "ident" and s == "func":
                j = i + 1
                at_start = i == 0 or toks[i - 1][0] == "semi"  # not "var f = func() {...}"
                if at_start and depth == 0 and not stack and j < n and (toks[j][1] == "(" or toks[j][0] == "ident"):
                    # A declaration: func Name(...) {...} or func (recv) Name(...) {...}.
                    recv = None
                    if toks[j][1] == "(":
                        k = self._close(toks, j)
                        if k is None:
                            i += 1
                            continue
                        recv = self._receiver(toks[j + 1 : k])
                        j = k + 1
                    if j < n and toks[j][0] == "ident":
                        name = toks[j][1] if recv is None else "%s.%s" % (recv, toks[j][1])
                        f = GoFunc(name, line)
                        self.funcs.append(f)
                        b = self._body(toks, j + 1, False)
                        if b is not None:
                            depth += 1
                            stack.append((f, depth))
                            i = b + 1
                            continue
                elif j < n and toks[j][1] == "(":
                    # A func literal (in a function, or in the initializer of a package variable)
                    # or a func type.
                    k = self._close(toks, j)
                    b = self._body(toks, k + 1, True) if k is not None else None
                    if b is not None:
                        if stack:
                            owner = stack[-1][0]
                            counts[id(owner)] = counts.get(id(owner), 0) + 1
                            f = GoFunc(self._lit_name(owner, counts[id(owner)]), line, True, owner)
                        else:
                            top_literals += 1
                            f = GoFunc("init.func%d" % top_literals, line, True)
                        self.funcs.append(f)
                        depth += 1
                        stack.append((f, depth))
                        i = b + 1
                        continue
            i += 1

    @staticmethod
    def _lit_name(owner, c):
        # Go's names: a literal in f is f.func1; a literal in that one is f.func1.1.
        if owner.literal:
            return "%s.%d" % (owner.name, c)
        return "%s.func%d" % (owner.name, c)

    @staticmethod
    def _close(toks, i):
        """The index of the token that closes the bracket at toks[i], or None."""
        open_ = toks[i][1]
        close = {"(": ")", "[": "]", "{": "}"}[open_]
        depth = 0
        for k in range(i, len(toks)):
            if toks[k][0] == "op":
                if toks[k][1] == open_:
                    depth += 1
                elif toks[k][1] == close:
                    depth -= 1
                    if depth == 0:
                        return k
        return None

    @staticmethod
    def _receiver(toks):
        """The receiver of a method as Go names it in a function name: T or (*T). toks are the
        tokens between the parentheses: (t *T), (T), (t T[K])."""
        ids = [t for t in toks if t[0] != "semi"]
        star = any(t[1] == "*" for t in ids)
        name = None
        for k, t in enumerate(ids):
            if t[1] == "[" and k > 0 and ids[k - 1][0] == "ident":
                name = ids[k - 1][1]  # T[K]: the type is before the "["
                break
        if name is None:
            for t in reversed(ids):
                if t[0] == "ident":
                    name = t[1]
                    break
        if name is None:
            return None
        return "(*%s)" % name if star else name

    def _body(self, toks, i, literal):
        """The index of the "{" that starts a body after the parameters (the signature starts
        at toks[i]), or None when there is none: a func type, or a declaration without a body.
        A struct or interface type in the signature has braces of its own."""
        n = len(toks)
        depth = 0
        prev = None
        while i < n:
            t = toks[i]
            if t[0] == "semi" and depth == 0:
                return None
            if t[1] in ("(", "["):
                depth += 1
            elif t[1] in (")", "]"):
                depth -= 1
                if depth < 0:
                    return None
            elif literal and depth == 0 and t[1] in (",", "=", ":", ";", "}"):
                return None
            elif t[1] == "{" and depth == 0:
                if prev is not None and prev[1] in ("struct", "interface"):
                    i = self._close(toks, i) or i
                else:
                    return i
            prev = t
            i += 1
        return None

    def line_text(self, line):
        """The text of the 1-based line, or ""."""
        return self.lines[line - 1] if 1 <= line <= len(self.lines) else ""

    def declared(self, plain):
        """The top-level function whose name is plain (T.M for (*T).M), or None."""
        for f in self.funcs:
            if not f.literal and f.plain_name == plain:
                return f
        return None

    def literal_at(self, line, start=0):
        """The innermost func literal that holds line; with start (the line where the debug info
        says the literal begins), the literals that begin there come first."""
        best = None
        for f in self.funcs:
            if not f.literal or not (f.start <= line <= f.end):
                continue
            if best is None:
                best = f
                continue
            # Inner literals start later or end sooner than their owners.
            bs, fs = best.start == start, f.start == start
            if fs != bs:
                if fs:
                    best = f
                continue
            if (f.start, -f.end) >= (best.start, -best.end):
                best = f
        return best

    def name_at(self, line):
        """The Go name, without the package, of the innermost function that holds line, or None."""
        inner = self.literal_at(line)
        if inner is not None:
            return inner.name
        for f in self.funcs:
            if not f.literal and f.start <= line <= f.end:
                return f.name
        return None


_sources = {}


MAX_SOURCE = 4 << 20  # the largest .gp file that is read (the debug info names the file, not us)


def go_source(path):
    """The GoSource of the file at path (cached by modification time), or None: a file that is
    not a regular .gp file of at most MAX_SOURCE bytes is not read."""
    if not path.endswith(".gp"):
        return None
    try:
        st = os.stat(path)
    except OSError:
        return None
    if not stat.S_ISREG(st.st_mode) or st.st_size > MAX_SOURCE:
        return None
    key = (st.st_mtime_ns, st.st_size)
    hit = _sources.get(path)
    if hit is not None and hit[0] == key:
        return hit[1]
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            src = GoSource(f.read(MAX_SOURCE))
    except OSError:
        return None
    _sources[path] = (key, src)
    return src


# The C++ names of the functions that gocpp writes.
_LAMBDA_CALLS = re.compile(r"operator\(\)<(\d+)>")
_GLUE = re.compile(
    r"^(gp::(func<.*>::(operator\(\)|gp_call_\w+)"
    r"|detail::(call_deferred|gp_call_\w+)\b"
    # the run of a deferred call, and of a goroutine's function, down to the thread
    r"|defer_frame::(gp_run|gp_node_of<.*>::gp_do)\b"
    r"|detail::call_task<.*>::run\b"
    r"|detail::\(anonymous namespace\)::(run_goroutine|worker_main)\b"
    r"|os::\(anonymous namespace\)::thread_main\b)"
    # how libstdc++ calls a callable, and how the thread starts (libc)
    r"|std::(__invoke_impl|__invoke|__apply_impl|apply)<"
    r"|(start_thread|__?clone3?|__GI___clone3)(\+\d+)?$)"
)


def is_lambda_name(name):
    """Whether the C++ name is a func literal's: g++ names it operator()<>, clang++ names it
    after the function that holds it (gpkg_main::f()::$_0::operator()<1>())."""
    return name.startswith(("operator()", "<lambda>")) or (name.startswith("gpkg_") and "operator()" in name)


def is_glue_name(name):
    """Whether the C++ name is a helper between two Go frames, or below the Go frames of a thread,
    which a Go program doesn't have: the call of a gp::func (gp::func<...>::operator() and its
    gp_call_obj), of a deferred call and of a goroutine's function (std::apply, std::__invoke,
    defer_frame::gp_run, call_task::run), and the thread's start (worker_main, start_thread,
    clone3)."""
    return bool(_GLUE.match(name))


def demangle_cxx(name):
    """The Go name of the C++ function name (as gdb shows a frame's function), without the
    debug info's and the source's help, or None when it isn't one of gocpp's own. A func
    literal is main.f.func1 only when the name says so (clang++)."""
    m = re.match(r"^(gpkg_\w+)::(.*)$", name)
    if not m:
        return None
    pkg = package_name(m.group(1))
    rest = m.group(2)
    if "operator()" in rest:
        fn = re.match(r"(\w+)\(\)", rest)
        nums = _LAMBDA_CALLS.findall(rest)
        if fn and nums:
            return "%s.%s.func%s%s" % (pkg, fn.group(1), nums[0], "".join("." + x for x in nums[1:]))
        return None
    rest = re.sub(r"[(<].*$", "", rest)
    inst = re.match(r"gp_inst_[0-9a-f]+_(\w+)$", rest)
    if inst:
        return "%s.%s[...]" % (pkg, inst.group(1))
    if rest == "gp_var_init":
        return pkg + ".init"
    m2 = re.fullmatch(r"gp_init_(\d+)", rest)
    if m2:
        return "%s.init.%d" % (pkg, int(m2.group(1)) - 1)
    return "%s.%s" % (pkg, rest.replace("::", "."))


def go_name_of(name, src, decl_line, line):
    """The Go name of the function that the C++ name, its declaration line (0 when the debug
    info has none: g++ for a lambda) and the .gp line of its frame belong to, using the .gp
    source src (a GoSource or None) for what the names don't tell."""
    pkg = src.package if src is not None else None
    if is_lambda_name(name):
        if src is not None:
            lit = src.literal_at(line, decl_line)
            if lit is not None:
                return "%s.%s" % (pkg, lit.name)
        return demangle_cxx(name)
    n = demangle_cxx(name)
    if n is None or src is None:
        return n
    m = re.match(r"^(gpkg_\w+)::(.*)$", name)
    rest = re.sub(r"[(<].*$", "", m.group(2))
    inst = re.match(r"gp_inst_[0-9a-f]+_(\w+)$", rest)
    key = (inst.group(1) if inst else rest).replace("::", ".")
    f = src.declared(key)
    if f is None:
        return n
    return "%s.%s%s" % (pkg, f.name, "[...]" if inst else "")


_DECL = re.compile(r"^\s*(?:(?:if|for|switch)\s+)?(?:var\s+(?P<v>[A-Za-z_]\w*)\s*(?P<t>[^=]*?)\s*(?:=\s*(?P<ve>.*))?|(?P<n>[A-Za-z_]\w*)\s*:=\s*(?P<e>.*))$")
_BOX_START = re.compile(r"""^(?:[0-9.+-]|["'`]|true\b|false\b|\[|map\[|chan\b|func\b|struct\b|make\(|(?:\w+\.)?\w+\s*\{)""")


def classify_decl(name, text):
    """What the declaration of the local name in the .gp line text says about the C++ variable
    that holds it. A variable that escapes (a closure captures it, or its address is taken) is
    a gp::ptr box on the heap in the C++, and its debug-info type is *T for the Go type T.
    Returns "box" (the Go type is not a pointer: the variable is the box's target), "ptr?" (a
    pointer to a new value, new(T), &T{} or var p *T: it is a box only when its target is a
    pointer too) or None (the line doesn't say: leave the variable as it is)."""
    m = _DECL.match(text.split("//", 1)[0].rstrip())
    if m is None:
        return None
    if m.group("v") is not None:
        if m.group("v") != name:
            return None
        typ = (m.group("t") or "").strip()
        if typ:
            return "ptr?" if typ.startswith("*") else "box"
        expr = m.group("ve") or ""
    else:
        if m.group("n") != name:
            return None
        expr = m.group("e") or ""
    expr = expr.strip()
    if expr.startswith("&"):
        return "ptr?" if re.match(r"&\s*(?:\w+\.)?\w+\s*\{", expr) else None
    if expr.startswith("new("):
        return "ptr?"
    if _BOX_START.match(expr):
        return "box"
    return None


def find_decl_kind(src, name, line, up=400):
    """classify_decl for the declaration of name that the debug info doesn't place (a func
    literal's capture has line 0): the nearest line at or above line that declares it."""
    for n in range(line, max(line - up, 0), -1):
        text = src.line_text(n)
        m = _DECL.match(text.split("//", 1)[0].rstrip())
        if m is not None and (m.group("v") or m.group("n")) == name:
            return classify_decl(name, text)
    return None


# How many elements one command prints in all: print elements is the limit of a list; the
# nested lists of a [][]int share a budget of a few times that.
_budget = [1 << 30]


def reset_budget():
    lim = elem_limit()
    _budget[0] = lim * 4 if lim < (1 << 20) else 1 << 30


# ---------------------------------------------------------------------------------------------
# gdb glue
# ---------------------------------------------------------------------------------------------

STUB = getattr(gdb, "GOCPP_STUB", False)

KIND_BASE = {
    "Bool": ("?", 1),
    "Int": ("<q", 8),
    "Int8": ("<b", 1),
    "Int16": ("<h", 2),
    "Int32": ("<i", 4),
    "Int64": ("<q", 8),
    "Uint": ("<Q", 8),
    "Uint8": ("<B", 1),
    "Uint16": ("<H", 2),
    "Uint32": ("<I", 4),
    "Uint64": ("<Q", 8),
    "Uintptr": ("<Q", 8),
    "Float32": ("<f", 4),
    "Float64": ("<d", 8),
}

MAX_DEPTH = 12


def _param(name, default):
    try:
        v = gdb.parameter(name)
    except Exception:
        return default
    return default if v is None else v


def elem_limit():
    """gdb's "print elements": how many elements and bytes a printer shows (200 by default)."""
    v = _param("print elements", 200)
    return 1 << 30 if v in (0, -1) else int(v)


def host_ascii():
    try:
        cs = (gdb.host_charset() or "").upper()
    except Exception:
        return False
    return cs in ("ANSI_X3.4-1968", "ASCII", "US-ASCII", "")


def print_style():
    return str(_param("gocpp print-style", "go"))


def unwrap(t):
    """The type with typedefs, qualifiers and references stripped."""
    t = t.strip_typedefs()
    while t.code in (gdb.TYPE_CODE_REF, gdb.TYPE_CODE_RVALUE_REF):
        t = t.target().strip_typedefs()
    return t.unqualified()


def type_tag(t):
    return t.tag or t.name or str(t)


_kind_cache = {}


def classify(t):
    """What the runtime or the emitter makes of the struct type t: "string", "slice", "array",
    "map", "ptr", "weak", "chan", "func", "iface", "newtype", "struct", or None."""
    tag = type_tag(t)
    hit = _kind_cache.get(tag)
    if hit is not None:
        return hit or None
    kind = None
    if tag == "gp::string":
        kind = "string"
    elif tag.startswith("gp::slice<"):
        kind = "slice"
    elif tag.startswith("gp::array<"):
        kind = "array"
    elif tag.startswith("gp::map<"):
        kind = "map"
    elif tag.startswith("gp::ptr<") or tag.startswith("gp::cptr<"):
        kind = "ptr"
    elif tag.startswith("gp::weak_ptr<"):
        kind = "weak"
    elif tag.startswith("gp::chan<"):
        kind = "chan"
    elif tag.startswith("gp::func<"):
        kind = "func"
    elif tag.startswith("gp::complex<"):
        kind = "complex"
    elif tag == "gp::iface" or _has_base(t, "gp::iface"):
        kind = "iface"
    elif tag.startswith("gp::newtype<") or _has_base_prefix(t, "gp::newtype<"):
        kind = "newtype"
    elif re.match(r"^gpkg_\w+::\w+$", tag) or tag.startswith("gp_anon_"):
        kind = "struct"
    _kind_cache[tag] = kind or ""
    return kind


def _bases(t):
    for f in t.fields():
        if f.is_base_class:
            yield f


def _has_base(t, tag):
    for f in _bases(t):
        bt = f.type.strip_typedefs()
        if type_tag(bt) == tag or _has_base(bt, tag):
            return True
    return False


def _has_base_prefix(t, prefix):
    for f in _bases(t):
        bt = f.type.strip_typedefs()
        if type_tag(bt).startswith(prefix) or _has_base_prefix(bt, prefix):
            return True
    return False


def read_bytes(addr, n):
    return bytes(gdb.selected_inferior().read_memory(addr, n))


NAME_CAP = 512  # the longest type name that a descriptor's name is read as


def c_string(addr, cap=NAME_CAP):
    """The text of the NUL-terminated string at addr: at most cap bytes, invalid UTF-8 replaced
    (the memory may be garbage: a descriptor that is not one yet)."""
    out = bytearray()
    while len(out) < cap:
        n = min(64 - (addr + len(out)) % 64, cap - len(out))  # never across a 64-byte line
        try:
            chunk = read_bytes(addr + len(out), n)
        except gdb.error:
            break
        i = chunk.find(b"\0")
        if i >= 0:
            out += chunk[:i]
            break
        out += chunk
    text = bytes(out).decode("utf-8", errors="replace")
    if host_ascii():  # gdb can't write the replacement character (or a UTF-8 name) in a C locale
        text = text.encode("ascii", "backslashreplace").decode("ascii")
    return text


def shared_ptr_addr(sp):
    """The pointer that a std::shared_ptr holds, as an int."""
    return int(sp["_M_ptr"])


def go_scalar(v):
    """The Go text of a C++ scalar value, or None when v isn't one."""
    t = unwrap(v.type)
    c = t.code
    if c == gdb.TYPE_CODE_BOOL:
        return "true" if int(v) else "false"
    if c in (gdb.TYPE_CODE_INT, gdb.TYPE_CODE_CHAR):
        return str(int(v))
    if c == gdb.TYPE_CODE_FLT:
        return go_float(float(v), t.sizeof)
    if c == gdb.TYPE_CODE_ENUM:
        return str(v)
    return None


def fmt(v, depth=0):
    """The Go text of the value v. depth is 0 for the value gdb prints, and grows for the parts
    in it: only the top of a value shows the address of a pointer's target, a slice's length
    and capacity and an interface's dynamic type, as Go's %+v does for the parts."""
    try:
        if depth == 0:
            reset_budget()
        if v.is_optimized_out:
            return "<optimized out>"
        return _fmt(v, depth)
    except gdb.MemoryError as e:
        return "<unreadable: %s>" % e
    except gdb.error as e:
        return "<error: %s>" % e
    except Exception as e:  # garbage in the program's memory must not abort a command
        return "<unreadable: %s>" % _exc_text(e)


def _exc_text(e):
    return "%s: %s" % (type(e).__name__, e) if str(e) else type(e).__name__


def _fmt(v, depth):
    t = unwrap(v.type)
    if t.code in (gdb.TYPE_CODE_REF, gdb.TYPE_CODE_RVALUE_REF):
        return _fmt(v.referenced_value(), depth)
    if depth > MAX_DEPTH:
        return "..."
    s = go_scalar(v)
    if s is not None:
        return s
    if t.code == gdb.TYPE_CODE_PTR:
        a = int(v)
        if a == 0:
            return "nil"
        tn = type_tag(t.target().strip_typedefs()) if t.target().strip_typedefs().code != gdb.TYPE_CODE_VOID else "void"
        if t.target().strip_typedefs().code == gdb.TYPE_CODE_INT and t.target().sizeof == 1:
            return str(v)  # a C string: gdb's own text
        return "0x%x" % a
    if t.code == gdb.TYPE_CODE_ARRAY:  # a C array: a plain list
        lo, hi = t.range()
        return _fmt_list([v[i] for i in range(lo, min(hi, lo + elem_limit() - 1) + 1)], hi - lo + 1, depth)
    if t.code != gdb.TYPE_CODE_STRUCT:
        return str(v)
    kind = classify(t)
    if kind is None:
        return str(v)
    return KINDS[kind](v, t, depth)


def _fmt_list(elems, total, depth):
    if len(elems) > _budget[0]:  # the nested lists of this command have printed enough
        elems = elems[: max(_budget[0], 0)]
    _budget[0] -= len(elems)
    parts = [fmt(e, depth + 1) for e in elems]
    if total > len(elems):
        parts.append("...")
    return "[" + " ".join(parts) + "]"


def fmt_string(v, t, depth):
    n = int(v["gp_len_"])
    p = int(v["gp_data_"])
    if n <= 0 or p == 0:  # (a negative length is garbage: the string is not initialized)
        return '""'
    lim = elem_limit()
    data = read_bytes(p, min(n, lim))
    text = quote_go(data, None, host_ascii())
    return text + ("..." if n > lim else "")


def slice_parts(v):
    return int(v["gp_len_"]), int(v["gp_cap_"]), v["gp_data_"]


def fmt_slice(v, t, depth):
    n, c, data = slice_parts(v)
    if int(data) == 0:
        body = "[]"
        return "nil" if depth > 0 else "[] (nil)"
    lim = elem_limit()
    body = _fmt_list([data[i] for i in range(min(n, lim))], n, depth)
    if depth == 0:
        body += " (len %d, cap %d)" % (n, c)
    return body


def array_elems(v):
    return v["gp_elems"]["_M_elems"] if v.type.strip_typedefs().fields() else None


def fmt_array(v, t, depth):
    try:
        arr = v["gp_elems"]["_M_elems"]
    except gdb.error:
        return "[]"
    lo, hi = arr.type.strip_typedefs().range()
    n = hi - lo + 1
    return _fmt_list([arr[i] for i in range(lo, min(hi, lo + elem_limit() - 1) + 1)], n, depth)


def map_entries(v, t, limit):
    """(count, [(key, value)...]) of a gp::map: up to limit entries, read from libstdc++'s hash
    table. The entries come in the table's order."""
    impl = v["gp_impl_"]
    addr = shared_ptr_addr(impl)
    if addr == 0:
        return None, []
    table = impl["_M_ptr"].dereference()["gp_t"]["_M_h"]
    count = int(table["_M_element_count"])
    valtype = table.type.strip_typedefs().template_argument(1)
    node = table["_M_before_begin"]["_M_nxt"]
    try:
        align = valtype.alignof
    except (AttributeError, gdb.error):
        align = 8
    off = (8 + align - 1) // align * align
    out = []
    ptype = valtype.pointer()
    while int(node) != 0 and len(out) < limit:
        ent = (node.cast(gdb.lookup_type("char").pointer()) + off).cast(ptype).dereference()
        out.append((ent["first"]["gp_k"], ent["second"]))
        node = node.dereference()["_M_nxt"]
    return count, out


_NUM = re.compile(r"[+-]?(?:\d+\.?\d*(?:[eE][+-]?\d+)?|\.\d+(?:[eE][+-]?\d+)?)")


def _sort_key(text):
    """Go's order of map keys in fmt: numbers by value (integers and floats), the rest by text."""
    if re.fullmatch(r"-?\d+", text):
        return (0, int(text), "")
    if _NUM.fullmatch(text):
        return (0, float(text), "")
    if text == "NaN":
        return (0, float("-inf"), "0")
    if text == "-Inf":
        return (0, float("-inf"), "1")
    if text == "+Inf":
        return (0, float("inf"), "1")
    return (1, 0, text)


def fmt_map(v, t, depth):
    lim = elem_limit()
    try:
        count, ents = map_entries(v, t, lim)
    except gdb.error as e:
        return "map[...] (%s)" % e
    if count is None:
        return "nil" if depth > 0 else "map[] (nil)"
    items = [(fmt(k, depth + 1), fmt(val, depth + 1)) for k, val in ents]
    if count <= lim:
        items.sort(key=lambda kv: _sort_key(kv[0]))
    parts = ["%s:%s" % kv for kv in items]
    if count > len(items):
        parts.append("...")
    text = "map[" + " ".join(parts) + "]"
    if depth == 0:
        text += " (len %d)" % count
    return text


def ptr_target(v):
    """(address, value of the pointee or None) of a gp::ptr."""
    sp = v["gp_sp_"]
    addr = shared_ptr_addr(sp)
    return addr, None if addr == 0 else sp["_M_ptr"].dereference()


def fmt_ptr(v, t, depth):
    addr, target = ptr_target(v)
    if addr == 0:
        return "nil"
    if depth > 0:
        return "0x%x" % addr
    return "&%s @0x%x" % (fmt(target, depth + 1), addr)


def fmt_weak(v, t, depth):
    return "weak(0x%x)" % int(v["gp_addr_"])


def chan_core(v):
    addr = shared_ptr_addr(v["gp_c"])
    return addr, None if addr == 0 else v["gp_c"]["_M_ptr"].dereference()


def queue_len(q):
    n = 0
    sg = q["first"]
    while int(sg) != 0 and n < 100000:
        n += 1
        sg = sg.dereference()["next"]
    return n


def chan_info(v):
    addr, core = chan_core(v)
    if core is None:
        return None
    return {
        "addr": addr,
        "len": int(core["count"]["_M_i"]),
        "cap": int(core["capacity"]),
        "closed": bool(core["closed"]),
        "recvq": queue_len(core["recvq"]),
        "sendq": queue_len(core["sendq"]),
    }


def fmt_chan(v, t, depth):
    info = chan_info(v)
    if info is None:
        return "nil"
    if depth > 0:
        return "0x%x" % info["addr"]
    text = "chan(0x%x len %d cap %d" % (info["addr"], info["len"], info["cap"])
    if info["closed"]:
        text += " closed"
    if info["recvq"]:
        text += " %d receiving" % info["recvq"]
    if info["sendq"]:
        text += " %d sending" % info["sendq"]
    return text + ")"


def symbol_of(ptr_value):
    """The name that gdb shows for an address, "0x... <name+off>" without the address."""
    m = re.match(r"^\S+\s+<(.*)>$", str(ptr_value))
    if not m:
        return None
    return m.group(1)


def func_target(v):
    """What a gp::func calls, from its code pointer's symbol: (text, address)."""
    code = v["gp_call_"]
    a = int(code)
    if a == 0:
        return None, 0
    sym = symbol_of(code) or ""
    # gp_call_static<&gpkg_main::add>: the function itself.
    m = re.search(r"gp_call_static<&?(gpkg_\w+(?:::\w+)+)", sym)
    if m:
        n = demangle_cxx(m.group(1))
        return n or m.group(1), a
    # gp_call_obj<L> / gp_call_empty<L>: a closure; the enclosing function is in L's name.
    m = re.search(r"gp_call_(?:obj|empty)<(gpkg_\w+)::(\w+)\(", sym)
    if m:
        return "closure in %s.%s" % (package_name(m.group(1)), m.group(2)), a
    return None, a


def fmt_func(v, t, depth):
    name, a = func_target(v)
    if a == 0:
        return "nil"
    if depth > 0:
        return "0x%x" % a
    return "func(0x%x%s)" % (a, " " + name if name else "")


DESC_BASIC_KINDS = KIND_BASE


def desc_kind(d):
    return str(d["gp_kind"]).rsplit("::", 1)[-1]


def desc_type(dp):
    """The gdb type of the Go type that the type_desc pointer dp describes, when it is a named
    type of the program (its descriptor is gpkg_X::gp_td_Name), else None."""
    sym = symbol_of(dp)
    if sym:
        m = re.match(r"^(gpkg_\w+)::gp_td_(\w+)", sym)
        if m:
            try:
                return gdb.lookup_type("%s::%s" % (m.group(1), m.group(2)))
            except gdb.error:
                return None
    return None


def iface_value(v):
    """(Go type name, text of the value) of the interface value v, or None for nil."""
    td = v["gp_type_"]
    if int(td) == 0:
        return None
    d = td.dereference()
    name = c_string(int(d["gp_name"]))
    kind = desc_kind(d)
    data = v["gp_data_"]
    addr = shared_ptr_addr(data)
    return name, kind, addr, d, data


def fmt_iface(v, t, depth):
    parts = iface_value(v)
    if parts is None:
        return "nil"
    name, kind, addr, d, data = parts
    text = iface_text(name, kind, addr, d, data, depth)
    if depth > 0:
        return text
    return "(%s) %s" % (name, text)


def iface_text(name, kind, addr, d, data, depth):
    if kind in KIND_BASE:
        code, size = KIND_BASE[kind]
        if addr == 0:
            return "<missing value>"
        raw = read_bytes(addr, size)
        x = struct.unpack(code, raw)[0]
        if kind == "Bool":
            return "true" if x else "false"
        if kind in ("Float32", "Float64"):
            return go_float(x, size)
        return str(x)
    if kind == "Pointer":
        if addr == 0:
            return "nil"
        if depth > 0:
            return "0x%x" % addr
        et = desc_type(d["gp_elem"])
        if et is not None:
            try:
                return "&%s @0x%x" % (fmt(gdb.Value(addr).cast(et.pointer()).dereference(), depth + 1), addr)
            except gdb.error:
                pass
        return "0x%x" % addr
    if kind in ("Chan", "Func", "UnsafePointer"):
        return "0x%x" % addr if addr else "nil"
    # A boxed value: find its C++ type from the box's control block, or from the descriptor.
    ty = box_type(d, data)
    if ty is None or addr == 0:
        return "<%s at 0x%x>" % (name, addr)
    try:
        return fmt(gdb.Value(addr).cast(ty.pointer()).dereference(), depth + 1)
    except gdb.error as e:
        return "<%s: %s>" % (name, e)


def box_type(d, data):
    """The gdb type of the value in an interface's box, or None. A box made by make_shared has
    its type in the control block; the descriptor of a named type of the program names it too."""
    try:
        pi = data["_M_refcount"]["_M_pi"]
        if int(pi) != 0:
            dt = pi.dereference().dynamic_type
            if "_Sp_counted_ptr_inplace" in str(dt):
                return dt.template_argument(0)
    except gdb.error:
        pass
    return desc_type(d)


KIND_TYPES = {
    "Bool": "bool",
    "Int": "long",
    "Int8": "signed char",
    "Int16": "short",
    "Int32": "int",
    "Int64": "long long",
    "Uint": "unsigned long",
    "Uint8": "unsigned char",
    "Uint16": "unsigned short",
    "Uint32": "unsigned int",
    "Uint64": "unsigned long long",
    "Uintptr": "unsigned long",
    "Float32": "float",
    "Float64": "double",
}


def iface_dynamic(v):
    """The value inside the interface value v as a gdb value (the pointee for a pointer), or
    None for nil and for what has no C++ type here."""
    parts = iface_value(v)
    if parts is None:
        return None
    name, kind, addr, d, data = parts
    if addr == 0:
        return None
    try:
        if kind in KIND_TYPES:
            return gdb.Value(addr).cast(gdb.lookup_type(KIND_TYPES[kind]).pointer()).dereference()
        if kind == "Pointer":
            ty = desc_type(d["gp_elem"])
        elif kind in ("Chan", "Func", "UnsafePointer"):
            return None
        else:
            ty = box_type(d, data)
        if ty is None:
            return None
        return gdb.Value(addr).cast(ty.pointer()).dereference()
    except gdb.error:
        return None


def fmt_newtype(v, t, depth):
    inner = newtype_value(v, t)
    if inner is None:
        return str(v)
    return fmt(inner, depth)


def newtype_value(v, t):
    """The underlying value of a named type over a number or a runtime class."""
    try:
        return v["gp_v"]
    except gdb.error:
        pass
    for f in _bases(t):
        bt = f.type.strip_typedefs()
        b = v[f]
        if type_tag(bt).startswith("gp::newtype<"):
            r = newtype_value(b, bt)
            if r is not None:
                return r
        elif classify(bt) is not None:
            return b
    return None


def fmt_complex(v, t, depth):
    re_, im = float(v["gp_re"]), float(v["gp_im"])
    size = v["gp_re"].type.sizeof
    sign = "+" if (im >= 0 or im != im) else ""
    return "(%s%s%si)" % (go_float(re_, size), sign, go_float(im, size))


def struct_fields(v, t):
    """(Go name, value) for the Go fields of a struct."""
    out = []
    for f in t.fields():
        if f.is_base_class or not hasattr(f, "bitpos") or f.name is None:
            continue  # a base class, or a static member such as gp_fields
        out.append((go_field_name(f.name), v[f]))
    return out


def fmt_struct(v, t, depth):
    parts = ["%s:%s" % (n, fmt(x, depth + 1)) for n, x in struct_fields(v, t)]
    return "{" + " ".join(parts) + "}"


KINDS = {
    "string": fmt_string,
    "slice": fmt_slice,
    "array": fmt_array,
    "map": fmt_map,
    "ptr": fmt_ptr,
    "weak": fmt_weak,
    "chan": fmt_chan,
    "func": fmt_func,
    "iface": fmt_iface,
    "newtype": fmt_newtype,
    "complex": fmt_complex,
    "struct": fmt_struct,
}


# ---- the printers: gdb's protocol ----


def tree_summary(v, t, kind):
    """The one-line text of a value in tree style: what it is, without its parts."""
    if kind in ("string", "newtype", "weak", "complex"):
        return fmt(v)
    if kind == "slice":
        n, c, data = slice_parts(v)
        return "nil" if int(data) == 0 else "%s len %d cap %d" % (go_type(type_tag(t)), n, c)
    if kind == "array":
        return go_type(type_tag(t))
    if kind == "map":
        count, _ = map_entries(v, t, 0)
        return "nil" if count is None else "%s len %d" % (go_type(type_tag(t)), count)
    if kind == "ptr":
        addr, _ = ptr_target(v)
        return "nil" if addr == 0 else "(%s) 0x%x" % (go_type(type_tag(t)), addr)
    if kind == "iface":
        parts = iface_value(v)
        return "nil" if parts is None else parts[0]
    if kind == "struct":
        return go_type(type_tag(t))
    return fmt(v)


def tree_children(v, t, kind):
    """The (name, value) pairs of a value in tree style."""
    lim = elem_limit()
    if kind == "struct":
        return struct_fields(v, t)
    if kind == "slice":
        n, c, data = slice_parts(v)
        return [("[%d]" % i, data[i]) for i in range(min(n, lim))] if int(data) else []
    if kind == "array":
        arr = v["gp_elems"]["_M_elems"]
        lo, hi = arr.type.strip_typedefs().range()
        return [("[%d]" % i, arr[i]) for i in range(lo, min(hi, lo + lim - 1) + 1)]
    if kind == "map":
        count, ents = map_entries(v, t, lim)
        out = []
        for i, (k, val) in enumerate(ents):
            out.append(("[%d]" % i, k))
            out.append(("[%d]" % i, val))
        return out
    if kind == "ptr":
        addr, target = ptr_target(v)
        return [] if addr == 0 else [("*", target)]
    if kind == "newtype":
        return []
    if kind == "iface":
        dyn = iface_dynamic(v)
        return [] if dyn is None else [("value", dyn)]
    return []


class Printer:
    def __init__(self, v, kind):
        self.v = v
        self.t = unwrap(v.type)
        self.kind = kind

    def _tree(self):
        return print_style() == "tree"

    def to_string(self):
        if self._tree():
            try:
                return tree_summary(self.v, self.t, self.kind)
            except Exception as e:
                return "<error: %s>" % _exc_text(e)
        return fmt(self.v, 0)

    def children(self):
        if not self._tree():
            return iter(())
        try:
            return iter(tree_children(self.v, self.t, self.kind))
        except Exception:
            return iter(())

    def display_hint(self):
        if not self._tree():
            return None
        if self.kind in ("slice", "array"):
            return "array"
        if self.kind == "map":
            return "map"
        return None


def lookup(v):
    """gdb's printer lookup: a Printer for the values of the runtime's and the emitter's types."""
    try:
        t = unwrap(v.type)
        if t.code != gdb.TYPE_CODE_STRUCT:
            return None
        kind = classify(t)
        if kind is None:
            return None
        return Printer(v, kind)
    except Exception:
        return None


lookup.__name__ = "gocpp_lookup"


# ---- frames: Go names in backtraces ----


def frame_source(frame):
    """(path of the .gp file, line) of a frame, or (None, 0)."""
    try:
        sal = frame.find_sal()
        st = sal.symtab
        if st is None or not st.filename.endswith(".gp"):
            return None, 0
        return st.fullname(), sal.line
    except gdb.error:
        return None, 0


def frame_go_name(frame):
    """The Go name of the function of frame, or None when it is not one of the program's."""
    name = frame.name()
    if not isinstance(name, str):
        return None
    path, line = frame_source(frame)
    src = go_source(path) if path is not None else None
    decl = 0
    try:
        fn = frame.function()
        decl = fn.line if fn is not None else 0
    except gdb.error:
        pass
    return go_name_of(name, src, decl, line)


def _symbol_name(item):
    """The name of the variable that a frame decorator's item stands for."""
    sym = item.symbol()
    return sym if isinstance(sym, str) else sym.print_name


if not STUB:
    from gdb.FrameDecorator import FrameDecorator

    class VarWrapper:
        """A variable of a frame under its Go name (long_ is the Go variable long)."""

        def __init__(self, name, value):
            self._name = name
            self._value = value

        def symbol(self):
            return self._name

        def value(self):
            return self._value

    class GoFrame(FrameDecorator):
        def _go(self):
            return str(_param("gocpp names", "go")) == "go"

        def function(self):
            if not self._go():
                return super().function()
            try:
                n = frame_go_name(self.inferior_frame())
            except Exception:
                n = None
            return n if n is not None else super().function()

        def _vars(self, items):
            if items is None or not self._go():
                return items
            out = []
            for it in items:
                sym = it.symbol()
                name = _symbol_name(it)
                if name.startswith("gp_"):
                    continue  # a temporary of the generated code
                val = it.value()
                if val is None and not isinstance(sym, str):
                    try:
                        val = sym.value(self.inferior_frame())
                    except gdb.error:
                        val = None
                if val is None:
                    out.append(it)
                else:
                    out.append(VarWrapper(go_field_name(name), val))
            return out

        def frame_args(self):
            items = super().frame_args()
            if items is not None and self._go() and is_lambda_name(str(self.inferior_frame().name())):
                # The closure object is not a parameter of the Go func literal (g++ calls it
                # __closure, clang++ this).
                items = [it for it in items if _symbol_name(it) not in ("__closure", "this")]
            return self._vars(items)

        def frame_locals(self):
            return self._vars(super().frame_locals())

    class GoFrameFilter:
        def __init__(self):
            self.name = "gocpp"
            self.priority = 100
            self.enabled = True

        def filter(self, it):
            go = str(_param("gocpp names", "go")) == "go"
            for f in it:
                if go and is_glue_name(str(f.function())):
                    continue  # the runtime's call of a func value: not a Go frame
                yield GoFrame(f)


# ---- goroutines ----

GOROUTINE_TLS = "gp::os::(anonymous namespace)::tslot"


def goroutine_record():
    """The record of the goroutine that runs the selected thread, or None (an idle thread, a
    thread that is no goroutine, a program without the runtime's symbols)."""
    try:
        slot = gdb.parse_and_eval("'%s'" % GOROUTINE_TLS)
        a = int(slot)
        if a == 0:
            return None
        return gdb.Value(a).cast(gdb.lookup_type("gp::detail::goroutine").pointer()).dereference()
    except gdb.error:
        return None


def top_go_frame(limit=64):
    """The innermost frame of the selected thread that is the program's, as "main.f (main.gp:12)"."""
    f = gdb.newest_frame()
    for _ in range(limit):
        if f is None:
            break
        try:
            path, line = frame_source(f)
            if path is not None:
                n = frame_go_name(f)
                return "%s (%s:%d)" % (n or f.name(), os.path.basename(path), line)
        except gdb.error:
            pass
        f = f.older()
    return None


def goroutine_rows():
    """One dict for each thread of the program, ordered by goroutine id, then thread."""
    inf = gdb.selected_inferior()
    cur = gdb.selected_thread()
    rows = []
    try:
        cur_frame = gdb.selected_frame()
    except gdb.error:
        cur_frame = None
    for th in inf.threads():
        try:
            th.switch()
            rec = goroutine_record()
            row = {"thread": th.num, "current": cur is not None and th.num == cur.num, "name": th.name or ""}
            if rec is None:
                row.update(id=None, state="idle or foreign thread", reason="", top=None)
            else:
                reason = rec["wait_reason"]
                reason_s = c_string(int(reason), 64) if int(reason) != 0 else ""
                blocked = bool(rec["is_blocked"]) or bool(reason_s)
                row.update(id=int(rec["id"]), state="waiting" if blocked else "running", reason=reason_s, top=top_go_frame())
            rows.append(row)
        except gdb.error as e:
            rows.append({"thread": th.num, "current": False, "name": "", "id": None, "state": "unreadable: %s" % e, "reason": "", "top": None})
    try:
        if cur is not None:
            cur.switch()
            if cur_frame is not None and cur_frame.is_valid():
                cur_frame.select()
    except gdb.error:
        pass
    rows.sort(key=lambda r: (r["id"] is None, r["id"] or 0, r["thread"]))
    return rows


def format_goroutines(rows, all_threads=False):
    lines = []
    goroutines = [r for r in rows if r["id"] is not None]
    others = [r for r in rows if r["id"] is None]
    for r in goroutines:
        state = r["state"] if not r["reason"] else "%s [%s]" % (r["state"], r["reason"])
        lines.append("%s %4d  %-28s thread %-3d %s" % ("*" if r["current"] else " ", r["id"], state, r["thread"], r["top"] or "-"))
    if others:
        if all_threads:
            for r in others:
                lines.append("%s   -  %-28s thread %-3d %s" % ("*" if r["current"] else " ", r["state"], r["thread"], r["name"]))
        else:
            lines.append("  (%d other thread%s: idle goroutine threads or threads of C libraries; gp goroutines -a lists them)" % (len(others), "" if len(others) == 1 else "s"))
    if not goroutines and not others:
        lines.append("no threads: the program is not running")
    return "\n".join(lines)


# ---- commands ----


def _block_vars(frame):
    try:
        b = frame.block()
    except RuntimeError:
        return
    seen = set()
    while b is not None and not b.is_global and not b.is_static:
        for sym in b:
            if (sym.is_variable or sym.is_argument) and sym.name not in seen and not sym.name.startswith("gp_"):
                seen.add(sym.name)
                yield sym
        if b.function is not None:
            break
        b = b.superblock


def go_type_of(v):
    return go_type(type_tag(unwrap(v.type)))


def _user_errors(invoke):
    """A command's gdb errors (no frame, no symbol) as the message that gdb's own commands print,
    not as a Python exception."""

    def wrapped(self, arg, from_tty):
        try:
            return invoke(self, arg, from_tty)
        except gdb.GdbError:
            raise
        except gdb.error as e:
            raise gdb.GdbError(str(e))

    wrapped.__doc__ = invoke.__doc__
    return wrapped


def unbox_local(sym, v, path=None, line=0):
    """The Go variable that the C++ local sym holds. A variable that escapes is a gp::ptr to a
    heap box in the C++ and would print as (*int) = &20: the .gp line of its declaration says
    whether the Go variable is the box (var x int; x := 20) or a pointer (p := &T{})."""
    try:
        if sym.line and sym.symtab is not None:
            src = go_source(sym.symtab.fullname())
            kind = classify_decl(sym.name, src.line_text(sym.line)) if src is not None else None
        else:  # (a capture in a func literal has no line: look above the frame's line)
            src = go_source(path) if path else None
            kind = find_decl_kind(src, sym.name, line) if src is not None else None
        return unbox(v, kind)
    except Exception:
        return v


def unbox(v, kind):
    """The box's target when kind (classify_decl's answer) says v is the box of a Go variable."""
    if kind is None:
        return v
    t = unwrap(v.type)
    if t.code != gdb.TYPE_CODE_STRUCT or classify(t) != "ptr":
        return v
    addr, target = ptr_target(v)
    if addr == 0:
        return v
    if kind == "ptr?":
        tt = unwrap(target.type)
        if tt.code != gdb.TYPE_CODE_STRUCT or classify(tt) != "ptr":
            return v
    return target


def capture_lines(closure, skip=(), path=None, line=0):
    """The lines of gp locals for a func literal's closure object (g++ names it __closure,
    clang++ this): the variables that the literal captured (g++ also has a local for each one:
    those in skip are listed already)."""
    t = unwrap(closure.type)
    if t.code == gdb.TYPE_CODE_PTR:
        closure = closure.dereference()
        t = unwrap(closure.type)
    src = go_source(path) if path else None
    out = []
    for f in t.fields():
        if f.is_base_class or not hasattr(f, "bitpos") or f.name is None:
            continue
        name = f.name[2:] if f.name.startswith("__") else f.name  # (g++ names a capture __x)
        if name in skip:
            continue
        v = closure[f]
        if src is not None:
            try:
                v = unbox(v, find_decl_kind(src, name, line))
            except Exception:
                pass
        out.append("%s (%s) = %s" % (go_field_name(name), go_type_of(v), fmt(v)))
    return out


class GP(gdb.Command):
    """gocpp debugging commands: gp goroutines | bt | locals | print | whatis | break | skip-runtime."""

    def __init__(self):
        super().__init__("gp", gdb.COMMAND_USER, gdb.COMPLETE_NONE, True)


class GPGoroutines(gdb.Command):
    """List the goroutines of the program: gp goroutines [-a].

Each goroutine is a thread of the program. The list shows its id, whether it runs or waits (and
what for: chan receive, chan send, select, ...), its thread and its innermost Go frame. -a also
lists the threads that run no goroutine."""

    def __init__(self, name="gp goroutines"):
        super().__init__(name, gdb.COMMAND_STATUS)

    @_user_errors
    def invoke(self, arg, from_tty):
        if not gdb.selected_inferior().threads():
            raise gdb.GdbError("The program is not being run.")
        gdb.write(format_goroutines(goroutine_rows(), arg.strip() == "-a") + "\n")


class GPBt(gdb.Command):
    """Backtrace with Go names: gp bt [bt options]."""

    def __init__(self):
        super().__init__("gp bt", gdb.COMMAND_STACK)

    @_user_errors
    def invoke(self, arg, from_tty):
        old = gdb.parameter("gocpp names") if not STUB else "go"
        gdb.execute("set gocpp names go")
        try:
            gdb.execute("backtrace " + arg)
        finally:
            gdb.execute("set gocpp names " + str(old))


class GPLocals(gdb.Command):
    """Print the arguments and local variables of the selected frame, with Go types: gp locals."""

    def __init__(self):
        super().__init__("gp locals", gdb.COMMAND_DATA)

    @_user_errors
    def invoke(self, arg, from_tty):
        frame = gdb.selected_frame()
        # An optimized build can stop in the runtime's call of a func value, which the compiler
        # inlined into the Go function: the locals are those of the Go function above it.
        for _ in range(16):
            name = frame.name()
            if not isinstance(name, str) or not is_glue_name(name) or frame.older() is None:
                break
            frame = frame.older()
        n = 0
        lam = is_lambda_name(str(frame.name()))
        cur_path, cur_line = frame_source(frame)
        syms = list(_block_vars(frame))
        names = {x.name for x in syms}  # (g++ also has a local for each capture)
        for sym in syms:
            # One variable that can't be read must not hide the others (the frame may be
            # before the declarations: its slots hold whatever the stack held).
            try:
                if lam and sym.name in ("__closure", "this"):
                    lines = capture_lines(sym.value(frame), names, cur_path, cur_line)
                else:
                    v = sym.value(frame)
                    if not sym.is_argument:
                        v = unbox_local(sym, v, cur_path, cur_line)
                    lines = ["%s (%s) = %s" % (go_field_name(sym.name), go_type_of(v), fmt(v))]
            except Exception as e:
                lines = ["%s = <unreadable: %s>" % (go_field_name(sym.name), _exc_text(e))]
            for line in lines:
                gdb.write(line + "\n")
                n += 1
        if n == 0:
            gdb.write("No locals.\n")


class GPPrint(gdb.Command):
    """Print an expression as Go text, with its Go type: gp print EXPR."""

    def __init__(self, name="gp print"):
        super().__init__(name, gdb.COMMAND_DATA, gdb.COMPLETE_EXPRESSION)

    @_user_errors
    def invoke(self, arg, from_tty):
        v = go_eval(arg)
        gdb.write("(%s) %s\n" % (go_type_of(v), fmt(v)))


class GPWhatis(gdb.Command):
    """Show the Go type of an expression: gp whatis EXPR."""

    def __init__(self):
        super().__init__("gp whatis", gdb.COMMAND_DATA, gdb.COMPLETE_EXPRESSION)

    @_user_errors
    def invoke(self, arg, from_tty):
        gdb.write("type = %s\n" % go_type_of(go_eval(arg)))


def _arity(args, n, name):
    if len(args) != n:
        raise gdb.GdbError("%s takes %d argument%s, not %d" % (name, n, "" if n == 1 else "s", len(args)))


def go_deref(v, name="$gp_deref"):
    """The value that the Go pointer v (a gp::ptr or a C++ pointer) points to."""
    t = unwrap(v.type)
    if t.code == gdb.TYPE_CODE_STRUCT and classify(t) == "ptr":
        addr, target = ptr_target(v)
        if addr == 0:
            raise gdb.GdbError("invalid memory address or nil pointer dereference")
        return target
    if t.code == gdb.TYPE_CODE_PTR:
        if int(v) == 0:
            raise gdb.GdbError("invalid memory address or nil pointer dereference")
        return v.dereference()
    raise gdb.GdbError("%s: %s is not a pointer" % (name, go_type_of(v)))


def go_index(v, i, name="$gp_index"):
    """Element i of a gp::slice or gp::array, or byte i of a gp::string."""
    t = unwrap(v.type)
    kind = classify(t) if t.code == gdb.TYPE_CODE_STRUCT else None
    if kind == "ptr" or t.code == gdb.TYPE_CODE_PTR:  # p[i] of a pointer to an array, as in Go
        target = go_deref(v, name)
        tt = unwrap(target.type)
        if tt.code == gdb.TYPE_CODE_STRUCT and classify(tt) == "array":
            v, t, kind = target, tt, "array"
    i = int(i)
    if kind == "slice":
        n, c, data = slice_parts(v)
        if not 0 <= i < n:
            raise gdb.GdbError("index out of range [%d] with length %d" % (i, n))
        return data[i]
    if kind == "array":
        arr = v["gp_elems"]["_M_elems"]
        lo, hi = arr.type.strip_typedefs().range()
        if not 0 <= i <= hi - lo:
            raise gdb.GdbError("index out of range [%d] with length %d" % (i, hi - lo + 1))
        return arr[i]
    if kind == "string":
        n = int(v["gp_len_"])
        if not 0 <= i < n:
            raise gdb.GdbError("index out of range [%d] with length %d" % (i, n))
        return v["gp_data_"][i].cast(gdb.lookup_type("unsigned char"))
    raise gdb.GdbError("%s: %s is not a slice, array or string" % (name, go_type_of(v)))


def _int(n):
    return gdb.Value(n).cast(gdb.lookup_type("long"))


def go_len(v, name="$gp_len"):
    """The length of a gp::string, slice, array or map, or the buffered values of a gp::chan."""
    t = unwrap(v.type)
    kind = classify(t) if t.code == gdb.TYPE_CODE_STRUCT else None
    if kind == "string":
        return _int(max(int(v["gp_len_"]), 0))
    if kind == "slice":
        return _int(slice_parts(v)[0])
    if kind == "array":
        lo, hi = v["gp_elems"]["_M_elems"].type.strip_typedefs().range()
        return _int(hi - lo + 1)
    if kind == "map":
        count, _ = map_entries(v, t, 0)
        return _int(count or 0)
    if kind == "chan":
        info = chan_info(v)
        return _int(0 if info is None else info["len"])
    raise gdb.GdbError("%s: %s has no length" % (name, go_type_of(v)))


def go_field(v, name):
    """The field name of the struct v, or of the struct that the pointer v points to (as Go's
    x.f does)."""
    for _ in range(4):
        t = unwrap(v.type)
        if t.code == gdb.TYPE_CODE_STRUCT and classify(t) == "ptr" or t.code == gdb.TYPE_CODE_PTR:
            v = go_deref(v)
            continue
        break
    t = unwrap(v.type)
    if t.code != gdb.TYPE_CODE_STRUCT:
        raise gdb.GdbError("%s has no field %s" % (go_type_of(v), name))
    for n, x in struct_fields(v, t):
        if n == name:
            return x
    raise gdb.GdbError("%s has no field %s" % (go_type_of(v), name))


_IDENT = re.compile(r"[A-Za-z_]\w*")


def _unref(v):
    t = v.type.strip_typedefs()
    if t.code in (gdb.TYPE_CODE_REF, gdb.TYPE_CODE_RVALUE_REF):
        return v.referenced_value()
    return v


def unbox_named(name, v):
    """v, the value of the variable name in the selected frame, as the Go variable (see
    unbox_local): gp print and gp locals agree on a boxed variable."""
    try:
        sym = gdb.lookup_symbol(name)[0]
        if sym is None or not sym.is_variable or sym.is_argument:
            return v
        path, line = frame_source(gdb.selected_frame())
        return unbox_local(sym, v, path, line)
    except Exception:
        return v


_GOISH = re.compile(r"\*|len\s*\(")


def go_eval(arg):
    """The value of the expression arg: gdb's own reading, and when gdb can't parse it, Go's:
    x.f (also through a pointer), x[i], *p and len(x) over names of the program. An expression
    that starts with * or len( is read as Go first: gdb would call the runtime's operator* in
    the program (and give a reference that does not print). A variable that is a heap box
    is shown as the Go variable, as gp locals does."""
    text = arg.strip()
    goish = _GOISH.match(text) is not None
    if goish:
        try:
            return _unref(_go_expr(text))
        except (gdb.error, gdb.GdbError, ValueError):
            pass
    try:
        v = _unref(gdb.parse_and_eval(text))
    except gdb.error as first:
        if not goish:
            try:
                return _unref(_go_expr(text))
            except (gdb.error, gdb.GdbError, ValueError):
                pass
        raise first
    if _IDENT.fullmatch(text):
        v = unbox_named(text, v)
    return v


def _go_expr(s):
    m = re.fullmatch(r"len\s*\((.*)\)", s, re.S)
    if m:
        return go_len(_go_expr(m.group(1).strip()), "len")
    if s.startswith("*"):
        return go_deref(_go_expr(s[1:].strip()), "*")
    m = _IDENT.match(s)
    if m is None:
        raise ValueError(s)
    v = unbox_named(m.group(0), _unref(gdb.parse_and_eval(m.group(0))))
    rest = s[m.end():].lstrip()
    while rest:
        if rest[0] == ".":
            m = _IDENT.match(rest[1:].lstrip())
            if m is None:
                raise ValueError(s)
            v = go_field(v, m.group(0))
            rest = rest[1:].lstrip()[m.end():].lstrip()
        elif rest[0] == "[":
            depth, j = 0, 0
            for j, c in enumerate(rest):
                depth += c == "["
                depth -= c == "]"
                if depth == 0:
                    break
            if depth != 0:
                raise ValueError(s)
            v = go_index(v, _unref(gdb.parse_and_eval(rest[1:j])), "[]")
            rest = rest[j + 1:].lstrip()
        else:
            raise ValueError(s)
    return v


if not STUB:

    class GpDeref(gdb.Function):
        """$gp_deref(p): the value that the gp::ptr p points to (gdb can't call operator*)."""

        def __init__(self):
            super().__init__("gp_deref")

        def invoke(self, *args):
            _arity(args, 1, "$gp_deref")
            return go_deref(args[0])

    class GpIndex(gdb.Function):
        """$gp_index(x, i): element i of a gp::slice or gp::array, or byte i of a gp::string
        (gdb can't call operator[])."""

        def __init__(self):
            super().__init__("gp_index")

        def invoke(self, *args):
            _arity(args, 2, "$gp_index")
            return go_index(args[0], args[1])

    class GpBlocked(gdb.Function):
        """$gp_blocked(): how many goroutines are blocked in a channel operation or a select (the
        runtime's own count; 0 in a program without goroutines). A breakpoint condition such as
        $gp_blocked() == 3 waits for them."""

        def __init__(self):
            super().__init__("gp_blocked")

        def invoke(self, *args):
            _arity(args, 0, "$gp_blocked")
            try:
                v = gdb.parse_and_eval("'gp::detail::(anonymous namespace)::blocked'")
                # The debug info names the counter even when the linker dropped it (a program
                # that never starts a goroutine): then its address is garbage.
                sym = gdb.execute("info symbol %d" % int(v.address), to_string=True)
                if "blocked" not in sym:
                    return _int(0)
                return v["_M_i"]
            except (gdb.error, TypeError, ValueError, OverflowError):
                return _int(0)

    class GpLen(gdb.Function):
        """$gp_len(x): the length of a gp::string, slice, array or map, or the buffered values of a gp::chan."""

        def __init__(self):
            super().__init__("gp_len")

        def invoke(self, *args):
            _arity(args, 1, "$gp_len")
            return go_len(args[0])


# The rules that need no path. (No backslash in a rule: gdb takes it as an escape of its own.)
SKIP_RULES = [
    "skip -rfu ^std::",
    "skip -rfu ^__gnu_cxx::",
    "skip -rfu ^gp::(string|slice|array|map|ptr|cptr|weak_ptr|chan|iface|any|error|newtype|detail|fmt|os|complex)(::|<|$)",
]


def _glob_escape(path):
    """path as a fnmatch pattern that matches it only."""
    return path.replace("[", "[[]").replace("*", "[*]").replace("?", "[?]")


def runtime_rules(root=None):
    """The skip rules for the files of the runtime: its sources and its headers (but not the
    headers of the generated code, include/gocpp/f*.hpp: those are the program's). root is the
    folder of the runtime that the program was built with (rt/<version>, found from the file of
    gp::run_main); without it the folder is matched by its place in the cache, gocpp/rt, so that
    a C++ file of the user's that happens to be in a folder rt/x/src is not skipped."""
    if root and not re.search(r"[\s\"'\\]", root):
        base = _glob_escape(root.rstrip("/"))
    else:
        base = "*/gocpp/rt/*"
    return [
        "skip -gfi %s/src/*.cpp" % base,
        "skip -gfi %s/include/gocpp/[!f]*.hpp" % base,
        "skip -gfi %s/include/gocpp/fmt*.hpp" % base,
    ]


def runtime_root():
    """The folder of the runtime that this program was built with (the one with src/ and
    include/), from the file that the debug info names for gp::run_main, or None."""
    try:
        sym = gdb.lookup_global_symbol("gp::run_main")
        path = sym.symtab.fullname() if sym is not None else None
    except Exception:
        return None
    if not path or os.path.basename(os.path.dirname(path)) != "src":
        return None
    return os.path.dirname(os.path.dirname(path))


_skips = []  # the numbers of the skips that we made (gdb's info skip)


def _skip_numbers():
    out = gdb.execute("info skip", to_string=True)
    return {int(m.group(1)) for m in re.finditer(r"^(\d+)\s", out, re.M)}


def skip_runtime(on=True):
    """Turns the runtime's skip rules on (once: a second call changes nothing) or off. Returns
    the number of rules that are on."""
    if on and not _skips:
        before = _skip_numbers()
        for rule in SKIP_RULES + runtime_rules(runtime_root()):
            gdb.execute(rule, to_string=True)
        _skips.extend(sorted(_skip_numbers() - before))
    elif not on and _skips:
        for n in _skips:
            gdb.execute("skip delete %d" % n, to_string=True)
        del _skips[:]
    return len(_skips)


class GPSkipRuntime(gdb.Command):
    """Make step skip the runtime's helpers: gp skip-runtime [on|off].

The rules are on when the script is loaded (unless GOCPP_GDB_NOSKIP is set): step goes into the
program's functions and closures, and over gp::string, gp::slice, gp::map, fmt and the C++
library. gp skip-runtime off removes them (to step into the runtime), on sets them again. (Calls
through a func value still show gp::func's two frames: step into one and gdb stops in the
closure.)"""

    def __init__(self):
        super().__init__("gp skip-runtime", gdb.COMMAND_RUNNING)

    @_user_errors
    def invoke(self, arg, from_tty):
        word = arg.strip()
        if word not in ("", "on", "off"):
            raise gdb.GdbError("usage: gp skip-runtime [on|off]")
        n = skip_runtime(word != "off")
        if word == "off":
            gdb.write("The runtime's skip rules are off: step goes into the runtime.\n")
        else:
            gdb.write("%d skip rules are on (info skip lists them; gp skip-runtime off removes them).\n" % n)


def go_cxx_name(name):
    """The C++ name of a Go function name (main.f, main.T.M, main.(*T).M, main.init and
    main.init.N for the initializers), or None."""
    m = re.fullmatch(r"(\w+)\.init(?:\.(\d+))?", name)
    if m is not None:  # the package's variable initializers, and init() number n (0-based)
        return "gpkg_%s::%s" % (m.group(1), "gp_var_init" if m.group(2) is None else "gp_init_%d" % (int(m.group(2)) + 1))
    m = re.fullmatch(r"(\w+)\.(\(\*(\w+)\)|\w+)(?:\.(\w+))?", name)
    if m is None:
        return None
    pkg, first, ptr_type, method = m.groups()
    parts = [ptr_type or first] + ([method] if method else [])
    return "gpkg_%s::%s" % (pkg, "::".join(parts))


if not STUB:

    class GoBreakpoint(gdb.Breakpoint):
        """A breakpoint on a function by its C++ name. gdb matches the func literals inside the
        function too (main.main.func1 is gpkg_main::main()::{lambda}): they don't stop."""

        def stop(self):
            try:
                return not is_lambda_name(str(gdb.selected_frame().name()))
            except gdb.error:
                return True


class GPBreak(gdb.Command):
    """Set a breakpoint on a Go function: gp break main.main | main.T.M | main.(*T).M | main.init | main.init.N [if COND]."""

    def __init__(self):
        super().__init__("gp break", gdb.COMMAND_BREAKPOINTS)

    @_user_errors
    def invoke(self, arg, from_tty):
        words = arg.split(None, 1)
        if not words:
            raise gdb.GdbError("usage: gp break GO-FUNCTION [if CONDITION]")
        cxx = go_cxx_name(words[0])
        if cxx is None:
            raise gdb.GdbError("%s is not a Go function name (main.f, main.T.M or main.(*T).M)" % words[0])
        cond = None
        if len(words) > 1:
            m = re.match(r"if\s+(.+)$", words[1].strip(), re.S)
            if m is None:
                raise gdb.GdbError("usage: gp break GO-FUNCTION [if CONDITION]")
            cond = m.group(1)
        bp = GoBreakpoint(cxx, qualified=True)
        if cond:
            bp.condition = cond


def register_params():
    class PrintStyle(gdb.Parameter):
        """Set how gocpp values print: go (one line of Go text) or tree (a summary and children, for IDEs)."""

        set_doc = "Set how gocpp values print."
        show_doc = "Show how gocpp values print."

        def __init__(self):
            super().__init__("gocpp print-style", gdb.COMMAND_DATA, gdb.PARAM_ENUM, ["go", "tree"])
            self.value = "go"

        def get_set_string(self):
            return ""

        def get_show_string(self, svalue):
            return "gocpp values print in style %s." % svalue

    class Names(gdb.Parameter):
        """Set whether backtraces show Go names (go) or C++ names (cpp)."""

        set_doc = "Set the names that backtraces show."
        show_doc = "Show the names that backtraces show."

        def __init__(self):
            super().__init__("gocpp names", gdb.COMMAND_STACK, gdb.PARAM_ENUM, ["go", "cpp"])
            self.value = "go"

        def get_set_string(self):
            return ""

        def get_show_string(self, svalue):
            return "backtraces show %s names." % svalue

    class Prefix(gdb.Command):
        """Settings of the gocpp debugging support."""

        def __init__(self, name, kind):
            super().__init__(name, kind, gdb.COMPLETE_NONE, True)

    Prefix("set gocpp", gdb.COMMAND_DATA)
    Prefix("show gocpp", gdb.COMMAND_DATA)
    return PrintStyle(), Names()


_keep = []


def register(objfile=None):
    """Registers the printers, the frame filter and the commands. Loading the script again
    replaces the earlier registration."""
    target = objfile if objfile is not None else gdb
    plist = target.pretty_printers
    plist[:] = [p for p in plist if getattr(p, "__name__", "") != "gocpp_lookup"]
    plist.append(lookup)
    if not STUB:
        flt = GoFrameFilter()
        target.frame_filters[flt.name] = flt
    if getattr(gdb, "_gocpp_registered", False):
        return
    gdb._gocpp_registered = True
    _keep.append(register_params())
    GP()
    GPGoroutines()
    GPGoroutines("info goroutines")
    GPBt()
    GPLocals()
    GPPrint()
    GPWhatis()
    GPSkipRuntime()
    GPBreak()
    GpDeref()
    GpIndex()
    GpLen()
    GpBlocked()


if not STUB:
    register(gdb.current_objfile())
    if not os.environ.get("GOCPP_GDB_NOSKIP"):
        try:
            skip_runtime(True)
        except gdb.error:
            pass
