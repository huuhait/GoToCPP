# Go 1.25.14's tests of encoding/json (internal tests and examples); see convert_std.py.
FILES = ['all']
skip_file(
    'bench_test.go',
    'benchmarks (it needs internal/zstd and testdata/code.json.zst); no test function of it is run '
    'by go test',
)
# (The v2_*_test.go files are for goexperiment.jsonv2, which is off.)
patch(
    'decode_test.go',
    '\t"net"\n',
    '',
    'package net is not in gocpp: the one test case that decodes into net.IP is dropped',
)
patch(
    'decode_test.go',
    '\t\t{Name(""), `123`, new(net.IP), &UnmarshalTypeError{Value: "number", Type: '
    'reflect.TypeFor[*net.IP](), Offset: 3}},\n',
    '',
    'package net is not in gocpp',
)
drop(
    'encode_test.go',
    'TestSynctestMarshal',
    'needs testing/synctest (bubbles), which gocpp does not have',
)
patch(
    'stream_test.go',
    '\t"net"\n',
    '',
    'package net is not in gocpp: TestBlocking uses io.Pipe, which blocks the same way',
)
patch('stream_test.go', 'r, w := net.Pipe()', 'r, w := io.Pipe()', 'package net is not in gocpp')
drop('stream_test.go', 'TestHTTPDecoding', 'needs net/http and httptest')
patch(
    'encode_test.go',
    'type C int\n\nfunc (C) MarshalJSON',
    'type CInt int\n\nfunc (CInt) MarshalJSON',
    'gocpp reserves the package-level name C (C interop): type C is CInt',
)
patch('encode_test.go', '\tvar c C\n', '\tvar c CInt\n', 'type C is CInt')
patch('stream_test.go', '\tvar c C\n', '\tvar c CInt\n', 'type C is CInt')
