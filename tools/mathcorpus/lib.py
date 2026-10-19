# Generates tests/run/math_*.gp programs whose // Output: blocks come from REAL Go (the `go` command of the dev shell).
# See README.md. Same idea as tools/fmtcorpus.
import os, subprocess, sys, tempfile, shutil

S = os.environ.get("MATHCORPUS_WORK", tempfile.gettempdir())
OUT = os.environ.get("CORPUS_OUT", os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "tests", "run"))
tests = {}   # name -> dict

def case(name, desc, src, exitcode=0, stderr=None, env=None, phase=9):
    """src: a whole Go program (package main)."""
    assert name not in tests, name
    tests[name] = dict(desc=desc, src=src, exitcode=exitcode, stderr=stderr, env=env or {}, phase=phase)

def run_go(src, name, env):
    d = os.path.join(S, "mathcorpus-work", name)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    open(os.path.join(d, "main.go"), "w").write(src)
    open(os.path.join(d, "go.mod"), "w").write("module corpus\ngo 1.25\n")
    e = dict(os.environ)
    e.update(env)
    r = subprocess.run(["go", "run", "main.go"], cwd=d, capture_output=True, env=e)
    try:
        o = r.stdout.decode("utf-8")
    except UnicodeDecodeError:
        return 99, "INVALID UTF-8 IN OUTPUT", ""
    return r.returncode, o, r.stderr.decode("utf-8", "replace")

def comment_lines(text):
    return ["//" if line == "" else "// " + line for line in text.split("\n")]

def emit(only=None):
    bad = []
    n = 0
    for name, t in sorted(tests.items()):
        if only and not any(o in name for o in only):
            continue
        header = "// phase: %02d\n" % t["phase"]
        for l in t["desc"].strip().split("\n"):
            header += ("// " + l).rstrip() + "\n"
        header += "\n"
        src = t["src"]
        code, out, err = run_go(src, name, t["env"])
        if code != t["exitcode"]:
            bad.append((name, "exit code %d: %s" % (code, err[:400])))
            continue
        if not out.endswith("\n"):
            out += "\n"
        lines = out.rstrip("\n").split("\n") if out.strip("\n") else []
        for i, l in enumerate(lines):
            if l != l.rstrip():
                bad.append((name, "line %d ends with a space: %r" % (i + 1, l)))
        if any(("\x00" in l) for l in lines):
            bad.append((name, "NUL in output"))
            continue
        text = header + src + "\n// Output:\n" + "\n".join(comment_lines("\n".join(lines))) + "\n"
        if lines == []:
            text = header + src + "\n// Output:\n"
        # a // Stderr: or // ExitCode: line goes at the top of the file, before the package clause
        pre = ""
        if t["exitcode"]:
            pre += "// ExitCode: %d\n" % t["exitcode"]
        if t["stderr"]:
            pre += "// Stderr: %s\n" % t["stderr"]
        if pre:
            text = text.replace("\n\npackage main", "\n" + pre + "\npackage main", 1)
        path = os.path.join(OUT, name + ".gp")
        old = open(path).read() if os.path.exists(path) else None
        if old != text:
            open(path, "w").write(text)
        n += 1
    print("wrote %d tests; %d problems" % (n, len(bad)))
    for name, why in bad:
        print("PROBLEM", name, why)
    return bad
