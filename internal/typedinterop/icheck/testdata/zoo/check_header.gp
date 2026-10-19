// A header that does not exist is an error of the opaque check.
package main

import "<nosuchheader.h>"

func main() {
	nosuchheader.f()
}
