# Prints the evaluated value of every gp_probe_* declaration in a filtered clang JSON dump.
# Usage: clang++ -std=c++20 -fsyntax-only -Xclang -ast-dump=json \
#          -Xclang -ast-dump-filter=gp_probe values.cpp 2>/dev/null | python3 readvalues.py
import json
import sys

KINDS = ("ConstantExpr", "StringLiteral", "FloatingLiteral")


def values(node, out):
    if isinstance(node, dict):
        if node.get("kind") in KINDS and "value" in node:
            out.append((node["kind"], node["value"], node["type"]["qualType"]))
        for v in node.values():
            values(v, out)
    elif isinstance(node, list):
        for v in node:
            values(v, out)


text, dec, i = sys.stdin.read(), json.JSONDecoder(), 0
while i < len(text):
    while i < len(text) and text[i].isspace():
        i += 1
    if i >= len(text):
        break
    decl, i = dec.raw_decode(text, i)  # the filtered dump is a stream of JSON objects
    found = []
    values(decl, found)
    print(f"{decl['name']}  (type name: {decl['type']['qualType']})")
    for kind, value, qual in found[:1]:
        print(f"    {kind} value = {value}  [{qual}]")
