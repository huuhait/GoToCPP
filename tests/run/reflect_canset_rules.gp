// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type In struct{ V int }
type T struct {
	Pub  int
	priv int
	In
	in  In
	P   *In
	p   *In
	Sl  []In
	M   map[string]In
	Arr [2]In
}

func main() {
	x := T{P: &In{}, p: &In{}, Sl: []In{{}}, M: map[string]In{"a": {}}}
	pv := reflect.ValueOf(&x)
	v := pv.Elem()
	check := func(name string, r reflect.Value) {
		fmt.Printf("%-18s canset=%-5v canaddr=%-5v caninterface=%v\n", name, r.CanSet(), r.CanAddr(), r.CanInterface())
	}
	check("&x", pv)
	check("x", v)
	check("Pub", v.Field(0))
	check("priv", v.Field(1))
	check("In (embedded)", v.Field(2))
	check("In.V", v.Field(2).Field(0))
	check("in", v.Field(3))
	check("in.V", v.Field(3).Field(0))
	check("P", v.Field(4))
	check("P.V", v.Field(4).Elem().Field(0))
	check("p", v.Field(5))
	check("p.V", v.Field(5).Elem().Field(0))
	check("Sl[0]", v.Field(6).Index(0))
	check("Sl[0].V", v.Field(6).Index(0).Field(0))
	check("M[a]", v.Field(7).MapIndex(reflect.ValueOf("a")))
	check("Arr", v.Field(8))
	check("Arr[1].V", v.Field(8).Index(1).Field(0))
	check("copy", reflect.ValueOf(x))
	check("copy.Pub", reflect.ValueOf(x).Field(0))
	check("copy.Sl[0]", reflect.ValueOf(x).Field(6).Index(0))
	check("copy.P.V", reflect.ValueOf(x).Field(4).Elem().Field(0))
}

// Output:
// &x                 canset=false canaddr=false caninterface=true
// x                  canset=true  canaddr=true  caninterface=true
// Pub                canset=true  canaddr=true  caninterface=true
// priv               canset=false canaddr=true  caninterface=false
// In (embedded)      canset=true  canaddr=true  caninterface=true
// In.V               canset=true  canaddr=true  caninterface=true
// in                 canset=false canaddr=true  caninterface=false
// in.V               canset=false canaddr=true  caninterface=false
// P                  canset=true  canaddr=true  caninterface=true
// P.V                canset=true  canaddr=true  caninterface=true
// p                  canset=false canaddr=true  caninterface=false
// p.V                canset=false canaddr=true  caninterface=false
// Sl[0]              canset=true  canaddr=true  caninterface=true
// Sl[0].V            canset=true  canaddr=true  caninterface=true
// M[a]               canset=false canaddr=false caninterface=true
// Arr                canset=true  canaddr=true  caninterface=true
// Arr[1].V           canset=true  canaddr=true  caninterface=true
// copy               canset=false canaddr=false caninterface=true
// copy.Pub           canset=false canaddr=false caninterface=true
// copy.Sl[0]         canset=true  canaddr=true  caninterface=true
// copy.P.V           canset=true  canaddr=true  caninterface=true
