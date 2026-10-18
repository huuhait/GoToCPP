// phase: 04

// <unordered_map>: iteration order is unspecified, so the program sorts the keys it reads;
// try_emplace, contains, at(), operator[], load factor, and an int key with a string value.
package main

import (
	"fmt"

	"<algorithm>"
	"<string>"
	"<unordered_map>"
	"<vector>"
)

func main() {
	m := std.unordered_map[int, std.string]{}
	m[1] = std.string("one")
	m[2] = std.string("two")
	m.try_emplace(3, "three")
	m.try_emplace(1, "uno") // the key exists: no change
	fmt.Println(int(m.size()), m.contains(2), m.contains(9), string(m.at(1)))

	var keys std.vector[int]
	for k := range m {
		keys.push_back(k)
	}
	std.sort(keys.begin(), keys.end())
	fmt.Println(int(keys[0]), int(keys[1]), int(keys[2]))
	fmt.Println(float64(m.load_factor()) > 0, int(m.erase(2)), int(m.size()))
}

// Output:
// 3 true false one
// 1 2 3
// true 1 2
