package main

import "<cstring>"

func main() {
	m := map[string]int{"a": 1}
	_ = cstring.strlen(m)
}
