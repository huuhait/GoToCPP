#!/usr/bin/env python3
"""Makes the test files that run Go 1.25.14's own tests of a standard package with gocpp test:
tests/stdgo/<pkg>/ (step 9.10).

usage: python3 tools/portstd/gotests/convert_std.py PKG...
(from the repository root; PKG is an import path such as strings)

Unlike convert.py (a copy of the port in a module), the tests run in the package that gocpp ships:
tests/stdgo is the value of GOCPP_TEST_STDLIB, and gocpp test strings adds
tests/stdgo/strings/*_test.gp to the embedded package (cmd/gocpp/stdtests.go). Internal tests
(package strings) see the unexported names, external tests (package strings_test) import the
package as usual.

For each package the script reads std/PKG.py (the specification: files, patches, dropped functions,
expected failures, each with its reason), takes Go's test files from the pinned tarball of
tools/portstd (the cache; the script never downloads), applies the patches, writes them as .gp
files, copies the testdata folder and the shim packages the tests import (std-shim/, packages such
as internal/testenv that Go's tree has and gocpp does not), and writes manifest/PKG.json: every
Test, Example, Fuzz and Benchmark function of Go's test files of the package for linux/amd64 with
what happened to it. tools/stdgocount reads the manifest and the output of gocpp test -v and
computes the counts of the counting rule (Phase 09 README, 2.8)."""
import glob
import json
import os
import re
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..', '..'))
# GOCPP_TEST_STDGO_OUT: tests/stdgo/stdgo_test.go makes the tests in a temporary folder to compare
# them with the committed ones.
OUT = os.environ.get('GOCPP_TEST_STDGO_OUT') or os.path.join(ROOT, 'tests', 'stdgo')

pin = dict(line.split() for line in open(os.path.join(ROOT, 'tools', 'portstd', 'PINS'))
           if line.split() and line.split()[0] in ('version', 'sha256'))
cache = os.path.join(os.environ.get('XDG_CACHE_HOME') or os.path.expanduser('~/.cache'),
                     'gocpp', 'portstd')
GOSRC = os.path.join(cache, '%s-%s' % (pin['version'], pin['sha256'][:12]), 'src')

# The build constraints of a test file for linux/amd64 with cgo: what go test on the usual machine
# of a Go developer selects.
TAGS = {'linux', 'unix', 'amd64', 'gc', 'cgo'} | {'go1.%d' % i for i in range(1, 26)}
# The tags for the test-only copies of Go's packages: the pure-Go variants (no assembly).
EXTRA_TAGS = TAGS | {'purego', 'math_big_pure_go'}
KNOWN_OS = {
    'aix', 'android', 'darwin', 'dragonfly', 'freebsd', 'hurd', 'illumos', 'ios', 'js', 'linux',
    'nacl', 'netbsd', 'openbsd', 'plan9', 'solaris', 'wasip1', 'windows', 'zos',
}
KNOWN_ARCH = {
    '386', 'amd64', 'amd64p32', 'arm', 'armbe', 'arm64', 'arm64be', 'loong64', 'mips', 'mipsle',
    'mips64', 'mips64le', 'mips64p32', 'mips64p32le', 'ppc', 'ppc64', 'ppc64le', 'riscv', 'riscv64',
    's390', 's390x', 'sparc', 'sparc64', 'wasm',
}


def name_allowed(fname):
    """The GOOS/GOARCH suffix rule of go/build for a file name such as x_linux_test.go."""
    parts = fname[:-len('.go')].split('_')
    if parts and parts[-1] == 'test':
        parts = parts[:-1]
    if len(parts) >= 3 and parts[-2] in KNOWN_OS and parts[-1] in KNOWN_ARCH:
        return parts[-2] == 'linux' and parts[-1] == 'amd64'
    if len(parts) >= 2 and parts[-1] in KNOWN_OS:
        return parts[-1] == 'linux'
    if len(parts) >= 2 and parts[-1] in KNOWN_ARCH:
        return parts[-1] == 'amd64'
    return True


def eval_constraint(expr, tags=None):
    """Evaluates the expression of a //go:build line with TAGS."""
    toks = re.findall(r'\|\||&&|!|\(|\)|[A-Za-z0-9_.]+', expr)
    pos = [0]

    def peek():
        return toks[pos[0]] if pos[0] < len(toks) else None

    def take():
        t = toks[pos[0]]
        pos[0] += 1
        return t

    def parse_or():
        v = parse_and()
        while peek() == '||':
            take()
            r = parse_and()
            v = v or r
        return v

    def parse_and():
        v = parse_not()
        while peek() == '&&':
            take()
            r = parse_not()
            v = v and r
        return v

    def parse_not():
        if peek() == '!':
            take()
            return not parse_not()
        if peek() == '(':
            take()
            v = parse_or()
            take()
            return v
        return take() in (tags or TAGS)

    return parse_or()


