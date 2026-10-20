# Go 1.25.14's tests of encoding/json (internal tests and examples); see convert.py.
FILES = ['decode_test.go', 'encode_test.go', 'scanner_test.go', 'stream_test.go', 'fold_test.go',
         'number_test.go', 'tagkey_test.go', 'tags_test.go', 'fuzz_test.go', 'example_test.go',
         'example_marshaling_test.go', 'example_text_marshaling_test.go']
# Not taken: bench_test.go (benchmarks; needs internal/zstd and testdata/code.json.zst) and the v2_* files (goexperiment.jsonv2).
patch('decode_test.go', '\t"image"\n', '\t"example.com/m/image"\n', 'package image is not in gocpp: ./image has its Point type')
patch('decode_test.go', '\t"math/big"\n', '\t"example.com/m/big"\n', 'package math/big is not in gocpp: ./big has a small Int with the text methods the tests use')
patch('decode_test.go', '\t"net"\n', '', 'package net is not in gocpp: the one test case that decodes into net.IP is dropped')
patch('decode_test.go', '''		{Name(""), `123`, new(net.IP), &UnmarshalTypeError{Value: "number", Type: reflect.TypeFor[*net.IP](), Offset: 3}},\n''', '', 'package net is not in gocpp')
patch('encode_test.go', '\t"testing/synctest"\n', '', 'package testing/synctest is not in gocpp (TestSynctestMarshal is dropped)')
drop('encode_test.go', 'TestSynctestMarshal', 'needs testing/synctest (bubbles), which gocpp does not have')
patch('stream_test.go', '\t"net"\n', '', 'package net is not in gocpp: TestBlocking uses io.Pipe, which blocks the same way')
patch('stream_test.go', '\t"net/http"\n\t"net/http/httptest"\n', '', 'packages net/http are not in gocpp (TestHTTPDecoding is dropped)')
patch('stream_test.go', 'r, w := net.Pipe()', 'r, w := io.Pipe()', 'package net is not in gocpp')
drop('stream_test.go', 'TestHTTPDecoding', 'needs net/http and httptest')
patch('encode_test.go', 'type C int\n\nfunc (C) MarshalJSON', 'type CInt int\n\nfunc (CInt) MarshalJSON', 'gocpp reserves the package-level name C (C interop): type C is CInt')
patch('encode_test.go', '\tvar c C\n', '\tvar c CInt\n', 'type C is CInt')
patch('stream_test.go', '\tvar c C\n', '\tvar c CInt\n', 'type C is CInt')
patch('encode_test.go', '\t"sync"\n', '', 'only TestSynctestMarshal used sync')
patch('decode_test.go', 'strings.Repeat(`{"a":`, 10000-1) + `0` + strings.Repeat(`}`, 10000-1)', 'strings.Repeat(`{"a":`, 4000) + `0` + strings.Repeat(`}`, 4000)', 'decoding objects nested 9999 deep needs more than the 8 MiB stack of a goroutine in some builds (fixed stacks, D24): 4000 fits in every build mode and compiler (the stack guard of the package, which gives an error instead, is replaced in this copy, see PORT_PATCHES)')
PORT_PATCHES = [('//gocpp:native gp::runtime::stack_room\nfunc stackRoom() int64\n', 'func stackRoom() int64 { return 1 << 40 }\n')] # a program cannot declare a native: in the copy the stack guard never trips, as Go's stacks grow
patch('stream_test.go', '\t"log"\n', '', 'only TestHTTPDecoding (dropped) used log')
