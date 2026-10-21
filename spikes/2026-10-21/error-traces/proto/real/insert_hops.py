#!/usr/bin/env python3
"""insert_hops.py WORK OUT: variants of the generated C++ of bench.gp for the measurement.

base: the functions with `!` marked noinline. hop: + gp::etrace::hop(&site, must_eN, return address)
in each error branch, and errors.Wrapf calls gp::etrace::rekey. (Everything is noinline: the
hop's cost on the nil path is the question, and inlined chains compile to different loops.)
"""
import os
import re
import sys

work, out = sys.argv[1], sys.argv[2]
os.makedirs(out, exist_ok=True)
pre = """namespace gp::etrace {
struct site { const char* file; const char* func; int line; };
[[gnu::noinline, gnu::cold]] void hop(const site*, gp::error, const void*) noexcept;
void rekey(const gp::error&, const gp::error&) noexcept;
}
"""
src = open(os.path.join(work, 'gen/gpkg/command-line-arguments/bench.gp.cpp')).read()
src = re.sub(r'^std::tuple<int64_t, gp::error> (leaf|mid|top|app|midw|topw|appw)\(',
             r'[[gnu::noinline]] std::tuple<int64_t, gp::error> \1(', src, flags=re.M)
open(os.path.join(out, 'bench_base.cpp'), 'w').write(src)
count = [0]


def patch(m):
    count[0] += 1
    site = 'static constexpr gp::etrace::site gp_site{"main.gp", "main.f", %d};' % count[0]
    call = 'gp::etrace::hop(&gp_site, %s, __builtin_return_address(0));' % m.group(1)
    return m.group(0) + '\n        { %s %s }' % (site, call)


text = re.sub(r'if \((must_e\d+) != nullptr\) \{', patch, src)
text = text.replace('namespace gpkg_main {', pre + 'namespace gpkg_main {', 1)
open(os.path.join(out, 'bench_hop.cpp'), 'w').write(text)

wr = open(os.path.join(work, 'gen/gpkg/errors/wrapf.gp.cpp')).read()
open(os.path.join(out, 'wrapf_base.cpp'), 'w').write(wr)
wr = wr.replace('namespace gpkg_errors {', pre + 'namespace gpkg_errors {', 1)
wr = wr.replace('    *errp = gp::to_iface', '    gp::error gp_inner = *errp;\n    *errp = gp::to_iface', 1)
wr = wr.replace('gp_tdp_wrapfError);\n}', 'gp_tdp_wrapfError);\n    gp::etrace::rekey(gp_inner, *errp);\n}', 1)
open(os.path.join(out, 'wrapf_hop.cpp'), 'w').write(wr)
