// phase: 09
// encoding/json numbers: decoding at the limits of every integer and float kind, overflow
// errors, UseNumber and json.Number methods, and number validation when encoding a Number.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

func tryDecode[T any](label string, inputs ...string) {
	for _, input := range inputs {
		var value T
		err := json.Unmarshal([]byte(input), &value)
		fmt.Printf("%-8s %-28s => %v err=%v\n", label, input, value, err)
	}
}

func main() {
	fmt.Println("== integer limits")
	tryDecode[int8]("int8", "127", "128", "-128", "-129", "1e2", "1.0", "1e0", "0e5", "-0")
	tryDecode[int16]("int16", "32767", "32768", "-32768", "-32769")
	tryDecode[int32]("int32", "2147483647", "2147483648", "-2147483648", "-2147483649")
	tryDecode[int64]("int64", "9223372036854775807", "9223372036854775808", "-9223372036854775808", "-9223372036854775809", "1e18", "9.2e18")
	tryDecode[uint8]("uint8", "255", "256", "-1", "0", "-0")
	tryDecode[uint16]("uint16", "65535", "65536")
	tryDecode[uint32]("uint32", "4294967295", "4294967296")
	tryDecode[uint64]("uint64", "18446744073709551615", "18446744073709551616", "-1", "1e19")
	tryDecode[uint]("uint", "18446744073709551615", "18446744073709551616")

	fmt.Println("== float limits")
	tryDecode[float64]("float64", "1.7976931348623157e308", "1.7976931348623159e308", "1e309", "-1e309", "4.9e-324", "2e-324", "1e-400", "0.1", "123456789012345678901234567890", "-0", "0e0", "1E2", "1e+2", "1e-2", "9007199254740993")
	tryDecode[float32]("float32", "3.4028234663852886e38", "3.4028235677973366e38", "3.5e38", "1e39", "1.4e-45", "1e-46", "16777217", "0.1")

	fmt.Println("== encoding at the exponent thresholds")
	thresholds32 := []float32{1e20, 3e20, 9.9e20, math.Nextafter32(1e21, 0), 1e21, math.Nextafter32(1e21, math.MaxFloat32), 2e21,
		1e-6, math.Nextafter32(1e-6, 0), math.Nextafter32(1e-6, 1), 9.9e-7, 1.5e-6, -1e20, -1e21, -1e-6, -9.9e-7}
	for _, value := range thresholds32 {
		encoded, err := json.Marshal(value)
		fmt.Printf("float32 %v => %s %v\n", value, encoded, err)
	}
	thresholds64 := []float64{1e20, 9.9e20, math.Nextafter(1e21, 0), 1e21, math.Nextafter(1e21, math.MaxFloat64), 1e-6, math.Nextafter(1e-6, 0), math.Nextafter(1e-6, 1), 9.9e-7, -1e21, -1e-7}
	for _, value := range thresholds64 {
		encoded, err := json.Marshal(value)
		fmt.Printf("float64 %v => %s %v\n", value, encoded, err)
	}
	encoded32, _ := json.Marshal(map[string]any{"a": float32(1e20), "b": float32(1e21), "c": []float32{1e-6, 1e-7}})
	fmt.Printf("%s\n", encoded32)

	fmt.Println("== any numbers")
	for _, input := range []string{"0", "1", "1.0", "1e3", "-0", "9007199254740993", "1e400", "123456789012345678901234567890"} {
		var value any
		err := json.Unmarshal([]byte(input), &value)
		fmt.Printf("any %-34s => %v (%T) err=%v\n", input, value, value, err)
	}

	fmt.Println("== UseNumber")
	for _, input := range []string{"0", "1.50", "1e3", "-0", "9007199254740993", "1e400", "[1,2.5,{\"a\":3}]", `{"n":12345678901234567890}`} {
		decoder := json.NewDecoder(strings.NewReader(input))
		decoder.UseNumber()
		var value any
		err := decoder.Decode(&value)
		fmt.Printf("%-30s => %v (%T) err=%v\n", input, value, value, err)
	}

	fmt.Println("== Number methods")
	for _, text := range []string{"0", "42", "-7", "1.5", "1e3", "9223372036854775808", "1e400", "abc", "", "0x10", "1_000"} {
		number := json.Number(text)
		integer, intErr := number.Int64()
		float, floatErr := number.Float64()
		fmt.Printf("%-22q String=%q Int64=(%d,%v) Float64=(%v,%v)\n", text, number.String(), integer, intErr, float, floatErr)
	}

	fmt.Println("== encoding Number")
	for _, text := range []string{"", "0", "-0", "1", "01", "1.", ".5", "1e5", "1E+5", "1e", "+1", "--1", "1.5.5", "0x1", "NaN", " 1", "1 ", "123456789012345678901234567890", "-", "1e-0010"} {
		encoded, err := json.Marshal(json.Number(text))
		fmt.Printf("%-34q => %s err=%v\n", text, encoded, err)
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		err = encoder.Encode(map[string]json.Number{"n": json.Number(text)})
		fmt.Printf("%-34s in map => %q err=%v\n", "", buffer.String(), err)
	}

	fmt.Println("== string option")
	type Quoted struct {
		I int64       `json:",string"`
		U uint8       `json:",string"`
		F float64     `json:",string"`
		B bool        `json:",string"`
		S string      `json:",string"`
		N json.Number `json:",string"`
	}
	for _, input := range []string{`{"I":"1","U":"2","F":"3.5","B":"true","S":"\"x\"","N":"7"}`, `{"I":1}`, `{"I":"x"}`, `{"U":"300"}`, `{"F":"1e999"}`, `{"B":"yes"}`, `{"S":"x"}`, `{"I":null}`, `{"I":"null"}`, `{"I":" 1"}`, `{"I":"01"}`, `{"N":"abc"}`, `{"F":"NaN"}`} {
		var value Quoted
		err := json.Unmarshal([]byte(input), &value)
		fmt.Printf("%-60s => %+v err=%v\n", input, value, err)
	}
	encoded, err := json.Marshal(Quoted{I: -5, U: 200, F: 2.5, B: true, S: "q\"x", N: "12"})
	fmt.Printf("%s %v\n", encoded, err)
	encoded, err = json.Marshal(Quoted{F: math.Inf(1)})
	fmt.Printf("%s %v\n", encoded, err)
}

