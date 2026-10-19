#!/usr/bin/env python3
"""shadowfuzz: random Go programs that shadow the names of results, parameters and receivers.

usage: gen.py OUTDIR COUNT [SEED] [--chan]

Writes OUTDIR/pNNNN.go (valid Go that prints what every function returns). Compare `go run`
with `gocpp run` on each one (run.sh). The programs hunt for a lowering that refers to a Go
variable by its C++ name where an inner variable of the same name is in scope (named results
and defers: the bug of fix/named-result-shadow). With --chan the functions also use select,
for range over a channel and goroutines (Phase 08).
"""
import os
import random
import sys

NAMES = ["i", "j", "x", "v", "s", "r", "p", "n", "err", "ok", "t", "u"]
TYPES = ["int", "string", "bool", "[]int", "P", "any"]


class Gen:
    def __init__(self, rnd, chan):
        self.r = rnd
        self.chan = chan
        self.scopes = []  # list of dict name -> type
        self.resnames = {}  # result name -> type
        self.uid = 0
        self.lines = []
        self.ind = 1
        self.defer_depth = 0
        self.rname = None
        self.returned = False

    # ---- helpers
    def emit(self, s):
        self.lines.append("\t" * self.ind + s)

    def lookup(self, name):
        for sc in reversed(self.scopes):
            if name in sc:
                return sc[name]
        return None

    def visible(self, typ):
        seen = set()
        out = []
        for sc in reversed(self.scopes):
            for n, t in sc.items():
                if n in seen:
                    continue
                seen.add(n)
                if t.rstrip("#") == typ:
                    out.append(n)
        return out

    def lit(self, typ):
        r = self.r
        if typ == "E":
            return "*new(E)"
        if typ == "int":
            return str(r.randint(0, 9))
        if typ == "string":
            return '"%s"' % r.choice(["a", "b", "cd", ""])
        if typ == "bool":
            return r.choice(["true", "false"])
        if typ == "[]int":
            return "[]int{%s}" % ", ".join(str(r.randint(0, 9)) for _ in range(r.randint(0, 3)))
        if typ == "P":
            return "(P{%d, %d})" % (r.randint(0, 9), r.randint(0, 9))
        return r.choice(["any(1)", 'any("z")', "any(P{1, 2})"])

    def expr(self, typ, depth=0):
        r = self.r
        vs = self.visible(typ)
        if typ == "int" and self.rname and self.lookup(self.rname) == "T" and r.random() < 0.25:
            return "%s.%s" % (self.rname, r.choice(["i", "j", "x", "v"]))
        if vs and r.random() < 0.7:
            return r.choice(vs)
        if depth < 2 and r.random() < 0.4:
            if typ == "int":
                a = self.expr("int", depth + 1)
                k = r.randint(0, 3)
                if k == 0:
                    return "%s + %s" % (a, self.expr("int", depth + 1))
                if k == 1:
                    return "%s * %d" % (a, r.randint(2, 5))
                if k == 2:
                    return "len(%s)" % self.expr("[]int", depth + 1)
                return "%s.A" % self.expr("P", depth + 1)
            if typ == "string":
                return "%s + %s" % (self.expr("string", depth + 1), self.expr("string", depth + 1))
            if typ == "bool":
                return "%s < %s" % (self.expr("int", depth + 1), self.expr("int", depth + 1))
            if typ == "[]int":
                return "append(%s, %s)" % (self.expr("[]int", depth + 1), self.expr("int", depth + 1))
            if typ == "P":
                return "(P{%s, %s})" % (self.expr("int", depth + 1), self.expr("int", depth + 1))
            if typ == "any":
                return "any(%s)" % self.expr(r.choice(["int", "string", "[]int"]), depth + 1)
        return self.lit(typ)

    def newname(self, prefer_taken=True):
        r = self.r
        taken = [n for n in NAMES if self.lookup(n) is not None]
        if taken and prefer_taken and r.random() < 0.75:
            return r.choice(taken)
        return r.choice(NAMES)

    def use(self, name):
        self.emit("_ = %s" % name)

    # ---- statements
    def block(self, depth, body_fn=None):
        self.scopes.append({})
        self.ind += 1
        n = self.r.randint(1, 3)
        if body_fn:
            body_fn()
        self.returned = False
        for _ in range(n):
            self.stmt(depth + 1)
            if self.returned:
                break  # (no dead code after a return)
        self.returned = False
        self.ind -= 1
        self.scopes.pop()

    def decl_in_scope(self, name, typ):
        self.scopes[-1][name] = typ

    def stmt(self, depth):
        r = self.r
        k = r.random()
        if depth >= 3:
            k = r.choice([0.1, 0.2, 0.9, 0.95])
        if k < 0.20:
            return self.s_return()
        if k < 0.30:
            return self.s_assign()
        if k < 0.38:
            return self.s_for_range(depth)
        if k < 0.46:
            return self.s_for_clause(depth)
        if k < 0.54:
            return self.s_if_init(depth)
        if k < 0.60:
            return self.s_switch_init(depth)
        if k < 0.65:
            return self.s_typeswitch(depth)
        if k < 0.72:
            return self.s_closure_call(depth)
        if k < 0.78:
            return self.s_block(depth)
        if k < 0.82:
            return self.s_define(depth)
        if k < 0.86 and self.chan:
            return self.s_chan(depth)
        if k < 0.88:
            return self.s_capture(depth)
        if k < 0.90:
            return self.s_defer(depth)
        if k < 0.93:
            return self.s_panic()
        return self.s_assign()

    def s_return(self):
        r = self.r
        self.returned = True
        rets = self.rets
        if not rets:
            self.emit("return")
            return
        if self.resnames and r.random() < 0.25:
            # a bare return needs every named result to be visible
            ok = all(self.lookup(n) == t and self.is_result(n) for n, t in self.resnames.items())
            if ok:
                self.emit("return")
                return
        self.emit("return " + ", ".join(self.expr(t) for t in rets))

    def is_result(self, name):
        # the innermost visible variable of that name is the result (the outermost scope)
        for i in range(len(self.scopes) - 1, -1, -1):
            if name in self.scopes[i]:
                return i == self.res_scope
        return False

    def s_assign(self):
        r = self.r
        if self.rname and self.lookup(self.rname) == "T" and r.random() < 0.2:
            return self.emit("%s.%s %s %s" % (self.rname, r.choice(["i", "j", "x", "v"]), r.choice(["=", "+="]), self.expr("int")))
        seen = {}
        for sc in self.scopes:
            seen.update(sc)
        seen = {n: t for n, t in seen.items() if t in TYPES}
        if not seen:
            return self.emit("_ = 0")
        n = r.choice(list(seen))
        t = seen[n]
        if r.random() < 0.3 and t == "int":
            self.emit("%s++" % n)
        elif r.random() < 0.3 and len(seen) > 1:
            m = r.choice(list(seen))
            if seen[m] == t and m != n:
                self.emit("%s, %s = %s, %s" % (n, m, m, n))
                return
            self.emit("%s = %s" % (n, self.expr(t)))
        else:
            self.emit("%s = %s" % (n, self.expr(t)))

    def s_define(self, depth):
        n = self.newname()
        t = self.r.choice(TYPES[:5])
        e = self.expr(t)
        # a := that is a new variable in this scope, or an assignment of an existing one
        if n in self.scopes[-1]:
            if self.scopes[-1][n] in TYPES:
                self.emit("%s = %s" % (n, self.expr(self.scopes[-1][n])))
            return
        self.emit("%s := %s" % (n, e))
        self.decl_in_scope(n, t)
        self.use(n)

    def s_for_range(self, depth):
        r = self.r
        n, m = self.newname(), self.newname()
        what = r.choice(["slice", "int", "string", "map"])
        if what == "slice":
            src = self.expr("[]int")
            self.emit("for %s, %s := range %s {" % (n, m, src) if n != m else "for %s := range %s {" % (n, src))
            self.block(depth, lambda: self.bind([(n, "int")] + ([(m, "int")] if n != m else [])))
        elif what == "int":
            self.emit("for %s := range %d {" % (n, r.randint(1, 3)))
            self.block(depth, lambda: self.bind([(n, "int")]))
        elif what == "string":
            self.emit("for %s, %s := range %s {" % (n, m, self.expr("string")) if n != m else "for %s := range %s {" % (n, self.expr("string")))
            self.block(depth, lambda: self.bind([(n, "int")] + ([(m, "rune")] if n != m else [])))
        else:
            self.emit("for %s, %s := range map[string]int{\"k\": %s} {" % (n, m, self.expr("int")) if n != m else "for %s := range map[string]int{\"k\": 1} {" % n)
            self.block(depth, lambda: self.bind([(n, "string")] + ([(m, "int")] if n != m else [])))
        self.emit("}")

    def bind(self, pairs):
        for n, t in pairs:
            self.decl_in_scope(n, t)
            self.use(n)

    def s_for_clause(self, depth):
        n = self.newname()
        self.emit("for %s := 0; %s < %d; %s++ {" % (n, n, self.r.randint(1, 3), n))
        # (int#: the loop variable is read only, an assignment could make the loop endless)
        self.block(depth, lambda: self.bind([(n, "int#")]))
        self.emit("}")

    def s_if_init(self, depth):
        r = self.r
        n = self.newname()
        t = r.choice(["int", "string"])
        self.scopes.append({})
        self.emit("if %s := %s; %s {" % (n, self.expr(t), self.cond(n, t)))
        self.decl_in_scope(n, t)
        self.block(depth, lambda: self.use(n))
        if r.random() < 0.5:
            self.emit("} else {")
            self.block(depth, lambda: self.use(n))
        self.emit("}")
        self.scopes.pop()

    def cond(self, n, t):
        if t == "int":
            return "%s > %d" % (n, self.r.randint(0, 6))
        return '%s != ""' % n

    def s_switch_init(self, depth):
        n = self.newname()
        self.scopes.append({})
        self.emit("switch %s := %s; {" % (n, self.expr("int")))
        self.decl_in_scope(n, "int")
        self.emit("case %s > %d:" % (n, self.r.randint(0, 5)))
        self.block(depth, lambda: self.use(n))
        self.emit("default:")
        self.block(depth, lambda: self.use(n))
        self.emit("}")
        self.scopes.pop()

    def s_typeswitch(self, depth):
        n = self.newname()
        self.emit("switch %s := %s.(type) {" % (n, self.expr("any")))
        for head, t in (("case int:", "int"), ("case string:", "string"), ("default:", "any")):
            self.emit(head)
            self.block(depth, lambda: self.bind([(n, t)]))
        self.emit("}")

    def s_block(self, depth):
        self.emit("{")
        self.block(depth)
        self.emit("}")

    def s_closure_call(self, depth):
        # a func literal with its own results and parameters: returns stay inside it
        r = self.r
        n = self.newname()
        rn = self.newname()
        saved = (self.rets, self.resnames, self.res_scope)
        self.rets = ["int"]
        self.resnames = {}
        self.emit("%s := func(%s int) (%s int) {" % ("gp_unused_" + str(self.uid), n, rn) if False else "func(%s int) (%s int) {" % (n, rn) if n != rn else "func(%s int) int {" % n)
        self.scopes.append({})
        self.res_scope = len(self.scopes) - 1
        self.decl_in_scope(n, "int")
        if n != rn:
            self.decl_in_scope(rn, "int")
            self.resnames = {rn: "int"}
        self.ind += 1
        if r.random() < 0.5:
            self.emit("defer func() {}()")
        for _ in range(r.randint(1, 3)):
            self.stmt(depth + 1)
        self.emit("return %s" % self.expr("int"))
        self.ind -= 1
        self.scopes.pop()
        self.rets, self.resnames, self.res_scope = saved
        self.emit("}(%s)" % self.expr("int"))

    def s_capture(self, depth):
        # closures that capture a loop variable, called after the loop (a copy for each iteration)
        r = self.r
        n = self.newname()
        fs = "fs%d" % self.uid
        self.uid += 1
        self.emit("var %s []func() int" % fs)
        if r.random() < 0.5:
            self.emit("for %s := 0; %s < 3; %s++ {" % (n, n, n))
        else:
            self.emit("for %s := range 3 {" % n)
        self.block(depth, lambda: (self.bind([(n, "int#")]), self.emit("%s = append(%s, func() int { %s++; return %s })" % (fs, fs, n, n))))
        self.emit("}")
        targets = [t for t in self.visible("int") if self.lookup(t) == "int"]
        if targets:
            t = r.choice(targets)
            self.emit("for _, f := range %s {" % fs)
            self.emit("\t%s += f()" % t)
            self.emit("}")
        else:
            self.emit("_ = %s" % fs)

    def s_defer(self, depth):
        # a deferred closure that changes variables by name
        r = self.r
        self.emit("defer func() {")
        self.scopes.append({})
        self.ind += 1
        for _ in range(r.randint(1, 2)):
            if r.random() < 0.5:
                self.s_assign()
            else:
                self.s_define(depth + 1)
        self.ind -= 1
        self.scopes.pop()
        self.emit("}()")

    def s_panic(self):
        self.emit("if %s > 7 {" % self.expr("int"))
        self.emit('\tpanic("boom")')
        self.emit("}")

    def s_chan(self, depth):
        r = self.r
        n = self.newname()
        if n == "ok":
            n = "t"
        c = "ch%d" % self.uid
        self.uid += 1
        self.emit("%s := make(chan int, 3)" % c)
        self.emit("%s <- %s" % (c, self.expr("int")))
        if r.random() < 0.5:
            self.emit("select {")
            self.emit("case %s, ok := <-%s:" % (n, c))
            self.block(depth, lambda: self.bind([(n, "int"), ("ok", "bool")]))
            self.emit("default:")
            self.block(depth)
            self.emit("}")
        else:
            self.emit("close(%s)" % c)
            self.emit("for %s := range %s {" % (n, c))
            self.block(depth, lambda: self.bind([(n, "int")]))
            self.emit("}")

    # ---- functions
    def function(self, idx):
        r = self.r
        self.scopes = [{}]
        self.res_scope = 0
        self.rname = None
        pool = NAMES[:]
        r.shuffle(pool)
        nparams = r.randint(0, 2)
        nres = r.randint(1, 3)
        recv = r.random() < 0.35
        ptr = r.random() < 0.6
        generic = not recv and r.random() < 0.3
        params = []
        for _ in range(nparams):
            params.append((pool.pop(), r.choice(TYPES[:5])))
        if generic:
            params.append((pool.pop(), "E"))
        named = r.random() < 0.8
        results = []
        for _ in range(nres):
            results.append((pool.pop() if named else "", r.choice(TYPES + ["E"] if generic else TYPES)))
        rname = pool.pop() if recv else None
        if named and r.random() < 0.15:
            results = [("_" if r.random() < 0.3 else n, t) for n, t in results]
        self.rets = [t for _, t in results]
        self.resnames = {n: t for n, t in results if n and n != "_"}
        sig = ", ".join("%s %s" % (n, t) for n, t in params)
        rs = ", ".join(("%s %s" % (n, t)) if named else t for n, t in results)
        name = "f%d" % idx
        if recv:
            head = "func (%s %sT) %s(%s) (%s) {" % (rname, "*" if ptr else "", name, sig, rs)
            self.rname = rname
            self.scopes[0][rname] = "T"
        else:
            head = "func %s%s(%s) (%s) {" % (name, "[E any]" if generic else "", sig, rs)
        for n, t in params:
            self.scopes[0][n] = t
        for n, t in results:
            if n and n != "_":
                self.scopes[0][n] = t
        self.lines = [head]
        self.ind = 1
        if recv:
            self.emit("_ = %s" % rname)
        for n, t in params:
            self.emit("_ = %s" % n)
        has_defer = r.random() < 0.8
        if has_defer:
            rec = r.random() < 0.6
            self.emit("defer func() {")
            if rec:
                self.emit("\tif e := recover(); e != nil {")
                self.emit("\t\tfmt.Println(\"recovered\", e)")
                self.emit("\t}")
            for n, t in results:
                if n and n != "_" and r.random() < 0.5:
                    if t == "int":
                        self.emit("\t%s += 100" % n)
                    elif t == "string":
                        self.emit("\t%s += \"!\"" % n)
                    elif t == "bool":
                        self.emit("\t%s = !%s" % (n, n))
            self.emit("}()")
        for n, t in results:
            if n and n != "_":
                self.emit("_ = %s" % n)
        for _ in range(r.randint(2, 5)):
            self.stmt(0)
        # the end: return
        if self.rets:
            self.emit("return " + ", ".join(self.expr(t) for t in self.rets))
        self.lines.append("}")
        body = "\n".join(self.lines)
        # the call in main
        targ = r.choice(["int", "string", "P"])
        args = ", ".join(("*new(%s)" % targ) if t == "E" else self.lit(t) for _, t in params)
        if recv:
            call = "(&T{1, 2, 3, 4}).%s(%s)" % (name, args) if ptr else "T{1, 2, 3, 4}.%s(%s)" % (name, args)
        else:
            call = "%s%s(%s)" % (name, "[%s]" % targ if generic else "", args)
        return body, name, call, len(results)