def file_allowed(path, tags=None):
    if not name_allowed(os.path.basename(path)):
        return False
    for line in open(path, encoding='utf8'):
        if line.startswith('package '):
            break
        m = re.match(r'//go:build (.*)', line)
        if m:
            return eval_constraint(m.group(1), tags)
    return True


def go_test_funcs(src):
    """The Test, Example, Fuzz and Benchmark functions of a test file: (kind, name)."""
    funcs = []
    pattern = r'(?m)^func (Test|Example|Fuzz|Benchmark)([A-Z_0-9][A-Za-z0-9_]*)?\('
    for m in re.finditer(pattern, src):
        if m.group(1) + (m.group(2) or '') == 'TestMain':
            continue  # TestMain(m *testing.M) is the entry point of the binary, not a test
        kind = m.group(1).lower()
        if kind == 'example':
            end = src.find('\n}\n', m.start())
            body = src[m.start():end if end >= 0 else len(src)]
            if not re.search(r'(?mi)^\s*// (unordered )?output:', body):
                # go test compiles an example without an output comment and does not run it.
                kind = 'compileonly'
        funcs.append((kind, m.group(1) + (m.group(2) or '')))
    return funcs


def remove_func(src, name, replacement=''):
    """The top-level func named name (or the method Type.name), with its doc comment, is cut out
    (and the text put in its place)."""
    if '.' in name:
        recv, meth = name.split('.')
        m = re.search(r'(?m)^func \(\w+ \*?%s\) %s\b' % (re.escape(recv), re.escape(meth)), src)
    else:
        m = re.search(r'(?m)^func %s\b' % re.escape(name), src)
    if not m:
        raise SystemExit('function %s not found' % name)
    before = src[:m.start()].split('\n')
    k = len(before) - 1
    while k > 0 and before[k - 1].startswith('//'):
        k -= 1
    start = len('\n'.join(before[:k])) + (1 if k > 0 else 0)
    line_end = src.index('\n', m.start())
    if src[m.start():line_end].rstrip().endswith('}'):
        end = line_end + 1  # a function on one line
    else:
        end = src.index('\n}\n', m.start()) + 3
    return src[:start] + replacement + src[end:]


# Go's own packages that gocpp does not ship and that Go's tests import: they are copied from the
# tarball as test-only packages.
AUTO_EXTRA = {'testing/iotest', 'testing/fstest', 'math/cmplx', 'compress/bzip2', 'math/big'}


def imports_of(path):
    """The import paths of a Go source file."""
    text = open(path, encoding='utf8').read()
    found = set()
    m = re.search(r'(?ms)^import \((.*?)^\)', text)
    block = m.group(1) if m else ''
    for line in re.findall(r'(?m)^import (?:\w+ |\. |_ )?"([^"]+)"', text):
        found.add(line)
    for line in re.findall(r'(?m)^\s*(?:\w+ |\. |_ )?"([^"]+)"', block):
        found.add(line)
    return found


def clean_dir(d):
    """Empties the files of a folder (the folders of other packages in it stay: math/bits is in
    math/) and the testdata folder."""
    os.makedirs(d, exist_ok=True)
    for name in os.listdir(d):
        path = os.path.join(d, name)
        if os.path.isfile(path):
            os.remove(path)
    shutil.rmtree(os.path.join(d, 'testdata'), ignore_errors=True)


def prune_imports(src):
    """Removes the imports that nothing uses any more after functions were dropped (Go rejects an
    unused import)."""
    m = re.search(r'(?ms)^import \((.*?)^\)\n', src)
    if not m:
        return src
    body = src[m.end():]
    lines = []
    for line in m.group(1).split('\n'):
        im = re.match(r'^\t(\S+ )?"([^"]+)"$', line)
        if im:
            alias = (im.group(1) or '').strip()
            if alias in ('_', '.'):
                lines.append(line)
                continue
            name = alias or im.group(2).split('/')[-1]
            if not re.search(r'(?<![A-Za-z0-9_.])%s\.' % re.escape(name), body):
                continue
        lines.append(line)
    return src[:m.start(1)] + '\n'.join(lines) + src[m.end(1):]


