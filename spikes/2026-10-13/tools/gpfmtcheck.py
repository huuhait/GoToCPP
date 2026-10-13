# gofmt-check gocpp code by mapping gocpp-only syntax to same-shape Go placeholders.
import re, subprocess, sys, textwrap
SUBS = [("std.map", "std.MAP"), (".type()", ".TYPE()"), (".select(", ".SELEC("), ("std.array[int, 5]", "std.array[int, N]"),
        ("std.array[byte, 256]", "std.array[byte, NNN]"), ("std.bitset[64]", "std.bitset[NN]")]
def to_go(code):
    for a, b in SUBS:
        code = code.replace(a, b)
    code = re.sub(r"^macro (\w)", r"func \1", code, flags=re.M)            # macro decl (package level)
    code = re.sub(r"(\(|, )(\w+) lazy ", r"\1\2 ", code)                    # `msg lazy string` -> `msg string`
    code = re.sub(r"^#(\w[^\n]*)$", r"// ONTOP \1", code, flags=re.M)  # on-top macro line (a plain comment, not a directive)
    code = re.sub(r"(\w)!\(", lambda m: m.group(1) + "ǃ(", code)
    code = re.sub(r"(\w)!\[", lambda m: m.group(1) + "ǃ[", code)
    code = re.sub(r"#(?=[A-Za-z_])", "ǂ", code)
    return code
def check(code):
    code = to_go(textwrap.dedent(code))
    full = bool(re.search(r"^package \w+", code, re.M))
    cands = [code] if full else ["package p\n\n" + code,
             "package p\n\nfunc _() {\n" + "".join("\t" + l if l.strip() else l for l in code.splitlines(True)) + "}\n"]
    for src in cands:
        r = subprocess.run(["gofmt"], input=src, capture_output=True, text=True)
        if r.returncode == 0:
            return "clean" if r.stdout == src else "NOT CLEAN"
    return "CANNOT PARSE: " + r.stderr.strip().splitlines()[0]
if __name__ == "__main__":
    print(check(open(sys.argv[1]).read()))
