// phase: 07

// A struct of main embeds an instance of another package's generic type: promoted methods satisfy another package's interface.
package main

import (
	"fmt"

	"example.com/gtembed/lib"
)

type User struct {
	lib.Entity[int]
	Name string
}

type Doc struct {
	*lib.Entity[string]
}

func main() {
	u := User{lib.Entity[int]{ID: 1}, "ann"}
	u.Tag("admin")
	d := Doc{&lib.Entity[string]{ID: "d"}}
	d.Tag("draft")
	for _, x := range []lib.Describer{u, d, &u} {
		fmt.Println(x.Describe())
	}
	r := lib.NewRepo[int, User]()
	r.Save(1, u)
	got, ok := r.Find(1)
	_, none := r.Find(2)
	fmt.Println(got.Name, ok, none)
	fmt.Printf("%T %v\n", u, u)
}

// Output:
// entity 1 [admin]
// entity d [draft]
// entity 1 [admin]
// ann true false
// main.User {{1 [admin]} ann}
