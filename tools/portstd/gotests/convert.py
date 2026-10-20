#!/usr/bin/env python3
"""Makes a module that runs Go 1.25.14's own tests of a ported package with gocpp test: tests/gotest/testdata/<pkg>_internal.

usage: python3 tools/portstd/gotests/convert.py PKG [OUTDIR]     (from the repository root; PKG is an import path such as encoding/json)

The module (module path example.com/m) holds a copy of the port (stdlib/PKG/*.gp, so that the tests of package PKG, which name unexported
functions and fields, run in it) and Go's test files with the patches of packages/PKG.py (each patch has its reason). tests/gotest runs
the module with gocpp test and with real go test (the .gp files renamed to .go) and compares the output. shim/PKG/ holds the files
and small packages that stand in for the ones gocpp does not have (math/rand, image, math/big, log...). The Go tarball must be in the
cache of tools/portstd (any earlier run of it put it there); the script never downloads."""
import os, re, sys, shutil, glob

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..', '..'))
if len(sys.argv) < 2:
    raise SystemExit(__doc__)
PKG = sys.argv[1]
KEY = PKG.replace('/', '__')
pin = dict(l.split() for l in open(os.path.join(ROOT, 'tools', 'portstd', 'PINS')) if l.split() and l.split()[0] in ('version', 'sha256'))
cache = os.path.join(os.environ.get('XDG_CACHE_HOME') or os.path.expanduser('~/.cache'), 'gocpp', 'portstd')
GO = os.path.join(cache, '%s-%s' % (pin['version'], pin['sha256'][:12]), 'src', *PKG.split('/'))
PORT = os.path.join(ROOT, 'stdlib', *PKG.split('/'))
out = sys.argv[2] if len(sys.argv) > 2 else os.path.join(ROOT, 'tests', 'gotest', 'testdata', PKG.replace('/', '_') + '_internal')

FILES = []        # the _test.go files taken from Go
DROP_FUNCS = {}   # file -> {top-level function: reason}
PATCHES = []      # (file, old text, new text, count, reason)
ARGS = ['-v', '.']
PORT_PATCHES = [] # (old, new): changes of the copy of the port (the check of tests/gotest applies the same ones to the port): an import of internal/... becomes one of a copy in the module
SHARED = []       # shim/_shared/NAME folders that the module gets as NAME/
EXTRA_PORTS = []  # (package folder in stdlib/, folder in the module): another port that the tests need, copied from stdlib/ with PORT_PATCHES (tests/gotest checks it like the main port)
EXTRA = []        # (package folder under Go's src, folder in the module): packages of Go's tree that the tests import (internal/testhash)

def drop(f, name, reason):
    DROP_FUNCS.setdefault(f, {})[name] = reason

def patch(f, old, new, reason, count=1):
    PATCHES.append((f, old, new, count, reason))

exec(open(os.path.join(HERE, 'packages', KEY + '.py')).read())

def remove_func(src, name):
    """The top-level func (or method) named name, with its doc comment, is cut out."""
    m = re.search(r'(?m)^func (\([^)]*\) )?%s\b' % re.escape(name), src)
    if not m:
        raise SystemExit('function %s not found' % name)
    start = m.start()
    before = src[:start].split('\n')
    k = len(before) - 1
    while k > 0 and before[k - 1].startswith('//'):
        k -= 1
    start = len('\n'.join(before[:k])) + (1 if k > 0 else 0)
    end = src.index('\n}\n', m.start()) + 3
    return src[:start] + src[end:]

shutil.rmtree(out, ignore_errors=True)
os.makedirs(out)
open(os.path.join(out, 'gocpp.mod'), 'w').write('module example.com/m\n\ngo 1.25\n')
for f in sorted(glob.glob(PORT + '/*.gp')):
    text = open(f, encoding='utf8').read()
    for (old, new) in PORT_PATCHES:
        text = text.replace(old, new)
    open(os.path.join(out, os.path.basename(f)), 'w', encoding='utf8').write(text)
for (src_pkg, dst) in EXTRA_PORTS:
    os.makedirs(os.path.join(out, dst), exist_ok=True)
    for f in sorted(glob.glob(os.path.join(ROOT, 'stdlib', *src_pkg.split('/'), '*.gp'))):
        text = open(f, encoding='utf8').read()
        for (old, new) in PORT_PATCHES:
            text = text.replace(old, new)
        open(os.path.join(out, dst, os.path.basename(f)), 'w', encoding='utf8').write(text)
if PORT_PATCHES or EXTRA_PORTS:
    import json
    open(os.path.join(out, 'portpatches.json'), 'w').write(json.dumps({'patches': PORT_PATCHES, 'extra': EXTRA_PORTS}, indent=1) + '\n')
for f in FILES:
    src = open(os.path.join(GO, f), encoding='utf8').read()
    for name in DROP_FUNCS.get(f, {}):
        src = remove_func(src, name)
    for (pf, old, new, count, reason) in PATCHES:
        if pf != f:
            continue
        n = src.count(old)
        if n != count:
            raise SystemExit('%s: patch %r found %d times, want %d' % (f, old[:60], n, count))
        src = src.replace(old, new)
    open(os.path.join(out, f.replace('.go', '.gp')), 'w', encoding='utf8').write(src)
for name in SHARED:
    shutil.copytree(os.path.join(HERE, 'shim', '_shared', name), os.path.join(out, name))
for (src_pkg, dst) in EXTRA:
    d = os.path.join(out, dst)
    os.makedirs(d, exist_ok=True)
    srcdir = os.path.join(GO[: GO.index('/src/') + 4], *src_pkg.split('/'))
    for f in sorted(glob.glob(os.path.join(srcdir, '*.go'))):
        if f.endswith('_test.go'):
            continue
        text = open(f, encoding='utf8').read()
        key = dst + '/' + os.path.basename(f)
        for (pf, old, new, count, reason) in PATCHES:
            if pf == key:
                if text.count(old) != count:
                    raise SystemExit('%s: patch %r found %d times, want %d' % (key, old[:60], text.count(old), count))
                text = text.replace(old, new)
        open(os.path.join(d, os.path.basename(f)[:-3] + '.gp'), 'w', encoding='utf8').write(text)
shimdir = os.path.join(HERE, 'shim', KEY)
for root, dirs, files in (os.walk(shimdir) if os.path.isdir(shimdir) else []):
    rel = os.path.relpath(root, shimdir)
    os.makedirs(os.path.join(out, rel), exist_ok=True)
    for f in files:
        shutil.copy(os.path.join(root, f), os.path.join(out, rel, f))
open(os.path.join(out, 'args'), 'w').write('\n'.join(ARGS) + '\n')
