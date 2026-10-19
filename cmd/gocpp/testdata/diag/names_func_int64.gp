package main

import "<cstdlib>"

func main() {
	f := func(a int64, b []uint64) int64 { return a }
	_ = cstdlib.abs(f)
}
