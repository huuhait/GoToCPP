import sys, importlib
sys.path.insert(0, ".")
import lib
mods = sys.argv[1].split(",")
for m in mods:
    importlib.import_module(m)
only = sys.argv[2:] or None
bad = lib.emit(only)
sys.exit(1 if bad else 0)
