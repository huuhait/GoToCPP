# Generates tests/run/fmt_*.gp programs whose // Output: blocks come from REAL Go (the `go` command
# of the dev shell). See README.md.
import os, subprocess, sys, tempfile, shutil, textwrap

S = os.environ.get("FMTCORPUS_WORK", tempfile.gettempdir())
OUT = os.environ.get("CORPUS_OUT", os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "tests", "run"))
tests = {}   # name -> dict(desc, src, exitcode, stderr)

def case(name, desc, decls, body, imports=("fmt",), exitcode=0, stderr=None, phase=5):
    assert name not in tests, name
    tests[name] = dict(desc=desc, decls=decls, body=body, imports=imports, exitcode=exitcode, stderr=stderr, phase=phase)

def program(t):
    imps = "\n".join('\t"%s"' % i for i in t["imports"])
    imp = 'import (\n%s\n)' % imps if len(t["imports"]) > 1 else 'import "%s"' % t["imports"][0]
    decls = t["decls"].strip("\n")
    body = t["body"].rstrip("\n")
    parts = ["package main", imp]
    if decls:
        parts.append(decls)
    parts.append("func main() {\n" + body + "\n}")
    return "\n\n".join(parts) + "\n"

def run_go(src, name):
    d = os.path.join(S, "fmtcorpus-work", name)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    open(os.path.join(d, "main.go"), "w").write(src)
    open(os.path.join(d, "go.mod"), "w").write("module corpus\ngo 1.25\n")
    r = subprocess.run(["go", "run", "main.go"], cwd=d, capture_output=True)
    try:
        o = r.stdout.decode("utf-8")
    except UnicodeDecodeError:
        o = "INVALID UTF-8 IN OUTPUT"
        return 99, o, ""
    return r.returncode, o, r.stderr.decode("utf-8", "replace")

def comment_lines(text):
    out = []
    for line in text.split("\n"):
        out.append("//" if line == "" else "// " + line)
    return out

def emit(only=None):
    bad = []
    for name, t in sorted(tests.items()):
        if only and not any(o in name for o in only):
            continue
        src = program(t)
        code, out, err = run_go(src, name)
        if code != t["exitcode"]:
            bad.append((name, "exit code %d: %s" % (code, err[:400])))
            continue
        if not out.endswith("\n"):
            out += "\n"
        lines = out.rstrip("\n").split("\n") if out.strip("\n") else []
        for i, l in enumerate(lines):
            if l != l.rstrip():
                bad.append((name, "line %d ends with a space: %r" % (i + 1, l)))
        try:
            out.encode("utf-8")
        except UnicodeEncodeError:
            pass
        head = ["// phase: %02d" % t["phase"]]
        if t["exitcode"]:
            head.append("// ExitCode: %d" % t["exitcode"])
        if t["stderr"]:
            head.append("// Stderr: " + t["stderr"])
        desc = textwrap.fill(t["desc"], 98, initial_indent="// ", subsequent_indent="// ")
        text = "\n".join(head) + "\n\n" + desc + "\n" + src + "\n// Output:\n" + "\n".join(comment_lines("\n".join(lines))) + ("\n" if lines else "")
        if not lines:
            text = text.rstrip("\n") + "\n"
        open(os.path.join(OUT, name + ".gp"), "w", encoding="utf-8", errors="surrogateescape").write(text)
    for n, m in bad:
        print("BAD", n, m, file=sys.stderr)
    return not bad
