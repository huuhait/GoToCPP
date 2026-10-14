// Names that hide C++ namespaces and header packages, as Go scoping says:
// locals, parameters, results, struct fields and package-level names.
package main

import (
	"fmt"

	"./shapes.hpp"
	"<cstdio>"
	"<vector>"
	fs "::std::filesystem"
	"<filesystem>"
)

type config struct {
	std int // a field named std hides nothing: c.std
	geo string
}

func useGeo(geo int) int { // a parameter named like the namespace geo
	return geo + 1
}

func results() (std int, fs string) {
	std = 2
	fs = "x"
	return
}

func main() {
	v := std.vector[int]{1}
	{
		std := 3 // hides the namespace std here
		fmt.Println(std + 1)
	}
	fmt.Println(int(v.size()))
	cstdio := "a local named like the header package"
	fmt.Println(cstdio)
	c := config{std: 1, geo: "g"}
	fmt.Println(c.std, c.geo, useGeo(2))
	n, name := results()
	fmt.Println(n, name)
	for fs := range 2 {
		fmt.Println(fs)
	}
	p := fs.path("/")
	f := func(geo geo.Vec) float64 { return geo.x }
	fmt.Println(p.string(), f(geo.Vec{}))
	if std := len("ab"); std > 1 {
		fmt.Println(std)
	}
	switch geo := any(1).(type) {
	case int:
		fmt.Println(geo + 1)
	}
}
