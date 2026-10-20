#!/usr/bin/env python3
"""Runs Go's range-over-func torture test (test/rangegen.go) with gocpp, in shards (step 9.10).

usage: python3 tools/portstd/gotests/rangegen_shards.py [-n UNITS] [-j JOBS] [-gocpp PATH]
                                                        [-gotree DIR] [-out DIR] [-only SHARD...]

rangegen.go writes one Go program of 242,000 lines: 800 units (testFunc, testSlice and test for
one nesting of loops, labels, defers, panics and goto, each compared between a range over a
function and a range over a slice) and a main that calls them. Go's own runner compiles that
program as one; the long form of rangegen.go cuts it into batches of 1,000 lines for the build
machines. This script does the same with different sizes: it runs rangegen.go with the real go
command (as `go run rangegen.go` does; the tarball of tools/portstd has the file, and the output
is not stored in the repository), cuts the program into shards of UNITS units each (default 25),
builds and runs every shard with gocpp (JOBS at a time), and prints the shards that passed,
failed or did not build, with the first lines of the output of each failure. A unit counts as
passed when its shard passes; the last line says how many units that is. For a failing shard,
rerun it with -n 1 -only to find the units (a unit that fails prints "mismatch testFuncN
testSliceN")."""
import argparse
import concurrent.futures
import os
import re
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..', '..'))

PIN_KEYS = ('version', 'sha256')
pin = dict(line.split() for line in open(os.path.join(ROOT, 'tools', 'portstd', 'PINS'))
           if line.split() and line.split()[0] in PIN_KEYS)
cache = os.path.join(os.environ.get('XDG_CACHE_HOME') or os.path.expanduser('~/.cache'),
                     'gocpp', 'portstd')
GOTREE = os.path.join(cache, '%s-%s' % (pin['version'], pin['sha256'][:12]))

# The environment of the gocpp commands: the phase gates of the dev builds open up to phase 15,
# as tools/gotest does.
ENV = dict(os.environ, GOCPP_GATE_PHASE='15', GOCPP_MODCACHE='off', GOCPP_OFFLINE='1')

# The one change to Go's program: output.print writes a trace entry of another type than int and
# string with the builtin print, which gocpp does not have for an interface value (print writes
# the two words of the interface, "(0x4a2ce0,0xc00001c0a8)"). It runs only when a unit has
# failed, to show the trace.
PRINT_ANY = '\t\tdefault:\n\t\t\tprint(x)\n'
PRINT_ANY_PATCHED = '\t\tdefault:\n\t\t\tprint("(a value of another type)")\n'


def generate(work, gotree):
    """Runs rangegen.go with go and returns the text of the program it writes."""
    src = os.path.join(work, 'rangegen.go')
    shutil.copy(os.path.join(gotree, 'test', 'rangegen.go'), src)
    out = subprocess.run(['go', 'run', 'rangegen.go'], cwd=work, capture_output=True, text=True,
                         timeout=600)
    if out.returncode != 0:
        raise SystemExit('go run rangegen.go: %s' % out.stderr)
    return out.stdout


def split(program):
    """Cuts the program into (header, units, helpers): a unit is [name, text] with name the
    suffix after testFunc."""
    starts = [(m.start(), m.group(1)) for m in re.finditer(r'(?m)^func testFunc(\w+)\(', program)]
    main = re.search(r'(?m)^func main\(\)', program).start()
    after_main = program.index('\n}\n', main) + 3
    header = program[:starts[0][0]]
    units = []
    for i, (pos, name) in enumerate(starts):
        end = starts[i + 1][0] if i + 1 < len(starts) else main
        units.append([name, program[pos:end]])
    return header, units, program[after_main:]


def write_shards(out, header, units, helpers, size):
    """Writes shard directories out/shardNNN/{main.gp,gocpp.mod}; returns {name: unit count}."""
    shutil.rmtree(out, ignore_errors=True)
    sizes = {}
    patched_helpers = helpers.replace(PRINT_ANY, PRINT_ANY_PATCHED)
    for index in range(0, len(units), size):
        chunk = units[index:index + size]
        name = 'shard%03d' % (index // size)
        directory = os.path.join(out, name)
        os.makedirs(directory)
        calls = ''.join('\ttest%s()\n' % unit[0] for unit in chunk)
        text = (header + ''.join(unit[1] for unit in chunk) + 'func main() {\n' + calls + '}\n'
                + patched_helpers)
        with open(os.path.join(directory, 'main.gp'), 'w') as fh:
            fh.write(text)
        with open(os.path.join(directory, 'gocpp.mod'), 'w') as fh:
            fh.write('module rangegen\n')
        sizes[name] = len(chunk)
    return sizes


def run_shard(gocpp, out, name):
    """Builds and runs one shard: (name, status, output)."""
    directory = os.path.join(out, name)
    exe = os.path.join(directory, 'prog')
    try:
        built = subprocess.run([gocpp, 'build', '-o', exe, 'main.gp'], cwd=directory,
                               capture_output=True, text=True, timeout=1800, env=ENV)
    except subprocess.TimeoutExpired:
        return name, 'build timeout', ''
    if built.returncode != 0:
        return name, 'build failed', built.stdout + built.stderr
    try:
        ran = subprocess.run([exe], cwd=directory, capture_output=True, text=True, timeout=300,
                             env=ENV)
    except subprocess.TimeoutExpired:
        return name, 'run timeout', ''
    os.remove(exe)
    if ran.returncode != 0:
        return name, 'run failed', ran.stdout + ran.stderr
    return name, 'pass', ''


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-n', type=int, default=25, help='units per shard')
    parser.add_argument('-j', type=int, default=4, help='shards at a time')
    parser.add_argument('-gocpp', default='gocpp')
    parser.add_argument('-gotree', default=GOTREE,
                        help='the extracted tarball of tools/portstd (it has test/rangegen.go)')
    parser.add_argument('-out', default=os.path.join(ROOT, 'build', 'rangegen'))
    parser.add_argument('-only', nargs='*', default=[],
                        help='run only these shards (names such as shard003)')
    args = parser.parse_args()

    work = os.path.join(args.out, 'gen')
    os.makedirs(work, exist_ok=True)
    program = generate(work, args.gotree)
    header, units, helpers = split(program)
    print('rangegen wrote %d lines, %d units' % (program.count('\n'), len(units)))

    shards_dir = os.path.join(args.out, 'shards')
    sizes = write_shards(shards_dir, header, units, helpers, args.n)
    names = [name for name in sizes if not args.only or name in args.only]

    results = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.j) as pool:
        for result in pool.map(lambda name: run_shard(args.gocpp, shards_dir, name), names):
            results.append(result)
            print('%s: %s' % (result[0], result[1]), flush=True)

    passed = [name for name, status, _ in results if status == 'pass']
    units_passed = sum(sizes[name] for name in passed)
    units_run = sum(sizes[name] for name, _, _ in results)
    print('%d of %d units in %d of %d shards pass' % (units_passed, units_run, len(passed),
                                                     len(results)))
    for name, status, output in results:
        if status != 'pass':
            print('--- %s: %s' % (name, status))
            print('\n'.join(output.splitlines()[:12]))
    return 0 if len(passed) == len(results) else 1


if __name__ == '__main__':
    sys.exit(main())
