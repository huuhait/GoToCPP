#!/usr/bin/env python3
# Converts the scenario .gp programs (with !) to plain Go that calls Hop (throwaway, design note 10.7a).
import re, sys

ZERO = {"int": "0", "string": '""', "bool": "false", "": None}

def zero_of(t):
    t = t.strip()
    return ZERO.get(t, "nil")

def parse_results(r):
    r = r.strip()
    if r.startswith("("):
        parts = [p.strip() for p in r.strip("()").split(",")]
        types = []
        for p in parts:
            types.append(p.split()[-1])
        return types
    return [r] if r else []

def convert(src):
    out = []
    stack = []  # (depth, info) info = dict(kind, zeros, must, named_err)
    depth = 0
    n = 0
    thunk_next = False
    for line in src.splitlines():
        indent = re.match(r"\s*", line).group(0)
        body = line.strip()
        m = re.match(r"(?:(\w+)\s*:?=\s*)?func(?:\s+(?:\([^)]*\)\s*)?(\w+)(!?))?\s*\(([^)]*)\)\s*(.*?)\s*\{$", body)
        new = line
        if body == "// sim: thunk":
            thunk_next = True
        if m and body.startswith(("func", )) or (re.search(r":?= func\(", body) and body.endswith("{")):
            m2 = re.match(r"(.*?func\s*(?:\([^)]*\)\s*(?=\w))?(\w*)(!?)\s*)\(([^)]*)\)\s*(.*?)\s*\{$", body)
            head, name, bang, params, res = m2.groups()
            types = parse_results(res)
            if bang and types and types[-1] == "error":
                new = indent + head.replace("!", "") + "(" + params + ") " + res + " {"
                info = dict(must=False, zeros=[zero_of(t) for t in types[:-1]], ret_err=True, named=True, name=name)
            elif bang:
                zeros = [zero_of(t) for t in types]
                newres = "(" + ", ".join(types + ["error"]) + ")" if types else "error"
                new = indent + head.replace("!", "") + "(" + params + ") " + newres + " {"
                info = dict(must=True, zeros=zeros, ret_err=True, named=False, name=name, noresult=not types)
            else:
                info = dict(must=False, zeros=[zero_of(t) for t in types[:-1]] if types and types[-1] == "error" else [zero_of(t) for t in types],
                            ret_err=bool(types and types[-1] == "error"), named=bool(re.search(r"\berr error", res)), name=name)
            info["thunk"] = thunk_next
            thunk_next = False
            stack.append((depth, info))
        else:
            info = stack[-1][1] if stack else None
            if info is not None:
                # defer X!(args)
                md = re.match(r"defer ([\w.]+)!\((.*)\)$", body)
                ms = re.match(r"(?:(\w+)\s*(:?=)\s*)?([\w.]+)!\((.*)\)$", body)
                if md:
                    n += 1
                    new = (indent + "defer func() { if e := %s(%s); e != nil && err == nil { HopAs(e, \"main.%s\"); err = e } }()" % (md.group(1), md.group(2), info["name"]))
                elif ms:
                    n += 1
                    var, op, fn, args = ms.groups()
                    e = "e_%d" % n
                    hop = "HopT" if info.get("thunk") else "Hop"
                    zs = ", ".join(info["zeros"] + [e]) if info["ret_err"] else None
                    if info["ret_err"]:
                        fail = "return " + (zs if info["zeros"] else e)
                    else:
                        fail = "panic(%s)" % e
                    if var:
                        new = indent + "%s, %s %s %s(%s); if %s != nil { %s(%s); %s }" % (var, e, ":=" if op == ":=" else "=", fn, args, e, hop, e, fail)
                        if op == "=":
                            new = indent + "var %s error; %s, %s = %s(%s); if %s != nil { %s(%s); %s }" % (e, var, e, fn, args, e, hop, e, fail)
                    else:
                        # statement: the call returns only an error, or values to drop
                        new = indent + "if %s := %s(%s); %s != nil { %s(%s); %s }" % (e, fn, args, e, hop, e, fail)
                elif info["must"] and re.match(r"return\b", body):
                    if body == "return":
                        new = indent + "return nil"
                    else:
                        new = indent + body + ", nil"
        parts = new.split('"')
        for i in range(0, len(parts), 2):  # outside string literals only
            parts[i] = parts[i].replace("errors.Trace(", "Trace(").replace("errors.Traced(", "Traced(").replace("errors.Wrapf(", "Wrapf(").replace("errors.Join(", "Join(").replace("fmt.Errorf(", "Errorf(")
        new = '"'.join(parts)
        if body == "}" and stack and depth - 1 == stack[-1][0] and stack[-1][1].get("noresult"):
            new = indent + "return nil }"
        out.append(new)
        depth += body.count("{") - body.count("}")
        while stack and depth <= stack[-1][0] and body.startswith("}"):
            stack.pop()
    return "\n".join(out) + "\n"

if __name__ == "__main__":
    sys.stdout.write(convert(open(sys.argv[1]).read()))