def program(seed, chan):
    rnd = random.Random(seed)
    g = Gen(rnd, chan)
    out = ["package main", "", 'import "fmt"', "", "type P struct{ A, B int }", "", "type T struct{ i, j, x, v int }", ""]
    calls = []
    nf = rnd.randint(3, 6)
    for i in range(nf):
        body, name, call, nres = g.function(i)
        out.append(body)
        out.append("")
        calls.append((name, call, nres))
    out.append("func main() {")
    for name, call, nres in calls:
        out.append('\tfunc() {')
        out.append('\t\tdefer func() {')
        out.append('\t\t\tif e := recover(); e != nil {')
        out.append('\t\t\t\tfmt.Println("%s panic:", e)' % name)
        out.append('\t\t\t}')
        out.append('\t\t}()')
        out.append('\t\tfmt.Println("%s", %s)' % (name, ", ".join("a%d" % i for i in range(nres))) if False else "\t\tfmt.Println(\"%s\")" % name)
        out.append("\t\tfmt.Println(%s)" % call)
        out.append("\t}()")
    out.append("}")
    return "\n".join(out) + "\n"


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    chan = "--chan" in sys.argv
    outdir, count = args[0], int(args[1])
    seed = int(args[2]) if len(args) > 2 else 1
    os.makedirs(outdir, exist_ok=True)
    for i in range(count):
        with open(os.path.join(outdir, "p%04d.go" % i), "w") as f:
            f.write(program(seed * 100000 + i, chan))


if __name__ == "__main__":
    main()
