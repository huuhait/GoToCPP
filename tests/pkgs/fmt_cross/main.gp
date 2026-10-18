// phase: 06

// fmt of values whose types are declared in other packages: the descriptors are complete (fields with
// offsets, elements, methods) wherever they are defined; %T names the package; unexported fields and
// embedded types of other packages; interface values holding them.

package main

import (
	"fmt"

	"example.com/fmtx/model"
	"example.com/fmtx/util"
)

func main() {
	a := model.NewAccount("ann")
	a.Next = nil
	fmt.Printf("%v\n%+v\n", *a, *a)
	fmt.Println(a.Balance, a.Tags, a.ByName, a.Err)
	fmt.Printf("%T %T %T %T\n", a, a.Balance, a.Tags, a.ByName)
	fmt.Printf("%#v\n", a.Tags)
	w := util.Wrap(a)
	fmt.Printf("%v\n%+v\n", w, w)
	fmt.Printf("%T %v\n", w.Pts[0], w.Pts)
	fmt.Printf("%#v\n", w.Pts)
	var e error = a.Err
	fmt.Printf("%T %v %q\n", e, e, e)
	fmt.Println(fmt.Errorf("wrap: %w", e))
	fmt.Printf("%s %d %x\n", model.Money(255), model.Money(255), model.Money(255))
}

// Output:
// {ann $5 6 [a b] map[k:$7] <nil> model error 3}
// {Owner:ann Balance:$5 hidden:6 Tags:[a b] ByName:map[k:$7] Next:<nil> Err:model error 3}
// $5 [a b] map[k:$7] model error 3
// *model.Account model.Money model.Tags model.Index
// model.Tags{"a", "b"}
// {{ann $5 6 [a b] map[k:$7] <nil> model error 3} $9 [{1 2} $3]}
// {Account:{Owner:ann Balance:$5 hidden:6 Tags:[a b] ByName:map[k:$7] Next:<nil> Err:model error 3} Extra:$9 Pts:[{X:1 Y:2} $3]}
// model.pt [{1 2} $3]
// []interface {}{model.pt{X:1, Y:2}, 3}
// *model.Err model error 3 "model error 3"
// wrap: model error 3
// $255 255 24323535
