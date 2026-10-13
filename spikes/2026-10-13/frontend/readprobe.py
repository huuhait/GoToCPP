import sys, json
dec = json.JSONDecoder(); data = sys.stdin.read(); i = 0
def find_value(n):
    if isinstance(n, dict):
        if n.get("kind") == "ConstantExpr" and "value" in n:
            return n["value"]
        for v in n.values():
            r = find_value(v)
            if r is not None:
                return r
    elif isinstance(n, list):
        for v in n:
            r = find_value(v)
            if r is not None:
                return r
    return None
while i < len(data):
    while i < len(data) and data[i] in " \n\r\t":
        i += 1
    if i >= len(data):
        break
    obj, i = dec.raw_decode(data, i)
    t = obj.get("type", {})
    if obj.get("kind") == "TypeAliasDecl":
        print("  %s: %s" % (obj["name"], t.get("desugaredQualType", t.get("qualType"))))
    elif obj.get("kind") == "VarDecl":
        print("  %s: %s = %s" % (obj["name"], t.get("qualType"), find_value(obj)))
