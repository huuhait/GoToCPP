# tools/mathcorpus

Generates the differential tests of package `math` (step 9.5, [ADR-071](../../docs/adr/071-math-bit-exact.md)):
`tests/run/math_*.gp`, 122 programs. Each program is Go source in `cats*.py`; the tool runs it with the real
`go` command (Go 1.25, amd64) and writes the program with its output as the `// Output:` block, so the expected
output is Go's, never ours.

- `cats1.py`: one sweep per exported function: about 100,000 inputs each (special values and their
  neighbours, random bit patterns, random values of several ranges), the FNV hash of the result bits and 200
  sampled raw results.
- `cats2.py`: constants, float to integer and integer to float conversions, no-contraction tests, arithmetic
  and comparison edge cases, float32, negative zero, long sums.
- `cats3.py`: dense range sweeps at the branch points of each algorithm, special-case tables, printing with
  `fmt`, function values, generics, goroutines, import forms.
- `cats4.py`: float32 wrappers, complex arithmetic, identities, performance workloads, NaN storage.
- `cxxdata.py`: writes `runtime/test/math_data.hpp` (Go's `Exp` with and without FMA, `Log`, `FMA` with NaN
  operands) for the C++ unit test `runtime/test/math_test.cpp`.

```sh
cd tools/mathcorpus
nix develop -c python3 run_all.py cats1,cats2,cats3,cats4            # every test
nix develop -c python3 run_all.py cats1,cats2,cats3,cats4 math_exp   # the tests whose name contains math_exp
nix develop -c python3 cxxdata.py
```

`MATHCORPUS_WORK` is the folder of the scratch programs, `CORPUS_OUT` the folder of the output. A bit that
differs between gocpp and Go is a bug in gocpp, except the payload of a NaN that an operator makes (ADR-071).