// Output:
// == integer limits
// int8     127                          => 127 err=<nil>
// int8     128                          => 0 err=json: cannot unmarshal number 128 into Go value of type int8
// int8     -128                         => -128 err=<nil>
// int8     -129                         => 0 err=json: cannot unmarshal number -129 into Go value of type int8
// int8     1e2                          => 0 err=json: cannot unmarshal number 1e2 into Go value of type int8
// int8     1.0                          => 0 err=json: cannot unmarshal number 1.0 into Go value of type int8
// int8     1e0                          => 0 err=json: cannot unmarshal number 1e0 into Go value of type int8
// int8     0e5                          => 0 err=json: cannot unmarshal number 0e5 into Go value of type int8
// int8     -0                           => 0 err=<nil>
// int16    32767                        => 32767 err=<nil>
// int16    32768                        => 0 err=json: cannot unmarshal number 32768 into Go value of type int16
// int16    -32768                       => -32768 err=<nil>
// int16    -32769                       => 0 err=json: cannot unmarshal number -32769 into Go value of type int16
// int32    2147483647                   => 2147483647 err=<nil>
// int32    2147483648                   => 0 err=json: cannot unmarshal number 2147483648 into Go value of type int32
// int32    -2147483648                  => -2147483648 err=<nil>
// int32    -2147483649                  => 0 err=json: cannot unmarshal number -2147483649 into Go value of type int32
// int64    9223372036854775807          => 9223372036854775807 err=<nil>
// int64    9223372036854775808          => 0 err=json: cannot unmarshal number 9223372036854775808 into Go value of type int64
// int64    -9223372036854775808         => -9223372036854775808 err=<nil>
// int64    -9223372036854775809         => 0 err=json: cannot unmarshal number -9223372036854775809 into Go value of type int64
// int64    1e18                         => 0 err=json: cannot unmarshal number 1e18 into Go value of type int64
// int64    9.2e18                       => 0 err=json: cannot unmarshal number 9.2e18 into Go value of type int64
// uint8    255                          => 255 err=<nil>
// uint8    256                          => 0 err=json: cannot unmarshal number 256 into Go value of type uint8
// uint8    -1                           => 0 err=json: cannot unmarshal number -1 into Go value of type uint8
// uint8    0                            => 0 err=<nil>
// uint8    -0                           => 0 err=json: cannot unmarshal number -0 into Go value of type uint8
// uint16   65535                        => 65535 err=<nil>
// uint16   65536                        => 0 err=json: cannot unmarshal number 65536 into Go value of type uint16
// uint32   4294967295                   => 4294967295 err=<nil>
// uint32   4294967296                   => 0 err=json: cannot unmarshal number 4294967296 into Go value of type uint32
// uint64   18446744073709551615         => 18446744073709551615 err=<nil>
// uint64   18446744073709551616         => 0 err=json: cannot unmarshal number 18446744073709551616 into Go value of type uint64
// uint64   -1                           => 0 err=json: cannot unmarshal number -1 into Go value of type uint64
// uint64   1e19                         => 0 err=json: cannot unmarshal number 1e19 into Go value of type uint64
// uint     18446744073709551615         => 18446744073709551615 err=<nil>
// uint     18446744073709551616         => 0 err=json: cannot unmarshal number 18446744073709551616 into Go value of type uint
// == float limits
// float64  1.7976931348623157e308       => 1.7976931348623157e+308 err=<nil>
// float64  1.7976931348623159e308       => 0 err=json: cannot unmarshal number 1.7976931348623159e308 into Go value of type float64
// float64  1e309                        => 0 err=json: cannot unmarshal number 1e309 into Go value of type float64
// float64  -1e309                       => 0 err=json: cannot unmarshal number -1e309 into Go value of type float64
// float64  4.9e-324                     => 5e-324 err=<nil>
// float64  2e-324                       => 0 err=<nil>
// float64  1e-400                       => 0 err=<nil>
// float64  0.1                          => 0.1 err=<nil>
// float64  123456789012345678901234567890 => 1.2345678901234568e+29 err=<nil>
// float64  -0                           => -0 err=<nil>
// float64  0e0                          => 0 err=<nil>
// float64  1E2                          => 100 err=<nil>
// float64  1e+2                         => 100 err=<nil>
// float64  1e-2                         => 0.01 err=<nil>
// float64  9007199254740993             => 9.007199254740992e+15 err=<nil>
// float32  3.4028234663852886e38        => 3.4028235e+38 err=<nil>
// float32  3.4028235677973366e38        => 3.4028235e+38 err=<nil>
// float32  3.5e38                       => 0 err=json: cannot unmarshal number 3.5e38 into Go value of type float32
// float32  1e39                         => 0 err=json: cannot unmarshal number 1e39 into Go value of type float32
// float32  1.4e-45                      => 1e-45 err=<nil>
// float32  1e-46                        => 0 err=<nil>
// float32  16777217                     => 1.6777216e+07 err=<nil>
// float32  0.1                          => 0.1 err=<nil>
// == encoding at the exponent thresholds
// float32 1e+20 => 100000000000000000000 <nil>
// float32 3e+20 => 300000000000000000000 <nil>
// float32 9.9e+20 => 990000000000000000000 <nil>
// float32 9.9999995e+20 => 999999950000000000000 <nil>
// float32 1e+21 => 1e+21 <nil>
// float32 1.0000001e+21 => 1.0000001e+21 <nil>
// float32 2e+21 => 2e+21 <nil>
// float32 1e-06 => 0.000001 <nil>
// float32 9.999999e-07 => 9.999999e-7 <nil>
// float32 1.0000001e-06 => 0.0000010000001 <nil>
// float32 9.9e-07 => 9.9e-7 <nil>
// float32 1.5e-06 => 0.0000015 <nil>
// float32 -1e+20 => -100000000000000000000 <nil>
// float32 -1e+21 => -1e+21 <nil>
// float32 -1e-06 => -0.000001 <nil>
// float32 -9.9e-07 => -9.9e-7 <nil>
// float64 1e+20 => 100000000000000000000 <nil>
// float64 9.9e+20 => 990000000000000000000 <nil>
// float64 9.999999999999999e+20 => 999999999999999900000 <nil>
// float64 1e+21 => 1e+21 <nil>
// float64 1.0000000000000001e+21 => 1.0000000000000001e+21 <nil>
// float64 1e-06 => 0.000001 <nil>
// float64 9.999999999999997e-07 => 9.999999999999997e-7 <nil>
// float64 1.0000000000000002e-06 => 0.0000010000000000000002 <nil>
// float64 9.9e-07 => 9.9e-7 <nil>
// float64 -1e+21 => -1e+21 <nil>
// float64 -1e-07 => -1e-7 <nil>
// {"a":100000000000000000000,"b":1e+21,"c":[0.000001,1e-7]}
// == any numbers
// any 0                                  => 0 (float64) err=<nil>
// any 1                                  => 1 (float64) err=<nil>
// any 1.0                                => 1 (float64) err=<nil>
// any 1e3                                => 1000 (float64) err=<nil>
// any -0                                 => -0 (float64) err=<nil>
// any 9007199254740993                   => 9.007199254740992e+15 (float64) err=<nil>
// any 1e400                              => <nil> (<nil>) err=json: cannot unmarshal number 1e400 into Go value of type float64
// any 123456789012345678901234567890     => 1.2345678901234568e+29 (float64) err=<nil>
// == UseNumber
// 0                              => 0 (json.Number) err=<nil>
// 1.50                           => 1.50 (json.Number) err=<nil>
// 1e3                            => 1e3 (json.Number) err=<nil>
// -0                             => -0 (json.Number) err=<nil>
// 9007199254740993               => 9007199254740993 (json.Number) err=<nil>
// 1e400                          => 1e400 (json.Number) err=<nil>
// [1,2.5,{"a":3}]                => [1 2.5 map[a:3]] ([]interface {}) err=<nil>
// {"n":12345678901234567890}     => map[n:12345678901234567890] (map[string]interface {}) err=<nil>
// == Number methods
// "0"                    String="0" Int64=(0,<nil>) Float64=(0,<nil>)
// "42"                   String="42" Int64=(42,<nil>) Float64=(42,<nil>)
// "-7"                   String="-7" Int64=(-7,<nil>) Float64=(-7,<nil>)
// "1.5"                  String="1.5" Int64=(0,strconv.ParseInt: parsing "1.5": invalid syntax) Float64=(1.5,<nil>)
// "1e3"                  String="1e3" Int64=(0,strconv.ParseInt: parsing "1e3": invalid syntax) Float64=(1000,<nil>)
// "9223372036854775808"  String="9223372036854775808" Int64=(9223372036854775807,strconv.ParseInt: parsing "9223372036854775808": value out of range) Float64=(9.223372036854776e+18,<nil>)
// "1e400"                String="1e400" Int64=(0,strconv.ParseInt: parsing "1e400": invalid syntax) Float64=(+Inf,strconv.ParseFloat: parsing "1e400": value out of range)
// "abc"                  String="abc" Int64=(0,strconv.ParseInt: parsing "abc": invalid syntax) Float64=(0,strconv.ParseFloat: parsing "abc": invalid syntax)
// ""                     String="" Int64=(0,strconv.ParseInt: parsing "": invalid syntax) Float64=(0,strconv.ParseFloat: parsing "": invalid syntax)
// "0x10"                 String="0x10" Int64=(0,strconv.ParseInt: parsing "0x10": invalid syntax) Float64=(0,strconv.ParseFloat: parsing "0x10": invalid syntax)
// "1_000"                String="1_000" Int64=(0,strconv.ParseInt: parsing "1_000": invalid syntax) Float64=(1000,<nil>)
// == encoding Number
// ""                                 => 0 err=<nil>
//                                    in map => "{\"n\":0}\n" err=<nil>
// "0"                                => 0 err=<nil>
//                                    in map => "{\"n\":0}\n" err=<nil>
// "-0"                               => -0 err=<nil>
//                                    in map => "{\"n\":-0}\n" err=<nil>
// "1"                                => 1 err=<nil>
//                                    in map => "{\"n\":1}\n" err=<nil>
// "01"                               =>  err=json: invalid number literal "01"
//                                    in map => "" err=json: invalid number literal "01"
// "1."                               =>  err=json: invalid number literal "1."
//                                    in map => "" err=json: invalid number literal "1."
// ".5"                               =>  err=json: invalid number literal ".5"
//                                    in map => "" err=json: invalid number literal ".5"
// "1e5"                              => 1e5 err=<nil>
//                                    in map => "{\"n\":1e5}\n" err=<nil>
// "1E+5"                             => 1E+5 err=<nil>
//                                    in map => "{\"n\":1E+5}\n" err=<nil>
// "1e"                               =>  err=json: invalid number literal "1e"
//                                    in map => "" err=json: invalid number literal "1e"
// "+1"                               =>  err=json: invalid number literal "+1"
//                                    in map => "" err=json: invalid number literal "+1"
// "--1"                              =>  err=json: invalid number literal "--1"
//                                    in map => "" err=json: invalid number literal "--1"
// "1.5.5"                            =>  err=json: invalid number literal "1.5.5"
//                                    in map => "" err=json: invalid number literal "1.5.5"
// "0x1"                              =>  err=json: invalid number literal "0x1"
//                                    in map => "" err=json: invalid number literal "0x1"
// "NaN"                              =>  err=json: invalid number literal "NaN"
//                                    in map => "" err=json: invalid number literal "NaN"
// " 1"                               =>  err=json: invalid number literal " 1"
//                                    in map => "" err=json: invalid number literal " 1"
// "1 "                               =>  err=json: invalid number literal "1 "
//                                    in map => "" err=json: invalid number literal "1 "
// "123456789012345678901234567890"   => 123456789012345678901234567890 err=<nil>
//                                    in map => "{\"n\":123456789012345678901234567890}\n" err=<nil>
// "-"                                =>  err=json: invalid number literal "-"
//                                    in map => "" err=json: invalid number literal "-"
// "1e-0010"                          => 1e-0010 err=<nil>
//                                    in map => "{\"n\":1e-0010}\n" err=<nil>
// == string option
// {"I":"1","U":"2","F":"3.5","B":"true","S":"\"x\"","N":"7"}   => {I:1 U:2 F:3.5 B:true S:x N:7} err=<nil>
// {"I":1}                                                      => {I:0 U:0 F:0 B:false S: N:} err=json: invalid use of ,string struct tag, trying to unmarshal unquoted value into int64
// {"I":"x"}                                                    => {I:0 U:0 F:0 B:false S: N:} err=json: invalid use of ,string struct tag, trying to unmarshal "x" into int64
// {"U":"300"}                                                  => {I:0 U:0 F:0 B:false S: N:} err=json: cannot unmarshal number 300 into Go struct field Quoted.U of type uint8
// {"F":"1e999"}                                                => {I:0 U:0 F:0 B:false S: N:} err=json: cannot unmarshal number 1e999 into Go struct field Quoted.F of type float64
// {"B":"yes"}                                                  => {I:0 U:0 F:0 B:false S: N:} err=json: invalid use of ,string struct tag, trying to unmarshal "yes" into bool
// {"S":"x"}                                                    => {I:0 U:0 F:0 B:false S: N:} err=json: invalid use of ,string struct tag, trying to unmarshal "x" into string
// {"I":null}                                                   => {I:0 U:0 F:0 B:false S: N:} err=<nil>
// {"I":"null"}                                                 => {I:0 U:0 F:0 B:false S: N:} err=<nil>
// {"I":" 1"}                                                   => {I:0 U:0 F:0 B:false S: N:} err=json: invalid use of ,string struct tag, trying to unmarshal " 1" into int64
// {"I":"01"}                                                   => {I:1 U:0 F:0 B:false S: N:} err=<nil>
// {"N":"abc"}                                                  => {I:0 U:0 F:0 B:false S: N:} err=json: invalid use of ,string struct tag, trying to unmarshal "abc" into json.Number
// {"F":"NaN"}                                                  => {I:0 U:0 F:0 B:false S: N:} err=json: invalid use of ,string struct tag, trying to unmarshal "NaN" into float64
// {"I":"-5","U":"200","F":"2.5","B":"true","S":"\"q\\\"x\"","N":"12"} <nil>
//  json: unsupported value: +Inf
