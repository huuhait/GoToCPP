// phase: 09
// std-internal: tests internal/godebug, which only the standard library may import

// internal/godebug gives the value of the last "name=value" of $GODEBUG for the name, "" when there
// is none, and a change of the variable through os.Setenv or os.Unsetenv shows at the next call (the
// value is kept until then). A change that the C library makes is not seen, as in Go, until the
// program sets the variable again.
package main

import (
	"fmt"
	"internal/godebug"
	"os"

	"<stdlib.h>"
)

func set(v string) {
	os.Setenv("GODEBUG", v)
}

func main() {
	os.Unsetenv("GODEBUG")
	s := godebug.New("zzdebug")
	fmt.Printf("unset: %q\n", s.Value())

	for _, v := range []string{
		"zzdebug=1",
		"a=1,zzdebug=2,b=3",
		"zzdebug=1,zzdebug=2",
		"zzdebug=2,zzdebug=1",
		"zzdebug",
		"zzdebug=",
		"zzdebugx=5",
		"xzzdebug=5",
		"zzdebug=a=b",
		",,zzdebug=7,,",
		"b=3",
		"",
	} {
		set(v)
		fmt.Printf("GODEBUG=%q: %q\n", v, s.Value())
	}

	set("zzdebug=on,other=off")
	other := godebug.New("other")
	fmt.Println(s.Name(), s.Value(), s.Undocumented(), s.String())
	fmt.Println(other.Name(), other.Value(), other.Undocumented(), other.String())
	hidden := godebug.New("#hidden")
	set("hidden=yes")
	fmt.Println(hidden.Name(), hidden.Value(), hidden.Undocumented(), hidden.String())
	s.IncNonDefault() // counts nothing here, and must not fail
	os.Unsetenv("GODEBUG")
	fmt.Printf("unset again: %q\n", s.Value())

	set("zzdebug=go")
	fmt.Println(s.Value())
	stdlib.setenv(C.cstr("GODEBUG"), C.cstr("zzdebug=c"), 1)
	fmt.Println(s.Value(), C.GoString(stdlib.getenv(C.cstr("GODEBUG"))))
	os.Setenv("OTHER_VARIABLE", "1")
	fmt.Println(s.Value())
	set("zzdebug=go2")
	fmt.Println(s.Value())
	os.Clearenv()
	fmt.Printf("cleared: %q\n", s.Value())
}

// Output:
// unset: ""
// GODEBUG="zzdebug=1": "1"
// GODEBUG="a=1,zzdebug=2,b=3": "2"
// GODEBUG="zzdebug=1,zzdebug=2": "2"
// GODEBUG="zzdebug=2,zzdebug=1": "1"
// GODEBUG="zzdebug": ""
// GODEBUG="zzdebug=": ""
// GODEBUG="zzdebugx=5": ""
// GODEBUG="xzzdebug=5": ""
// GODEBUG="zzdebug=a=b": "a=b"
// GODEBUG=",,zzdebug=7,,": "7"
// GODEBUG="b=3": ""
// GODEBUG="": ""
// zzdebug on false zzdebug=on
// other off false other=off
// hidden yes true hidden=yes
// unset again: ""
// go
// go zzdebug=c
// go
// go2
// cleared: ""
