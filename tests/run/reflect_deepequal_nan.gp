// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct{ F float64 }

func main() {
	zero := 0.0
	nan := zero / zero
	negz := -zero
	fmt.Println(reflect.DeepEqual(nan, nan), reflect.DeepEqual(S{nan}, S{nan}), reflect.DeepEqual([]float64{nan}, []float64{nan}), reflect.DeepEqual(map[string]float64{"a": nan}, map[string]float64{"a": nan}))
	fmt.Println(reflect.DeepEqual(0.0, negz), reflect.DeepEqual([]float64{0}, []float64{negz}))
	p := &S{nan}
	fmt.Println(reflect.DeepEqual(p, p), reflect.DeepEqual(&S{nan}, &S{nan}))
	s := []float64{nan}
	fmt.Println(reflect.DeepEqual(s, s), reflect.DeepEqual(s, []float64{nan}))
	m := map[string]float64{"a": nan}
	fmt.Println(reflect.DeepEqual(m, m))
	fmt.Println(reflect.DeepEqual(complex(nan, 0), complex(nan, 0)), reflect.DeepEqual(float32(nan), float32(nan)))
	var i1, i2 any = nan, nan
	fmt.Println(reflect.DeepEqual(i1, i2), reflect.DeepEqual([]any{nan}, []any{nan}))
}

// Output:
// false false false false
// true true
// true false
// true false
// true
// false false
// false false
