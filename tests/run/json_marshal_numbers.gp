// phase: 09
// encoding/json number encoding: integers of every size, float formatting (exponent forms, shortest representation, float32), NaN and Inf errors, ",string".

package main

import (
	"encoding/json"
	"fmt"
	"math"
)

func show(label string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: %s\n", label, b)
}

type F struct {
	A float64 `json:",string"`
	B float32 `json:",string"`
}

func main() {
	for _, f := range []float64{0, 1, -1, 0.1, 0.5, 1.5, 100, 1e20, 1e21, 1e-6, 1e-7, 123456789, 1.2345678901234567e+30,
		3.141592653589793, 2.5e-5, 1e100, 1e-100, 5e-324, math.MaxFloat64, math.SmallestNonzeroFloat64, -0.0, math.Copysign(0, -1),
		100000000000000000000, 999999999999999900000, 1e21 + 1e5, 0.000001, 0.0000001, 123e-9, 12345.678e10, 4.9e-324, 1.7976931348623157e308} {
		show(fmt.Sprintf("float64 %v", f), f)
	}
	for _, f := range []float32{0, 1, 0.1, 3.4028235e38, 1e-7, 1e21, 16777216, 1.17549435e-38, 1e-6, 123456789} {
		show(fmt.Sprintf("float32 %v", f), f)
	}
	show("NaN", math.NaN())
	show("+Inf", math.Inf(1))
	show("-Inf", math.Inf(-1))
	show("NaN32", float32(math.NaN()))
	show("slice with NaN", []float64{1, math.NaN()})
	show("map with Inf", map[string]float64{"a": math.Inf(1)})
	show("string opt", F{1.5, 2.5})
	show("int limits", []any{int8(127), int16(-32768), int32(2147483647), int64(math.MinInt64), uint16(65535), uint32(4294967295), uint64(math.MaxUint64)})
	show("max int", math.MaxInt64)
	show("array of ints", [4]int{0, -1, 1 << 40, -1 << 40})
	show("complex", complex(1, 2))
	show("uintptr", uintptr(math.MaxUint64))
	var f64 float64 = 1 << 53
	show("2^53", f64)
	show("2^53+2", f64+2)
	show("1/3", 1.0/3.0)
	show("2/3", 2.0/3.0)
	show("float32 1/3", float32(1.0)/3)
}

// Output:
// float64 0: 0
// float64 1: 1
// float64 -1: -1
// float64 0.1: 0.1
// float64 0.5: 0.5
// float64 1.5: 1.5
// float64 100: 100
// float64 1e+20: 100000000000000000000
// float64 1e+21: 1e+21
// float64 1e-06: 0.000001
// float64 1e-07: 1e-7
// float64 1.23456789e+08: 123456789
// float64 1.2345678901234567e+30: 1.2345678901234567e+30
// float64 3.141592653589793: 3.141592653589793
// float64 2.5e-05: 0.000025
// float64 1e+100: 1e+100
// float64 1e-100: 1e-100
// float64 5e-324: 5e-324
// float64 1.7976931348623157e+308: 1.7976931348623157e+308
// float64 5e-324: 5e-324
// float64 0: 0
// float64 -0: -0
// float64 1e+20: 100000000000000000000
// float64 9.999999999999999e+20: 999999999999999900000
// float64 1.0000000000000001e+21: 1.0000000000000001e+21
// float64 1e-06: 0.000001
// float64 1e-07: 1e-7
// float64 1.23e-07: 1.23e-7
// float64 1.2345678e+14: 123456780000000
// float64 5e-324: 5e-324
// float64 1.7976931348623157e+308: 1.7976931348623157e+308
// float32 0: 0
// float32 1: 1
// float32 0.1: 0.1
// float32 3.4028235e+38: 3.4028235e+38
// float32 1e-07: 1e-7
// float32 1e+21: 1e+21
// float32 1.6777216e+07: 16777216
// float32 1.1754944e-38: 1.1754944e-38
// float32 1e-06: 0.000001
// float32 1.2345679e+08: 123456790
// NaN: error: json: unsupported value: NaN
// +Inf: error: json: unsupported value: +Inf
// -Inf: error: json: unsupported value: -Inf
// NaN32: error: json: unsupported value: NaN
// slice with NaN: error: json: unsupported value: NaN
// map with Inf: error: json: unsupported value: +Inf
// string opt: {"A":"1.5","B":"2.5"}
// int limits: [127,-32768,2147483647,-9223372036854775808,65535,4294967295,18446744073709551615]
// max int: 9223372036854775807
// array of ints: [0,-1,1099511627776,-1099511627776]
// complex: error: json: unsupported type: complex128
// uintptr: 18446744073709551615
// 2^53: 9007199254740992
// 2^53+2: 9007199254740994
// 1/3: 0.3333333333333333
// 2/3: 0.6666666666666666
// float32 1/3: 0.33333334
