# gocpp_lldb.py: LLDB support for gocpp programs (Phase 13, step 13.2). Best effort: gdb is the
# debugger with the full support (gocpp-gdb.py: Go names in backtraces, goroutine list, locals
# in Go syntax); this script gives LLDB the summaries of the runtime's values.
#
#   (lldb) command script import runtime/debug/gocpp_lldb.py       (gocpp debug -debugger lldb does it)
#
# Summaries: gp::string, gp::slice, gp::array, gp::map, gp::ptr, gp::chan, gp::func, gp::any
# and gp::error (and the interface structs), the structs of the program ({X:1 Y:2}). The text is
# the one that gocpp-gdb.py prints, except where LLDB's own value display is better kept
# (scalars). `gp print EXPR` prints an expression as Go text. Everything is read from the
# program's memory; nothing runs in the program.

import re
import struct

import lldb

MAX_DEPTH = 12
LIMIT = 200

SCALAR_KINDS = {
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
    if m:
        s = "%se%s%02d" % (m.group(1), m.group(2), int(m.group(3)))
    return s


def quote_go(data, limit=None):
    """data (bytes) as a Go string literal; with limit, only the first limit bytes and "..."."""
    cut = limit is not None and len(data) > limit
    if cut:
        data = data[:limit]
    out = ['"']
    for ch in data.decode("utf-8", errors="surrogateescape"):
        o = ord(ch)
        esc = {'"': '\\"', "\\": "\\\\", "\n": "\\n", "\t": "\\t", "\r": "\\r"}.get(ch)
        if esc:
            out.append(esc)
        elif 0xDC80 <= o <= 0xDCFF:
            out.append("\\x%02x" % (o - 0xDC00))
        elif o < 0x20 or o == 0x7F:
            out.append("\\x%02x" % o)
        elif o > 0x7F and not ch.isprintable():
            out.append("\\u%04x" % o if o <= 0xFFFF else "\\U%08x" % o)
        else:
            out.append(ch)
    out.append('"')
    return "".join(out) + ("..." if cut else "")


def tag(v):
    """The C++ name of the type of the SBValue v, without references, typedefs and qualifiers."""
    t = v.GetType()
    if t.IsReferenceType():
        t = t.GetDereferencedType()
    t = t.GetUnqualifiedType().GetCanonicalType()
    return t.GetName() or ""


def kind_of(v):
    """What the runtime or the emitter makes of the value's type."""
    n = tag(v)
    if n == "gp::string":
        return "string"
    for prefix, kind in (
        ("gp::slice<", "slice"),
        ("gp::array<", "array"),
        ("gp::map<", "map"),
        ("gp::ptr<", "ptr"),
        ("gp::cptr<", "ptr"),
        ("gp::chan<", "chan"),
        ("gp::func<", "func"),
        ("gp::complex<", "complex"),
        ("gp::newtype<", "newtype"),
    ):
        if n.startswith(prefix):
            return kind
    t = v.GetType().GetCanonicalType()
    if n == "gp::iface" or _has_base(t, "gp::iface"):
        return "iface"
    if _has_base_prefix(t, "gp::newtype<"):
        return "newtype"
    if re.match(r"^gpkg_\w+::\w+$", n) or n.startswith("gp_anon_"):
        return "struct"
    return None


def _has_base(t, name):
    for i in range(t.GetNumberOfDirectBaseClasses()):
        b = t.GetDirectBaseClassAtIndex(i).GetType()
        if b.GetName() == name or _has_base(b, name):
            return True
    return False


def _has_base_prefix(t, prefix):
    for i in range(t.GetNumberOfDirectBaseClasses()):
        b = t.GetDirectBaseClassAtIndex(i).GetType()
        if (b.GetName() or "").startswith(prefix) or _has_base_prefix(b, prefix):
            return True
    return False


def member(v, name):
    """The member name of v, read from memory (LLDB's own view of std::shared_ptr and the like
    is a synthetic one, without the members)."""
    return v.GetNonSyntheticValue().GetChildMemberWithName(name)


def uint(v):
    return v.GetValueAsUnsigned()


def read(process, addr, n):
    err = lldb.SBError()
    data = process.ReadMemory(addr, n, err)
    if err.Fail():
        raise ValueError(str(err))
    return data


def shared_addr(sp):
    return uint(member(sp, "_M_ptr"))


def fmt(v, depth=0):
    try:
        return _fmt(v, depth)
    except ValueError as e:
        return "<unreadable: %s>" % e


def _fmt(v, depth):
    if depth > MAX_DEPTH:
        return "..."
    t = v.GetType()
    if t.IsReferenceType():
        v = v.Dereference()
        t = v.GetType()
    kind = kind_of(v)
    if kind is not None:
        return KINDS[kind](v, depth)
    bt = t.GetCanonicalType().GetBasicType()
    if bt == lldb.eBasicTypeBool:
        return "true" if uint(v) else "false"
    if bt in (lldb.eBasicTypeFloat, lldb.eBasicTypeDouble):
        return go_float(float(v.GetValue()), t.GetByteSize())
    if t.IsPointerType():
        a = uint(v)
        return "nil" if a == 0 else v.GetSummary() or "0x%x" % a
    if t.IsArrayType():
        n = v.GetNumChildren()
        return _list([v.GetChildAtIndex(i) for i in range(min(n, LIMIT))], n, depth)
    if t.GetCanonicalType().GetTypeClass() == lldb.eTypeClassBuiltin and bt != lldb.eBasicTypeInvalid:
        try:
            return str(v.GetValueAsSigned() if bt in SIGNED else v.GetValueAsUnsigned())
        except Exception:
            pass
    s = v.GetValue()
    if s is not None:
        return s
    return v.GetSummary() or "?"


SIGNED = frozenset([
    lldb.eBasicTypeChar, lldb.eBasicTypeSignedChar, lldb.eBasicTypeShort, lldb.eBasicTypeInt,
    lldb.eBasicTypeLong, lldb.eBasicTypeLongLong,
])


def _list(elems, total, depth):
    parts = [fmt(e, depth + 1) for e in elems]
    if total > len(elems):
        parts.append("...")
    return "[" + " ".join(parts) + "]"


def fmt_string(v, depth):
    n, p = uint(member(v, "gp_len_")), uint(member(v, "gp_data_"))
    if n == 0 or p == 0:
        return '""'
    data = read(v.GetProcess(), p, min(n, LIMIT))
    return quote_go(data) + ("..." if n > LIMIT else "")


def fmt_slice(v, depth):
    n, c = uint(member(v, "gp_len_")), uint(member(v, "gp_cap_"))
    data = member(v, "gp_data_")
    if uint(data) == 0:
        return "nil" if depth > 0 else "[] (nil)"
    text = _list([data.GetChildAtIndex(i, lldb.eNoDynamicValues, True) for i in range(min(n, LIMIT))], n, depth)
    if depth == 0:
        text += " (len %d, cap %d)" % (n, c)
    return text


def fmt_array(v, depth):
    arr = member(member(v, "gp_elems"), "_M_elems")
    n = arr.GetNumChildren()
    return _list([arr.GetChildAtIndex(i) for i in range(min(n, LIMIT))], n, depth)


def fmt_map(v, depth):
    impl = member(v, "gp_impl_")
    if shared_addr(impl) == 0:
        return "nil" if depth > 0 else "map[] (nil)"
    table = member(member(member(impl.GetChildMemberWithName("_M_ptr").Dereference(), "gp_t"), "_M_h"), "_M_element_count")
    count = uint(table)
    text = "map[...]"
    if depth == 0:
        text += " (len %d)" % count
    return text


def fmt_ptr(v, depth):
    sp = member(v, "gp_sp_")
    addr = shared_addr(sp)
    if addr == 0:
        return "nil"
    if depth > 0:
        return "0x%x" % addr
    target = member(sp, "_M_ptr").Dereference()
    if not target.IsValid():
        return "0x%x" % addr
    return "&%s @0x%x" % (fmt(target, depth + 1), addr)


def fmt_chan(v, depth):
    c = member(v, "gp_c")
    addr = shared_addr(c)
    if addr == 0:
        return "nil"
    if depth > 0:
        return "0x%x" % addr
    core = member(c, "_M_ptr").Dereference()
    n = uint(member(member(core, "count"), "_M_i"))
    text = "chan(0x%x len %d cap %d" % (addr, n, uint(member(core, "capacity")))
    if uint(member(core, "closed")):
        text += " closed"
    return text + ")"


def fmt_func(v, depth):
    code = uint(member(v, "gp_call_"))
    if code == 0:
        return "nil"
    return "0x%x" % code if depth > 0 else "func(0x%x)" % code


def at(v, addr, type_name):
    """The value of the C++ type type_name at addr, or None when the debug info has no such type."""
    target = v.GetTarget()
    t = target.FindFirstType(type_name)
    if not t.IsValid():
        return None
    return target.CreateValueFromAddress("v", lldb.SBAddress(addr, target), t)


def fmt_ptr_to(pointee, depth):
    if depth > 0:
        return "0x%x" % pointee.GetLoadAddress()
    return "&%s @0x%x" % (fmt(pointee, depth + 1), pointee.GetLoadAddress())


def fmt_iface(v, depth):
    td = uint(member(v, "gp_type_"))
    if td == 0:
        return "nil"
    d = member(v, "gp_type_").Dereference()
    err = lldb.SBError()
    name = v.GetProcess().ReadCStringFromMemory(uint(member(d, "gp_name")), 256, err)
    if err.Fail():
        name = "?"
    kind = (member(d, "gp_kind").GetValue() or "").rsplit("::", 1)[-1]
    addr = shared_addr(member(v, "gp_data_"))
    text = "<%s at 0x%x>" % (name, addr)
    if kind in SCALAR_KINDS and addr:
        code, size = SCALAR_KINDS[kind]
        x = struct.unpack(code, read(v.GetProcess(), addr, size))[0]
        text = ("true" if x else "false") if kind == "Bool" else go_float(x, size) if kind.startswith("Float") else str(x)
    elif kind == "String" and addr:
        text = fmt_string(at(v, addr, "gp::string"), depth + 1)
    elif addr:
        # A boxed value of a type of the program (main.Point), or a pointer to one (*main.Point).
        m = re.fullmatch(r"(\*?)(\w+)\.(\w+)", name)
        if m:
            box = at(v, addr, "gpkg_%s::%s" % (m.group(2), m.group(3)))
            if box is not None:
                text = fmt_ptr_to(box, depth) if m.group(1) else fmt(box, depth + 1)
    if depth > 0:
        return text
    return "(%s) %s" % (name, text)


def fmt_newtype(v, depth):
    inner = member(v, "gp_v")
    if inner.IsValid():
        return fmt(inner, depth)
    # A named type over a runtime class (IDs over gp::slice): the base that has a kind.
    for i in range(v.GetNumChildren()):
        c = v.GetChildAtIndex(i)
        k = kind_of(c)
        if k == "newtype":
            return fmt_newtype(c, depth)
        if k is not None:
            return fmt(c, depth)
    return v.GetSummary() or "?"


def fmt_complex(v, depth):
    re_, im = float(member(v, "gp_re").GetValue()), float(member(v, "gp_im").GetValue())
    size = member(v, "gp_re").GetType().GetByteSize()
    return "(%s%s%si)" % (go_float(re_, size), "+" if im >= 0 or im != im else "", go_float(im, size))


def go_field_name(name):
    return name[:-1] if name.endswith("_") and name[:-1] in KEYWORDS else name


KEYWORDS = frozenset("new class delete default template this operator register union".split())


def fmt_struct(v, depth):
    parts = []
    for i in range(v.GetNumChildren()):
        c = v.GetChildAtIndex(i)
        if c.GetName() and not c.GetName().startswith("gp_"):
            parts.append("%s:%s" % (go_field_name(c.GetName()), fmt(c, depth + 1)))
    return "{" + " ".join(parts) + "}"


KINDS = {
    "string": fmt_string,
    "slice": fmt_slice,
    "array": fmt_array,
    "map": fmt_map,
    "ptr": fmt_ptr,
    "chan": fmt_chan,
    "func": fmt_func,
    "iface": fmt_iface,
    "newtype": fmt_newtype,
    "complex": fmt_complex,
    "struct": fmt_struct,
}


def summary(valobj, internal_dict):
    try:
        return fmt(valobj, 0)
    except Exception as e:  # a summary that raises shows nothing: say why
        import traceback
        traceback.print_exc()
        return "<gocpp summary error: %s>" % e


def gp_print(debugger, command, result, internal_dict):
    """gp print EXPR: an expression as Go text."""
    frame = debugger.GetSelectedTarget().GetProcess().GetSelectedThread().GetSelectedFrame()
    v = frame.EvaluateExpression(command)
    if not v.IsValid() or v.GetError().Fail():
        result.SetError(str(v.GetError()))
        return
    result.AppendMessage(fmt(v))


def gp_locals(debugger, command, result, internal_dict):
    """gp locals: the arguments and locals of the selected frame as Go text."""
    frame = debugger.GetSelectedTarget().GetProcess().GetSelectedThread().GetSelectedFrame()
    for v in frame.GetVariables(True, True, False, True):
        if not v.GetName().startswith("gp_"):
            result.AppendMessage("%s = %s" % (go_field_name(v.GetName()), fmt(v)))


def __lldb_init_module(debugger, internal_dict):
    mod = __name__
    for rx in (
        r"^gp::string$",
        r"^gp::slice<.*>$",
        r"^gp::array<.*>$",
        r"^gp::map<.*>$",
        r"^gp::c?ptr<.*>$",
        r"^gp::chan<.*>$",
        r"^gp::func<.*>$",
        r"^gp::iface$",
        r"^gp::any$",
        r"^gp::error$",
        r"^gp::complex<.*>$",
        r"^gpkg_\w+::\w+$",
        r"^gp_anon_\w+$",
    ):
        debugger.HandleCommand('type summary add -x "%s" -F %s.summary' % (rx, mod))
    debugger.HandleCommand("command script add -f %s.gp_print gp_print" % mod)
    debugger.HandleCommand("command script add -f %s.gp_locals gp_locals" % mod)
    print("gocpp: summaries loaded (gp_print EXPR, gp_locals)")
