package main

import "<cstring>"

func main() {
	var v any = "x"
	var e error
	_ = cstring.strlen(v)
	_ = cstring.strlen(e)
}
