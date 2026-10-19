// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Big struct {
	F00, F01, F02, F03, F04, F05, F06, F07, F08, F09 int
	G00, G01, G02, G03, G04, G05, G06, G07, G08, G09 string
	H00, H01, H02, H03, H04, H05, H06, H07, H08, H09 float64
	Arr                                              [64]int
	Name                                             string `custom:"big"`
}

func main() {
	var b Big
	v := reflect.ValueOf(&b).Elem()
	t := v.Type()
	fmt.Println(t.NumField())
	for i := 0; i < 10; i++ {
		v.Field(i).SetInt(int64(i * i))
		v.Field(10 + i).SetString(fmt.Sprint("s", i))
		v.Field(20 + i).SetFloat(float64(i) / 4)
	}
	for i := 0; i < 64; i++ {
		v.Field(30).Index(i).SetInt(int64(i))
	}
	v.Field(31).SetString("named")
	sum := 0
	for i := 0; i < 64; i++ {
		sum += int(v.Field(30).Index(i).Int())
	}
	fmt.Println(b.F09, b.G07, b.H05, b.Arr[63], b.Name, sum, t.Field(31).Tag.Get("custom"), t.Field(0).Name, t.Field(29).Name)
	c := v.Interface().(Big)
	c.F00 = -1
	fmt.Println(b.F00, c.F00, c.G09, reflect.DeepEqual(b, c))
}

// Output:
// 32
// 81 s7 1.25 63 named 2016 big F00 H09
// 0 -1 s9 false