def convert_extra(epkg, patches, seen):
    """A package of Go's tree that gocpp does not ship, as a test-only standard package of the
    overlay: its non-test files for linux
    (patches are named testing/iotest/reader.go, with the import path)."""
    src = os.path.join(GOSRC, *epkg.split('/'))
    dst = os.path.join(OUT, *epkg.split('/'))
    clean_dir(dst)
    for path in sorted(glob.glob(os.path.join(src, '*.go'))):
        base = os.path.basename(path)
        if base.endswith('_test.go') or not file_allowed(path, EXTRA_TAGS):
            continue
        text = open(path, encoding='utf8').read()
        key = epkg + '/' + base
        for i, (pf, old, new, count, reason) in enumerate(patches):
            if pf != key or old is None:
                continue
            n = text.count(old)
            if n != count:
                raise SystemExit('%s: patch %r found %d times, want %d' % (key, old[:60], n, count))
            text = text.replace(old, new)
            seen.add(i)
        # The loader reads a //go:build line again, without Go's tags (purego...): the lines that
        # are true here go.
        text = re.sub(r'(?m)^//go:build [^\n]*\n\n?', '', text, count=1)
        open(os.path.join(dst, base[:-3] + '.gp'), 'w', encoding='utf8').write(text)


def convert(pkg):
    key = pkg.replace('/', '__')
    gosrc = os.path.join(GOSRC, *pkg.split('/'))
    if not os.path.isdir(gosrc):
        raise SystemExit('no Go source in %s: run tools/portstd once to fill the cache' % gosrc)
    files = []          # test files taken from Go
    skipped_files = {}  # test file -> reason it is not taken
    patches = []        # (file, old, new, count, reason)
    dropped = {}        # (file, function) -> reason
    expected = {}       # function -> (class, reason): fails, or is skipped, in gocpp
    shims = []          # packages of std-shim/ that the tests import

    def patch(f, old, new, reason, count=1):
        patches.append((f, old, new, count, reason))

    def drop(f, name, reason):
        dropped[(f, name)] = reason

    def skip_file(f, reason):
        skipped_files[f] = reason

    def expect(name, cls, reason):
        if cls not in ('divergence', 'bug', 'unsupported', 'flaky'):
            raise SystemExit('%s: unknown class %s' % (name, cls))
        expected[name] = (cls, reason)

    added = {}          # name of a test file that gocpp adds (not Go's) -> its text

    def add_file(name, text):
        added[name] = text

    def replace_func(f, name, text, reason):
        """Replaces a function of a test file by new text."""
        replaced[(f, name)] = (text, reason)
        patches.append((f, None, None, 0, reason))

    replaced = {}

    def only(f, keep, reason, taken=True):
        """Keeps only the functions in keep (helpers) from a test file: the others are dropped with
        the reason. With taken=False the file does not count as selected (its tests are left out
        like those of a skip_file) and only its helpers are built."""
        only_files[f] = (set(keep), reason)
        if not taken:
            helper_files[f] = reason

    only_files = {}
    helper_files = {}  # files of which only the helpers are built: not selected
    env = {
        'FILES': files, 'EXTRA': [], 'add_file': add_file, 'replace_func': replace_func,
        'only': only, 'patch': patch, 'drop': drop, 'skip_file': skip_file, 'expect': expect,
        'SHIMS': shims, 'TESTDATA': False, 'ARGS': [],
    }
    exec(open(os.path.join(HERE, 'std', key + '.py')).read(), env)
    files = list(env['FILES'])
    if files == ['all']:
        files = None  # every test file of Go that skip_file does not name (decided below)
    shims = list(env['SHIMS'])
    # Packages of Go's tree that the tests import and gocpp does not ship: test-only packages.
    extra = list(env['EXTRA'])
    testdata = env['TESTDATA']  # copy the testdata folder
    args = env['ARGS']          # more arguments of gocpp test

    out = os.path.join(OUT, *pkg.split('/'))
    clean_dir(out)

    # Every test file of Go for linux/amd64.
    everything = sorted(os.path.basename(p)
                        for p in glob.glob(os.path.join(gosrc, '*_test.go')) if file_allowed(p))
    if files is None:
        files = [f for f in everything if f not in skipped_files]
    for f in files:
        if f not in everything:
            raise SystemExit('%s: %s is not a test file for linux/amd64' % (pkg, f))
    for f in skipped_files:
        if f not in everything or f in files:
            raise SystemExit('%s: skip_file %s is not a test file that is left out' % (pkg, f))
    for f in everything:
        if f not in files and f not in skipped_files:
            raise SystemExit('%s: test file %s is neither taken (FILES) nor left out with a '
                             'reason (skip_file)' % (pkg, f))

    manifest_tests = []
    seen_patches = set()
    for f in everything:
        src = open(os.path.join(gosrc, f), encoding='utf8').read()
        funcs = go_test_funcs(src)
        if f in skipped_files:
            for kind, name in funcs:
                manifest_tests.append({'name': name, 'file': f, 'kind': kind,
                                       'status': 'excluded', 'reason': skipped_files[f]})
            continue
        had_drop = False
        if f in only_files:
            keep, reason = only_files[f]
            for m in re.finditer(r'(?m)^func (?:\(\w+ \*?(\w+)\) )?(\w+)\(', src):
                name = m.group(2) if not m.group(1) else m.group(1) + '.' + m.group(2)
                if name not in keep:
                    dropped[(f, name)] = reason
        for (df, name), reason in dropped.items():
            if df == f:
                src = remove_func(src, name)
                had_drop = True
        for (rf, name), (text, reason) in replaced.items():
            if rf == f:
                src = remove_func(src, name, text)
                had_drop = True
        if had_drop:
            src = prune_imports(src)
        for i, (pf, old, new, count, reason) in enumerate(patches):
            if pf != f:
                continue
            if old is None:
                seen_patches.add(i)  # the bookkeeping entry of replace_func
                continue
            n = src.count(old)
            if n != count:
                raise SystemExit('%s: patch %r found %d times, want %d' % (f, old[:60], n, count))
            src = src.replace(old, new)
            seen_patches.add(i)
        open(os.path.join(out, f[:-3] + '.gp'), 'w', encoding='utf8').write(src)
        for kind, name in funcs:
            entry = {'name': name, 'file': f, 'kind': kind}
            if (f, name) in dropped:
                entry['status'] = 'excluded'
                entry['reason'] = dropped[(f, name)]
            else:
                entry['status'] = 'run'
            manifest_tests.append(entry)
    for (pf, name) in dropped:
        if pf not in files:
            raise SystemExit('%s: drop of %s in %s, which is not a taken file' % (pkg, name, pf))
    names = {t['name'] for t in manifest_tests if t['status'] == 'run'}
    for name in expected:
        if name not in names:
            raise SystemExit('%s: expect(%s): not a test function that runs' % (pkg, name))

    # The packages the tests import that gocpp does not ship: shims (std-shim/) and test-only copies
    # of Go's own packages (AUTO_EXTRA).
    imported = set()
    for f in files:
        imported |= imports_of(os.path.join(out, f[:-3] + '.gp'))
    for imp in sorted(imported):
        if glob.glob(os.path.join(HERE, 'std-shim', *imp.split('/'), '*.gp')) and imp not in shims:
            shims.append(imp)
    todo = [i for i in sorted(imported) if i in AUTO_EXTRA and i not in extra] + list(extra)
    extra = []
    while todo:
        epkg = todo.pop(0)
        if epkg in extra:
            continue
        extra.append(epkg)
        for path in glob.glob(os.path.join(GOSRC, *epkg.split('/'), '*.go')):
            if not path.endswith('_test.go') and file_allowed(path, EXTRA_TAGS):
                for imp in imports_of(path):
                    if imp in AUTO_EXTRA and imp not in extra:
                        todo.append(imp)
    for epkg in extra:
        convert_extra(epkg, patches, seen_patches)
    for i, p in enumerate(patches):
        if i not in seen_patches:
            raise SystemExit('%s: the patch of %s (%r) names a file that is not taken'
                             % (pkg, p[0], p[1][:40]))
    for name, text in added.items():
        open(os.path.join(out, name[:-3] + '.gp'), 'w', encoding='utf8').write(text)
    if testdata:
        td = os.path.join(gosrc, 'testdata')
        if os.path.isdir(td):
            shutil.copytree(td, os.path.join(out, 'testdata'))
    for shim in shims:
        src = os.path.join(HERE, 'std-shim', *shim.split('/'))
        dst = os.path.join(OUT, *shim.split('/'))
        shutil.rmtree(dst, ignore_errors=True)
        shutil.copytree(src, dst)

    manifest = {
        'package': pkg,
        'go': pin['version'],
        'args': args,
        'files': {**{f: {'status': 'run'} for f in files if f not in helper_files},
                  **{f: {'status': 'excluded', 'reason': r}
                     for f, r in {**skipped_files, **helper_files}.items()}},
        'tests': manifest_tests,
        'expected': {n: {'class': c, 'reason': r} for n, (c, r) in sorted(expected.items())},
        'patches': [{'file': pf, 'reason': reason} for (pf, old, new, count, reason) in patches],
    }
    os.makedirs(os.path.join(OUT, 'manifest'), exist_ok=True)
    with open(os.path.join(OUT, 'manifest', key + '.json'), 'w') as fh:
        json.dump(manifest, fh, indent=1)
        fh.write('\n')


if len(sys.argv) < 2:
    raise SystemExit(__doc__)
for p in sys.argv[1:]:
    convert(p)
