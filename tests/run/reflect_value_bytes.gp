// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Raw []byte
type Digest [4]byte

func main() {
	b := []byte("hello")
	v := reflect.ValueOf(b)
	got := v.Bytes()
	got[0] = 'J'
	fmt.Println(string(b), string(got))
	r := reflect.ValueOf(Raw("raw"))
	fmt.Println(string(r.Bytes()), r.Type())
	var d Digest = Digest{1, 2, 3, 4}
	dv := reflect.ValueOf(&d).Elem()
	db := dv.Bytes()
	db[0] = 99
	fmt.Println(d, db)
	reflect.ValueOf(&b).Elem().SetBytes([]byte("replaced"))
	fmt.Println(string(b))
	for _, f := range []func(){
		func() { reflect.ValueOf(d).Bytes() },
		func() { reflect.ValueOf([]int{1}).Bytes() },
		func() { reflect.ValueOf("str").Bytes() },
		func() { reflect.ValueOf(&[]int{1}).Elem().SetBytes(nil) },
		func() { reflect.ValueOf([2]int{}).Bytes() },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// Jello Jello
// raw main.Raw
// [99 2 3 4] [99 2 3 4]
// replaced
// recovered: reflect.Value.Bytes of unaddressable byte array
// recovered: reflect.Value.Bytes of non-byte slice
// recovered: reflect: call of reflect.Value.Bytes on string Value
// recovered: reflect.Value.SetBytes of non-byte slice
// recovered: reflect.Value.Bytes of non-byte array
