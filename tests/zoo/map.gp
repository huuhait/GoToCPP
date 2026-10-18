// phase: 04

// <map>: operator[] inserts, find/end, count, erase, ordered iteration (key and value are C++
// values: convert them), emplace, and a map with a struct value.
package main

import (
	"fmt"

	"<map>"
	"<string>"
)

type Rec struct{ N int }

func main() {
	ages := std.map[std.string, int]{}
	ages["bob"] = 31
	ages["alice"] = 29
	ages["carol"]++
	fmt.Println(int(ages.size()), int(ages["bob"]), int(ages.count("alice")), int(ages.count("zed")))

	for k, v := range ages {
		fmt.Println(string(k), int(v))
	}

	it := ages.find("bob")
	if it != ages.end() {
		fmt.Println("found", string((*it).first), int((*it).second))
	}
	ages.erase("bob")
	fmt.Println(int(ages.size()), ages.find("bob") == ages.end())

	recs := std.map[int, Rec]{}
	recs[3] = Rec{30}
	recs.emplace(1, Rec{10})
	for k, v := range recs {
		fmt.Println(int(k), v.N)
	}
}

// Output:
// 3 31 1 0
// alice 29
// bob 31
// carol 1
// found bob 31
// 2 true
// 1 10
// 3 30
