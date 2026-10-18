import sys, importlib
sys.path.insert(0, ".")
import lib
mods = sys.argv[1].split(",") if len(sys.argv) > 1 else ["cats1"]
only = sys.argv[2:] or None
for m in mods:
    importlib.import_module(m)
ok = lib.emit(only)
print(len(lib.tests), "tests", "OK" if ok else "FAILED")
