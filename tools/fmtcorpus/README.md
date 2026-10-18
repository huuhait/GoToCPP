# tools/fmtcorpus

Generates the differential tests of `fmt` (step 5.11): `tests/run/fmt_*.gp`, about 190 programs
that print every verb, flag, width and precision for every kind of value. Each program is Go
source in `cats*.py`; the tool runs it with the real `go` command and writes the program with its
output as the `// Output:` block, so the expected output is Go's, never ours.

```sh
cd tools/fmtcorpus
python3 run_all.py cats1,cats2,cats3,cats4,cats5,cats6          # every test
python3 run_all.py cats1,cats2,cats3,cats4,cats5,cats6 fmt_map   # the tests whose name contains fmt_map
```

Run it in the dev shell (`nix develop -c python3 ...`) so that `go` is Go 1.25. A program whose
output has a line that ends in a space, invalid UTF-8 or a NUL byte is reported and not written
(the harness drops the spaces at the end of a comment line). `FMTCORPUS_WORK` is the folder of the
scratch programs and `CORPUS_OUT` the folder of the output.
