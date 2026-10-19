// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Base struct{ ID int }
type User struct {
	Base
	Name    string `json:"name"`
	Age     int    `json:"age,omitempty" db:"age"`
	email   string
	Friends []*User
	Meta    map[string]any
	*Extra
}
type Extra struct{ Note string }

func main() {
	t := reflect.TypeOf(User{})
	fmt.Println(t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fmt.Printf("%d %s type=%v tag=%q anon=%v pkg=%q index=%v exported=%v\n", i, f.Name, f.Type, f.Tag, f.Anonymous, f.PkgPath, f.Index, f.IsExported())
	}
	var base Base
	var u User
	fmt.Println(t.Field(0).Offset == 0, uintptr(t.Field(1).Offset) > t.Field(0).Offset, t.Field(2).Offset > t.Field(1).Offset)
	_ = base
	_ = u
}

// Output:
// 7
// 0 Base type=main.Base tag="" anon=true pkg="" index=[0] exported=true
// 1 Name type=string tag="json:\"name\"" anon=false pkg="" index=[1] exported=true
// 2 Age type=int tag="json:\"age,omitempty\" db:\"age\"" anon=false pkg="" index=[2] exported=true
// 3 email type=string tag="" anon=false pkg="main" index=[3] exported=false
// 4 Friends type=[]*main.User tag="" anon=false pkg="" index=[4] exported=true
// 5 Meta type=map[string]interface {} tag="" anon=false pkg="" index=[5] exported=true
// 6 Extra type=*main.Extra tag="" anon=true pkg="" index=[6] exported=true
// true true true
