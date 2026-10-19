package main

import "<cstdlib>"

func main() {
	f := func(s string) (int, error) { return 0, nil }
	_ = cstdlib.abs(f)
}
