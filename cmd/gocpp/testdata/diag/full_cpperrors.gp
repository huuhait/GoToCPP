//test: cpperrors=full
package main

import "<cstdlib>"

func main() {
	s := []string{"a"}
	_ = cstdlib.abs(s)
}
